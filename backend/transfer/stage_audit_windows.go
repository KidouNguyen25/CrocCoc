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

// AuditStagedPayload reads the stage created by our native writer and verifies
// that its files, directories and sizes match the frozen manifest exactly.
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
		// Inspect the Win32 reparse flag separately: junctions are not always
		// exposed by generic filesystem walkers as symbolic links.
		ptr, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return ErrInvalidStaging
		}
		attrs, err := windows.GetFileAttributes(ptr)
		if err != nil || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return ErrInvalidStaging
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 ||
			info.IsDir() != expected.Directory {
			return ErrInvalidStaging
		}
		if !expected.Directory &&
			(!info.Mode().IsRegular() || info.Size() != expected.SizeBytes ||
				!s.completed[key]) {
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
