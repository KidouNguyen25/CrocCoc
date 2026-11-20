package transfer

import (
	"testing"
	"time"
)

func TestReceivePreflightPermitBoundToOfferAndConsent(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	c, waiting := newWaitingReceive(t)
	entries := []ManifestEntry{{Path: "photos", Directory: true}, {Path: "photos\\a.txt", SizeBytes: 5}}
	_, offer, err := c.PresentReceiveOffer(waiting.TransferID, entries, now)
	if err != nil {
		t.Fatal(err)
	}
	permit := ReceivePermit{
		transferID: waiting.TransferID,
		offerID:    offer.OfferID,
		digest:     fingerprintManifest(entries),
		stageID:    "native-stage-for-test",
	}
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, permit, now); ok {
		t.Fatal("advanced receiver without explicit consent")
	}
	if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now); !ok {
		t.Fatal("consent failed")
	}
	bad := permit
	bad.stageID = ""
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, bad, now); ok {
		t.Fatal("stage with empty identity authorized")
	}
	bad = permit
	bad.digest = fingerprintManifest([]ManifestEntry{{Path: "changed.txt"}})
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, bad, now); ok {
		t.Fatal("changed metadata authorized")
	}
	bad = permit
	bad.offerID = "wrong"
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, bad, now); ok {
		t.Fatal("stale offer authorized")
	}
	committed, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, permit, now)
	if !ok || committed.State != Transferring || committed.Revision != 4 {
		t.Fatalf("correct permit rejected: %+v %v", committed, ok)
	}
	if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, permit, now); ok {
		t.Fatal("permit reused")
	}
}

func TestReceivePermitCannotBeatCancellationOrExpiry(t *testing.T) {
	now := time.Now()
	for _, expired := range []bool{false, true} {
		c, waiting := newWaitingReceive(t)
		entries := []ManifestEntry{{Path: "a.txt", SizeBytes: 1}}
		_, offer, err := c.PresentReceiveOffer(waiting.TransferID, entries, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now); !ok {
			t.Fatal("consent failed")
		}
		permit := ReceivePermit{
			transferID: waiting.TransferID, offerID: offer.OfferID,
			digest: fingerprintManifest(entries), stageID: "native-stage-for-test",
		}
		at := now
		if expired {
			at = now.Add(OfferDecisionTimeout)
		} else if _, ok := c.Cancel(waiting.TransferID); !ok {
			t.Fatal("cancel did not win")
		}
		if _, ok := c.CommitReceivePreflight(waiting.TransferID, offer.OfferID, permit, at); ok {
			t.Fatal("stale/cancelled offer granted transfer")
		}
	}
}

func TestFingerprintManifestNormalizesSeparatorsAndOrder(t *testing.T) {
	first := []ManifestEntry{{Path: "dir", Directory: true}, {Path: "dir\\a.txt", SizeBytes: 1}}
	second := []ManifestEntry{{Path: "dir/a.txt", SizeBytes: 1}, {Path: "dir", Directory: true}}
	if fingerprintManifest(first) != fingerprintManifest(second) {
		t.Fatal("equivalent manifest changed fingerprint")
	}
	second[0].SizeBytes = 2
	if fingerprintManifest(first) == fingerprintManifest(second) {
		t.Fatal("different content size produced matching fingerprint")
	}
}
