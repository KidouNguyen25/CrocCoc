package transfer

import "testing"

func TestCanTransitionExhaustively(t *testing.T) {
	states := []State{
		Idle, Preparing, WaitingForPeer, AwaitingAcceptance,
		Transferring, Finalizing, Cancelling, Completed, Failed, Cancelled, "UNKNOWN",
	}
	allowed := map[State]map[State]bool{
		Preparing:          {WaitingForPeer: true, Cancelling: true, Failed: true},
		WaitingForPeer:     {AwaitingAcceptance: true, Transferring: true, Cancelling: true, Failed: true},
		AwaitingAcceptance: {Transferring: true, Cancelling: true, Failed: true},
		Transferring:       {Finalizing: true, Cancelling: true, Failed: true},
		Finalizing:         {Completed: true, Failed: true},
		Cancelling:         {Cancelled: true, Failed: true},
	}
	for _, from := range states {
		for _, to := range states {
			want := allowed[from][to]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%q, %q) = %t, want %t", from, to, got, want)
			}
		}
	}
}

func TestTerminalStatesCannotTransition(t *testing.T) {
	for _, state := range []State{Completed, Failed, Cancelled} {
		if !IsTerminal(state) {
			t.Errorf("%s should be terminal", state)
		}
		if CanTransition(state, Preparing) || CanTransition(state, state) {
			t.Errorf("%s escaped terminal state", state)
		}
	}
	for _, state := range []State{Preparing, WaitingForPeer, AwaitingAcceptance, Transferring, Finalizing, Cancelling} {
		if IsTerminal(state) {
			t.Errorf("%s should not be terminal", state)
		}
	}
	if CanTransition(Transferring, Completed) || CanTransition(Cancelling, Completed) ||
		CanTransition(Finalizing, Cancelling) || CanTransition(Failed, Completed) {
		t.Fatal("unsafe finalization/cancellation edge accepted")
	}
}
