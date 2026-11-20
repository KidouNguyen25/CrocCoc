//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReceivePreflightIsReadOnlyAndPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(file, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	lease, err := PreflightReceiveDestination(dir, []ManifestEntry{{Path: "keep.txt", SizeBytes: 5}})
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test directory is not supported local NTFS")
	}
	if !errors.Is(err, ErrDestinationConflict) || lease != nil {
		t.Fatalf("existing root was not blocked: %v", err)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "untouched" {
		t.Fatalf("existing file changed on conflict: %v", err)
	}
	lease, err = PreflightReceiveDestination(dir, []ManifestEntry{{Path: "new-file.txt", SizeBytes: 1}})
	if errors.Is(err, ErrResourceLimit) {
		t.Skip("volume is below the required 256 MiB free-space reserve")
	}
	if err != nil || lease == nil {
		t.Fatalf("safe metadata preflight failed: %v", err)
	}
	if err := lease.VerifyCurrentPath(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new-file.txt")); !os.IsNotExist(err) {
		t.Fatalf("preflight unexpectedly wrote a payload: %v", err)
	}
}

func TestReceivePreflightFailsBeforeAllocatingHandles(t *testing.T) {
	lease, err := PreflightReceiveDestination("not-absolute", []ManifestEntry{{Path: "CON.txt"}})
	if !errors.Is(err, ErrUnsafeOffer) || lease != nil {
		t.Fatalf("unsafe name accepted: %v", err)
	}
}
