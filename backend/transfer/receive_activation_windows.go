//go:build windows

package transfer

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

var errReceiveOfferInactive = errors.New("receive offer is not active and accepted")

// nativeReceiveOffer owns the destination lease from preview through native
// activation. Only the backend should hold this value, never the UI or engine.
type nativeReceiveOffer struct {
	mu                  sync.Mutex
	coordinator         *Coordinator
	transferID, offerID string
	lease               *DestinationLease
	entries             []ManifestEntry
	digest              [32]byte
	attempted           bool
}

// presentNativeReceiveOffer verifies the destination without creating staging
// or writing anything before user consent. It freezes a canonical copy of the
// manifest so a caller cannot change paths or byte counts after review.
func presentNativeReceiveOffer(c *Coordinator, id, destination string, entries []ManifestEntry) (*nativeReceiveOffer, SessionSnapshot, ReceiveOffer, error) {
	if c == nil {
		return nil, SessionSnapshot{}, ReceiveOffer{}, ErrBusy
	}
	frozen, err := freezeNativeManifest(entries)
	if err != nil {
		return nil, SessionSnapshot{}, ReceiveOffer{}, err
	}
	lease, err := PreflightReceiveDestination(destination, frozen)
	if err != nil {
		return nil, SessionSnapshot{}, ReceiveOffer{}, err
	}
	snapshot, offer, err := c.PresentReceiveOffer(id, frozen, time.Now())
	if err != nil {
		_ = lease.Close()
		return nil, SessionSnapshot{}, ReceiveOffer{}, err
	}
	return &nativeReceiveOffer{
		coordinator: c, lease: lease, transferID: id, offerID: offer.OfferID,
		entries: frozen, digest: fingerprintManifest(frozen),
	}, snapshot, offer, nil
}

func freezeNativeManifest(entries []ManifestEntry) ([]ManifestEntry, error) {
	if _, err := ValidateManifest(entries); err != nil {
		return nil, err
	}
	frozen := slices.Clone(entries)
	dirs := make(map[string]struct{}, len(frozen))
	for i := range frozen {
		frozen[i].Path = strings.ReplaceAll(frozen[i].Path, "\\", "/")
		if frozen[i].Directory {
			dirs[frozen[i].Path] = struct{}{}
		}
	}
	for _, entry := range frozen {
		if i := strings.LastIndexByte(entry.Path, '/'); i >= 0 {
			if _, declared := dirs[entry.Path[:i]]; !declared {
				return nil, ErrUnsafeOffer
			}
		}
	}
	sort.Slice(frozen, func(i, j int) bool { return frozen[i].Path < frozen[j].Path })
	return frozen, nil
}

// activate returns a writable stage only after consent, durable preparation
// and an atomic coordinator transition. The caller owns the returned stage
// and must keep this offer's destination lease open through finalization.
// This does not itself send a decision or start the engine.
func (p *nativeReceiveOffer) activate(ctx context.Context) (*ReceiveStage, SessionSnapshot, error) {
	return p.activateWith(ctx, time.Now, PrepareReceiveStage)
}

func (p *nativeReceiveOffer) activateWith(ctx context.Context, now func() time.Time, prepare func(*DestinationLease, []ManifestEntry) (*ReceiveStage, error)) (*ReceiveStage, SessionSnapshot, error) {
	if p == nil || ctx == nil {
		return nil, SessionSnapshot{}, errReceiveOfferInactive
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, SessionSnapshot{}, err
	}
	if p.lease == nil || p.attempted ||
		!p.coordinator.hasAcceptedReceiveOffer(p.transferID, p.offerID, p.digest, now()) {
		return nil, SessionSnapshot{}, errReceiveOfferInactive
	}
	p.attempted = true

	// PrepareReceiveStage checks the leased identity, free space and names
	// again. No coordinator lock is held during filesystem operations.
	stage, err := prepare(p.lease, p.entries)
	if err != nil {
		return nil, SessionSnapshot{}, err
	}
	activated := false
	defer func() {
		if !activated {
			// Keep the owned, journaled directory for verified recovery.
			// Closing handles alone never implies that deleting it is safe.
			_ = stage.Close()
		}
	}()
	if err := stage.prepareStageDirectories(p.entries); err != nil {
		return nil, SessionSnapshot{}, err
	}
	if err := p.lease.VerifyCurrentPath(); err != nil {
		return nil, SessionSnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, SessionSnapshot{}, err
	}
	permit := stage.receivePermit(p.transferID, p.offerID)
	snapshot, ok := p.coordinator.CommitReceivePreflight(p.transferID, p.offerID, permit, now())
	if !ok {
		return nil, SessionSnapshot{}, errReceiveOfferInactive
	}
	activated = true
	return stage, snapshot, nil
}

func (p *nativeReceiveOffer) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lease == nil {
		return nil
	}
	lease := p.lease
	p.lease = nil
	return lease.Close()
}
