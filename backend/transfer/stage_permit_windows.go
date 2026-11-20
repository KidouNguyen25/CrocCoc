//go:build windows

package transfer

// receivePermit is issued only after the native stage exists, its ownership
// marker and journal were flushed, and its approved directories were created.
// It does not itself give access to payload I/O; Coordinator must separately
// verify consent, offer identity and cancellation under its lock.
func (s *ReceiveStage) receivePermit(transferID, offerID string) ReceivePermit {
	if s == nil || transferID == "" || offerID == "" {
		return ReceivePermit{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage == 0 || s.payload == 0 || !s.prepared ||
		s.ownership.Status != "prepared" ||
		s.ownership.StageName != s.StageName || s.journalPath == "" {
		return ReceivePermit{}
	}
	return ReceivePermit{
		transferID: transferID,
		offerID:    offerID,
		digest:     s.manifestDigest,
		stageID:    s.StageName,
	}
}
