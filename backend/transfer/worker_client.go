package transfer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

const workerWireVersion = 2

var (
	ErrWorkerUnavailable    = errors.New("ENGINE_UNAVAILABLE: không thể khởi chạy engine")
	ErrWorkerProtocol       = errors.New("ENGINE_FAILED: engine trả giao thức không hợp lệ")
	ErrPayloadNotIntegrated = errors.New("ENGINE_UNAVAILABLE: engine chưa hỗ trợ payload")
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
	Version int             `json:"version"`
	Status  string          `json:"status"`
	OfferID string          `json:"offerID,omitempty"`
	Entries []ManifestEntry `json:"entries,omitempty"`
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
}

func NewWorkerClient(path string) *WorkerClient {
	return &WorkerClient{path: strings.TrimSpace(path)}
}

// ReceiveManifest obtains one sanitized offer and sends exactly one decision.
// The current helper stops before payload I/O; callers must stage and publish
// through the native receive path after a future payload-capable worker.
func (w *WorkerClient) ReceiveManifest(
	ctx context.Context,
	code, relayAddress, relayPassword string,
	decide func(ReceiveOffer) (bool, string, error),
) error {
	return w.receiveManifest(ctx, "", false, code, relayAddress, relayPassword, decide)
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
	return w.receiveManifest(ctx, workingDir, true, code, relayAddress, relayPassword, decide)
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
	if w == nil || w.path == "" || ctx == nil || code == "" || relayPassword == "" ||
		len(paths) == 0 || len(manifestDigest) != 64 || strings.ContainsAny(code, "\r\n\x00") ||
		strings.ContainsAny(relayPassword, "\r\n\x00") {
		return ErrWorkerProtocol
	}
	cmd := exec.CommandContext(ctx, w.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ErrWorkerUnavailable
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return ErrWorkerUnavailable
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return ErrWorkerUnavailable
	}
	defer func() { _ = stdin.Close(); _ = stdout.Close() }()
	if err := json.NewEncoder(stdin).Encode(workerStartRequest{
		Version: workerWireVersion, Mode: "send", Code: code,
		RelayAddress: relayAddress, RelayPassword: relayPassword,
		ManifestDigest: manifestDigest, Paths: append([]string(nil), paths...),
	}); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}
	reader := bufio.NewReaderSize(stdout, 8192)
	var result workerManifestFrame
	if err := readWorkerFrame(reader, &result, code, relayPassword); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		return ErrWorkerProtocol
	}
	if ctx.Err() != nil {
		return ctx.Err()
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
) error {
	if w == nil || w.path == "" || ctx == nil || decide == nil ||
		code == "" || relayPassword == "" ||
		strings.ContainsAny(code, "\r\n\x00") || strings.ContainsAny(relayPassword, "\r\n\x00") {
		return ErrWorkerProtocol
	}
	cmd := exec.CommandContext(ctx, w.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ErrWorkerUnavailable
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return ErrWorkerUnavailable
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return ErrWorkerUnavailable
	}
	cleanup := func() {
		_ = stdin.Close()
		_ = stdout.Close()
	}
	defer cleanup()

	encoder := json.NewEncoder(stdin)
	request := workerStartRequest{
		Version: workerWireVersion, Code: code,
		RelayAddress: relayAddress, RelayPassword: relayPassword,
		ContinuePayload: continuePayload,
	}
	if err := encoder.Encode(request); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}

	reader := bufio.NewReaderSize(stdout, 8192)
	var offerFrame workerManifestFrame
	if err := readWorkerFrame(reader, &offerFrame, code, relayPassword); err != nil ||
		offerFrame.Version != workerWireVersion || offerFrame.Status != "awaiting_decision" ||
		offerFrame.OfferID == "" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}
	offer, err := PrepareReceiveOffer(offerFrame.Entries, nowUTC())
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
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
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}
	var result workerManifestFrame
	if err := readWorkerFrame(reader, &result, code, relayPassword); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ErrWorkerProtocol
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		return ErrWorkerProtocol
	}
	if ctx.Err() != nil {
		return ctx.Err()
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
	case "metadata_failed", "cancelled":
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
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(frame, []byte(secret)) {
			return ErrWorkerProtocol
		}
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

func nowUTC() time.Time {
	return time.Now().UTC()
}
