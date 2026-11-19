package transfer

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestUnavailableServiceDoesNotReportRealTransfer(t *testing.T) {
	service := NewUnavailableService()
	if info := service.GetCrocInfo(); info.Available || info.Message == "" {
		t.Fatalf("unexpected Croc info: %#v", info)
	}
	if id, err := service.StartSend([]string{"example.txt"}); id != "" || !errors.Is(err, ErrNotIntegrated) {
		t.Fatalf("unexpected send result: %q, %v", id, err)
	}
	const code = "mock-private-code"
	if id, err := service.StartReceive(code, "Downloads"); id != "" || !errors.Is(err, ErrNotIntegrated) || strings.Contains(err.Error(), code) {
		t.Fatalf("unexpected receive result: %q, %v", id, err)
	}
	if ok, err := service.CancelTransfer("test-id"); ok || !errors.Is(err, ErrNotIntegrated) {
		t.Fatalf("unexpected cancel result: %t, %v", ok, err)
	}
}

func TestUpdateWireShape(t *testing.T) {
	zero := 0.0
	cases := []struct {
		update Update
		want   string
	}{
		{Update{TransferID: "a", State: WaitingForPeer}, `{"transferID":"a","state":"WAITING_FOR_PEER"}`},
		{Update{TransferID: "a", State: Transferring, Progress: &zero}, `{"transferID":"a","state":"TRANSFERRING","progress":0}`},
	}
	for _, c := range cases {
		data, err := json.Marshal(c.update)
		if err != nil || string(data) != c.want {
			t.Fatalf("marshal %q: got %s, %v, want %s", c.update.State, data, err, c.want)
		}
	}
}
