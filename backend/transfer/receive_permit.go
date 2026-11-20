package transfer

import (
	"crypto/sha256"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ReceivePermit binds one native stage to the exact frozen offer and session.
// Its fields are intentionally private: Wails/TypeScript cannot fabricate
// proof of local preflight or advance a receiver to TRANSFERRING.
type ReceivePermit struct {
	transferID string
	offerID    string
	digest     [32]byte
	stageID    string
}

func fingerprintManifest(entries []ManifestEntry) [32]byte {
	canonical := make([]ManifestEntry, len(entries))
	copy(canonical, entries)
	for i := range canonical {
		canonical[i].Path = strings.ReplaceAll(canonical[i].Path, "\\", "/")
	}
	sort.Slice(canonical, func(i, j int) bool {
		return strings.ToUpper(canonical[i].Path) < strings.ToUpper(canonical[j].Path)
	})
	data, _ := json.Marshal(canonical)
	return sha256.Sum256(data)
}

// CommitReceivePreflight is the only receiver-specific transition to
// TRANSFERRING. It atomically checks current user consent, the offerID,
// the stage's immutable manifest fingerprint, and the cancellation gate.
// It still does NOT perform I/O or start an engine. Callers must ensure
// the permit was produced by a durable native staging writer.
func (c *Coordinator) CommitReceivePreflight(
	transferID, offerID string, permit ReceivePermit, now time.Time,
) (SessionSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != transferID ||
		c.active.State != AwaitingAcceptance || !c.receiving ||
		!c.decided || c.offer == nil || c.offer.OfferID != offerID ||
		c.offer.Expired(now) || permit.transferID != transferID ||
		permit.offerID != offerID || permit.stageID == "" ||
		permit.digest != c.manifestDigest {
		return SessionSnapshot{}, false
	}
	c.active.State = Transferring
	c.active.Revision++
	c.offer = nil
	c.decided = false
	c.manifestDigest = [32]byte{}
	return *c.active, true
}
