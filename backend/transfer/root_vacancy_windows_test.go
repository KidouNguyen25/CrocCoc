//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeReceiveRootVacancyDetectsFileFolderAndCase(t *testing.T) {
	destination := t.TempDir()
	lease, err := OpenReceiveDestinationLease(destination)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.CheckReceiveRootVacancy([]ManifestEntry{{Path: "new.txt"}}); err != nil {
		t.Fatalf("missing root should be vacant: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "RealFile.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(destination, "MyFolder"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []ManifestEntry{
		{Path: "realfile.txt"}, {Path: "MYFOLDER", Directory: true},
	} {
		if err := lease.CheckReceiveRootVacancy([]ManifestEntry{entry}); !errors.Is(err, ErrDestinationConflict) {
			t.Fatalf("existing root %s escaped vacancy: %v", entry.Path, err)
		}
	}
	if err := lease.CheckReceiveRootVacancy([]ManifestEntry{{Path: "new.txt"}}); err != nil {
		t.Fatalf("unrelated item failed: %v", err)
	}
}
func TestNativeReceiveRootVacancyRejectsUnsafeOffer(t *testing.T) {
	lease, err := OpenReceiveDestinationLease(t.TempDir())
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.CheckReceiveRootVacancy([]ManifestEntry{{Path: "CON.txt"}}); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("device name escaped: %v", err)
	}
}
