package transfer

import (
	"errors"
	"testing"
	"time"
)

func newWaitingReceive(t *testing.T) (*Coordinator, SessionSnapshot) {
	t.Helper()
	c := &Coordinator{}
	created, err := c.BeginReceive()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := c.Advance(created.TransferID, WaitingForPeer)
	if !ok || waiting.Revision != 2 {
		t.Fatalf("receiver could not wait: %#v, %t", waiting, ok)
	}
	if _, ok := c.Advance(created.TransferID, Transferring); ok {
		t.Fatal("receiver fetched payload before manifest or consent")
	}
	if _, ok := c.Advance(created.TransferID, AwaitingAcceptance); ok {
		t.Fatal("receiver skipped metadata verification")
	}
	return c, waiting
}

func TestReceiveGateRequiresMatchingOfferAndConsent(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	c, waiting := newWaitingReceive(t)
	entries := []ManifestEntry{{Path: "folder", Directory: true}, {Path: "folder/file.txt", SizeBytes: 10}}
	presented, offer, err := c.PresentReceiveOffer(waiting.TransferID, entries, now)
	if err != nil || presented.State != AwaitingAcceptance || presented.Revision != 3 {
		t.Fatalf("offer rejected: %#v, %#v, %v", presented, offer, err)
	}
	entries[1].Path = "changed.txt"
	copied, ok := c.GetReceiveOffer(waiting.TransferID)
	if !ok || copied.OfferID != offer.OfferID || copied.Items[1].Path != "folder/file.txt" {
		t.Fatal("stored metadata changed with caller's mutable input")
	}
	copied.Items[1].Path = "tampered"
	again, ok := c.GetReceiveOffer(waiting.TransferID)
	if !ok || again.Items[1].Path != "folder/file.txt" {
		t.Fatal("stored offer exposed a mutable slice")
	}
	if _, ok := c.DecideReceive(waiting.TransferID, "wrong-offer", true, now); ok {
		t.Fatal("accepted incorrect offer ID")
	}
	if _, ok := c.DecideReceive("wrong-transfer", offer.OfferID, true, now); ok {
		t.Fatal("accepted incorrect transfer ID")
	}
	accepted, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now)
	if !ok || accepted.State != AwaitingAcceptance || accepted.Revision != 3 {
		t.Fatalf("wrong consent state: %#v, %t", accepted, ok)
	}
	if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now); ok {
		t.Fatal("duplicate consent succeeded")
	}
	if _, ok := c.Advance(waiting.TransferID, Transferring); ok {
		t.Fatal("consent bypassed required native destination recheck")
	}
	canceled, ok := c.Cancel(waiting.TransferID)
	if !ok || canceled.State != Cancelling || canceled.Revision != 4 {
		t.Fatal("receiver cannot cancel after consent")
	}
	if _, ok := c.GetReceiveOffer(waiting.TransferID); ok {
		t.Fatal("offer not erased when cancelling")
	}
}

func TestReceiveGateRejectAndTimeout(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	c, waiting := newWaitingReceive(t)
	if _, _, err := c.PresentReceiveOffer(waiting.TransferID, []ManifestEntry{{Path: "CON"}}, now); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("invalid manifest should fail before consent: %v", err)
	}
	_, offer, err := c.PresentReceiveOffer(waiting.TransferID, []ManifestEntry{{Path: "file.txt"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now.Add(OfferDecisionTimeout)); ok {
		t.Fatal("expired consent succeeded")
	}
	rejected, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, false, now.Add(time.Second))
	if !ok || rejected.State != Cancelling {
		t.Fatal("reject must enter cancelling")
	}
	if _, ok := c.DecideReceive(waiting.TransferID, offer.OfferID, true, now); ok {
		t.Fatal("accepted after rejection")
	}
}

func TestSenderCannotPresentReceiveOffer(t *testing.T) {
	c := &Coordinator{}
	send, err := c.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Advance(send.TransferID, WaitingForPeer); !ok {
		t.Fatal("sender setup failed")
	}
	if _, _, err := c.PresentReceiveOffer(send.TransferID, []ManifestEntry{{Path: "file.txt"}}, time.Now()); !errors.Is(err, ErrBusy) {
		t.Fatalf("sender presented receive offer: %v", err)
	}
	if _, ok := c.Advance(send.TransferID, Transferring); !ok {
		t.Fatal("sender must retain normal transfer transition")
	}
}
