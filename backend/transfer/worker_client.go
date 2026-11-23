package transfer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	workerWireVersion  = 2
	workerRequestLimit = 8192 // keep equal to requestLimit in the worker
)

var (
	ErrWorkerUnavailable    = errors.New("ENGINE_UNAVAILABLE: không thể khởi chạy engine")
	ErrRelayUnavailable     = errors.New("RELAY_UNAVAILABLE: không tới được máy chủ relay hoặc kết nối relay bị gián đoạn; hãy kiểm tra mạng và các cổng 9009–9013")
	ErrWorkerProtocol       = errors.New("ENGINE_FAILED: engine trả giao thức không hợp lệ")
	ErrPayloadNotIntegrated = errors.New("ENGINE_UNAVAILABLE: engine chưa hỗ trợ payload")
	ErrPreparationTimeout   = errors.New("PREPARATION_TIMEOUT: chuẩn bị truyền quá thời hạn")
	ErrPayloadTimeout       = errors.New("PAYLOAD_TIMEOUT: truyền không có tiến triển quá thời hạn")
	ErrWorkerCleanupPending = errors.New("CLEANUP_FAILED: không xác nhận được worker đã dừng")
)

type workerStartRequest struct {
	Version         int      `json:"version"`
	Mode            string   `json:"mode,omitempty"`
	Code            string   `json:"code"`
	RelayAddress    string   `json:"relayAddress"`
	RelayPassword   string   `json:"relayPassword"`
	ManifestDigest  string   `json:"manifestDigest,omitempty"`
	ContinuePayload bool     `json:"continuePayload,omitempty"`
	Paths           []string `json:"paths,omitempty"`
}

type workerManifestFrame struct {
	Version       int             `json:"version"`
	Status        string          `json:"status"`
	OfferID       string          `json:"offerID,omitempty"`
	Entries       []ManifestEntry `json:"entries,omitempty"`
	ProgressBytes int64           `json:"progressBytes,omitempty"`
}

type workerDecisionFrame struct {
	Version    int    `json:"version"`
	OfferID    string `json:"offerID"`
	Accept     *bool  `json:"accept"`
	PayloadDir string `json:"payloadDir,omitempty"`
}

// WorkerClient speaks to the private engine helper over stdin/stdout. The
// helper path is supplied by the host; credentials never enter argv or env.
type WorkerClient struct {
	path string
	// Only test fixtures supply extra environment variables; production leaves this empty.
	testEnv []string
}

func NewWorkerClient(path string) *WorkerClient {
	return &WorkerClient{path: strings.TrimSpace(path)}
}

func workerEnvironment() []string {
	// The worker must not inherit credentials, proxies or personal Croc config.
	// Only OS paths required for Windows startup and temporary files are passed.
	keys := []string{"SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func startWorker(ctx context.Context, path string, testEnv ...string) (*exec.Cmd, *workerJob, io.WriteCloser, io.ReadCloser, context.CancelFunc, error) {
	workerCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(workerCtx, path)
	cmd.Env = append(workerEnvironment(), testEnv...)
	cmd.WaitDelay = WorkerCleanupTimeout
	if err := prepareWorkerCommand(cmd); err != nil {
		cancel()
		return nil, nil, nil, nil, nil, ErrWorkerUnavailable
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, nil, nil, ErrWorkerUnavailable
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return nil, nil, nil, nil, nil, ErrWorkerUnavailable
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		cancel()
		return nil, nil, nil, nil, nil, ErrWorkerUnavailable
	}
	job, err := attachWorkerJob(cmd)
	if err != nil {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, nil, nil, nil, ErrWorkerUnavailable
	}
	return cmd, job, stdin, stdout, cancel, nil
}

func stopWorker(cmd *exec.Cmd, job *workerJob, cancel context.CancelFunc) error {
	cancel()
	if job != nil {
		job.Terminate()
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	err := waitWorkerProcess(cmd, job, cancel)
	if job != nil {
		job.Close()
	}
	return err
}

func waitWorker(cmd *exec.Cmd, job *workerJob, stdin io.WriteCloser, cancel context.CancelFunc) error {
	_ = stdin.Close()
	err := waitWorkerProcess(cmd, job, cancel)
	if job != nil {
		job.Close()
	}
	return err
}

func waitWorkerProcess(cmd *exec.Cmd, job *workerJob, cancel context.CancelFunc) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(WorkerCleanupTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if errors.Is(err, exec.ErrWaitDelay) {
			return ErrWorkerCleanupPending
		}
		return nil
	case <-timer.C:
		cancel()
		if job != nil {
			job.Terminate()
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return ErrWorkerCleanupPending
	}
}

func readWorkerFrameWithin(ctx context.Context, reader *bufio.Reader, dst any, timeout time.Duration, timeoutErr error, secrets ...string) error {
	read := make(chan error, 1)
	go func() { read <- readWorkerFrame(reader, dst, secrets...) }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-read:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return timeoutErr
	}
}

func readWorkerCompletion(ctx context.Context, cmd *exec.Cmd, job *workerJob, cancel context.CancelFunc, stdin io.WriteCloser,
	reader *bufio.Reader, firstTimeout time.Duration, firstTimeoutErr error, payloadTimeout time.Duration,
	onTransferStarted func(), onProgress func(int64), secrets ...string) (workerManifestFrame, error) {
	timeout := firstTimeout
	timeoutErr := firstTimeoutErr
	for {
		var frame workerManifestFrame
		if err := readWorkerFrameWithin(ctx, reader, &frame, timeout, errWorkerFrameTimeout, secrets...); err != nil {
			if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
				return workerManifestFrame{}, cleanupErr
			}
			if ctx.Err() != nil {
				return workerManifestFrame{}, ctx.Err()
			}
			if errors.Is(err, errWorkerFrameTimeout) {
				return workerManifestFrame{}, timeoutErr
			}
			return workerManifestFrame{}, ErrWorkerProtocol
		}
		if frame.Version != workerWireVersion {
			if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
				return workerManifestFrame{}, cleanupErr
			}
			return workerManifestFrame{}, ErrWorkerProtocol
		}
		switch frame.Status {
		case "progress":
			if frame.ProgressBytes <= 0 {
				if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
					return workerManifestFrame{}, cleanupErr
				}
				return workerManifestFrame{}, ErrWorkerProtocol
			}
			if onProgress != nil {
				onProgress(frame.ProgressBytes)
			}
			timeout = payloadTimeout
			timeoutErr = ErrPayloadTimeout
		case "transferring":
			if onTransferStarted != nil {
				onTransferStarted()
				onTransferStarted = nil
			}
			timeout = payloadTimeout
			timeoutErr = ErrPayloadTimeout
		default:
			if err := waitWorker(cmd, job, stdin, cancel); err != nil {
				return workerManifestFrame{}, err
			}
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 0 {
				switch frame.Status {
				case "completed", "rejected", "accepted_no_payload":
					return workerManifestFrame{}, ErrWorkerProtocol
				}
			}
			return frame, nil
		}
	}
}

var errWorkerFrameTimeout = errors.New("worker frame timeout")

// ReceiveManifest obtains one sanitized offer and sends exactly one decision.
// This method stops before payload I/O, even when the caller accepts.
func (w *WorkerClient) ReceiveManifest(
	ctx context.Context,
	code, relayAddress, relayPassword string,
	decide func(ReceiveOffer) (bool, string, error),
) error {
	return w.receiveManifest(ctx, "", false, code, relayAddress, relayPassword, decide, nil)
}

// ReceiveAndContinue runs payload receive inside workingDir after the
// callback accepts the frozen offer. The caller must provide a private stage
// directory and audit it before publish.
func (w *WorkerClient) ReceiveAndContinue(
	ctx context.Context,
	workingDir string,
	code, relayAddress, relayPassword string,
	decide func(ReceiveOffer) (bool, string, error),
) error {
	return w.receiveManifest(ctx, workingDir, true, code, relayAddress, relayPassword, decide, nil)
}

// ReceiveAndContinueWithProgress extends ReceiveAndContinue with a delta-byte
// progress callback invoked on verified worker progress frames.
func (w *WorkerClient) ReceiveAndContinueWithProgress(
	ctx context.Context,
	workingDir string,
	code, relayAddress, relayPassword string,
	decide func(ReceiveOffer) (bool, string, error),
	onProgress func(int64),
) error {
	return w.receiveManifest(ctx, workingDir, true, code, relayAddress, relayPassword, decide, onProgress)
}

// Send transfers validated source paths through private worker stdin. Paths
// never enter argv or environment. The worker revalidates sources before it
// opens Croc.
func (w *WorkerClient) Send(
	ctx context.Context,
	code, relayAddress, relayPassword string,
	paths []string,
	manifestDigest string,
) error {
	return w.send(ctx, code, relayAddress, relayPassword, paths, manifestDigest, nil, nil)
}

func (w *WorkerClient) send(
	ctx context.Context,
	code, relayAddress, relayPassword string,
	paths []string,
	manifestDigest string,
	onTransferStarted func(),
	onProgress func(int64),
) error {
	if w == nil || w.path == "" || ctx == nil || code == "" || relayPassword == "" ||
		len(paths) == 0 || len(manifestDigest) != 64 || strings.ContainsAny(code, "\r\n\x00") ||
		strings.ContainsAny(relayPassword, "\r\n\x00") {
		return ErrWorkerProtocol
	}
	request, err := json.Marshal(workerStartRequest{
		Version: workerWireVersion, Mode: "send", Code: code,
		RelayAddress: relayAddress, RelayPassword: relayPassword,
		ManifestDigest: manifestDigest, Paths: append([]string(nil), paths...),
	})
	if err != nil {
		return ErrWorkerProtocol
	}
	// The worker rejects larger start frames; fail before launching it.
	if len(request)+1 > workerRequestLimit {
		return ErrResourceLimit
	}
	cmd, job, stdin, stdout, cancel, err := startWorker(ctx, w.path, w.testEnv...)
	if err != nil {
		return err
	}
	defer cancel()
	defer func() { _ = stdin.Close(); _ = stdout.Close(); job.Close() }()
	if _, err := stdin.Write(append(request, '\n')); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		return ErrWorkerProtocol
	}
	reader := bufio.NewReaderSize(stdout, 8192)
	var ready workerManifestFrame
	if err := readWorkerFrameWithin(ctx, reader, &ready, PreparationTimeout, errWorkerFrameTimeout, code, relayPassword); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errWorkerFrameTimeout) {
			return ErrRelayUnavailable
		}
		return ErrWorkerProtocol
	}
	if ready.Version != workerWireVersion || ready.Status != "waiting_for_peer" {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if ready.Version != workerWireVersion {
			return ErrWorkerProtocol
		}
		return workerStatusError(ready.Status)
	}
	result, err := readWorkerCompletion(ctx, cmd, job, cancel, stdin, reader, PeerWaitTimeout, ErrPeerTimeout,
		PayloadIdleTimeout, onTransferStarted, onProgress, code, relayPassword)
	if err != nil {
		return err
	}
	if result.Status != "completed" {
		return workerStatusError(result.Status)
	}
	return nil
}

func (w *WorkerClient) receiveManifest(
	ctx context.Context,
	workingDir string,
	continuePayload bool,
	code, relayAddress, relayPassword string,
	decide func(ReceiveOffer) (bool, string, error),
	onProgress func(int64),
) error {
	if w == nil || w.path == "" || ctx == nil || decide == nil ||
		code == "" || relayPassword == "" ||
		strings.ContainsAny(code, "\r\n\x00") || strings.ContainsAny(relayPassword, "\r\n\x00") {
		return ErrWorkerProtocol
	}
	requestBytes, err := json.Marshal(workerStartRequest{
		Version: workerWireVersion, Code: code,
		RelayAddress: relayAddress, RelayPassword: relayPassword,
		ContinuePayload: continuePayload,
	})
	if err != nil {
		return ErrWorkerProtocol
	}
	if len(requestBytes)+1 > workerRequestLimit {
		return ErrResourceLimit
	}
	cmd, job, stdin, stdout, cancel, err := startWorker(ctx, w.path, w.testEnv...)
	if err != nil {
		return err
	}
	defer cancel()
	cleanup := func() {
		_ = stdin.Close()
		_ = stdout.Close()
		job.Close()
	}
	defer cleanup()

	encoder := json.NewEncoder(stdin)
	if _, err := stdin.Write(append(requestBytes, '\n')); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		return ErrWorkerProtocol
	}

	reader := bufio.NewReaderSize(stdout, 8192)
	var ready workerManifestFrame
	if err := readWorkerFrameWithin(ctx, reader, &ready, PreparationTimeout, errWorkerFrameTimeout, code, relayPassword); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errWorkerFrameTimeout) {
			return ErrRelayUnavailable
		}
		return ErrWorkerProtocol
	}
	if ready.Version != workerWireVersion || ready.Status != "waiting_for_peer" {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if ready.Version != workerWireVersion {
			return ErrWorkerProtocol
		}
		return workerStatusError(ready.Status)
	}
	var offerFrame workerManifestFrame
	if err := readWorkerFrameWithin(ctx, reader, &offerFrame, PeerWaitTimeout, errWorkerFrameTimeout, code, relayPassword); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errWorkerFrameTimeout) {
			return ErrPeerTimeout
		}
		return ErrWorkerProtocol
	}
	if offerFrame.Version != workerWireVersion || offerFrame.Status != "awaiting_decision" || offerFrame.OfferID == "" {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		if offerFrame.Version != workerWireVersion {
			return ErrWorkerProtocol
		}
		return workerStatusError(offerFrame.Status)
	}
	offer, err := PrepareReceiveOffer(offerFrame.Entries, nowUTC())
	if err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		return err
	}
	offer.OfferID = offerFrame.OfferID
	accept, payloadDir, err := decide(offer)
	decisionErr := err
	if err != nil {
		accept = false
	}
	if continuePayload && accept && payloadDir == "" {
		payloadDir = workingDir
	}
	if !continuePayload {
		payloadDir = ""
	}
	if err := encoder.Encode(workerDecisionFrame{
		Version: workerWireVersion, OfferID: offer.OfferID, Accept: &accept, PayloadDir: payloadDir,
	}); err != nil {
		if cleanupErr := stopWorker(cmd, job, cancel); cleanupErr != nil {
			return cleanupErr
		}
		return ErrWorkerProtocol
	}
	result, err := readWorkerCompletion(ctx, cmd, job, cancel, stdin, reader, PayloadIdleTimeout, ErrPayloadTimeout,
		PayloadIdleTimeout, nil, onProgress, code, relayPassword)
	if err != nil {
		return err
	}
	switch result.Status {
	case "accepted_no_payload":
		if !accept || continuePayload {
			return ErrWorkerProtocol
		}
		if decisionErr != nil {
			return decisionErr
		}
		return ErrPayloadNotIntegrated
	case "completed":
		if !accept || !continuePayload {
			return ErrWorkerProtocol
		}
		if decisionErr != nil {
			return decisionErr
		}
		return nil
	case "rejected":
		if accept {
			return ErrWorkerProtocol
		}
		if decisionErr != nil {
			return decisionErr
		}
		return nil
	default:
		return workerStatusError(result.Status)
	}
}

func workerStatusError(status string) error {
	switch status {
	case "unsafe_offer":
		return ErrUnsafeOffer
	case "resource_limit":
		return ErrResourceLimit
	case "source_unavailable":
		return ErrSourceUnavailable
	case "source_changed":
		return ErrSourceChanged
	case "invalid_input":
		return ErrInvalidInput
	case "timeout":
		return ErrPeerTimeout
	case "metadata_failed":
		return ErrRelayUnavailable
	case "cancelled":
		return ErrTransferFailed
	default:
		return ErrWorkerProtocol
	}
}

func readWorkerFrame(reader *bufio.Reader, dst any, secrets ...string) error {
	frame, err := reader.ReadBytes('\n')
	if err != nil || len(frame) > 8192 || bytes.IndexByte(frame, 0) >= 0 {
		return ErrWorkerProtocol
	}
	// Match secrets against string values only, so a short secret cannot
	// collide with JSON keys or numbers such as "version":2.
	var generic any
	if json.Unmarshal(frame, &generic) != nil || jsonValuesContain(generic, secrets) {
		return ErrWorkerProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(frame))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return ErrWorkerProtocol
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return ErrWorkerProtocol
	}
	return nil
}

func jsonValuesContain(value any, secrets []string) bool {
	switch v := value.(type) {
	case string:
		for _, secret := range secrets {
			if secret != "" && strings.Contains(v, secret) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if jsonValuesContain(item, secrets) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if jsonValuesContain(item, secrets) {
				return true
			}
		}
	}
	return false
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
