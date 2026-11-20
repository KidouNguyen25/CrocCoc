package transfer

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateManifestAcceptsNestedUnicodeFiles(t *testing.T) {
	entries := []ManifestEntry{
		{Path: "Dữ liệu", Directory: true},
		{Path: "Dữ liệu\\ảnh 1.png", SizeBytes: 9},
		{Path: "Dữ liệu/ảnh 2.png", SizeBytes: 11},
		{Path: "README.txt", SizeBytes: 0},
	}
	total, err := ValidateManifest(entries)
	if err != nil || total != 20 {
		t.Fatalf("total=%d, err=%v", total, err)
	}
}

func TestValidateManifestRejectsUnsafePaths(t *testing.T) {
	bad := []string{
		"", ".", "..", "../secret", "a/../b", "a//b",
		"/absolute", "\\\\server\\share", "C:\\file.txt",
		"foo:stream", "a/b.", "a/b ", "CON.txt", "con", "LPT1.dat",
		"COM¹.txt", "a/<bad>", "a/one?two", "x\u202Etxt.exe",
		strings.Repeat("a", 259), string([]byte{0xff, 0xfe}),
	}
	for i, path := range bad {
		t.Run(strings.ReplaceAll(strings.TrimSpace(path), "/", "_"), func(t *testing.T) {
			if _, err := ValidateManifest([]ManifestEntry{{Path: path}}); !errors.Is(err, ErrUnsafeOffer) {
				t.Fatalf("case %d: got %v, want ErrUnsafeOffer", i, err)
			}
		})
	}
}

func TestValidateManifestRejectsCollisionsAndInvalidSizes(t *testing.T) {
	cases := [][]ManifestEntry{
		{{Path: "Photo.JPG"}, {Path: "photo.jpg"}},
		{{Path: "folder", Directory: false}, {Path: "folder/file.txt"}},
		{{Path: "folder/file.txt"}, {Path: "folder", Directory: false}},
		{{Path: "directory", Directory: true, SizeBytes: 1}},
		{{Path: "file", SizeBytes: -1}},
		nil,
	}
	for i, entries := range cases {
		if _, err := ValidateManifest(entries); !errors.Is(err, ErrUnsafeOffer) {
			t.Fatalf("case %d: got %v", i, err)
		}
	}
}

func TestValidateManifestEnforcesQuota(t *testing.T) {
	cases := [][]ManifestEntry{
		{{Path: "big", SizeBytes: MaxManifestBytes + 1}},
		{{Path: "a", SizeBytes: MaxManifestBytes}, {Path: "b", SizeBytes: 1}},
	}
	for _, entries := range cases {
		if _, err := ValidateManifest(entries); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("got %v, want ErrResourceLimit", err)
		}
	}
	many := make([]ManifestEntry, MaxManifestEntries+1)
	if _, err := ValidateManifest(many); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("entry quota: got %v", err)
	}
}
