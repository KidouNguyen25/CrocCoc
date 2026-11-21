//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var ErrInvalidStaging = errors.New("TRANSFER_FAILED: vùng nhận không khớp danh sách đã duyệt")

// AdoptEnginePayload records identities for files written by the reviewed
// engine worker. It does not trust worker metadata: every object is reopened
// by handle, checked against the frozen manifest, and rejected on reparse or
// size mismatch before publication can use it.
func (s *ReceiveStage) AdoptEnginePayload() error {
	if s == nil {
		return ErrInvalidStaging
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage == 0 || s.payload == 0 || !s.prepared || s.ownership.Status != "prepared" {
		return ErrInvalidStaging
	}
	if err := s.verifyStagingIdentity(); err != nil {
		return ErrInvalidStaging
	}
	directory := filepath.Join(s.destinationPath, s.StageName, "payload")
	seen := make(map[string]bool, len(s.allowed))
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrInvalidStaging
		}
		if path == directory {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return ErrInvalidStaging
		}
		name := filepath.ToSlash(relative)
		key := strings.ToUpper(name)
		expected, ok := s.allowed[key]
		if !ok || strings.ReplaceAll(expected.Path, "\\", "/") != name || seen[key] {
			return ErrInvalidStaging
		}
		seen[key] = true
		parts := strings.Split(name, "/")
		parent, closeParents, err := s.openStageParent(parts[:len(parts)-1])
		if err != nil {
			return ErrInvalidStaging
		}
		h, err := openStagedRoot(parent, parts[len(parts)-1], expected.Directory, s.ownership.Stage.VolumeSerial)
		if err != nil {
			closeParents()
			return ErrInvalidStaging
		}
		if expected.Directory {
			actualID := stagedIdentityFromHandle(h)
			actual, found := s.stageDirectories[expected.Path]
			_ = windows.CloseHandle(h)
			closeParents()
			if !found || actual != actualID {
				return ErrInvalidStaging
			}
			return nil
		}
		var info windows.ByHandleFileInformation
		valid := windows.GetFileInformationByHandle(h, &info) == nil
		if valid {
			size := uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)
			valid = expected.SizeBytes >= 0 && size == uint64(expected.SizeBytes) && info.NumberOfLinks == 1
		}
		if valid {
			s.completed[key] = stagedObjectIdentity(info)
		}
		_ = windows.CloseHandle(h)
		closeParents()
		if !valid {
			return ErrInvalidStaging
		}
		return nil
	})
	if err != nil || len(seen) != len(s.allowed) {
		return ErrInvalidStaging
	}
	return nil
}

// AuditStagedPayload reads the stage created by our native writer and verifies
// that its files, directories, sizes and identities match the staged objects.
// A missing, partial, extra or reparse-point entry fails the audit.
//
// Security boundary: this is a pre-publication integrity check, NOT a Croc
// cryptographic integrity proof. Callers must separately verify the engine's
// authenticated hashes. A successful audit never authorizes publish by itself.
func (s *ReceiveStage) AuditStagedPayload() error {
	if s == nil {
		return ErrInvalidStaging
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage == 0 || s.payload == 0 || !s.prepared ||
		s.ownership.Status != "prepared" {
		return ErrInvalidStaging
	}
	if err := s.verifyStagingIdentity(); err != nil {
		return ErrInvalidStaging
	}
	directory := filepath.Join(s.destinationPath, s.StageName, "payload")
	seen := make(map[string]bool, len(s.allowed))
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrInvalidStaging
		}
		if path == directory {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return ErrInvalidStaging
		}
		name := filepath.ToSlash(relative)
		key := strings.ToUpper(name)
		expected, ok := s.allowed[key]
		if !ok || strings.ReplaceAll(expected.Path, "\\", "/") != name || seen[key] {
			return ErrInvalidStaging
		}
		seen[key] = true
		parts := strings.Split(name, "/")
		parent, closeParents, err := s.openStageParent(parts[:len(parts)-1])
		if err != nil {
			return ErrInvalidStaging
		}
		h, err := openStagedRoot(parent, parts[len(parts)-1], expected.Directory, s.ownership.Stage.VolumeSerial)
		if err != nil {
			closeParents()
			return ErrInvalidStaging
		}
		matched := s.matchesStagedObject(h, expected)
		_ = windows.CloseHandle(h)
		closeParents()
		if !matched {
			return ErrInvalidStaging
		}
		return nil
	})
	if err != nil || len(seen) != len(s.allowed) {
		return ErrInvalidStaging
	}
	if err := s.verifyStagingIdentity(); err != nil {
		return ErrInvalidStaging
	}
	return nil
}

func (s *ReceiveStage) verifyStagingIdentity() error {
	lease, err := OpenReceiveDestinationLease(s.destinationPath)
	if err != nil {
		return ErrInvalidStaging
	}
	defer lease.Close()
	if lease.Identity() != s.ownership.Destination {
		return ErrInvalidStaging
	}
	root := lease.handles[len(lease.handles)-1]
	h, id, err := openChildDirectory(root, s.StageName, lease.id.VolumeSerial)
	if err != nil || id != s.ownership.Stage {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		return ErrInvalidStaging
	}
	defer windows.CloseHandle(h)
	payload, id, err := openChildDirectory(h, "payload", lease.id.VolumeSerial)
	if err != nil || id != s.payloadID {
		if payload != 0 {
			_ = windows.CloseHandle(payload)
		}
		return ErrInvalidStaging
	}
	_ = windows.CloseHandle(payload)
	return nil
}

func stagedObjectIdentity(info windows.ByHandleFileInformation) DirectoryIdentity {
	return DirectoryIdentity{
		VolumeSerial: info.VolumeSerialNumber,
		FileIndex:    uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}
}

// matchesStagedObject uses the opened handle, not a second lookup by path.
func (s *ReceiveStage) matchesStagedObject(h windows.Handle, entry ManifestEntry) bool {
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil {
		return false
	}
	id := stagedObjectIdentity(info)
	if entry.Directory {
		expected, found := s.stageDirectories[entry.Path]
		return found && expected == id
	}
	expected, found := s.completed[strings.ToUpper(entry.Path)]
	size := uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)
	return found && expected == id && entry.SizeBytes >= 0 &&
		size == uint64(entry.SizeBytes) && info.NumberOfLinks == 1
}

func stagedIdentityFromHandle(h windows.Handle) DirectoryIdentity {
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil {
		return DirectoryIdentity{}
	}
	return stagedObjectIdentity(info)
}
