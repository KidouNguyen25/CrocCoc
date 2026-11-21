//go:build windows

package transfer

import (
	"context"
	"errors"
	"os"
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
}

type engineService struct {
	cfg         EngineServiceConfig
	worker      *WorkerClient
	coordinator Coordinator

	mu       sync.Mutex
	sessions map[string]*engineSession
	sendCode map[string]string
}

// NewEngineService creates the Windows backend. It reports unavailable until
// worker path, relay address and relay password are configured.
func NewEngineService(config EngineServiceConfig) EngineController {
	return &engineService{
		cfg: config, worker: NewWorkerClient(config.WorkerPath),
		sessions: make(map[string]*engineSession), sendCode: make(map[string]string),
	}
}

func (s *engineService) GetCrocInfo() CrocInfo {
	if s == nil || s.worker == nil || s.cfg.RelayAddress == "" || s.cfg.RelayPassword == "" {
		return CrocInfo{Available: false, Message: ErrWorkerUnavailable.Error()}
	}
	if _, err := os.Stat(s.cfg.WorkerPath); err != nil {
		return CrocInfo{Available: false, Message: ErrWorkerUnavailable.Error()}
	}
	return CrocInfo{Available: true, Version: PinnedCrocVersion}
}

func (s *engineService) StartSend(paths []string) (string, error) {
	if s == nil || !s.GetCrocInfo().Available {
		return "", ErrWorkerUnavailable
	}
	if _, err := InspectSendSources(paths); err != nil {
		return "", err
	}
	code, err := newConnectionCode()
	if err != nil {
		return "", err
	}
	started, err := s.coordinator.Begin()
	if err != nil {
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
		return "", ErrWorkerProtocol
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &engineSession{id: waiting.TransferID, cancel: cancel}
	s.mu.Lock()
	s.sessions[session.id] = session
	s.sendCode[session.id] = code
	s.mu.Unlock()
	s.emit(waiting, code, "")
	go s.runSend(ctx, session.id, code, paths)
	return session.id, nil
}

func (s *engineService) runSend(ctx context.Context, id, code string, paths []string) {
	err := s.worker.Send(ctx, code, s.cfg.RelayAddress, s.cfg.RelayPassword, paths)
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
		return "", ErrWorkerUnavailable
	}
	if strings.TrimSpace(code) == "" || strings.ContainsAny(code, "\r\n\x00") || outputDir == "" {
		return "", ErrInvalidInput
	}
	started, err := s.coordinator.BeginReceive()
	if err != nil {
		return "", err
	}
	waiting, ok := s.coordinator.Advance(started.TransferID, WaitingForPeer)
	if !ok {
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
	err := s.worker.ReceiveAndContinue(ctx, "", code, s.cfg.RelayAddress, s.cfg.RelayPassword,
		func(offer ReceiveOffer) (bool, string, error) {
			pending, snapshot, _, err := presentNativeReceiveOfferWithID(&s.coordinator, session.id, outputDir, offerToManifest(offer), offer.OfferID)
			if err != nil {
				return false, "", err
			}
			s.mu.Lock()
			session.pending = pending
			s.mu.Unlock()
			s.emit(snapshot, "", offer.OfferID)
			timer := time.NewTimer(OfferDecisionTimeout)
			defer timer.Stop()
			select {
			case decision := <-session.decision:
				if !decision.accept {
					_ = pending.Close()
					return false, "", nil
				}
				stage, active, err := pending.activate(ctx)
				if err != nil {
					_ = pending.Close()
					return false, "", err
				}
				s.mu.Lock()
				session.stage = stage
				s.mu.Unlock()
				s.emit(active, "", "")
				return true, stage.PayloadPath(), nil
			case <-timer.C:
				s.coordinator.ExpireReceiveOffer(session.id, time.Now())
				_ = pending.Close()
				return false, "", nil
			case <-ctx.Done():
				_ = pending.Close()
				return false, "", ctx.Err()
			}
		})
	s.mu.Lock()
	stage := session.stage
	pending := session.pending
	s.mu.Unlock()
	if err == nil && stage != nil && pending != nil {
		if adoptErr := stage.AdoptEnginePayload(); adoptErr != nil {
			s.failSession(session.id, adoptErr)
			s.releaseSession(session.id)
			return
		}
		final, result, finalErr := FinalizeNativeReceive(&s.coordinator, session.id, pending.lease, stage, pending.entries)
		if finalErr != nil {
			s.failSession(session.id, finalErr)
		} else {
			s.emit(final, "", "")
			_ = result
		}
	} else if err == nil {
		if snapshot, ok := s.coordinator.Snapshot(session.id); ok && snapshot.State == Cancelling {
			if cancelled, advanced := s.coordinator.Advance(session.id, Cancelled); advanced {
				s.emit(cancelled, "", "")
			}
		} else {
			s.failSession(session.id, ErrWorkerProtocol)
		}
	} else {
		if pending != nil {
			_ = pending.Close()
		}
		if stage != nil {
			_ = stage.Close()
		}
		s.failSession(session.id, err)
	}
	s.releaseSession(session.id)
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
		s.emit(snapshot, cause.Error(), "")
		return
	}
	if snapshot, ok := s.coordinator.Advance(id, Cancelled); ok {
		s.emit(snapshot, cause.Error(), "")
	}
}

func (s *engineService) releaseSession(id string) {
	if snapshot, ok := s.coordinator.Snapshot(id); ok && IsTerminal(snapshot.State) {
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
		Message: message, ConnectionCode: code})
}
