//go:build windows && !croc_no_tailcat

package transfer

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

type receiveDecision struct {
	accept bool
}

type engineSession struct {
	id       string
	cancel   context.CancelFunc
	decision chan receiveDecision
	pending  *nativeReceiveOffer
	stage    *ReceiveStage
	sources  *SendSourceLease
}

type engineService struct {
	cfg         EngineServiceConfig
	worker      *WorkerClient
	workerPin   *SendSourceLease
	recoveryErr error
	coordinator Coordinator

	mu       sync.Mutex
	sessions map[string]*engineSession
	sendCode map[string]string
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
	if s != nil && s.recoveryErr != nil {
		return CrocInfo{Available: false, Message: s.recoveryErr.Error()}
	}
	if s == nil || s.worker == nil || s.workerPin == nil || s.cfg.RelayAddress == "" || s.cfg.RelayPassword == "" {
		return CrocInfo{Available: false, Message: ErrWorkerUnavailable.Error()}
	}
	return CrocInfo{Available: true, Version: PinnedCrocVersion}
}

func (s *engineService) unavailableError() error {
	if s != nil && s.recoveryErr != nil {
		return s.recoveryErr
	}
	return ErrWorkerUnavailable
}

func (s *engineService) StartSend(paths []string) (string, error) {
	if s == nil || !s.GetCrocInfo().Available {
		return "", s.unavailableError()
	}
	sources, manifest, err := AcquireSendSourceLease(paths)
	if err != nil {
		return "", err
	}
	digest, err := SendManifestDigest(manifest)
	if err != nil {
		_ = sources.Close()
		return "", err
	}
	code, err := newConnectionCode()
	if err != nil {
		_ = sources.Close()
		return "", err
	}
	started, err := s.coordinator.Begin()
	if err != nil {
		_ = sources.Close()
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		_, _ = s.coordinator.Advance(started.TransferID, Failed)
		_ = s.coordinator.Release(started.TransferID)
		_ = sources.Close()
		return "", ErrWorkerProtocol
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &engineSession{id: waiting.TransferID, cancel: cancel, sources: sources}
	s.mu.Lock()
	s.sessions[session.id] = session
	s.sendCode[session.id] = code
	s.mu.Unlock()
	s.emit(waiting, "", code)
	go s.runSend(ctx, session.id, code, paths, digest)
	return session.id, nil
}

func (s *engineService) runSend(ctx context.Context, id, code string, paths []string, manifestDigest string) {
	err := s.worker.Send(ctx, code, s.cfg.RelayAddress, s.cfg.RelayPassword, paths, manifestDigest)
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil || session.sources == nil || session.sources.Close() != nil {
		if err == nil {
			err = ErrSourceUnavailable
		}
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
	} else {
		s.failSession(id, err)
	}
	s.releaseSession(id)
}

func (s *engineService) StartReceive(code, outputDir string) (string, error) {
	if s == nil || !s.GetCrocInfo().Available {
		return "", s.unavailableError()
	}
	if strings.TrimSpace(code) == "" || strings.ContainsAny(code, "\r\n\x00") || outputDir == "" {
		return "", ErrInvalidInput
	}
	if err := CheckReceiveDestinationEligibility(outputDir); err != nil {
		return "", err
	}
	started, err := s.coordinator.BeginReceive()
	if err != nil {
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		_, _ = s.coordinator.Advance(started.TransferID, Failed)
		_ = s.coordinator.Release(started.TransferID)
		return "", ErrWorkerProtocol
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &engineSession{id: waiting.TransferID, cancel: cancel, decision: make(chan receiveDecision, 1)}
	s.mu.Lock()
	s.sessions[session.id] = session
	s.mu.Unlock()
	s.emit(waiting, "", "")
	go s.runReceive(ctx, session, code, outputDir)
	return session.id, nil
}

func (s *engineService) runReceive(ctx context.Context, session *engineSession, code, outputDir string) {
	activationAttempted := false
	err := s.worker.ReceiveAndContinue(ctx, "", code, s.cfg.RelayAddress, s.cfg.RelayPassword,
		func(offer ReceiveOffer) (bool, string, error) {
			pending, snapshot, _, err := presentNativeReceiveOfferWithID(&s.coordinator, session.id, outputDir, offerToManifest(offer), offer.OfferID)
			if err != nil {
				return false, "", err
			}
			s.mu.Lock()
			session.pending = pending
			s.mu.Unlock()
			s.emitOffer(snapshot, offer)
			timer := time.NewTimer(OfferDecisionTimeout)
			defer timer.Stop()
			select {
			case decision := <-session.decision:
				if !decision.accept {
					return false, "", nil
				}
				activationAttempted = true
				stage, active, err := pending.activate(ctx)
				if err != nil {
					return false, "", err
				}
				s.mu.Lock()
				session.stage = stage
				s.mu.Unlock()
				s.emit(active, "", "")
				return true, stage.PayloadPath(), nil
			case <-timer.C:
				s.coordinator.ExpireReceiveOffer(session.id, time.Now())
				return false, "", nil
			case <-ctx.Done():
				return false, "", ctx.Err()
			}
		})
	s.mu.Lock()
	stage := session.stage
	pending := session.pending
	s.mu.Unlock()
	if err == nil && stage != nil && pending != nil {
		if adoptErr := stage.AdoptEnginePayload(); adoptErr != nil {
			_ = stage.Close()
			_ = pending.Close()
			s.finishReceiveSession(session.id, adoptErr, false, true)
			return
		}
		final, result, finalErr := FinalizeNativeReceive(&s.coordinator, session.id, pending.lease, stage, pending.entries)
		if finalErr != nil {
			_ = stage.Close()
			_ = pending.Close()
			s.finishReceiveSession(session.id, finalErr, result.Partial, true)
		} else {
			s.emit(final, "", "")
			_ = result
			s.releaseSession(session.id)
		}
		return
	}
	cleanupPending := activationAttempted || stage != nil
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
	if _, ok := s.coordinator.DecideReceive(id, offerID, accept, time.Now()); !ok {
		return false, nil
	}
	session.decision <- receiveDecision{accept: accept}
	return true, nil
}

func (s *engineService) GetReceiveOffer(id string) (ReceiveOffer, bool) {
	return s.coordinator.GetReceiveOffer(id)
}

func (s *engineService) GetTransferSnapshot(id string) (SessionSnapshot, bool) {
	return s.coordinator.Snapshot(id)
}

func (s *engineService) CancelTransfer(id string) (bool, error) {
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil {
		return false, nil
	}
	if _, ok := s.coordinator.Cancel(id); !ok {
		return false, nil
	}
	session.cancel()
	select {
	case session.decision <- receiveDecision{accept: false}:
	default:
	}
	return true, nil
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
		if snapshot, advanced := s.coordinator.Advance(id, Cancelled); advanced {
			s.emit(snapshot, cause.Error(), "")
		}
		return
	}
	if snapshot, ok := s.coordinator.Advance(id, Failed); ok {
		s.emitFailure(snapshot, cause)
		return
	}
	if snapshot, ok := s.coordinator.Advance(id, Cancelled); ok {
		s.emit(snapshot, cause.Error(), "")
	}
}

func (s *engineService) finishReceiveSession(id string, cause error, partialOutput, cleanupPending bool) {
	snapshot, ok := s.coordinator.Snapshot(id)
	if !ok {
		return
	}
	if !IsTerminal(snapshot.State) {
		next := Failed
		if snapshot.State == Cancelling {
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

func (s *engineService) releaseSession(id string) {
	if snapshot, ok := s.coordinator.Snapshot(id); ok && IsTerminal(snapshot.State) {
		if snapshot.CleanupPending {
			return
		}
		_ = s.coordinator.Release(id)
	}
	s.mu.Lock()
	delete(s.sessions, id)
	delete(s.sendCode, id)
	s.mu.Unlock()
}

func (s *engineService) emit(snapshot SessionSnapshot, message, code string) {
	if s.cfg.EventSink == nil {
		return
	}
	s.cfg.EventSink(Update{TransferID: snapshot.TransferID, State: snapshot.State,
		Revision: snapshot.Revision, Message: message, ConnectionCode: code,
		PartialOutput: snapshot.PartialOutput, CleanupPending: snapshot.CleanupPending})
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
