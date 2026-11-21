//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPublishedReceiveStageRemovesOwnedDirectories(t *testing.T) {
	entries := []ManifestEntry{{Path: "received.txt", SizeBytes: 2}}
	lease, stage, destination := publishFixture(t, entries)
	journalPath := stage.JournalPath()
	stagePath := filepath.Join(destination, stage.StageName)
	if _, err := publishAuditedRoots(lease, stage, entries); err != nil {
		t.Fatal(err)
	}
	if err := cleanupPublishedReceiveStage(lease, stage); err != nil {
		items, readErr := os.ReadDir(stagePath)
		t.Fatalf("cleanup published stage: %v; children=%v; read=%v", err, items, readErr)
	}
	if _, err := os.Stat(stagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published stage remains: %v", err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published journal remains: %v", err)
	}
}

func TestFinalizeNativeReceiveRemovesPublishedStageAndJournal(t *testing.T) {
	entries := []ManifestEntry{{Path: "received.txt", SizeBytes: 2}}
	lease, stage, destination := publishFixture(t, entries)
	journalPath := stage.JournalPath()
	stagePath := filepath.Join(destination, stage.StageName)
	var coordinator Coordinator
	started, err := coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, next := range []State{WaitingForPeer, Transferring} {
		if _, ok := coordinator.Advance(started.TransferID, next); !ok {
			t.Fatalf("failed to enter %s", next)
		}
	}

	completed, result, err := FinalizeNativeReceive(&coordinator, started.TransferID, lease, stage, entries)
	if err != nil || completed.State != Completed || result.Partial {
		t.Fatalf("finalize failed: %+v %+v %v", completed, result, err)
	}
	if _, err := os.Stat(stagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published stage remains: %v", err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published journal remains: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "received.txt")); err != nil || string(data) != "xx" {
		t.Fatalf("published file changed: %q %v", data, err)
	}
	if _, err := coordinator.Begin(); err != nil {
		t.Fatalf("clean terminal session kept coordinator busy: %v", err)
	}
}

func TestPublishedStageCleanupKeepsUnexpectedContents(t *testing.T) {
	entries := []ManifestEntry{{Path: "received.txt", SizeBytes: 2}}
	lease, stage, destination := publishFixture(t, entries)
	if _, err := publishAuditedRoots(lease, stage, entries); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(stage.PayloadPath(), "unexpected.txt")
	if err := os.WriteFile(extra, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupPublishedReceiveStage(lease, stage); !errors.Is(err, ErrStageCleanup) {
		t.Fatalf("unexpected stage content removed: %v", err)
	}
	if data, err := os.ReadFile(extra); err != nil || string(data) != "keep" {
		t.Fatalf("unexpected stage content changed: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(destination, stage.StageName)); err != nil {
		t.Fatalf("owned stage disappeared after failed cleanup: %v", err)
	}
}
