//go:build windows

package transfer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func stageActivationFixture(t *testing.T, entries []ManifestEntry) (*Coordinator, *nativeReceiveOffer, ReceiveOffer, string) {
	t.Helper()
	c, waiting := newWaitingReceive(t)
	destination := t.TempDir()
	pending, snapshot, offer, err := presentNativeReceiveOffer(c, waiting.TransferID, destination, entries)
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("requires fixed local NTFS")
	}
	if err != nil || snapshot.State != AwaitingAcceptance {
		t.Fatalf("present offer: %+v, %v", snapshot, err)
	}
	t.Cleanup(func() { _ = pending.Close() })
	return c, pending, offer, destination
}

func TestNativeReceiveActivationStagesOnlyAcceptedFrozenEntries(t *testing.T) {
	// The worker may give children first. Preflight canonicalizes and freezes
	// the paths, then stages parents before their children.
	entries := []ManifestEntry{
		{Path: "Album\\ảnh.txt", SizeBytes: 4},
		{Path: "Album", Directory: true},
		{Path: "Album/Empty", Directory: true},
	}
	c, pending, offer, destination := stageActivationFixture(t, entries)
	ctx := t.Context()
	journal := filepath.Join(t.TempDir(), "journal")
	prepares := 0
	prepare := func(lease *DestinationLease, frozen []ManifestEntry) (*ReceiveStage, error) {
		prepares++
		return prepareReceiveStageAt(lease, frozen, journal)
	}
	if _, _, err := pending.activateWith(ctx, time.Now, prepare); !errors.Is(err, errReceiveOfferInactive) || prepares != 0 {
		t.Fatalf("unaccepted offer reached staging: %v, calls %d", err, prepares)
	}
	if items, err := os.ReadDir(destination); err != nil || len(items) != 0 {
		t.Fatal("preview or unaccepted activation changed the destination")
	}
	entries[0] = ManifestEntry{Path: "not-approved.txt", SizeBytes: 5}
	if _, ok := c.DecideReceive(pending.transferID, "wrong offer", true, time.Now()); ok {
		t.Fatal("stale offer accepted")
	}
	if _, ok := c.DecideReceive(pending.transferID, offer.OfferID, true, time.Now()); !ok {
		t.Fatal("valid consent rejected")
	}
	stage, snapshot, err := pending.activateWith(ctx, time.Now, prepare)
	if err != nil || snapshot.State != Transferring || prepares != 1 {
		t.Fatalf("activate: %+v, %v, calls %d", snapshot, err, prepares)
	}
	defer stage.Close()
	if _, err := os.Stat(stage.JournalPath()); err != nil {
		t.Fatalf("stage has no durable journal: %v", err)
	}
	if err := stage.writeStageChunks(ctx, entries[0], func(io.WriterAt) error {
		t.Fatal("unapproved path received a writer")
		return nil
	}); !errors.Is(err, ErrUnsafeOffer) {
		t.Fatalf("unapproved write: %v", err)
	}
	file := ManifestEntry{Path: "Album/ảnh.txt", SizeBytes: 4}
	if err := stage.writeStageChunks(ctx, file, func(out io.WriterAt) error {
		_, err := out.WriteAt([]byte("test"), 0)
		return err
	}); err != nil {
		t.Fatalf("approved stage write: %v", err)
	}
	payload := filepath.Join(destination, stage.StageName, "payload", "Album")
	if bytes, err := os.ReadFile(filepath.Join(payload, "ảnh.txt")); err != nil || string(bytes) != "test" {
		t.Fatalf("staged bytes: %q, %v", bytes, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "Album")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("activation exposed the final destination: %v", err)
	}
	if _, _, err := pending.activateWith(ctx, time.Now, prepare); !errors.Is(err, errReceiveOfferInactive) || prepares != 1 {
		t.Fatalf("repeated activation created staging: %v, calls %d", err, prepares)
	}
}

func TestNativeReceiveActivationRejectsStaleConditions(t *testing.T) {
	for _, tc := range []string{"rejected", "expired", "destination conflict", "cancel during prepare", "expire during prepare"} {
		t.Run(tc, func(t *testing.T) {
			entries := []ManifestEntry{{Path: "fixture.txt", SizeBytes: 4}}
			c, pending, offer, destination := stageActivationFixture(t, entries)
			if _, ok := c.DecideReceive(pending.transferID, offer.OfferID, tc != "rejected", time.Now()); !ok {
				t.Fatal("failed to record test decision")
			}
			if tc == "destination conflict" {
				if err := os.WriteFile(filepath.Join(destination, "fixture.txt"), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			journal := filepath.Join(t.TempDir(), "journal")
			var prepared *ReceiveStage
			var prepares int
			clock := time.Now()
			if tc == "expired" {
				clock = offer.ExpiresAt
			}
			stage, _, err := pending.activateWith(t.Context(), func() time.Time { return clock }, func(lease *DestinationLease, frozen []ManifestEntry) (*ReceiveStage, error) {
				prepares++
				var err error
				prepared, err = prepareReceiveStageAt(lease, frozen, journal)
				if err == nil && tc == "cancel during prepare" {
					if _, ok := c.Cancel(pending.transferID); !ok {
						t.Fatal("cancel did not win before activation")
					}
				}
				if tc == "expire during prepare" {
					clock = offer.ExpiresAt
				}
				return prepared, err
			})
			if err == nil || stage != nil {
				t.Fatal("stale conditions granted a stage")
			}
			got, ok := c.Snapshot(pending.transferID)
			if !ok || got.State == Transferring {
				t.Fatalf("unexpected coordinator state: %+v, %t", got, ok)
			}
			if (tc == "rejected" || tc == "expired") && prepares != 0 {
				t.Fatal("staging attempted before admission")
			}
			if tc == "destination conflict" {
				if data, err := os.ReadFile(filepath.Join(destination, "fixture.txt")); err != nil || string(data) != "preserve" {
					t.Fatal("existing destination was modified")
				}
			}
			if tc == "cancel during prepare" || tc == "expire during prepare" {
				if prepared == nil || prepared.receivePermit(pending.transferID, offer.OfferID).stageID != "" {
					t.Fatal("aborted stage retained handles or never reached native preparation")
				}
				if _, err := os.Stat(prepared.JournalPath()); err != nil {
					t.Fatalf("lost ownership journal on abort: %v", err)
				}
			}
		})
	}
}

func TestNativeReceiveActivationRejectsMissingOrMismatchedParent(t *testing.T) {
	for _, entries := range [][]ManifestEntry{
		{{Path: "Album/ảnh.txt", SizeBytes: 4}},
		{{Path: "Album", Directory: true}, {Path: "album/ảnh.txt", SizeBytes: 4}},
	} {
		if _, err := freezeNativeManifest(entries); !errors.Is(err, ErrUnsafeOffer) {
			t.Fatalf("invalid explicit parent accepted: %v", err)
		}
	}
}

func TestNativeReceiveActivationHonorsCancelledContext(t *testing.T) {
	c, pending, offer, _ := stageActivationFixture(t, []ManifestEntry{{Path: "fixture.txt", SizeBytes: 4}})
	if _, ok := c.DecideReceive(pending.transferID, offer.OfferID, true, time.Now()); !ok {
		t.Fatal("valid consent rejected")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if stage, _, err := pending.activate(ctx); stage != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled activation: %v, %v", stage, err)
	}
}
