package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

var ErrBusy = errors.New("BUSY: đang có phiên truyền chưa giải phóng tài nguyên")

// SessionSnapshot is an immutable copy of a native session's position.
// Event publication and the API wire are integrated separately.
type SessionSnapshot struct {
	TransferID string `json:"transferID"`
	Revision   uint64 `json:"revision"`
	State      State  `json:"state"`
}

// Coordinator arbitrates transitions and cancellation under one short lock.
// Engine I/O and cleanup must run outside this lock. Terminal sessions remain
// busy until Release confirms the caller has completed resource cleanup.
type Coordinator struct {
	mu        sync.Mutex
	active    *SessionSnapshot
	recent    *SessionSnapshot
	receiving bool
	offer     *ReceiveOffer
	decided   bool
}

// Begin allocates one fresh opaque ID. It does not start an engine.
func (c *Coordinator) Begin() (SessionSnapshot, error) {
	return c.begin(false)
}

// BeginReceive starts a receiver session; unlike a sender it must pass
// manifest presentation and explicit consent before any payload processing.
func (c *Coordinator) BeginReceive() (SessionSnapshot, error) {
	return c.begin(true)
}

func (c *Coordinator) begin(receiving bool) (SessionSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil {
		return SessionSnapshot{}, ErrBusy
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return SessionSnapshot{}, errors.New("không thể tạo mã định danh phiên")
	}
	snapshot := SessionSnapshot{
		TransferID: hex.EncodeToString(raw[:]),
		Revision:   1,
		State:      Preparing,
	}
	c.active = &snapshot
	c.recent = nil
	c.receiving = receiving
	c.offer = nil
	c.decided = false
	return snapshot, nil
}

// Advance commits one valid lifecycle transition; false means it changed
// nothing. In particular, it cannot change a terminal state.
func (c *Coordinator) Advance(id string, next State) (SessionSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		!CanTransition(c.active.State, next) ||
		next == AwaitingAcceptance || (c.receiving && next == Transferring) {
		return SessionSnapshot{}, false
	}
	c.active.State = next
	c.active.Revision++
	if next != AwaitingAcceptance {
		c.offer = nil
		c.decided = false
	}
	return *c.active, true
}

// Cancel attempts to acquire the cancellation gate before finalization.
// A successful call records CANCELLING immediately; no I/O is awaited here.
func (c *Coordinator) Cancel(id string) (SessionSnapshot, bool) {
	return c.Advance(id, Cancelling)
}

// Snapshot reads the live session or most recently released terminal session.
func (c *Coordinator) Snapshot(id string) (SessionSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil && c.active.TransferID == id {
		return *c.active, true
	}
	if c.recent != nil && c.recent.TransferID == id {
		return *c.recent, true
	}
	return SessionSnapshot{}, false
}

// Release may only be called after engine resources and owned staging have
// been conclusively handled. It never guesses cleanup from a terminal state.
func (c *Coordinator) Release(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.TransferID != id ||
		!IsTerminal(c.active.State) {
		return false
	}
	snapshot := *c.active
	c.recent = &snapshot
	c.active = nil
	return true
}
