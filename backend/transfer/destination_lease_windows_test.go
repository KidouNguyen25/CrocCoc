//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDestinationLeaseRetainsDirectoryIdentity(t *testing.T) {
	directory := t.TempDir()
	lease, err := OpenReceiveDestinationLease(directory)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test volume is not a supported local NTFS destination")
	}
	if err != nil {
		t.Fatal(err)
	}
	id := lease.Identity()
	if id.FileIndex == 0 {
		t.Fatal("empty directory file index")
	}
	again, err := OpenReceiveDestinationLease(directory)
	if err != nil {
		t.Fatal(err)
	}
	if again.Identity() != id {
		t.Fatalf("different identities: %#v vs %#v", id, again.Identity())
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDestinationLeaseRejectsUnsupportedPaths(t *testing.T) {
	for _, path := range []string{"", "relative", `\\remote\share`, `C:\missing-croccoc-directory`} {
		lease, err := OpenReceiveDestinationLease(path)
		if !errors.Is(err, ErrUnsupportedDestination) || lease != nil {
			t.Fatalf("unsupported path not rejected: %q: %v", path, err)
		}
	}
}

func TestDestinationLeaseRejectsReparsePoint(t *testing.T) {
	directory := t.TempDir()
	base, err := OpenReceiveDestinationLease(directory)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test volume is not local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = base.Close()
	target := filepath.Join(directory, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "junction-like-symlink")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink without privileges: %v", err)
	}
	lease, err := OpenReceiveDestinationLease(link)
	if !errors.Is(err, ErrUnsupportedDestination) || lease != nil {
		t.Fatalf("followed a reparse point: %v", err)
	}
}
func TestDestinationLeaseDetectsRenamedDirectory(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "held")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	lease, err := OpenReceiveDestinationLease(child)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test volume is not supported local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.VerifyCurrentPath(); err != nil {
		t.Fatalf("unchanged path failed identity check: %v", err)
	}
	moved := filepath.Join(parent, "moved")
	if err := os.Rename(child, moved); err == nil {
		// A rename may succeed despite the read-only handles. Re-using the
		// original name for a different object must fail the identity check.
		if err := os.Mkdir(child, 0700); err != nil {
			t.Fatal(err)
		}
		if err := lease.VerifyCurrentPath(); !errors.Is(err, ErrUnsupportedDestination) {
			t.Fatalf("replaced destination was accepted: %v", err)
		}
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.VerifyCurrentPath(); !errors.Is(err, ErrUnsupportedDestination) {
		t.Fatalf("closed lease was accepted: %v", err)
	}
}
