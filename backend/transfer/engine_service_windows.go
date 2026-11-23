//go:build windows && !croc_no_tailcat

package transfer

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

type receiveDecision struct {
	accept bool
}

type engineSession struct {
	id               string
	cancel           context.CancelFunc
	done             chan struct{}
	decision         chan receiveDecision
	pending          *nativeReceiveOffer
	stage            *ReceiveStage
	sources          *SendSourceLease
	bytesTotal       int64
	bytesTransferred int64
	lastProgressEmit time.Time
}

func manifestTotalBytes(entries []ManifestEntry) int64 {
	var total int64
	for _, entry := range entries {
		if !entry.Directory && entry.SizeBytes > 0 {
			total += entry.SizeBytes
		}
	}
	return total
}

type engineService struct {
	cfg             EngineServiceConfig
	worker          *WorkerClient
	workerPin       *SendSourceLease
	recoveryErr     error
	coordinator     Coordinator
	shutdownTimeout time.Duration

	mu                   sync.Mutex
	shuttingDown         bool
	shutdownDone         chan struct{}
	preflightContext     context.Context
	cancelPreflights     context.CancelFunc
	preflightCount       int
	preflightDone        chan struct{}
	sessions             map[string]*engineSession
	sendCode             map[string]string
	lastProgressID       string
	lastBytesTransferred int64
	lastBytesTotal       int64
}

// NewEngineService creates the Windows backend. It reports unavailable until
// worker path, relay address and relay password are configured.
func NewEngineService(config EngineServiceConfig) EngineController {
	recoveryErr := recoverOwnedReceiveStages()
	var workerPin *SendSourceLease
	if recoveryErr == nil {
		workerPin, _ = pinWorkerExecutable(config.WorkerPath, PinnedWorkerSHA256)
	}
	return &engineService{
		cfg: config, worker: NewWorkerClient(config.WorkerPath),
		workerPin: workerPin, recoveryErr: recoveryErr,
		sessions: make(map[string]*engineSession), sendCode: make(map[string]string),
	}
}

func (s *engineService) GetCrocInfo() CrocInfo {
	if s == nil {
		return CrocInfo{Available: false, Message: ErrWorkerUnavailable.Error()}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recoveryErr != nil {
		return CrocInfo{Available: false, Message: s.recoveryErr.Error()}
	}
	if s.cfg.RelayConfigErr != nil {
		return CrocInfo{Available: false, Message: s.cfg.RelayConfigErr.Error()}
	}
	if s.shuttingDown || s.worker == nil || s.workerPin == nil || s.cfg.RelayAddress == "" || s.cfg.RelayPassword == "" {
		return CrocInfo{Available: false, Message: ErrWorkerUnavailable.Error()}
	}
	return CrocInfo{Available: true, Version: PinnedCrocVersion, RelayLabel: s.cfg.RelayLabel}
}

// Preflight is admitted under the same lock as Shutdown's lifecycle latch.
// The drain channel prevents Shutdown from releasing the pinned executable
// while an admitted Start* is still inspecting sources/destinations.
func (s *engineService) beginPreflight() (context.Context, context.CancelFunc, error) {
	if s == nil {
		return nil, nil, ErrWorkerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recoveryErr != nil {
		return nil, nil, s.recoveryErr
	}
	if s.cfg.RelayConfigErr != nil {
		return nil, nil, s.cfg.RelayConfigErr
	}
	if s.shuttingDown || s.worker == nil || s.workerPin == nil || s.cfg.RelayAddress == "" || s.cfg.RelayPassword == "" {
		return nil, nil, ErrWorkerUnavailable
	}
	if s.preflightCount == 0 {
		s.preflightDone = make(chan struct{})
	}
	if s.preflightContext == nil {
		s.preflightContext, s.cancelPreflights = context.WithCancel(context.Background())
	}
	s.preflightCount++
	ctx, cancel := context.WithTimeout(s.preflightContext, PreparationTimeout)
	return ctx, cancel, nil
}

func (s *engineService) endPreflight() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preflightCount--
	if s.preflightCount == 0 {
		close(s.preflightDone)
		s.preflightDone = nil
	}
}

func (s *engineService) StartSend(paths []string) (string, error) {
	preparationCtx, preparationCancel, err := s.beginPreflight()
	if err != nil {
		return "", err
	}
	defer preparationCancel()
	defer s.endPreflight()
	sources, manifest, err := AcquireSendSourceLeaseContext(preparationCtx, paths)
	if err != nil {
		if errors.Is(preparationCtx.Err(), context.Canceled) {
			return "", ErrWorkerUnavailable
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(preparationCtx.Err(), context.DeadlineExceeded) {
			return "", ErrPreparationTimeout
		}
		return "", err
	}
	if errors.Is(preparationCtx.Err(), context.Canceled) {
		_ = sources.Close()
		return "", ErrWorkerUnavailable
	}
	digest, err := SendManifestDigest(manifest)
	if err != nil {
		_ = sources.Close()
		return "", err
	}
	if err := preparationCtx.Err(); err != nil {
		_ = sources.Close()
		if errors.Is(err, context.Canceled) {
			return "", ErrWorkerUnavailable
		}
		return "", ErrPreparationTimeout
	}
	code, err := newConnectionCode()
	if err != nil {
		_ = sources.Close()
		return "", err
	}
	if err := preparationCtx.Err(); err != nil {
		_ = sources.Close()
		if errors.Is(err, context.Canceled) {
			return "", ErrWorkerUnavailable
		}
		return "", ErrPreparationTimeout
	}
	// Begin/registration is atomic with Shutdown's admission latch. If
	// shutdown won while the source lease was being acquired, fail closed.
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		_ = sources.Close()
		return "", ErrWorkerUnavailable
	}
	started, err := s.coordinator.Begin()
	if err != nil {
		s.mu.Unlock()
		_ = sources.Close()
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		_, _ = s.coordinator.Fail(started.TransferID, ErrWorkerProtocol)
		_ = s.coordinator.Release(started.TransferID)
		s.mu.Unlock()
		_ = sources.Close()
		return "", ErrWorkerProtocol
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &engineSession{
		id:         waiting.TransferID,
		cancel:     cancel,
		done:       make(chan struct{}),
		sources:    sources,
		bytesTotal: manifestTotalBytes(manifest),
	}
	s.sessions[session.id] = session
	s.sendCode[session.id] = code
	s.mu.Unlock()
	s.emit(waiting, "", code)
	go s.runSend(ctx, session, code, paths, digest)
	return session.id, nil
}

func (s *engineService) runSend(ctx context.Context, session *engineSession, code string, paths []string, manifestDigest string) {
	defer close(session.done)
	id := session.id
	err := s.worker.send(ctx, code, s.cfg.RelayAddress, s.cfg.RelayPassword, paths, manifestDigest, func() {
		s.markSendTransferring(id)
	}, func(delta int64) {
		s.recordProgress(id, delta)
	})
	cleanupPending := errors.Is(err, ErrWorkerCleanupPending)
	s.mu.Lock()
	live := s.sessions[id]
	s.mu.Unlock()
	if live == nil || live.sources == nil {
		if err == nil {
			err = ErrSourceUnavailable
		}
	} else if !cleanupPending && live.sources.Close() != nil {
		cleanupPending = true
		err = ErrWorkerCleanupPending
	}
	if err == nil {
		if transferring, ok := s.coordinator.Advance(id, Transferring); ok {
			s.emit(transferring, "", "")
		}
		if finalizing, ok := s.coordinator.Advance(id, Finalizing); ok {
			s.emit(finalizing, "", "")
		}
		if completed, ok := s.coordinator.Advance(id, Completed); ok {
			s.emit(completed, "", "")
		}
		// Cancellation and completion use the coordinator's state gate as
		// their linearization point. If cancellation won before FINALIZING,
		// report CANCELLED even if the worker concurrently returned success;
		// claiming COMPLETED would contradict the accepted cancel. Do not
		// strand the session in CANCELLING (which would keep BUSY forever).
		if cancelled, ok := s.coordinator.Advance(id, Cancelled); ok {
			s.emit(cancelled, "", "")
		}
		if current, ok := s.coordinator.Snapshot(id); ok && !IsTerminal(current.State) {
			s.failSession(id, ErrWorkerProtocol)
		}
	} else {
		s.failSession(id, err)
	}
	s.releaseSession(id)
}

func (s *engineService) markSendTransferring(id string) {
	if transferring, ok := s.coordinator.Advance(id, Transferring); ok {
		s.emit(transferring, "", "")
	}
}

func (s *engineService) StartReceive(code, outputDir string) (string, error) {
	preparationCtx, preparationCancel, err := s.beginPreflight()
	if err != nil {
		return "", err
	}
	defer preparationCancel()
	defer s.endPreflight()
	if strings.TrimSpace(code) == "" || strings.ContainsAny(code, "\r\n\x00") || outputDir == "" {
		return "", ErrInvalidInput
	}
	// Windows directory probes use synchronous filesystem APIs that may block
	// inside the OS. Keep them off the StartReceive goroutine so Shutdown can
	// cancel this admitted preflight immediately without waiting for a hung
	// volume/handle query. The detached probe is read-only and owns/closes its
	// own temporary destination lease; it cannot register a session or publish.
	eligibilityDone := make(chan error, 1)
	go func() {
		eligibilityDone <- CheckReceiveDestinationEligibility(outputDir)
	}()
	select {
	case checkErr := <-eligibilityDone:
		if err := preparationCtx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return "", ErrWorkerUnavailable
			}
			return "", ErrPreparationTimeout
		}
		if checkErr != nil {
			return "", checkErr
		}
	case <-preparationCtx.Done():
		if errors.Is(preparationCtx.Err(), context.Canceled) {
			return "", ErrWorkerUnavailable
		}
		return "", ErrPreparationTimeout
	}
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return "", ErrWorkerUnavailable
	}
	started, err := s.coordinator.BeginReceive()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		_, _ = s.coordinator.Fail(started.TransferID, ErrWorkerProtocol)
		_ = s.coordinator.Release(started.TransferID)
		s.mu.Unlock()
		return "", ErrWorkerProtocol
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &engineSession{
		id:       waiting.TransferID,
		cancel:   cancel,
		done:     make(chan struct{}),
		decision: make(chan receiveDecision, 1),
	}
	s.sessions[session.id] = session
	s.mu.Unlock()
	s.emit(waiting, "", "")
	go s.runReceive(ctx, session, code, outputDir)
	return session.id, nil
}

func (s *engineService) runReceive(ctx context.Context, session *engineSession, code, outputDir string) {
	defer close(session.done)
	activationAttempted := false
	err := s.worker.ReceiveAndContinueWithProgress(ctx, "", code, s.cfg.RelayAddress, s.cfg.RelayPassword,
		func(offer ReceiveOffer) (bool, string, error) {
			preparationStarted := time.Now()
			pending, snapshot, presentedOffer, err := presentNativeReceiveOfferWithID(&s.coordinator, session.id, outputDir, offerToManifest(offer), offer.OfferID)
			if err != nil {
				return false, "", err
			}
			if time.Since(preparationStarted) >= PreparationTimeout {
				_ = pending.Close()
				return false, "", ErrPreparationTimeout
			}
			s.mu.Lock()
			session.pending = pending
			session.bytesTotal = manifestTotalBytes(pending.entries)
			s.mu.Unlock()
			s.emitOffer(snapshot, presentedOffer)
			waitDuration := OfferDecisionTimeout
			if !presentedOffer.ExpiresAt.IsZero() {
				if until := time.Until(presentedOffer.ExpiresAt); until > 0 {
					waitDuration = until
				}
			}
			timer := time.NewTimer(waitDuration)
			defer timer.Stop()
			select {
			case decision := <-session.decision:
				if !decision.accept {
					return false, "", nil
				}
				activationAttempted = true
				preparationCtx, cancel := context.WithTimeout(ctx, PreparationTimeout)
				stage, active, err := pending.activate(preparationCtx)
				preparationErr := preparationCtx.Err()
				cancel()
				if err == nil && errors.Is(preparationErr, context.DeadlineExceeded) {
					err = ErrPreparationTimeout
				} else if errors.Is(err, context.DeadlineExceeded) {
					err = ErrPreparationTimeout
				}
				if err != nil {
					return false, "", err
				}
				s.mu.Lock()
				session.stage = stage
				s.mu.Unlock()
				s.emit(active, "", "")
				return true, stage.PayloadPath(), nil
			case <-timer.C:
				expireNow := time.Now().UTC()
				if !presentedOffer.ExpiresAt.IsZero() && expireNow.Before(presentedOffer.ExpiresAt) {
					expireNow = presentedOffer.ExpiresAt
				}
				if cancelling, expired := s.coordinator.ExpireReceiveOffer(session.id, expireNow); expired {
					s.emit(cancelling, "", "")
				}
				return false, "", ErrOfferTimeout
			case <-ctx.Done():
				return false, "", ctx.Err()
			}
		}, func(delta int64) {
			s.recordProgress(session.id, delta)
		})
	s.mu.Lock()
	stage := session.stage
	pending := session.pending
	s.mu.Unlock()
	if err == nil && stage != nil && pending != nil {
		if adoptErr := stage.AdoptEnginePayload(); adoptErr != nil {
			s.finishReceiveSession(session.id, adoptErr, false, !recoverClosedStage(stage, pending))
			return
		}
		final, result, finalErr := FinalizeNativeReceive(&s.coordinator, session.id, pending.lease, stage, pending.entries,
			func(finalizing SessionSnapshot) { s.emit(finalizing, "", "") })
		if finalErr != nil {
			s.finishReceiveSession(session.id, finalErr, result.Partial, !recoverClosedStage(stage, pending))
		} else {
			s.emit(final, "", "")
			_ = result
			// FinalizeNativeReceive has already released the coordinator.
			// A newly admitted session may replace recent before we get here.
			// Do not rely on a second Snapshot lookup to drop this map entry.
			s.forgetSession(session.id, true)
		}
		return
	}
	workerCleanupPending := errors.Is(err, ErrWorkerCleanupPending)
	cleanupPending := activationAttempted || stage != nil || workerCleanupPending
	if stage != nil && !workerCleanupPending && stage.Close() == nil && pending != nil && pending.Close() == nil {
		if recoverOwnedReceiveStage(stage.JournalPath()) == nil {
			cleanupPending = false
			pending = nil
		}
	}
	if pending != nil && pending.Close() != nil {
		cleanupPending = true
	}
	if err == nil {
		if snapshot, ok := s.coordinator.Snapshot(session.id); ok && snapshot.State == Cancelling {
			s.finishReceiveSession(session.id, nil, false, cleanupPending)
			return
		}
		err = ErrWorkerProtocol
	}
	s.finishReceiveSession(session.id, err, false, cleanupPending)
}

// recoverClosedStage closes the stage and offer handles, then removes the
// owned stage through verified recovery. False means cleanup is still pending.
func recoverClosedStage(stage *ReceiveStage, pending *nativeReceiveOffer) bool {
	stageClosed := stage.Close() == nil
	pendingClosed := pending.Close() == nil
	return stageClosed && pendingClosed && recoverOwnedReceiveStage(stage.JournalPath()) == nil
}

func offerToManifest(offer ReceiveOffer) []ManifestEntry {
	entries := make([]ManifestEntry, 0, len(offer.Items))
	for _, item := range offer.Items {
		size := int64(0)
		if item.Type == "file" {
			parsed, err := strconv.ParseInt(item.SizeBytes, 10, 64)
			if err != nil {
				size = -1
			} else {
				size = parsed
			}
		}
		entries = append(entries, ManifestEntry{Path: item.Path, Directory: item.Type == "directory", SizeBytes: size})
	}
	return entries
}

func (s *engineService) DecideReceive(id, offerID string, accept bool) (bool, error) {
	s.mu.Lock()
	session := s.sessions[id]
	pending := (*nativeReceiveOffer)(nil)
	if session != nil {
		pending = session.pending
	}
	s.mu.Unlock()
	if session == nil || pending == nil || pending.offerID != offerID {
		return false, nil
	}
	snapshot, ok := s.coordinator.DecideReceive(id, offerID, accept, time.Now())
	if !ok {
		return false, nil
	}
	if !accept {
		s.emit(snapshot, "", "")
	}
	// The buffer holds one decision; if a cancel already filled it, the
	// session is ending and a blocking send would never return.
	select {
	case session.decision <- receiveDecision{accept: accept}:
	default:
	}
	return true, nil
}

func (s *engineService) GetReceiveOffer(id string) (ReceiveOffer, bool) {
	return s.coordinator.GetReceiveOffer(id)
}

func (s *engineService) GetTransferSnapshot(id string) (SessionSnapshot, bool) {
	return s.coordinator.Snapshot(id)
}

func (s *engineService) GetTransferUpdate(id string) (Update, bool) {
	snapshot, ok := s.coordinator.Snapshot(id)
	if !ok {
		return Update{}, false
	}
	update := Update{
		TransferID:     snapshot.TransferID,
		Revision:       snapshot.Revision,
		State:          snapshot.State,
		Error:          cloneTransferError(snapshot.Error),
		PartialOutput:  snapshot.PartialOutput,
		CleanupPending: snapshot.CleanupPending,
	}
	switch snapshot.State {
	case WaitingForPeer:
		s.mu.Lock()
		code := s.sendCode[id]
		s.mu.Unlock()
		if code != "" {
			update.ConnectionCode = code
		}
	case AwaitingAcceptance:
		if offer, found := s.coordinator.GetReceiveOffer(id); found {
			update.ReceiveOffer = &offer
		}
	case Transferring, Finalizing, Completed:
		update.BytesTransferred, update.BytesTotal = s.progressFields(id, snapshot.State == Completed)
	}
	return update, true
}

func (s *engineService) CancelTransfer(id string) (bool, error) {
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil {
		return false, nil
	}
	snapshot, ok := s.coordinator.Cancel(id)
	if !ok {
		return false, nil
	}
	s.emit(snapshot, "", "")
	if session.cancel != nil {
		session.cancel()
	}
	select {
	case session.decision <- receiveDecision{accept: false}:
	default:
	}
	return true, nil
}

func (s *engineService) Shutdown() {
	if s == nil {
		return
	}
	timeout := s.shutdownTimeout
	if timeout <= 0 {
		timeout = WorkerCleanupTimeout
	}
	s.mu.Lock()
	if s.shuttingDown {
		done := s.shutdownDone
		s.mu.Unlock()
		// Every caller observes the same bounded shutdown attempt. Completion
		// of this channel means Shutdown returned, NOT that a timed-out worker
		// was killed or a retained executable pin was released.
		if done != nil {
			<-done
		}
		return
	}
	s.shuttingDown = true
	s.shutdownDone = make(chan struct{})
	shutdownDone := s.shutdownDone
	cancelPreflights := s.cancelPreflights
	preflightDone := s.preflightDone
	ids := make([]string, 0, len(s.sessions))
	waitChans := make([]<-chan struct{}, 0, len(s.sessions))
	for id, session := range s.sessions {
		ids = append(ids, id)
		if session != nil && session.done != nil {
			waitChans = append(waitChans, session.done)
		}
	}
	s.mu.Unlock()
	defer close(shutdownDone)
	// Stop admitted filesystem preflights immediately; they still need to
	// release their leases and signal preflightDone before the pin may close.
	if cancelPreflights != nil {
		cancelPreflights()
	}
	for _, id := range ids {
		_, _ = s.CancelTransfer(id)
	}

	// One bounded deadline covers both preflight drain and worker cleanup.
	// A timed-out preflight must keep the worker executable pin locked, even
	// when no session was registered when Shutdown began.
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	allDone := true
	if preflightDone != nil {
		select {
		case <-preflightDone:
		case <-deadline.C:
			allDone = false
		}
	}
	if allDone {
		for _, ch := range waitChans {
			select {
			case <-ch:
			case <-deadline.C:
				allDone = false
			}
			if !allDone {
				break
			}
		}
	}

	// A worker cleanup timeout can close session.done without proving that
	// the worker process has exited. Preserve its executable pin until process
	// exit rather than treating a closed goroutine channel as OS cleanup proof.
	if allDone {
		for _, id := range ids {
			if snapshot, found := s.coordinator.Snapshot(id); found && snapshot.CleanupPending {
				allDone = false
				break
			}
		}
	}
	// Only release the pinned worker lease after all active sessions have
	// conclusively ended; otherwise keep it locked until OS process exit.
	if allDone {
		s.mu.Lock()
		workerPin := s.workerPin
		s.workerPin = nil
		s.mu.Unlock()
		if workerPin != nil {
			_ = workerPin.Close()
		}
	}
}

func (s *engineService) recordProgress(id string, delta int64) {
	if s == nil || delta <= 0 {
		return
	}
	now := time.Now()
	s.mu.Lock()
	session := s.sessions[id]
	if session == nil {
		s.mu.Unlock()
		return
	}
	session.bytesTransferred += delta
	if session.bytesTotal > 0 && session.bytesTransferred > session.bytesTotal {
		session.bytesTransferred = session.bytesTotal
	}
	transferred := session.bytesTransferred
	total := session.bytesTotal
	shouldEmit := session.lastProgressEmit.IsZero() ||
		now.Sub(session.lastProgressEmit) >= 150*time.Millisecond ||
		(total > 0 && transferred >= total)
	if shouldEmit {
		session.lastProgressEmit = now
	}
	s.mu.Unlock()
	if !shouldEmit {
		return
	}
	snapshot, ok := s.coordinator.TouchProgress(id)
	if !ok || s.cfg.EventSink == nil {
		return
	}
	update := Update{
		TransferID: snapshot.TransferID,
		Revision:   snapshot.Revision,
		State:      snapshot.State,
	}
	if total > 0 {
		update.BytesTransferred = strconv.FormatInt(transferred, 10)
		update.BytesTotal = strconv.FormatInt(total, 10)
	}
	s.cfg.EventSink(update)
}

func (s *engineService) GetSendCode(id string) (string, bool) {
	snapshot, ok := s.coordinator.Snapshot(id)
	if !ok || snapshot.State != WaitingForPeer {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	code, ok := s.sendCode[id]
	return code, ok
}

func (s *engineService) failSession(id string, cause error) {
	if cause == nil {
		cause = ErrWorkerProtocol
	}
	if current, ok := s.coordinator.Snapshot(id); ok && current.State == Cancelling {
		if errors.Is(cause, ErrWorkerCleanupPending) {
			if snapshot, advanced := s.coordinator.Fail(id, cause); advanced {
				snapshot, _ = s.coordinator.SetTerminalFlags(id, false, true)
				s.emitFailure(snapshot, cause)
			}
			return
		}
		if snapshot, advanced := s.coordinator.Advance(id, Cancelled); advanced {
			s.emit(snapshot, cancelMessage(cause), "")
		}
		return
	}
	if snapshot, ok := s.coordinator.Fail(id, cause); ok {
		if errors.Is(cause, ErrWorkerCleanupPending) {
			snapshot, _ = s.coordinator.SetTerminalFlags(id, false, true)
		}
		s.emitFailure(snapshot, cause)
		return
	}
	if snapshot, ok := s.coordinator.Advance(id, Cancelled); ok {
		s.emit(snapshot, cancelMessage(cause), "")
	}
}

// cancelMessage hides Go's raw "context canceled" text from the UI.
func cancelMessage(cause error) string {
	if errors.Is(cause, context.Canceled) {
		return ""
	}
	return cause.Error()
}

func (s *engineService) finishReceiveSession(id string, cause error, partialOutput, cleanupPending bool) {
	snapshot, ok := s.coordinator.Snapshot(id)
	if !ok {
		return
	}
	if !IsTerminal(snapshot.State) {
		next := Failed
		if snapshot.State == Cancelling && !cleanupPending && !errors.Is(cause, ErrOfferTimeout) {
			next = Cancelled
		}
		if advanced, changed := s.coordinator.Advance(id, next); changed {
			snapshot = advanced
		}
	}
	if !IsTerminal(snapshot.State) {
		return
	}
	if snapshot.State == Failed && cause == nil {
		cause = ErrTransferFailed
	}
	if snapshot.State == Failed {
		if failed, changed := s.coordinator.Fail(id, cause); changed {
			snapshot = failed
		}
	}
	if updated, changed := s.coordinator.SetTerminalFlags(id, partialOutput, cleanupPending); changed {
		snapshot = updated
	}
	if snapshot.State == Failed {
		s.emitFailure(snapshot, cause)
	} else {
		s.emit(snapshot, "", "")
	}
	if !cleanupPending {
		s.releaseSession(id)
	}
}

// progressFields keeps completed transfer metrics readable even after the
// session is released. Successful COMPLETED means all verified payload bytes
// were transferred, including when progress frames were coalesced/skipped.
func (s *engineService) progressFields(id string, completed bool) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var transferred, total int64
	if session := s.sessions[id]; session != nil {
		transferred, total = session.bytesTransferred, session.bytesTotal
	} else if s.lastProgressID == id {
		transferred, total = s.lastBytesTransferred, s.lastBytesTotal
	}
	if total <= 0 {
		return "", ""
	}
	if completed {
		transferred = total
	}
	return strconv.FormatInt(transferred, 10), strconv.FormatInt(total, 10)
}

func (s *engineService) releaseSession(id string) {
	snapshot, ok := s.coordinator.Snapshot(id)
	if !ok || !IsTerminal(snapshot.State) || snapshot.CleanupPending {
		// Never discard the cancellation handle for a live session, or free
		// owned state while cleanup is still pending. A receive finalizer may
		// already have released the terminal snapshot before reaching here.
		return
	}
	if !s.coordinator.Release(id) {
		return
	}
	s.forgetSession(id, snapshot.State == Completed)
}

// forgetSession is also used when the receive finalizer already called
// Coordinator.Release; there may no longer be a recent snapshot for id.
func (s *engineService) forgetSession(id string, completed bool) {
	s.mu.Lock()
	if session := s.sessions[id]; session != nil {
		s.lastProgressID = id
		s.lastBytesTransferred = session.bytesTransferred
		s.lastBytesTotal = session.bytesTotal
		if completed && session.bytesTotal > 0 {
			s.lastBytesTransferred = session.bytesTotal
		}
	}
	delete(s.sessions, id)
	delete(s.sendCode, id)
	s.mu.Unlock()
}

func (s *engineService) emit(snapshot SessionSnapshot, message, code string) {
	if s.cfg.EventSink == nil {
		return
	}
	update := Update{TransferID: snapshot.TransferID, State: snapshot.State,
		Revision: snapshot.Revision, Message: message, ConnectionCode: code,
		PartialOutput: snapshot.PartialOutput, CleanupPending: snapshot.CleanupPending}
	if snapshot.State == Transferring || snapshot.State == Finalizing || snapshot.State == Completed {
		update.BytesTransferred, update.BytesTotal = s.progressFields(snapshot.TransferID, snapshot.State == Completed)
	}
	s.cfg.EventSink(update)
}

func (s *engineService) emitOffer(snapshot SessionSnapshot, offer ReceiveOffer) {
	if s.cfg.EventSink == nil {
		return
	}
	copyOffer := offer.Clone()
	s.cfg.EventSink(Update{TransferID: snapshot.TransferID, Revision: snapshot.Revision,
		State: snapshot.State, ReceiveOffer: &copyOffer})
}

func (s *engineService) emitFailure(snapshot SessionSnapshot, cause error) {
	if s.cfg.EventSink == nil {
		return
	}
	s.cfg.EventSink(Update{TransferID: snapshot.TransferID, Revision: snapshot.Revision,
		State: snapshot.State, Error: transferError(cause),
		PartialOutput: snapshot.PartialOutput, CleanupPending: snapshot.CleanupPending})
}
