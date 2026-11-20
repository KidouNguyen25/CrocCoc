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

func auditFixture(t *testing.T) (*ReceiveStage, string, []ManifestEntry) {
	t.Helper()
	dir := t.TempDir()
	entries := []ManifestEntry{
		{Path: "album", Directory: true},
		{Path: "album/photo.txt", SizeBytes: 5},
	}
	lease, err := PreflightReceiveDestination(dir, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires local NTFS")
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
	return stage, dir, entries
}

func TestStagingAuditRequiresAllFilesAndExactContentSize(t *testing.T) {
	stage, dir, entries := auditFixture(t)
	if err := stage.AuditStagedPayload(); !errors.Is(err, ErrInvalidStaging) {
		t.Fatalf("missing payload passed audit: %v", err)
	}
	if err := stage.writeStageFile(context.Background(), entries[1], bytes.NewReader([]byte("hello"))); err != nil {
		t.Fatal(err)
	}
	if err := stage.AuditStagedPayload(); err != nil {
		t.Fatalf("valid staged payload rejected: %v", err)
	}
	file := filepath.Join(dir, stage.StageName, "payload", "album", "photo.txt")
	if err := os.WriteFile(file, []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := stage.AuditStagedPayload(); !errors.Is(err, ErrInvalidStaging) {
		t.Fatalf("modified file size accepted: %v", err)
	}
}

func TestStagingAuditRejectsExtraUnmanifestedFile(t *testing.T) {
	stage, dir, entries := auditFixture(t)
	if err := stage.writeStageFile(context.Background(), entries[1], bytes.NewReader([]byte("hello"))); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(dir, stage.StageName, "payload", "unexpected.txt")
	if err := os.WriteFile(extra, []byte("extra"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := stage.AuditStagedPayload(); !errors.Is(err, ErrInvalidStaging) {
		t.Fatalf("unmanifested extra file accepted: %v", err)
	}
}

func TestStagingAuditRejectsSameSizeReplacementAndHardLinks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(string, string) error
	}{
		{"same-size replacement", func(file, outside string) error {
			if err := os.Rename(file, outside); err != nil {
				return err
			}
			return os.WriteFile(file, []byte("other"), 0600)
		}},
		{"additional hard link", os.Link},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stage, dir, entries := auditFixture(t)
			if err := stage.writeStageFile(context.Background(), entries[1], bytes.NewReader([]byte("hello"))); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, stage.StageName, "payload", "album", "photo.txt")
			if err := tc.change(file, filepath.Join(dir, "test-fixture-copy.txt")); err != nil {
				t.Fatal(err)
			}
			if err := stage.AuditStagedPayload(); !errors.Is(err, ErrInvalidStaging) {
				t.Fatalf("changed object accepted: %v", err)
			}
		})
	}
}
