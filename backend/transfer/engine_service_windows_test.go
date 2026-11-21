//go:build windows && !croc_no_tailcat

package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
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

func TestPinWorkerExecutableLocksVerifiedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.exe")
	contents := []byte("pinned worker test")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	lease, err := pinWorkerExecutable(path, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatalf("pin exact worker bytes: %v", err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err == nil {
		_ = lease.Close()
		t.Fatal("worker changed while pinned")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatalf("worker remained locked after release: %v", err)
	}
}

func TestPinWorkerExecutableRejectsRelativePathAndHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.exe")
	if err := os.WriteFile(path, []byte("worker"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := pinWorkerExecutable("worker.exe", PinnedWorkerSHA256); !errors.Is(err, ErrWorkerPinMismatch) {
		t.Fatalf("relative path accepted: %v", err)
	}
	if _, err := pinWorkerExecutable(path, PinnedWorkerSHA256); !errors.Is(err, ErrWorkerPinMismatch) {
		t.Fatalf("wrong worker hash accepted: %v", err)
	}
}

func TestEngineServiceReportsOnlyPinnedWorkerAvailable(t *testing.T) {
	fakePath := filepath.Join(t.TempDir(), "worker.exe")
	if err := os.WriteFile(fakePath, []byte("fake worker"), 0600); err != nil {
		t.Fatal(err)
	}
	config := EngineServiceConfig{WorkerPath: fakePath, RelayAddress: "relay.example.test:9009", RelayPassword: "fixture"}
	if info := NewEngineService(config).GetCrocInfo(); info.Available {
		t.Fatal("unverified worker reported as available")
	}

	pinnedPath := filepath.Join("..", "engine-spike", "worker", "croccoc-engine-spike.exe")
	if _, err := os.Stat(pinnedPath); errors.Is(err, os.ErrNotExist) {
		t.Skip("pinned worker build is not present")
	}
	absolutePath, err := filepath.Abs(pinnedPath)
	if err != nil {
		t.Fatal(err)
	}
	config.WorkerPath = absolutePath
	info := NewEngineService(config).GetCrocInfo()
	if !info.Available || info.Version != PinnedCrocVersion {
		t.Fatalf("verified worker rejected: %+v", info)
	}
}
