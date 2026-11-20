//go:build windows

package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativeChildDirectoryAnchoredCreateAndExclusiveCollision(t *testing.T) {
	base := t.TempDir()
	lease, err := OpenReceiveDestinationLease(base)
	if err != nil {
		t.Skipf("unsupported test volume: %v", err)
	}
	defer lease.Close()

	// This read-only lease must not be used for mutation. Acquire a separate
	// capability with FILE_ADD_SUBDIRECTORY and confirm the same identity.
	parent, err := openWritableDestination(lease)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(parent)

	name := "safe-stage-dir"
	h, id, err := createChildDirectory(parent, name)
	if err != nil {
		t.Fatalf("anchored create: %v", err)
	}
	defer windows.CloseHandle(h)
	if id.VolumeSerial != lease.Identity().VolumeSerial || id.FileIndex == 0 {
		t.Fatalf("unexpected identity: %+v", id)
	}
	_, _, err = createChildDirectory(parent, name)
	if err == nil {
		t.Fatal("FILE_CREATE replaced or reopened existing directory")
	}
	other, otherID, err := openChildDirectory(parent, name, lease.Identity().VolumeSerial)
	if err != nil || otherID != id {
		t.Fatalf("cannot reopen same native child: %+v %v", otherID, err)
	}
	_ = windows.CloseHandle(other)
	if _, err := os.Stat(filepath.Join(base, name)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"..", "x/y", "CON", "a:stream"} {
		if h, _, err := createChildDirectory(parent, invalid); err == nil {
			_ = windows.CloseHandle(h)
			t.Fatalf("unsafe child created: %q", invalid)
		}
	}
}
