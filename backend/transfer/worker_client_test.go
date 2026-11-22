package transfer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const testWorkerModeEnv = "CROCCOC_TEST_WORKER_MODE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(testWorkerModeEnv); mode != "" {
		runTestWorker(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runTestWorker(mode string) {
	var request workerStartRequest
	if json.NewDecoder(os.Stdin).Decode(&request) != nil || request.Version != workerWireVersion {
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	if mode == "stalled" {
		if encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "waiting_for_peer"}) != nil {
			os.Exit(3)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "payload-stalled" {
		if encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "waiting_for_peer"}) != nil ||
			encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "transferring"}) != nil {
			os.Exit(3)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	if encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "waiting_for_peer"}) != nil ||
		encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "transferring"}) != nil ||
		encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "progress", ProgressBytes: 64}) != nil ||
		encoder.Encode(workerManifestFrame{Version: workerWireVersion, Status: "completed"}) != nil {
		os.Exit(4)
	}
}

func TestWorkerClientSendConsumesProgressFrames(t *testing.T) {
	t.Setenv(testWorkerModeEnv, "success")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	worker := NewWorkerClient(os.Args[0])
	transferStarted := false
	var progressBytes int64
	err := worker.send(ctx, "private-code", "relay.example.test:9009", "private-password",
		[]string{"source.txt"}, strings.Repeat("a", 64), func() { transferStarted = true }, func(delta int64) { progressBytes += delta })
	if err != nil {
		t.Fatalf("worker send failed: %v", err)
	}
	if !transferStarted {
		t.Fatal("worker transfer start was not reported")
	}
	if progressBytes <= 0 {
		t.Fatalf("worker progress bytes were not forwarded: %d", progressBytes)
	}
}

func TestWorkerClientSendAcceptsShortSecretsThatMatchJSONKeys(t *testing.T) {
	t.Setenv(testWorkerModeEnv, "success")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, password := range []string{"2", "status", "version"} {
		err := NewWorkerClient(os.Args[0]).send(ctx, "private-code", "relay.example.test:9009", password,
			[]string{"source.txt"}, strings.Repeat("a", 64), nil, nil)
		if err != nil {
			t.Fatalf("password %q rejected: %v", password, err)
		}
	}
}

func TestWorkerClientSendRejectsRequestOverWorkerLimit(t *testing.T) {
	t.Setenv(testWorkerModeEnv, "success")
	paths := make([]string, 100)
	for i := range paths {
		paths[i] = `C:\` + strings.Repeat("d", 100)
	}
	err := NewWorkerClient(os.Args[0]).send(t.Context(), "private-code", "relay.example.test:9009",
		"private-password", paths, strings.Repeat("a", 64), nil, nil)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("oversized request = %v, want ErrResourceLimit", err)
	}
}

func TestWorkerClientPeerDeadlineStopsWorker(t *testing.T) {
	t.Setenv(testWorkerModeEnv, "stalled")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd, job, stdin, stdout, stop, err := startWorker(ctx, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = stopWorker(cmd, job, stop)
		}
	}()
	defer stdout.Close()
	reader := bufio.NewReader(stdout)
	if err := json.NewEncoder(stdin).Encode(workerStartRequest{Version: workerWireVersion}); err != nil {
		t.Fatal(err)
	}
	var ready workerManifestFrame
	if err := readWorkerFrameWithin(ctx, reader, &ready, time.Second, errWorkerFrameTimeout); err != nil || ready.Status != "waiting_for_peer" {
		t.Fatalf("worker readiness frame: %+v, %v", ready, err)
	}
	_, err = readWorkerCompletion(ctx, cmd, job, stop, stdin, reader, 25*time.Millisecond, ErrPeerTimeout,
		PayloadIdleTimeout, nil, nil)
	stopped = true
	if !errors.Is(err, ErrPeerTimeout) {
		t.Fatalf("peer timeout was not enforced: %v", err)
	}
}

func TestWorkerClientPayloadDeadlineStopsWorker(t *testing.T) {
	t.Setenv(testWorkerModeEnv, "payload-stalled")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd, job, stdin, stdout, stop, err := startWorker(ctx, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = stopWorker(cmd, job, stop)
		}
	}()
	defer stdout.Close()
	reader := bufio.NewReader(stdout)
	if err := json.NewEncoder(stdin).Encode(workerStartRequest{Version: workerWireVersion}); err != nil {
		t.Fatal(err)
	}
	var ready workerManifestFrame
	if err := readWorkerFrameWithin(ctx, reader, &ready, time.Second, errWorkerFrameTimeout); err != nil || ready.Status != "waiting_for_peer" {
		t.Fatalf("worker readiness frame: %+v, %v", ready, err)
	}
	_, err = readWorkerCompletion(ctx, cmd, job, stop, stdin, reader, 25*time.Millisecond, ErrPeerTimeout,
		25*time.Millisecond, nil, nil)
	stopped = true
	if !errors.Is(err, ErrPayloadTimeout) {
		t.Fatalf("payload idle timeout was not enforced: %v", err)
	}
}

func TestWorkerTimeoutValuesMatchPlan(t *testing.T) {
	if PreparationTimeout != 2*time.Minute || PeerWaitTimeout != 5*time.Minute ||
		OfferDecisionTimeout != 2*time.Minute || PayloadIdleTimeout != 2*time.Minute ||
		WorkerCleanupTimeout != 10*time.Second {
		t.Fatal("worker phase deadlines differ from backend plan")
	}
}
