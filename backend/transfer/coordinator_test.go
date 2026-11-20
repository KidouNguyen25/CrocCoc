package transfer

import (
	"errors"
	"sync"
	"testing"
)

func TestCoordinatorLifecycleAndSnapshot(t *testing.T) {
	var c Coordinator
	start, err := c.Begin()
	if err != nil || start.TransferID == "" || start.Revision != 1 || start.State != Preparing {
		t.Fatalf("unexpected start: %#v, %v", start, err)
	}
	if _, err := c.Begin(); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected BUSY, got %v", err)
	}
	if c.Release(start.TransferID) {
		t.Fatal("released a live session before terminal cleanup")
	}
	if _, ok := c.Advance("wrong-id", Failed); ok {
		t.Fatal("accepted an unknown session ID")
	}
	waiting, ok := c.Advance(start.TransferID, WaitingForPeer)
	if !ok || waiting.State != WaitingForPeer || waiting.Revision != 2 {
		t.Fatalf("unexpected waiting state: %#v, %t", waiting, ok)
	}
	cancelled, ok := c.Cancel(start.TransferID)
	if !ok || cancelled.State != Cancelling || cancelled.Revision != 3 {
		t.Fatalf("cancel did not win: %#v, %t", cancelled, ok)
	}
	if _, ok := c.Cancel(start.TransferID); ok {
		t.Fatal("accepted a second cancel")
	}
	if _, ok := c.Advance(start.TransferID, Finalizing); ok {
		t.Fatal("published after cancellation won")
	}
	end, ok := c.Advance(start.TransferID, Cancelled)
	if !ok || end.Revision != 4 {
		t.Fatalf("unexpected terminal state: %#v, %t", end, ok)
	}
	if _, err := c.Begin(); !errors.Is(err, ErrBusy) {
		t.Fatalf("cleanup gate released prematurely: %v", err)
	}
	if !c.Release(start.TransferID) || c.Release(start.TransferID) {
		t.Fatal("terminal session should release exactly once")
	}
	saved, ok := c.Snapshot(start.TransferID)
	if !ok || saved != end {
		t.Fatalf("terminal snapshot changed: %#v, %t", saved, ok)
	}
	next, err := c.Begin()
	if err != nil || next.TransferID == start.TransferID || next.Revision != 1 {
		t.Fatalf("new session invalid: %#v, %v", next, err)
	}
	if _, ok := c.Snapshot(start.TransferID); ok {
		t.Fatal("old terminal snapshot retained after new session")
	}
}

func TestCoordinatorConcurrentBegin(t *testing.T) {
	var c Coordinator
	const callers = 32
	var wg sync.WaitGroup
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Begin()
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, busy := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrBusy):
			busy++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if success != 1 || busy != callers-1 {
		t.Fatalf("concurrent starts: success=%d busy=%d", success, busy)
	}
}

func TestCoordinatorCancelVsFinalizeRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		var c Coordinator
		start, err := c.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := c.Advance(start.TransferID, WaitingForPeer); !ok {
			t.Fatal("could not wait")
		}
		if _, ok := c.Advance(start.TransferID, Transferring); !ok {
			t.Fatal("could not transfer")
		}

		begin := make(chan struct{})
		results := make(chan bool, 2)
		go func() {
			<-begin
			_, ok := c.Cancel(start.TransferID)
			results <- ok
		}()
		go func() {
			<-begin
			_, ok := c.Advance(start.TransferID, Finalizing)
			results <- ok
		}()
		close(begin)
		first, second := <-results, <-results
		if first == second {
			t.Fatalf("exactly one race winner required: %t, %t", first, second)
		}
		snapshot, ok := c.Snapshot(start.TransferID)
		if !ok || snapshot.Revision != 4 ||
			(snapshot.State != Cancelling && snapshot.State != Finalizing) {
			t.Fatalf("race corruption: %#v, %t", snapshot, ok)
		}
	}
}
