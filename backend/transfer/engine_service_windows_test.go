//go:build windows && !croc_no_tailcat

package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSendTransferStartAdvancesCoordinatorOnce(t *testing.T) {
	updates := new([]Update)
	service := &engineService{
		cfg:      EngineServiceConfig{EventSink: func(update Update) { *updates = append(*updates, update) }},
		sessions: make(map[string]*engineSession),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting state")
	}
	id := waiting.TransferID
	service.markSendTransferring(id)
	service.markSendTransferring(id)

	snapshot, ok := service.GetTransferSnapshot(id)
	if !ok || snapshot.State != Transferring || snapshot.Revision != 3 {
		t.Fatalf("send did not enter transferring exactly once: %+v", snapshot)
	}
	if len(*updates) != 1 || (*updates)[0].State != Transferring {
		t.Fatalf("send emitted wrong transfer updates: %+v", *updates)
	}
}

func TestWorkerCleanupPendingKeepsCoordinatorBusy(t *testing.T) {
	service, id, updates := receiveServiceFixture(t)
	service.finishReceiveSession(id, ErrWorkerCleanupPending, false, true)
	snapshot, ok := service.GetTransferSnapshot(id)
	if !ok || snapshot.State != Failed || !snapshot.CleanupPending {
		t.Fatalf("worker cleanup state lost: %+v", snapshot)
	}
	if _, err := service.coordinator.Begin(); !errors.Is(err, ErrBusy) {
		t.Fatalf("new session admitted before worker cleanup: %v", err)
	}
	if len(*updates) != 1 || (*updates)[0].State != Failed || !(*updates)[0].CleanupPending {
		t.Fatalf("cleanup failure emitted wrong terminal update: %+v", *updates)
	}
}

func TestRecoveryPendingKeepsEngineUnavailable(t *testing.T) {
	service := &engineService{recoveryErr: ErrRecoveryPending}
	if info := service.GetCrocInfo(); info.Available || info.Message != ErrRecoveryPending.Error() {
		t.Fatalf("recovery failure did not disable engine: %+v", info)
	}
	if _, err := service.StartSend([]string{"source.txt"}); !errors.Is(err, ErrRecoveryPending) {
		t.Fatalf("send ignored pending recovery: %v", err)
	}
	if _, err := service.StartReceive("code", "output"); !errors.Is(err, ErrRecoveryPending) {
		t.Fatalf("receive ignored pending recovery: %v", err)
	}
}

func TestGetCrocInfoIncludesRelayLabelWithoutPassword(t *testing.T) {
	service := &engineService{
		cfg: EngineServiceConfig{
			RelayAddress:  "relay.example.test:9009",
			RelayPassword: "private-password",
			RelayLabel:    "Relay tùy chỉnh",
		},
		worker:    &WorkerClient{path: "worker.exe"},
		workerPin: &SendSourceLease{},
	}

	info := service.GetCrocInfo()
	if !info.Available || info.RelayLabel != "Relay tùy chỉnh" {
		t.Fatalf("GetCrocInfo() = %+v, want available custom relay label", info)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-password") || strings.Contains(string(encoded), "relayPassword") {
		t.Fatalf("GetCrocInfo exposed relay credentials: %s", encoded)
	}
}

func TestRelayConfigErrorDisablesEngineAndTransferStarts(t *testing.T) {
	configErr := errors.New("ENGINE_UNAVAILABLE: cần đặt cả hai biến relay")
	service := &engineService{cfg: EngineServiceConfig{RelayConfigErr: configErr}}

	if info := service.GetCrocInfo(); info.Available || info.Message != configErr.Error() {
		t.Fatalf("GetCrocInfo() = %+v, want relay configuration error", info)
	}
	if _, err := service.StartSend([]string{"source.txt"}); !errors.Is(err, configErr) {
		t.Fatalf("StartSend() error = %v, want relay configuration error", err)
	}
	if _, err := service.StartReceive("code", "output"); !errors.Is(err, configErr) {
		t.Fatalf("StartReceive() error = %v, want relay configuration error", err)
	}
}

func TestRelayConnectionFailureHasActionableMessage(t *testing.T) {
	err := workerStatusError("metadata_failed")
	if !errors.Is(err, ErrRelayUnavailable) ||
		!strings.Contains(err.Error(), "không tới được máy chủ relay") ||
		!strings.Contains(err.Error(), "9009–9013") {
		t.Fatalf("relay connection error = %v, want network and port guidance", err)
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

func TestCancelTransferEmitsCancellingAndRecordProgressAdvancesRevision(t *testing.T) {
	updates := new([]Update)
	service := &engineService{
		cfg:      EngineServiceConfig{EventSink: func(update Update) { *updates = append(*updates, update) }},
		sessions: make(map[string]*engineSession),
		sendCode: make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting")
	}
	id := waiting.TransferID
	cancelled := false
	service.sessions[id] = &engineSession{
		id:         id,
		cancel:     func() { cancelled = true },
		bytesTotal: 1000,
	}
	service.markSendTransferring(id)
	service.recordProgress(id, 400)
	if len(*updates) != 2 {
		t.Fatalf("expected transferring + progress updates, got %d: %+v", len(*updates), *updates)
	}
	progressUpdate := (*updates)[1]
	if progressUpdate.Revision <= (*updates)[0].Revision || progressUpdate.BytesTransferred != "400" || progressUpdate.BytesTotal != "1000" {
		t.Fatalf("unexpected progress update: %+v", progressUpdate)
	}
	okCancel, err := service.CancelTransfer(id)
	if !okCancel || err != nil || !cancelled {
		t.Fatalf("cancel failed: ok=%t err=%v cancelled=%t", okCancel, err, cancelled)
	}
	last := (*updates)[len(*updates)-1]
	if last.State != Cancelling || last.ConnectionCode != "" {
		t.Fatalf("expected immediate CANCELLING without code, got %+v", last)
	}
}

func TestShutdownWaitsForActiveSessionBeforeReleasingWorkerPin(t *testing.T) {
	workerPath := filepath.Join(t.TempDir(), "worker.exe")
	contents := []byte("shutdown pin fixture")
	if err := os.WriteFile(workerPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	pin, err := pinWorkerExecutable(workerPath, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	service := &engineService{
		workerPin:       pin,
		shutdownTimeout: 2 * time.Second,
		sessions:        make(map[string]*engineSession),
		sendCode:        make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting")
	}
	done := make(chan struct{})
	cancelObserved := make(chan struct{})
	service.sessions[waiting.TransferID] = &engineSession{
		id:   waiting.TransferID,
		done: done,
		cancel: func() {
			close(cancelObserved)
			go func() {
				time.Sleep(60 * time.Millisecond)
				close(done)
			}()
		},
	}

	start := time.Now()
	service.Shutdown()
	elapsed := time.Since(start)
	select {
	case <-cancelObserved:
	default:
		t.Fatal("Shutdown did not cancel active session")
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("Shutdown returned before session finished cleanup: %v", elapsed)
	}
	if err := os.WriteFile(workerPath, []byte("unlocked"), 0600); err != nil {
		t.Fatalf("worker pin remained locked after clean shutdown: %v", err)
	}
}

func TestShutdownKeepsWorkerPinLockedWhenSessionTimesOut(t *testing.T) {
	workerPath := filepath.Join(t.TempDir(), "worker.exe")
	contents := []byte("stuck shutdown pin fixture")
	if err := os.WriteFile(workerPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	pin, err := pinWorkerExecutable(workerPath, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()

	service := &engineService{
		workerPin:       pin,
		shutdownTimeout: 40 * time.Millisecond,
		sessions:        make(map[string]*engineSession),
		sendCode:        make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting")
	}
	service.sessions[waiting.TransferID] = &engineSession{
		id:     waiting.TransferID,
		done:   make(chan struct{}), // never closes
		cancel: func() {},
	}
	service.Shutdown()
	if err := os.WriteFile(workerPath, []byte("should-stay-locked"), 0600); err == nil {
		t.Fatal("worker pin was unlocked even though session did not finish")
	}
}

func TestGetTransferUpdateExposesCodeOnlyWhileWaitingForPeer(t *testing.T) {
	service := &engineService{
		sessions: make(map[string]*engineSession),
		sendCode: make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting")
	}
	service.sendCode[waiting.TransferID] = "secret-code-xyz"
	u, ok := service.GetTransferUpdate(waiting.TransferID)
	if !ok || u.ConnectionCode != "secret-code-xyz" || u.State != WaitingForPeer {
		t.Fatalf("expected code during WAITING_FOR_PEER, got %+v", u)
	}
	service.markSendTransferring(waiting.TransferID)
	u2, ok := service.GetTransferUpdate(waiting.TransferID)
	if !ok || u2.ConnectionCode != "" || u2.State != Transferring {
		t.Fatalf("code must not appear outside WAITING_FOR_PEER, got %+v", u2)
	}
}

func TestOfferTimeoutTransitionsToFailedWithOfferTimeoutCode(t *testing.T) {
	service, id, updates := receiveServiceFixture(t)
	if _, ok := service.coordinator.Cancel(id); !ok {
		t.Fatal("failed to enter cancelling")
	}
	service.finishReceiveSession(id, ErrOfferTimeout, false, false)
	if len(*updates) != 1 || (*updates)[0].State != Failed || (*updates)[0].Error == nil || (*updates)[0].Error.Code != "OFFER_TIMEOUT" {
		t.Fatalf("expected FAILED with OFFER_TIMEOUT, got %+v", *updates)
	}
}

func TestGetTransferUpdateRestoresMissedFailedErrorCCT13A(t *testing.T) {
	service, id, _ := receiveServiceFixture(t)
	service.finishReceiveSession(id, ErrOfferTimeout, false, false)

	update, ok := service.GetTransferUpdate(id)
	if !ok || update.State != Failed || update.Error == nil ||
		update.Error.Code != "OFFER_TIMEOUT" || update.Error.Message != transferError(ErrOfferTimeout).Message {
		t.Fatalf("snapshot did not restore FAILED error after missed event: %+v, %t", update, ok)
	}
}

func TestFinalizePublishFailurePreservesErrorInEventAndSnapshot(t *testing.T) {
	entries := []ManifestEntry{{Path: "received.txt", SizeBytes: 2}}
	lease, stage, destination := publishFixture(t, entries)
	if err := os.WriteFile(filepath.Join(destination, "received.txt"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	updates := new([]Update)
	service := &engineService{
		cfg: EngineServiceConfig{EventSink: func(update Update) { *updates = append(*updates, update) }},
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []State{WaitingForPeer, Transferring} {
		if _, ok := service.coordinator.Advance(started.TransferID, state); !ok {
			t.Fatalf("failed to enter %s", state)
		}
	}

	failed, result, err := FinalizeNativeReceive(&service.coordinator, started.TransferID, lease, stage, entries)
	if !errors.Is(err, ErrPublishFailed) || failed.State != Failed || failed.Error == nil || failed.Error.Code != "PUBLISH_FAILED" {
		t.Fatalf("publish failure was not stored in coordinator: snapshot=%+v err=%v", failed, err)
	}
	service.finishReceiveSession(started.TransferID, err, result.Partial, true)
	wantMessage := transferError(ErrPublishFailed).Message
	if len(*updates) != 1 || (*updates)[0].State != Failed || (*updates)[0].Error == nil ||
		(*updates)[0].Error.Code != "PUBLISH_FAILED" || (*updates)[0].Error.Message != wantMessage {
		t.Fatalf("FAILED event lost PUBLISH_FAILED: %+v", *updates)
	}
	snapshot, ok := service.GetTransferSnapshot(started.TransferID)
	if !ok || snapshot.State != Failed || snapshot.Error == nil || snapshot.Error.Code != "PUBLISH_FAILED" || snapshot.Error.Message != wantMessage {
		t.Fatalf("transfer snapshot lost PUBLISH_FAILED: %+v, %t", snapshot, ok)
	}
	update, ok := service.GetTransferUpdate(started.TransferID)
	if !ok || update.State != Failed || update.Error == nil || update.Error.Code != "PUBLISH_FAILED" || update.Error.Message != wantMessage {
		t.Fatalf("snapshot lost PUBLISH_FAILED: %+v, %t", update, ok)
	}
}

func pinnedEngineServiceFixture(t *testing.T, shutdownTimeout time.Duration) (*engineService, string) {
	t.Helper()
	workerPath := filepath.Join(t.TempDir(), "worker.exe")
	contents := []byte("preflight shutdown pin fixture")
	if err := os.WriteFile(workerPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	pin, err := pinWorkerExecutable(workerPath, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pin.Close() })
	return &engineService{
		cfg: EngineServiceConfig{
			WorkerPath: workerPath, RelayAddress: "relay.example.test:9009", RelayPassword: "fixture",
		},
		worker:          NewWorkerClient(workerPath),
		workerPin:       pin,
		shutdownTimeout: shutdownTimeout,
		sessions:        make(map[string]*engineSession),
		sendCode:        make(map[string]string),
	}, workerPath
}

func TestShutdownCancelsAndDrainsAdmittedPreflight(t *testing.T) {
	service, workerPath := pinnedEngineServiceFixture(t, time.Second)
	preflightCtx, preflightCancel, err := service.beginPreflight()
	if err != nil {
		t.Fatal(err)
	}
	preflightFinished := false
	defer func() {
		if !preflightFinished {
			preflightCancel()
			service.endPreflight()
		}
	}()

	shutdownDone := make(chan struct{})
	go func() {
		service.Shutdown()
		close(shutdownDone)
	}()
	select {
	case <-preflightCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not cancel the admitted preflight")
	}
	for name, start := range map[string]func() error{
		"send": func() error { _, err := service.StartSend([]string{"source.txt"}); return err },
		"receive": func() error {
			_, err := service.StartReceive("code", filepath.Dir(workerPath))
			return err
		},
	} {
		if err := start(); !errors.Is(err, ErrWorkerUnavailable) {
			t.Errorf("Start%s after shutdown = %v, want ErrWorkerUnavailable", strings.ToUpper(name[:1])+name[1:], err)
		}
	}
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned before the admitted preflight drained")
	case <-time.After(40 * time.Millisecond):
	}
	if err := os.WriteFile(workerPath, []byte("still pinned"), 0600); err == nil {
		t.Fatal("worker pin was released before preflight drained")
	}

	preflightCancel()
	service.endPreflight()
	preflightFinished = true
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not finish after the preflight drained")
	}
	if err := os.WriteFile(workerPath, []byte("released"), 0600); err != nil {
		t.Fatalf("worker pin remained locked after clean drain: %v", err)
	}
}

func TestShutdownPreflightDrainTimeoutKeepsWorkerPin(t *testing.T) {
	service, workerPath := pinnedEngineServiceFixture(t, 30*time.Millisecond)
	preflightCtx, preflightCancel, err := service.beginPreflight()
	if err != nil {
		t.Fatal(err)
	}
	preflightFinished := false
	defer func() {
		if !preflightFinished {
			preflightCancel()
			service.endPreflight()
		}
	}()

	service.Shutdown()
	select {
	case <-preflightCtx.Done():
	default:
		t.Fatal("Shutdown did not cancel the preflight context")
	}
	if err := os.WriteFile(workerPath, []byte("must stay pinned"), 0600); err == nil {
		t.Fatal("worker pin was released while preflight drain timed out")
	}
	preflightCancel()
	service.endPreflight()
	preflightFinished = true
}

func TestConcurrentShutdownCallersWaitForSharedCompletion(t *testing.T) {
	service := &engineService{
		shutdownTimeout: time.Second,
		sessions:        make(map[string]*engineSession),
		sendCode:        make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting state")
	}
	done := make(chan struct{})
	cancelEntered := make(chan struct{})
	allowCancelReturn := make(chan struct{})
	service.sessions[waiting.TransferID] = &engineSession{
		id: waiting.TransferID, done: done,
		cancel: func() {
			close(cancelEntered)
			<-allowCancelReturn
		},
	}
	cancelReleased := false
	doneClosed := false
	defer func() {
		if !cancelReleased {
			close(allowCancelReturn)
		}
		if !doneClosed {
			close(done)
		}
	}()

	firstReturned := make(chan struct{})
	go func() {
		service.Shutdown()
		close(firstReturned)
	}()
	select {
	case <-cancelEntered:
	case <-time.After(time.Second):
		t.Fatal("first Shutdown did not reach session cancellation")
	}
	secondStarted := make(chan struct{})
	secondReturned := make(chan struct{})
	go func() {
		close(secondStarted)
		service.Shutdown()
		close(secondReturned)
	}()
	<-secondStarted
	select {
	case <-secondReturned:
		t.Fatal("second Shutdown returned before the shared shutdown attempt completed")
	case <-time.After(40 * time.Millisecond):
	}

	close(allowCancelReturn)
	cancelReleased = true
	close(done)
	doneClosed = true
	for name, result := range map[string]<-chan struct{}{"first": firstReturned, "second": secondReturned} {
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Fatalf("%s Shutdown caller did not return", name)
		}
	}
}

func TestRunSendWorkerSuccessAfterCancelFinishesCancelled(t *testing.T) {
	updates := new([]Update)
	service := &engineService{
		cfg: EngineServiceConfig{
			RelayAddress: "relay.example.test:9009", RelayPassword: "fixture",
			EventSink: func(update Update) { *updates = append(*updates, update) },
		},
		worker:   &WorkerClient{path: os.Args[0], testEnv: []string{testWorkerModeEnv + "=success"}},
		sessions: make(map[string]*engineSession),
		sendCode: make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	waiting, ok := service.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		t.Fatal("failed to enter waiting state")
	}
	if _, ok := service.coordinator.Advance(waiting.TransferID, Transferring); !ok {
		t.Fatal("failed to enter transferring state")
	}
	if _, ok := service.coordinator.Cancel(waiting.TransferID); !ok {
		t.Fatal("failed to acquire cancellation gate")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	session := &engineSession{
		id: waiting.TransferID, done: make(chan struct{}), sources: &SendSourceLease{},
	}
	service.sessions[session.id] = session
	go service.runSend(ctx, session, "test-code", []string{"source.txt"}, strings.Repeat("a", 64))
	select {
	case <-session.done:
	case <-time.After(6 * time.Second):
		t.Fatal("runSend did not finish with the successful worker")
	}

	snapshot, ok := service.GetTransferSnapshot(session.id)
	if !ok || snapshot.State != Cancelled {
		t.Fatalf("worker success overrode accepted cancellation: %+v, %t", snapshot, ok)
	}
	for _, update := range *updates {
		if update.State == Completed {
			t.Fatalf("runSend emitted COMPLETED after cancellation: %+v", *updates)
		}
	}
	if _, err := service.coordinator.Begin(); err != nil {
		t.Fatalf("cancelled session kept coordinator busy: %v", err)
	}
}

func TestFinalizeNativeReceiveEmitsFinalizingAndCompletedByteMetrics(t *testing.T) {
	entries := []ManifestEntry{{Path: "finalizing.txt", SizeBytes: 23}}
	lease, stage, _ := publishFixture(t, entries)
	updates := new([]Update)
	service := &engineService{
		cfg:      EngineServiceConfig{EventSink: func(update Update) { *updates = append(*updates, update) }},
		sessions: make(map[string]*engineSession),
		sendCode: make(map[string]string),
	}
	started, err := service.coordinator.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []State{WaitingForPeer, Transferring} {
		if _, ok := service.coordinator.Advance(started.TransferID, state); !ok {
			t.Fatalf("failed to enter %s", state)
		}
	}
	service.sessions[started.TransferID] = &engineSession{
		id: started.TransferID, bytesTotal: manifestTotalBytes(entries),
	}

	completed, _, err := FinalizeNativeReceive(
		&service.coordinator, started.TransferID, lease, stage, entries,
		func(finalizing SessionSnapshot) { service.emit(finalizing, "", "") },
	)
	if err != nil || completed.State != Completed {
		t.Fatalf("receive finalization failed: %+v %v", completed, err)
	}
	service.emit(completed, "", "")
	service.forgetSession(started.TransferID, true)
	if len(*updates) != 2 || (*updates)[0].State != Finalizing || (*updates)[1].State != Completed {
		t.Fatalf("expected FINALIZING before COMPLETED, got %+v", *updates)
	}
	if (*updates)[1].BytesTransferred != "23" || (*updates)[1].BytesTotal != "23" {
		t.Fatalf("completed update did not report full bytes without progress frames: %+v", (*updates)[1])
	}
	update, ok := service.GetTransferUpdate(started.TransferID)
	if !ok || update.State != Completed || update.BytesTransferred != "23" || update.BytesTotal != "23" {
		t.Fatalf("completed snapshot lost byte metrics: %+v, %t", update, ok)
	}
}
