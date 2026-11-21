package transfer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInspectSendSourcesBuildsManifestWithoutPayload(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "Tài liệu")
	if err := os.MkdirAll(filepath.Join(folder, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "nested", "báo cáo.txt")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := InspectSendSources([]string{folder})
	if err != nil {
		t.Fatal(err)
	}
	want := []ManifestEntry{
		{Path: "Tài liệu", Directory: true},
		{Path: "Tài liệu/nested", Directory: true},
		{Path: "Tài liệu/nested/báo cáo.txt", SizeBytes: 5},
	}
	if !slices.Equal(entries, want) {
		t.Fatalf("manifest mismatch:\ngot  %#v\nwant %#v", entries, want)
	}
}

func TestInspectSendSourcesRejectsUntrustedInputs(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "file.txt")
	if err := os.WriteFile(source, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectSendSources(nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty selection: %v", err)
	}
	if _, err := InspectSendSources([]string{source, source}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate source: %v", err)
	}
	if _, err := InspectSendSources([]string{filepath.Join(base, "missing")}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := InspectSendSources([]string{" "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank path: %v", err)
	}
}

func TestInspectSendSourcesMatchingRejectsChangedTree(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := InspectSendSources([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := SendManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectSendSourcesMatching([]string{root}, digest); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("changed source tree accepted: %v", err)
	}
}

func TestInspectSendSourcesHonorsPreparationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := InspectSendSourcesContext(ctx, []string{t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled source scan continued: %v", err)
	}
}

func TestInspectSendSourcesRejectsSymlinkAndAncestors(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target.txt")
	if err := os.WriteFile(target, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "alias.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := InspectSendSources([]string{link}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("symlink source accepted: %v", err)
	}
	folder := filepath.Join(base, "content")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(folder, "nested-alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectSendSources([]string{folder}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("nested symlink accepted: %v", err)
	}
	parentLink := filepath.Join(base, "shortcut")
	if err := os.Symlink(folder, parentLink); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectSendSources([]string{filepath.Join(parentLink, "nested-alias")}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("symlink ancestor accepted: %v", err)
	}
}
