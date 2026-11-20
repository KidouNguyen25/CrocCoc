//go:build windows

package transfer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStageWriterAnchorsNestedFilesAndChecksExactSize(t *testing.T) {
	destination := t.TempDir()
	entries := []ManifestEntry{
		{Path: "folder", Directory: true},
		{Path: "folder/nested", Directory: true},
		{Path: "folder/nested/photo.txt", SizeBytes: 5},
	}
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	stage, err := prepareReceiveStageAt(lease, entries, filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if err := stage.prepareStageDirectories(entries); err != nil {
		t.Fatal(err)
	}
	if err := stage.writeStageFile(context.Background(), ManifestEntry{Path: "not-authorized.txt", SizeBytes: 1}, bytes.NewReader([]byte("x"))); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("unapproved file was accepted: %v", err)
	}
	if err := stage.writeStageFile(context.Background(), ManifestEntry{Path: entries[2].Path, SizeBytes: 100}, bytes.NewReader([]byte("x"))); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("changed declared size was accepted: %v", err)
	}
	if err := stage.writeStageFile(context.Background(), entries[2], bytes.NewReader([]byte("hello"))); err != nil {
		t.Fatalf("exact content write failed: %v", err)
	}
	actual, err := os.ReadFile(filepath.Join(destination, stage.StageName, "payload", "folder", "nested", "photo.txt"))
	if err != nil || string(actual) != "hello" {
		t.Fatalf("staging write mismatch: %q %v", actual, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "folder")); !os.IsNotExist(err) {
		t.Fatal("write leaked into the final destination before publish")
	}
	if err := stage.writeStageFile(context.Background(), entries[2], bytes.NewReader([]byte("hello"))); !errors.Is(err, ErrUnsafeStorage) {
		t.Fatalf("duplicate file should fail exclusive creation: %v", err)
	}
}

func TestStageWriterRejectsShortAndExcessPayload(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{"short", "abc"},
		{"excess", "abcdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := t.TempDir()
			entries := []ManifestEntry{{Path: "file.txt", SizeBytes: 5}}
			lease, err := PreflightReceiveDestination(dest, entries)
			if errors.Is(err, ErrUnsupportedDestination) {
				t.Skip("requires NTFS")
			}
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			stage, err := prepareReceiveStageAt(lease, entries, filepath.Join(t.TempDir(), "journal"))
			if err != nil {
				t.Fatal(err)
			}
			defer stage.Close()
			if err := stage.prepareStageDirectories(entries); err != nil {
				t.Fatal(err)
			}
			if err := stage.writeStageFile(context.Background(), entries[0], bytes.NewReader([]byte(tc.data))); !errors.Is(err, ErrPayloadMismatch) {
				t.Fatalf("bad payload accepted: %v", err)
			}
		})
	}
}

func TestReceivePreflightRejectsImplicitDirectory(t *testing.T) {
	entries := []ManifestEntry{{Path: "missing/child.txt", SizeBytes: 1}}
	dest := t.TempDir()
	probe, err := OpenReceiveDestinationLease(dest)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	probe.Close()

	lease, err := PreflightReceiveDestination(dest, entries)
	if lease != nil {
		lease.Close()
		t.Fatal("preflight granted a destination lease for an implicit parent")
	}
	if !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("implicit parent accepted: %v", err)
	}
	items, err := os.ReadDir(dest)
	if err != nil || len(items) != 0 {
		t.Fatalf("preflight modified destination: count=%d err=%v", len(items), err)
	}
}

func TestStageWriterRejectsReplacedParentAndCancelledWrite(t *testing.T) {
	destination := t.TempDir()
	entries := []ManifestEntry{
		{Path: "folder", Directory: true},
		{Path: "folder/photo.txt", SizeBytes: 1},
	}
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	stage, err := prepareReceiveStageAt(lease, entries, filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if err := stage.prepareStageDirectories(entries); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(destination, stage.StageName, "payload", "folder")
	file := filepath.Join(dir, "photo.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, invalid := range []context.Context{nil, ctx} {
		if err := stage.writeStageFile(invalid, entries[1], bytes.NewReader([]byte("x"))); !errors.Is(err, ErrPayloadMismatch) {
			t.Fatalf("invalid context accepted: %v", err)
		}
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("invalid context created a file: %v", err)
	}
	if err := os.Rename(dir, dir+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := stage.writeStageFile(context.Background(), entries[1], bytes.NewReader([]byte("x"))); !errors.Is(err, ErrUnsafeStorage) {
		t.Fatalf("replaced directory accepted: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("replacement directory was modified: %v", err)
	}
}
