//go:build windows

package transfer

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeStagePermitUnlocksOnlyAcceptedExactOffer(t *testing.T) {
	now := time.Now()
	entries := []ManifestEntry{{Path: "folder", Directory: true}, {Path: "folder/a.txt", SizeBytes: 1}}
	c, waiting := newWaitingReceive(t)
	_, offer, err := c.PresentReceiveOffer(waiting.TransferID, entries, now)
	if err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	lease, err := PreflightReceiveDestination(destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	stage, err := prepareReceiveStageAt(lease, entries, filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if permit := stage.receivePermit(waiting.TransferID, offer.OfferID); permit.stageID != "" {
		t.Fatal("stage without declared directories authorized a receive")
	}
	if err := stage.prepareStageDirectories(entries); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID,
		stage.receivePermit(waiting.TransferID, offer.OfferID), now); ok {
		t.Fatal("stage bypassed user consent")
	}
	if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now); !ok {
		t.Fatal("consent rejected")
	}
	snapshot, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID,
		stage.receivePermit(waiting.TransferID, offer.OfferID), now)
	if !ok || snapshot.State != Transferring {
		t.Fatalf("valid native preflight rejected: %+v %t", snapshot, ok)
	}
	_ = stage.Close()
	if permit := stage.receivePermit(waiting.TransferID, offer.OfferID); permit.stageID != "" {
		t.Fatal("closed staging handle issued a new permit")
	}
}
