package transfer

import (
	"errors"
	"io/fs"
	"reflect"
	"strconv"
	"testing"
)

func TestProjectCrocReceiveManifestCapturesExactTree(t *testing.T) {
	got, err := ProjectCrocReceiveManifest(CrocReceiveMetadata{
		Files: []CrocReceiveFile{
			{Folder: "Dữ liệu/Sub/", Name: "b.txt", SizeBytes: 4},
			{Folder: "./", Name: "README.txt"},
			{Folder: "Dữ liệu\\Sub\\", Name: "a.txt", SizeBytes: 3},
		},
		EmptyDirectories: []string{"Dữ liệu/Empty/", "OnlyEmpty\\"},
	})
	want := []ManifestEntry{
		{Path: "Dữ liệu", Directory: true},
		{Path: "Dữ liệu/Empty", Directory: true},
		{Path: "Dữ liệu/Sub", Directory: true},
		{Path: "Dữ liệu/Sub/a.txt", SizeBytes: 3},
		{Path: "Dữ liệu/Sub/b.txt", SizeBytes: 4},
		{Path: "OnlyEmpty", Directory: true},
		{Path: "README.txt"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %#v, err = %v", got, err)
	}
}

func TestProjectCrocReceiveManifestRejectsUnsupportedOrAmbiguousMetadata(t *testing.T) {
	cases := map[string]CrocReceiveMetadata{
		"text":            {SendingText: true, Files: []CrocReceiveFile{{Name: "sample.txt"}}},
		"link":            {Files: []CrocReceiveFile{{Name: "link", Symlink: true}}},
		"nonregular mode": {Files: []CrocReceiveFile{{Name: "pipe", Mode: fs.ModeNamedPipe}}},
		"archive":         {Files: []CrocReceiveFile{{Name: "box.zip", Archive: true}}},
		"basename":        {Files: []CrocReceiveFile{{Name: "folder/file.txt"}}},
		"root folder":     {EmptyDirectories: []string{"./"}},
		"empty with file": {Files: []CrocReceiveFile{{Folder: "dir/", Name: "file.txt"}}, EmptyDirectories: []string{"dir/"}},
		"nested empty":    {EmptyDirectories: []string{"dir/", "dir/sub/"}},
		"internal case":   {Files: []CrocReceiveFile{{Folder: "dir/", Name: "a.txt"}, {Folder: "DIR/", Name: "b.txt"}}},
		"duplicate":       {Files: []CrocReceiveFile{{Name: "a.txt"}, {Name: "A.txt"}}},
		"file as folder":  {Files: []CrocReceiveFile{{Name: "dir"}, {Folder: "dir/", Name: "a.txt"}}},
		"invalid folder":  {Files: []CrocReceiveFile{{Folder: "dir/../safe/", Name: "a.txt"}}},
		"negative size":   {Files: []CrocReceiveFile{{Name: "a.txt", SizeBytes: -1}}},
		"missing":         {},
	}
	for name, metadata := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := ProjectCrocReceiveManifest(metadata); got != nil || !errors.Is(err, ErrUnsafeOffer) {
				t.Fatalf("manifest = %#v, err = %v", got, err)
			}
		})
	}
}

func TestProjectCrocReceiveManifestCountsImplicitParentsAgainstLimit(t *testing.T) {
	files := make([]CrocReceiveFile, MaxManifestEntries/2+1)
	for i := range files {
		files[i] = CrocReceiveFile{Folder: "folder" + strconv.Itoa(i) + "/", Name: "file.txt"}
	}
	for _, metadata := range []CrocReceiveMetadata{
		{Files: files},
		{Files: make([]CrocReceiveFile, MaxManifestEntries+1)},
		{Files: []CrocReceiveFile{{Name: "a.txt", SizeBytes: MaxManifestBytes}, {Name: "b.txt", SizeBytes: 1}}},
	} {
		if got, err := ProjectCrocReceiveManifest(metadata); got != nil || !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("manifest size = %d, err = %v", len(got), err)
		}
	}
}
