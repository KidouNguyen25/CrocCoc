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
}
