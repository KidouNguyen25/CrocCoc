//go:build windows

package transfer

import "errors"

// ErrReceiveFinalization prevents a receiver from reporting success when
// native publish or resource cleanup did not complete.
var ErrReceiveFinalization = errors.New("TRANSFER_FAILED: không thể hoàn tất phiên nhận an toàn")

// FinalizeNativeReceive owns the receiver's finalization gate. The caller
// must invoke it only after the engine has verified protocol integrity and all
// approved files are staged. It publishes outside the coordinator lock, then
// removes the verified stage and journal before releasing the terminal session.
//
// A publish failure leaves the coordinator busy in FAILED until its caller
// completes any separately verified recovery. No new session is admitted
// while owned staging may still need inspection.
func FinalizeNativeReceive(
	c *Coordinator,
	id string,
	lease *DestinationLease,
	stage *ReceiveStage,
	entries []ManifestEntry,
	onFinalizing ...func(SessionSnapshot),
) (SessionSnapshot, PublishResult, error) {
	var result PublishResult
	if c == nil || lease == nil || stage == nil || id == "" {
		return SessionSnapshot{}, result, ErrReceiveFinalization
	}
	snapshot, ok := c.Advance(id, Finalizing)
	if !ok {
		return SessionSnapshot{}, result, ErrReceiveFinalization
	}
	if len(onFinalizing) > 0 && onFinalizing[0] != nil {
		onFinalizing[0](snapshot)
	}
	result, err := publishAuditedRoots(lease, stage, entries)
	if err != nil {
		if failed, advanced := c.Fail(id, err); advanced {
			snapshot = failed
		}
		return snapshot, result, err
	}
	if err := cleanupPublishedReceiveStage(lease, stage); err != nil {
		if failed, advanced := c.Fail(id, err); advanced {
			snapshot = failed
		}
		return snapshot, result, err
	}
	if err := stage.Close(); err != nil {
		failed, advanced := c.Fail(id, ErrReceiveFinalization)
		if advanced {
			snapshot = failed
		}
		return snapshot, result, ErrReceiveFinalization
	}
	if err := lease.Close(); err != nil {
		failed, advanced := c.Fail(id, ErrReceiveFinalization)
		if advanced {
			snapshot = failed
		}
		return snapshot, result, ErrReceiveFinalization
	}
	completed, ok := c.Advance(id, Completed)
	if !ok {
		return snapshot, result, ErrReceiveFinalization
	}
	if !c.Release(id) {
		return completed, result, ErrReceiveFinalization
	}
	return completed, result, nil
}
