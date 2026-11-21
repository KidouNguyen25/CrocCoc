//go:build windows

package transfer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSendSourceLeaseBlocksMutationUntilClose(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "bundle")
	if err := os.MkdirAll(filepath.Join(selected, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(selected, "nested", "payload.txt")
	if err := os.WriteFile(file, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := InspectSendSources([]string{selected})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := SendManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	lease, _, err := AcquireSendSourceLease([]string{selected})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("changed"), 0600); err == nil {
		_ = lease.Close()
		t.Fatal("source write succeeded while send lease was active")
	}
	if err := os.WriteFile(filepath.Join(selected, "new.txt"), []byte("new"), 0600); err != nil {
		_ = lease.Close()
		t.Fatalf("source directory unexpectedly blocked additions: %v", err)
	}
	if _, err := InspectSendSourcesMatching([]string{selected}, digest); err != ErrSourceChanged {
		_ = lease.Close()
		t.Fatalf("source addition did not invalidate frozen manifest: %v", err)
	}
	if err := os.Rename(selected, selected+"-moved"); err == nil {
		_ = lease.Close()
		t.Fatal("source rename succeeded while send lease was active")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("safe"), 0600); err != nil {
		t.Fatalf("source remained locked after close: %v", err)
	}
}

func TestSendSourceLeaseRejectsReparseSource(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if lease, _, err := AcquireSendSourceLease([]string{link}); err == nil {
		_ = lease.Close()
		t.Fatal("reparse source received a send lease")
	}
}
