package transfer

import (
	"errors"
	"testing"
)

func TestCheckReceiveRootConflicts(t *testing.T) {
	files := []ManifestEntry{
		{Path: "Photos", Directory: true},
		{Path: "Photos/2026/photo.jpg", SizeBytes: 20},
		{Path: "note.txt", SizeBytes: 5},
	}
	cases := []struct {
		name     string
		existing []string
		conflict bool
	}{
		{name: "empty target"},
		{name: "unrelated roots", existing: []string{"archive", "photo.jpg"}},
		{name: "same existing folder", existing: []string{"photos"}, conflict: true},
		{name: "same existing file", existing: []string{"NOTE.TXT"}, conflict: true},
		{name: "existing case-variant", existing: []string{"pHoToS"}, conflict: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckReceiveRootConflicts(files, tc.existing)
			if (err != nil) != tc.conflict {
				t.Fatalf("got %v, conflict=%t", err, tc.conflict)
			}
			if tc.conflict && !errors.Is(err, ErrDestinationConflict) {
				t.Fatalf("wrong conflict error: %v", err)
			}
		})
	}
}

func TestCheckReceiveRootConflictsRejectsInvalidManifestFirst(t *testing.T) {
	if err := CheckReceiveRootConflicts([]ManifestEntry{{Path: "C:/file.txt"}}, nil); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("invalid name escaped validation: %v", err)
	}
	entries := []ManifestEntry{{Path: "folder\\one"}, {Path: "folder/two"}}
	if err := CheckReceiveRootConflicts(entries, []string{"FOLDER"}); !errors.Is(err, ErrDestinationConflict) {
		t.Fatalf("nested root escaped conflict check: %v", err)
	}
}
