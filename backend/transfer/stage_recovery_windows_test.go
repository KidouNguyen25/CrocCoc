//go:build windows && !croc_no_tailcat

package transfer

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestStageJournalDirectoryUsesPrivateProtectedACL(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "CrocCoc", "staging")
	if err := ensurePrivateStageJournalDirectory(directory); err != nil {
		t.Fatal(err)
	}
	security, err := windows.GetNamedSecurityInfo(directory, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := security.Control()
	dacl, _, daclErr := security.DACL()
	if err != nil || daclErr != nil || control&windows.SE_DACL_PROTECTED == 0 || dacl == nil || dacl.AceCount == 0 {
		t.Fatalf("journal directory ACL is not private and protected: control=%x dacl=%v err=%v", control, dacl, err)
	}
}

func TestRecoveryRejectsOversizedJournal(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "journal")
	if err := os.Mkdir(journalDir, 0700); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(journalDir, "00000000000000000000000000000000.json")
	if err := os.WriteFile(journalPath, bytes.Repeat([]byte("x"), 8193), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recoverOwnedReceiveStagesAt(journalDir); !errors.Is(err, ErrRecoveryPending) {
		t.Fatalf("oversized journal was accepted: %v", err)
	}
}

func TestRecoveryCrashHelper(t *testing.T) {
	if os.Getenv("CROCCOC_RECOVERY_CRASH_CHILD") != "1" {
		return
	}
	destination := os.Getenv("CROCCOC_RECOVERY_DESTINATION")
	journalDir := os.Getenv("CROCCOC_RECOVERY_JOURNAL")
	entries := []ManifestEntry{{Path: "partial.txt", SizeBytes: 4}}
	lease, err := PreflightReceiveDestination(destination, entries)
	if err != nil {
		os.Exit(2)
	}
	stage, err := prepareReceiveStageAt(lease, entries, journalDir)
	if err != nil {
		os.Exit(3)
	}
	if err := stage.prepareStageDirectories(entries); err != nil {
		os.Exit(4)
	}
	if err := stage.writeStageFile(t.Context(), entries[0], bytes.NewReader([]byte("data"))); err != nil {
		os.Exit(5)
	}
	// Exit without closing handles to simulate abrupt process termination.
	os.Exit(0)
}

func TestRecoveryRemovesStageAfterProcessCrash(t *testing.T) {
	destination := t.TempDir()
	journalDir := filepath.Join(t.TempDir(), "journal")
	lease, err := PreflightReceiveDestination(destination, []ManifestEntry{{Path: "partial.txt", SizeBytes: 4}})
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestRecoveryCrashHelper$")
	command.Env = append(os.Environ(),
		"CROCCOC_RECOVERY_CRASH_CHILD=1",
		"CROCCOC_RECOVERY_DESTINATION="+destination,
		"CROCCOC_RECOVERY_JOURNAL="+journalDir,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("crash helper failed: %v\n%s", err, output)
	}
	if err := recoverOwnedReceiveStagesAt(journalDir); err != nil {
		t.Fatalf("recover after process crash: %v", err)
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), stagePrefix) {
			t.Fatalf("crashed stage remains: %q", entry.Name())
		}
	}
	journals, err := os.ReadDir(journalDir)
	if err != nil || len(journals) != 0 {
		t.Fatalf("crash journal remains: %v, err=%v", journals, err)
	}
}

func TestRecoveryRemovesOnlyJournalOwnedStage(t *testing.T) {
	destination := t.TempDir()
	journalDir := filepath.Join(t.TempDir(), "journal")
	entries := []ManifestEntry{{Path: "partial.txt", SizeBytes: 4}}
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	stage, err := prepareReceiveStageAt(lease, entries, journalDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.prepareStageDirectories(entries); err != nil {
		t.Fatal(err)
	}
	if err := stage.writeStageFile(t.Context(), entries[0], bytes.NewReader([]byte("data"))); err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(destination, stage.StageName)
	journalPath, lockPath := stage.JournalPath(), stage.journalLockPath
	if err := stage.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recoverOwnedReceiveStagesAt(journalDir); err != nil {
		t.Fatalf("recover owned stage: %v", err)
	}
	for _, path := range []string{stagePath, journalPath, lockPath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("recovery left owned path %q: %v", filepath.Base(path), err)
		}
	}
}

func TestRecoveryDoesNotRemoveStageWithLiveLease(t *testing.T) {
	destination := t.TempDir()
	journalDir := filepath.Join(t.TempDir(), "journal")
	entries := []ManifestEntry{{Path: "partial.txt", SizeBytes: 4}}
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	stage, err := prepareReceiveStageAt(lease, entries, journalDir)
	if err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(destination, stage.StageName)
	if err := recoverOwnedReceiveStagesAt(journalDir); !errors.Is(err, ErrRecoveryPending) {
		t.Fatalf("live stage recovered: %v", err)
	}
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("live stage disappeared: %v", err)
	}
	if err := stage.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recoverOwnedReceiveStagesAt(journalDir); err != nil {
		t.Fatalf("stale stage recovery failed: %v", err)
	}
}

func TestRecoveryDropsJournalWhenDestinationIsGone(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "destination")
	journalDir := filepath.Join(root, "journal")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	entries := []ManifestEntry{{Path: "a.txt", SizeBytes: 4}}
	lease, err := PreflightReceiveDestination(destination, entries)
	if err != nil {
		t.Fatal(err)
	}
	stage, err := prepareReceiveStageAt(lease, entries, journalDir)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := stage.JournalPath()
	_ = stage.Close()
	_ = lease.Close()
	if err := os.RemoveAll(destination); err != nil {
		t.Fatal(err)
	}
	if err := recoverOwnedReceiveStagesAt(journalDir); err != nil {
		t.Fatalf("missing destination blocks recovery: %v", err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal was kept: %v", err)
	}
}
