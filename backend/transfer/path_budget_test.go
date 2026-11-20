package transfer

import (
	"errors"
	"strings"
	"testing"
)

func TestReceivePathBudgetChecksStagingAndFinal(t *testing.T) {
	entries := []ManifestEntry{
		{Path: "folder", Directory: true},
		{Path: "folder/photo.jpg", SizeBytes: 100},
	}
	for _, destination := range []string{`C:\`, `D:\Downloads`, `C:\Users\Example\Documents`} {
		if err := CheckReceivePathBudget(destination, entries); err != nil {
			t.Fatalf("valid Windows destination %q failed: %v", destination, err)
		}
	}
	// The relative name and final destination fit, but the staging prefix
	// pushes this candidate over the conservative Windows path limit.
	longEntry := []ManifestEntry{{Path: strings.Repeat("x", 200)}}
	if _, err := ValidateManifest(longEntry); err != nil {
		t.Fatalf("relative path should pass manifest stage: %v", err)
	}
	if err := CheckReceivePathBudget(`C:\Downloads`, longEntry); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("staging path overrun was accepted: %v", err)
	}
}

func TestReceivePathBudgetCountsUTF16Units(t *testing.T) {
	relative := strings.Repeat("🌼", 100)
	if _, err := ValidateManifest([]ManifestEntry{{Path: relative}}); err != nil {
		t.Fatalf("relative unicode path unexpectedly rejected: %v", err)
	}
	if err := CheckReceivePathBudget(`C:\Folder`, []ManifestEntry{{Path: relative}}); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("emoji path did not count two UTF-16 units per symbol: %v", err)
	}
}

func TestReceivePathBudgetRejectsUnsupportedDestinations(t *testing.T) {
	entries := []ManifestEntry{{Path: "a.txt"}}
	for _, destination := range []string{
		"", "C:", "relative", `\\server\share`,
		`C:\folder\..\dest`, `C:\folder\\dest`,
		`C:\CON\files`, `C:\folder:stream`,
	} {
		if err := CheckReceivePathBudget(destination, entries); !errors.Is(err, ErrUnsupportedDestination) {
			t.Fatalf("invalid destination %q accepted: %v", destination, err)
		}
	}
}

func TestReceivePathBudgetStillValidatesManifest(t *testing.T) {
	if err := CheckReceivePathBudget(`C:\Folder`, []ManifestEntry{{Path: "CON.txt"}}); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("unsafe manifest was accepted: %v", err)
	}
}
