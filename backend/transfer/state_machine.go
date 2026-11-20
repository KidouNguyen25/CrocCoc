package transfer

// CanTransition permits only edges in the native transfer lifecycle.
// The caller must separately enforce transfer direction and obtain the
// coordinator lock before committing a transition. Initial sessions begin
// directly in PREPARING; IDLE belongs to the frontend only.
func CanTransition(from, to State) bool {
	switch from {
	case Preparing:
		return to == WaitingForPeer || to == Cancelling || to == Failed
	case WaitingForPeer:
		return to == AwaitingAcceptance || to == Transferring ||
			to == Cancelling || to == Failed
	case AwaitingAcceptance:
		return to == Transferring || to == Cancelling || to == Failed
	case Transferring:
		return to == Finalizing || to == Cancelling || to == Failed
	case Finalizing:
		return to == Completed || to == Failed
	case Cancelling:
		return to == Cancelled || to == Failed
	default:
		// No transitions from terminal, IDLE or unknown states.
		return false
	}
}

// IsTerminal recognizes states after which no new updates may be published.
func IsTerminal(state State) bool {
	return state == Completed || state == Failed || state == Cancelled
}
