package transfer

import "time"

// PresentReceiveOffer records validated, peer-supplied metadata in the
// receiver's WAITING_FOR_PEER session. Callers must also complete native
// destination preflight before presenting the offer. This method cannot
// start payload transfer or bypass user consent.
func (c *Coordinator) PresentReceiveOffer(
	id string, entries []ManifestEntry, now time.Time,
) (SessionSnapshot, ReceiveOffer, error) {
	offer, err := PrepareReceiveOffer(entries, now)
	if err != nil {
		return SessionSnapshot{}, ReceiveOffer{}, err
	}
	return c.presentReceiveOffer(id, entries, offer, now)
}

// PresentReceiveOfferWithID binds a validated engine offer to the coordinator
// without replacing its opaque identifier. The ID must come from the private
// worker frame and is never accepted from UI input.
func (c *Coordinator) PresentReceiveOfferWithID(
	id string, entries []ManifestEntry, offerID string, now time.Time,
) (SessionSnapshot, ReceiveOffer, error) {
	if offerID == "" {
		return SessionSnapshot{}, ReceiveOffer{}, ErrUnsafeOffer
	}
	offer, err := PrepareReceiveOffer(entries, now)
	if err != nil {
		return SessionSnapshot{}, ReceiveOffer{}, err
	}
	offer.OfferID = offerID
	return c.presentReceiveOffer(id, entries, offer, now)
}

func (c *Coordinator) presentReceiveOffer(
	id string, entries []ManifestEntry, offer ReceiveOffer, now time.Time,
) (SessionSnapshot, ReceiveOffer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		!c.receiving || c.active.State != WaitingForPeer {
		return SessionSnapshot{}, ReceiveOffer{}, ErrBusy
	}
	copyOffer := offer.Clone()
	c.offer = &copyOffer
	c.decided = false
	c.manifestDigest = fingerprintManifest(entries)
	c.active.State = AwaitingAcceptance
	c.active.Revision++
	return *c.active, offer.Clone(), nil
}

// GetReceiveOffer returns defensive copies of the pending metadata, without
// exposing source paths or retaining any mutable slice owned by a caller.
func (c *Coordinator) GetReceiveOffer(id string) (ReceiveOffer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		c.active.State != AwaitingAcceptance || c.offer == nil {
		return ReceiveOffer{}, false
	}
	return c.offer.Clone(), true
}

// ExpireReceiveOffer closes a receiver offer after its decision deadline.
// It wins the same cancellation gate as an explicit rejection and never
// starts payload processing.
func (c *Coordinator) ExpireReceiveOffer(id string, now time.Time) (SessionSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		!c.receiving || c.active.State != AwaitingAcceptance ||
		c.offer == nil || !c.offer.Expired(now) {
		return SessionSnapshot{}, false
	}
	c.active.State = Cancelling
	c.active.Revision++
	c.offer = nil
	c.decided = false
	c.manifestDigest = [32]byte{}
	return *c.active, true
}

// DecideReceive consumes an offer at most once. Accepting is only a recorded
// user decision, NOT an authorization to fetch payload: the backend must
// re-run native destination preflight and win the cancellation gate before
// moving to TRANSFERRING via CommitReceivePreflight.
// Rejecting enters CANCELLING and requires separate engine cleanup.
func (c *Coordinator) DecideReceive(
	id, offerID string, accept bool, now time.Time,
) (SessionSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		!c.receiving || c.active.State != AwaitingAcceptance ||
		c.offer == nil || c.offer.OfferID != offerID ||
		c.offer.Expired(now) || c.decided {
		return SessionSnapshot{}, false
	}
	c.decided = true
	if !accept {
		c.active.State = Cancelling
		c.active.Revision++
		c.offer = nil
		c.decided = false
		c.manifestDigest = [32]byte{}
	}
	return *c.active, true
}
