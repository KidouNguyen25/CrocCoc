package main

import (
	"errors"
	"slices"
	"testing"

	"croccoc/backend/transfer"
)

type recordingService struct {
	paths      []string
	code       string
	dir        string
	canceledID string
	receiveErr error
}

func (s *recordingService) GetCrocInfo() transfer.CrocInfo {
	return transfer.CrocInfo{Available: true, Version: "test-version"}
}

func (s *recordingService) StartSend(paths []string) (string, error) {
	s.paths = paths
	return "send-id", nil
}

func (s *recordingService) StartReceive(code, dir string) (string, error) {
	s.code, s.dir = code, dir
	return "", s.receiveErr
}

func (s *recordingService) CancelTransfer(id string) (bool, error) {
	s.canceledID = id
	return true, nil
}

func TestWailsBridgeDelegatesToService(t *testing.T) {
	backend := &recordingService{receiveErr: errors.New("receive unavailable")}
	app := newApp(backend)
	if info := app.GetCrocInfo(); !info.Available || info.Version != "test-version" {
		t.Fatalf("unexpected info: %#v", info)
	}
	paths := []string{"file with spaces.txt", "thư mục"}
	if id, err := app.StartSend(paths); id != "send-id" || err != nil || !slices.Equal(paths, backend.paths) {
		t.Fatalf("unexpected send result: %q, %v", id, err)
	}
	if id, err := app.StartReceive("sample-code", "Downloads"); id != "" || !errors.Is(err, backend.receiveErr) || backend.code != "sample-code" || backend.dir != "Downloads" {
		t.Fatalf("unexpected receive result: %q, %v", id, err)
	}
	if ok, err := app.CancelTransfer("send-id"); !ok || err != nil || backend.canceledID != "send-id" {
		t.Fatalf("unexpected cancel result: %t, %v", ok, err)
	}
	if snap := app.GetTransferSnapshot("send-id"); snap != nil {
		t.Fatalf("expected nil snapshot for non-controller service, got %#v", snap)
	}
	if offer := app.GetReceiveOffer("send-id"); offer != nil {
		t.Fatalf("expected nil offer for non-controller service, got %#v", offer)
	}
	if err := app.CopySendCode("send-id"); !errors.Is(err, transfer.ErrNotIntegrated) {
		t.Fatalf("expected ErrNotIntegrated for CopySendCode, got %v", err)
	}
	if _, err := app.SelectPaths("sendFiles"); err == nil {
		t.Fatal("expected error when desktop runtime context is not initialized")
	}
}

type fakeControllerService struct {
	recordingService
	snapshot    *transfer.SessionSnapshot
	offer       *transfer.ReceiveOffer
	sendCode    string
	decided     bool
	shutdownRan bool
}

func (f *fakeControllerService) DecideReceive(transferID, offerID string, accept bool) (bool, error) {
	f.decided = accept
	return transferID == "recv-1" && offerID == "offer-1", nil
}

func (f *fakeControllerService) GetReceiveOffer(transferID string) (transfer.ReceiveOffer, bool) {
	if f.offer != nil && transferID == "recv-1" {
		return *f.offer, true
	}
	return transfer.ReceiveOffer{}, false
}

func (f *fakeControllerService) GetTransferSnapshot(transferID string) (transfer.SessionSnapshot, bool) {
	if f.snapshot != nil && transferID == f.snapshot.TransferID {
		return *f.snapshot, true
	}
	return transfer.SessionSnapshot{}, false
}

func (f *fakeControllerService) GetSendCode(transferID string) (string, bool) {
	if f.sendCode != "" && transferID == "send-1" {
		return f.sendCode, true
	}
	return "", false
}

func (f *fakeControllerService) Shutdown() {
	f.shutdownRan = true
}

func TestAppControllerBindingsReturnPointersAndFailClosedOnUnknownCopy(t *testing.T) {
	controller := &fakeControllerService{
		snapshot: &transfer.SessionSnapshot{TransferID: "recv-1", Revision: 2, State: transfer.WaitingForPeer},
		offer:    &transfer.ReceiveOffer{OfferID: "offer-1", FileCount: 1, TotalBytes: "12"},
	}
	app := newApp(controller)

	if snap := app.GetTransferSnapshot("missing"); snap != nil {
		t.Fatalf("expected nil snapshot for missing id, got %+v", snap)
	}
	if snap := app.GetTransferSnapshot("recv-1"); snap == nil || snap.Revision != 2 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if offer := app.GetReceiveOffer("missing"); offer != nil {
		t.Fatalf("expected nil offer for missing id, got %+v", offer)
	}
	if offer := app.GetReceiveOffer("recv-1"); offer == nil || offer.OfferID != "offer-1" {
		t.Fatalf("unexpected offer: %+v", offer)
	}
	if err := app.CopySendCode("missing"); !errors.Is(err, transfer.ErrCopyUnsafe) {
		t.Fatalf("expected ErrCopyUnsafe for missing send code, got %v", err)
	}
	app.shutdown()
	if !controller.shutdownRan {
		t.Fatal("shutdown did not delegate to controller")
	}
}
