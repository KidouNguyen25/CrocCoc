//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativePublishNeverReplacesExistingRoot(t *testing.T) {
	destination := t.TempDir()
	lease, err := OpenReceiveDestinationLease(destination)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed NTFS volume")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	parent, err := openWritableDestination(lease)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(parent)
	stageH, _, err := createChildDirectory(parent, "stage-for-publish")
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(stageH)
	fromPath := filepath.Join(destination, "stage-for-publish", "report.txt")
	if err := os.WriteFile(fromPath, []byte("incoming"), 0600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destination, "report.txt")
	if err := os.WriteFile(existing, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	from, err := openStagedRoot(stageH, "report.txt", false, lease.Identity().VolumeSerial)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if from != 0 {
			_ = windows.CloseHandle(from)
		}
	}()
	if err := renameNativeNoReplace(from, parent, "report.txt"); !errors.Is(err, ErrPublishFailed) {
		t.Fatalf("conflicting destination was overwritten: %v", err)
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "original" {
		t.Fatalf("existing file changed: %q %v", data, err)
	}
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("source disappeared on failed publish: %v", err)
	}
	if err := renameNativeNoReplace(from, parent, "delivered.txt"); err != nil {
		t.Fatalf("safe same-volume rename failed: %v", err)
	}
	if err := windows.CloseHandle(from); err != nil {
		t.Fatal(err)
	}
	from = 0
	delivered, err := os.ReadFile(filepath.Join(destination, "delivered.txt"))
	if err != nil || string(delivered) != "incoming" {
		t.Fatalf("published file mismatch: %q %v", delivered, err)
	}
	if _, err := os.Stat(fromPath); !os.IsNotExist(err) {
		t.Fatal("source still exists after atomic move")
	}
}

func TestNativePublishRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", "..", "CON.txt", "x:ads", "folder/name"} {
		if err := renameNativeNoReplace(0, 0, name); !errors.Is(err, ErrPublishFailed) {
			t.Fatalf("unsafe name accepted: %q %v", name, err)
		}
	}
}

func TestNativePublishDirectoryIsNoReplace(t *testing.T) {
	destination := t.TempDir()
	lease, err := OpenReceiveDestinationLease(destination)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed NTFS volume")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	parent, err := openWritableDestination(lease)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(parent)
	stageH, _, err := createChildDirectory(parent, "folder-stage")
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(stageH)
	incoming := filepath.Join(destination, "folder-stage", "photos")
	if err := os.Mkdir(incoming, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "image.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destination, "photos")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	from, err := openStagedRoot(stageH, "photos", true, lease.Identity().VolumeSerial)
	if err != nil {
		t.Fatal(err)
	}
	if err := renameNativeNoReplace(from, parent, "photos"); !errors.Is(err, ErrPublishFailed) {
		_ = windows.CloseHandle(from)
		t.Fatalf("existing folder was replaced: %v", err)
	}
	if err := renameNativeNoReplace(from, parent, "delivered-photos"); err != nil {
		_ = windows.CloseHandle(from)
		t.Fatalf("new folder publish failed: %v", err)
	}
	if err := windows.CloseHandle(from); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(existing, "keep.txt"))
	if err != nil || string(before) != "original" {
		t.Fatalf("existing folder changed: %q %v", before, err)
	}
	after, err := os.ReadFile(filepath.Join(destination, "delivered-photos", "image.txt"))
	if err != nil || string(after) != "new" {
		t.Fatalf("published directory contents changed: %q %v", after, err)
	}
}
