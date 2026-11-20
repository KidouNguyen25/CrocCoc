//go:build windows

package transfer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func publishFixture(t *testing.T, entries []ManifestEntry) (*DestinationLease, *ReceiveStage, string) {
	t.Helper()
	destination := t.TempDir()
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Close() })
	stage, err := prepareReceiveStageAt(lease, entries, filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stage.Close() })
	if err := stage.prepareStageDirectories(entries); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Directory {
			continue
		}
		data := bytes.Repeat([]byte("x"), int(e.SizeBytes))
		if err := stage.writeStageFile(context.Background(), e, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	return lease, stage, destination
}

func TestPublishAuditedRootsMovesFilesAndDirectoriesWithoutReplacement(t *testing.T) {
	entries := []ManifestEntry{
		{Path: "folder", Directory: true},
		{Path: "folder/inside.txt", SizeBytes: 3},
		{Path: "file.txt", SizeBytes: 2},
	}
	lease, stage, destination := publishFixture(t, entries)
	result, err := publishAuditedRoots(lease, stage, entries)
	if err != nil || result.Partial || len(result.Published) != 2 {
		t.Fatalf("publish failed: %+v %v", result, err)
	}
	if stage.Ownership().Status != "published" {
		t.Fatalf("ownership not marked published: %q", stage.Ownership().Status)
	}
	data, err := os.ReadFile(filepath.Join(destination, "folder", "inside.txt"))
	if err != nil || string(data) != "xxx" {
		t.Fatalf("folder published incorrectly: %q %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(destination, "file.txt"))
	if err != nil || string(data) != "xx" {
		t.Fatalf("file published incorrectly: %q %v", data, err)
	}
	journalData, err := os.ReadFile(stage.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	var record StageOwnership
	if err := json.Unmarshal(journalData, &record); err != nil || record.Status != "published" {
		t.Fatalf("journal not updated: %+v %v", record, err)
	}
	if _, err := publishAuditedRoots(lease, stage, entries); !errors.Is(err, ErrInvalidStaging) {
		t.Fatalf("published stage reused: %v", err)
	}
}

func TestPublishAuditedRootsReportsLateConflictWithoutDeletingUserData(t *testing.T) {
	entries := []ManifestEntry{
		{Path: "first.txt", SizeBytes: 2},
		{Path: "second.txt", SizeBytes: 2},
	}
	lease, stage, destination := publishFixture(t, entries)
	conflict := filepath.Join(destination, "second.txt")
	if err := os.WriteFile(conflict, []byte("user-existing"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := publishAuditedRoots(lease, stage, entries)
	if !errors.Is(err, ErrPublishFailed) || !result.Partial ||
		len(result.Published) != 1 || result.Published[0] != "first.txt" {
		t.Fatalf("partial publish not reported: %+v %v", result, err)
	}
	existing, err := os.ReadFile(conflict)
	if err != nil || string(existing) != "user-existing" {
		t.Fatalf("user data modified: %q %v", existing, err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "first.txt"))
	if err != nil || string(data) != "xx" {
		t.Fatalf("first published entry lost: %q %v", data, err)
	}
	if stage.Ownership().Status != "partial" {
		t.Fatalf("partial journal status lost: %q", stage.Ownership().Status)
	}
}

func TestPublishRejectsAlteredManifestBeforeAnyDestinationChange(t *testing.T) {
	entries := []ManifestEntry{{Path: "file.txt", SizeBytes: 2}}
	lease, stage, destination := publishFixture(t, entries)
	other := []ManifestEntry{{Path: "file.txt", SizeBytes: 3}}
	if _, err := publishAuditedRoots(lease, stage, other); !errors.Is(err, ErrInvalidStaging) {
		t.Fatalf("changed manifest accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "file.txt")); !os.IsNotExist(err) {
		t.Fatal("unsafe publish created a final file")
	}
}
