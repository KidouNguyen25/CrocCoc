//go:build windows

package transfer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestReceiveStageCreatesExclusiveSameVolumeJournalAndMarker(t *testing.T) {
	dir := t.TempDir()
	ledgerDir := filepath.Join(t.TempDir(), "journal")
	entries := []ManifestEntry{{Path: "photo.txt", SizeBytes: 10}}
	lease, err := PreflightReceiveDestination(dir, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test directory not supported local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()

	a, err := prepareReceiveStageAt(lease, entries, ledgerDir)
	if err != nil {
		t.Fatalf("stage preparation: %v", err)
	}
	defer a.Close()
	b, err := prepareReceiveStageAt(lease, entries, ledgerDir)
	if err != nil {
		t.Fatalf("second stage preparation: %v", err)
	}
	defer b.Close()
	if a.StageName == b.StageName || !strings.HasPrefix(a.StageName, stagePrefix) {
		t.Fatalf("staging identifiers reused: %q %q", a.StageName, b.StageName)
	}
	record := a.Ownership()
	if record.Destination != lease.Identity() || record.Stage.FileIndex == 0 ||
		record.Stage.VolumeSerial != record.Destination.VolumeSerial ||
		record.Status != "prepared" || len(record.MarkerNonce) != 32 {
		t.Fatalf("bad durable record: %+v", record)
	}
	raw, err := os.ReadFile(a.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	var onDisk StageOwnership
	if err := json.Unmarshal(raw, &onDisk); err != nil || onDisk != record {
		t.Fatalf("journal doesn't match: %+v %v", onDisk, err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, a.StageName, ".croccoc-owner"))
	if err != nil || string(marker) != record.MarkerNonce {
		t.Fatalf("marker not persisted: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, a.StageName, "payload")); err != nil || !info.IsDir() {
		t.Fatalf("payload directory missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.txt")); !os.IsNotExist(err) {
		t.Fatal("preparing a stage wrote the final payload")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.JournalPath()); err != nil {
		t.Fatal("close must not silently remove the ownership record")
	}
}

func TestStageFailsBeforeWritingUnsafeManifest(t *testing.T) {
	dir := t.TempDir()
	lease, err := OpenReceiveDestinationLease(dir)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test directory not supported local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	stage, err := prepareReceiveStageAt(lease, []ManifestEntry{{Path: "CON.txt"}}, filepath.Join(t.TempDir(), "journal"))
	if !errors.Is(err, ErrUnsafeOffer) || stage != nil {
		t.Fatalf("invalid manifest got stage: %v", err)
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(before) {
		t.Fatalf("unsafe input mutated destination: %v", err)
	}
}

func TestReceiveStageProtectsItsACLAndInheritsIntoPayload(t *testing.T) {
	stage, destination, _ := auditFixture(t)
	root := filepath.Join(destination, stage.StageName)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	expectedACEs := uint16(2)
	if user.User.Sid.String() == "S-1-5-18" {
		expectedACEs = 1
	}
	for _, item := range []struct {
		path      string
		protected bool
	}{
		{root, true},
		{filepath.Join(root, "payload"), false},
		{filepath.Join(root, "payload", "album"), false},
	} {
		desc, err := windows.GetNamedSecurityInfo(item.path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		control, _, err := desc.Control()
		if err != nil || (control&windows.SE_DACL_PROTECTED != 0) != item.protected {
			t.Fatalf("unexpected DACL protection: control=%x err=%v", control, err)
		}
		dacl, _, err := desc.DACL()
		if err != nil || dacl == nil || dacl.AceCount != expectedACEs {
			t.Fatal("staging DACL differs from process-user and LocalSystem policy")
		}
	}
}
