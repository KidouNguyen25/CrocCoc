//go:build windows

package transfer

import (
	"errors"
	"testing"
)

func receiveServiceFixture(t *testing.T) (*engineService, string, *[]Update) {
	t.Helper()
	updates := new([]Update)
	service := NewEngineService(EngineServiceConfig{
		EventSink: func(update Update) { *updates = append(*updates, update) },
	}).(*engineService)
	started, err := service.coordinator.BeginReceive()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer); !ok {
		t.Fatal("failed to enter waiting state")
	}
	service.sessions[started.TransferID] = &engineSession{id: started.TransferID}
	return service, started.TransferID, updates
}

func TestReceiveFailureWithOwnedStageKeepsServiceBusy(t *testing.T) {
	service, id, updates := receiveServiceFixture(t)
	service.finishReceiveSession(id, ErrTransferFailed, true, true)

	snapshot, ok := service.GetTransferSnapshot(id)
	if !ok || snapshot.State != Failed || !snapshot.PartialOutput || !snapshot.CleanupPending {
		t.Fatalf("terminal snapshot lost recovery state: %+v", snapshot)
	}
	if _, err := service.coordinator.Begin(); !errors.Is(err, ErrBusy) {
		t.Fatalf("new session admitted while staging needs recovery: %v", err)
	}
	if len(*updates) != 1 || !(*updates)[0].CleanupPending || !(*updates)[0].PartialOutput {
		t.Fatalf("terminal update lost recovery flags: %+v", *updates)
	}
}

func TestReceiveFailureWithoutOwnedStageReleasesService(t *testing.T) {
	service, id, _ := receiveServiceFixture(t)
	service.finishReceiveSession(id, ErrTransferFailed, false, false)

	if _, err := service.coordinator.Begin(); err != nil {
		t.Fatalf("clean terminal session kept coordinator busy: %v", err)
	}
}
