//go:build windows

package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// PublishResult reports the roots that were actually published. A multi-root
// commit is not atomic: partial output is never automatically destroyed.
type PublishResult struct {
	Published []string
	Partial   bool
}

// publishAuditedRoots is the last filesystem step after both explicit user
// consent and VERIFIED CROc protocol integrity. It only checks local staging
// invariants; the caller must separately acquire the Coordinator FINALIZING
// gate after the engine has provided its authenticated integrity result.
//
// DO NOT expose this primitive through the service or Wails bridge directly.
func publishAuditedRoots(lease *DestinationLease, stage *ReceiveStage, entries []ManifestEntry) (result PublishResult, err error) {
	if stage == nil || lease == nil || lease.Identity() != stage.ownership.Destination ||
		lease.VerifyCurrentPath() != nil {
		return result, ErrPublishFailed
	}
	if err := stage.AuditStagedPayload(); err != nil {
		return result, err
	}
	stage.mu.Lock()
	defer stage.mu.Unlock()
	if stage.stage == 0 || stage.payload == 0 || !stage.prepared ||
		stage.ownership.Status != "prepared" ||
		!matchesStageEntries(stage, entries) {
		return result, ErrInvalidStaging
	}
	roots, err := explicitRootItems(entries)
	if err != nil {
		return result, err
	}
	dest, err := openWritableDestination(lease)
	if err != nil {
		return result, ErrPublishFailed
	}
	defer windows.CloseHandle(dest)
	if err := stage.setOwnershipStatus("publishing"); err != nil {
		return result, ErrUnsafeStorage
	}
	for _, entry := range roots {
		if err := lease.VerifyCurrentPath(); err != nil {
			result.Partial = len(result.Published) > 0
			_ = stage.setOwnershipStatus("partial")
			return result, ErrPublishFailed
		}
		from, err := openStagedRoot(stage.payload, entry.Path,
			entry.Directory, stage.ownership.Stage.VolumeSerial)
		if err != nil {
			result.Partial = len(result.Published) > 0
			_ = stage.setOwnershipStatus("partial")
			return result, ErrPublishFailed
		}
		err = renameNativeNoReplace(from, dest, entry.Path)
		closeErr := windows.CloseHandle(from)
		if err != nil || closeErr != nil {
			result.Partial = len(result.Published) > 0 || err == nil
			_ = stage.setOwnershipStatus("partial")
			return result, ErrPublishFailed
		}
		result.Published = append(result.Published, entry.Path)
	}
	if err := stage.setOwnershipStatus("published"); err != nil {
		result.Partial = len(result.Published) > 0
		return result, ErrUnsafeStorage
	}
	return result, nil
}

func explicitRootItems(entries []ManifestEntry) ([]ManifestEntry, error) {
	if _, err := ValidateManifest(entries); err != nil {
		return nil, err
	}
	roots := make([]ManifestEntry, 0)
	found := make(map[string]bool)
	for _, entry := range entries {
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		if !strings.Contains(path, "/") {
			entry.Path = path
			roots = append(roots, entry)
			found[strings.ToUpper(path)] = true
		}
	}
	for _, entry := range entries {
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		if pos := strings.IndexByte(path, '/'); pos >= 0 && !found[strings.ToUpper(path[:pos])] {
			return nil, ErrUnsafeOffer
		}
	}
	if len(roots) == 0 {
		return nil, ErrUnsafeOffer
	}
	return roots, nil
}

func matchesStageEntries(stage *ReceiveStage, entries []ManifestEntry) bool {
	if len(entries) != len(stage.allowed) {
		return false
	}
	for _, item := range entries {
		item.Path = strings.ReplaceAll(item.Path, "\\", "/")
		expected, ok := stage.allowed[strings.ToUpper(item.Path)]
		if !ok || expected != item {
			return false
		}
	}
	return true
}

// setOwnershipStatus flushes a replacement journal before each publish state.
// If a crash happens during replacement, recovery must still verify stage
// identity and marker, and never blindly remove user destination items.
func (s *ReceiveStage) setOwnershipStatus(status string) error {
	next := s.ownership
	next.Status = status
	data, err := json.Marshal(next)
	if err != nil {
		return ErrUnsafeStorage
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ErrUnsafeStorage
	}
	tmp := s.journalPath + "." + hex.EncodeToString(nonce[:]) + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrUnsafeStorage
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return ErrUnsafeStorage
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return ErrUnsafeStorage
	}
	if err := f.Close(); err != nil {
		return ErrUnsafeStorage
	}
	if err := os.Rename(tmp, filepath.Clean(s.journalPath)); err != nil {
		return ErrUnsafeStorage
	}
	s.ownership = next
	return nil
}
