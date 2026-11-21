//go:build croc_no_tailcat

// This worker is a private protocol helper launched by the desktop host.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"croccoc/backend/transfer"
	"github.com/schollz/croc/v11/src/croc"
	crocLog "github.com/schollz/croc/v11/src/logger"
)

const (
	wireVersion   = 2
	requestLimit  = 8192
	workerTimeout = 3 * time.Minute
)

type startRequest struct {
	Version         int      `json:"version"`
	Mode            string   `json:"mode,omitempty"`
	Code            string   `json:"code"`
	RelayAddress    string   `json:"relayAddress"`
	RelayPassword   string   `json:"relayPassword"`
	ManifestDigest  string   `json:"manifestDigest,omitempty"`
	ContinuePayload bool     `json:"continuePayload,omitempty"`
	Paths           []string `json:"paths,omitempty"`
}

type manifestResult struct {
	Version int                      `json:"version"`
	Status  string                   `json:"status"`
	OfferID string                   `json:"offerID,omitempty"`
	Entries []transfer.ManifestEntry `json:"entries,omitempty"`
}

type decisionRequest struct {
	Version    int    `json:"version"`
	OfferID    string `json:"offerID"`
	Accept     *bool  `json:"accept"`
	PayloadDir string `json:"payloadDir,omitempty"`
}

var (
	errInvalidRequest  = errors.New("invalid worker request")
	errInvalidDecision = errors.New("invalid worker decision")
	errOfferRejected   = errors.New("offer rejected")
	errControlWrite    = errors.New("control stream write failed")
)

// readFrame reads exactly one bounded, newline-terminated JSON value. The
// worker never waits for stdin EOF to start or to process a decision.
func readFrame(r *bufio.Reader, dst any) error {
	data, err := r.ReadSlice('\n')
	if err != nil || len(data) > requestLimit {
		return errInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return errInvalidRequest
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return errInvalidRequest
	}
	return nil
}

func readRequest(r *bufio.Reader) (startRequest, error) {
	var request startRequest
	if err := readFrame(r, &request); err != nil {
		return startRequest{}, err
	}
	if request.Version != wireVersion ||
		request.Code == "" || len(request.Code) > 128 || strings.ContainsAny(request.Code, "\r\n\x00") ||
		request.RelayPassword == "" || len(request.RelayPassword) > 128 ||
		!isRelayAddress(request.RelayAddress) ||
		(request.Mode != "" && request.Mode != "send") ||
		(request.Mode == "send" && (len(request.Paths) == 0 || request.ContinuePayload || len(request.ManifestDigest) != 64)) ||
		(request.Mode != "send" && (request.ManifestDigest != "" || len(request.Paths) != 0)) {
		return startRequest{}, errInvalidRequest
	}
	return request, nil
}

func isRelayAddress(address string) bool {
	if address == "" || len(address) > 255 || strings.ContainsAny(address, "\r\n\x00") {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if addr, parseErr := netip.ParseAddr(host); parseErr == nil {
		if addr.Zone() != "" || addr.IsUnspecified() || addr.IsMulticast() {
			return false
		}
	} else if !validRelayHostname(host) {
		return false
	}
	p, err := strconv.ParseUint(port, 10, 16)
	return err == nil && p != 0
}

func validRelayHostname(host string) bool {
	if host == "" || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
				(r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// captureManifest lets the host review the authenticated manifest before Croc
// requests payload. Approved requests continue into Croc's normal receive path.
func captureManifest(ctx context.Context, request startRequest, consume func(context.Context, []transfer.ManifestEntry) error) error {
	client, err := croc.NewCtx(ctx, croc.Options{
		SharedSecret:     request.Code,
		RelayAddress:     request.RelayAddress,
		RelayPassword:    request.RelayPassword,
		DisableLocal:     true,
		DisableClipboard: true,
		NoUpdateCheck:    true,
		NoPrompt:         true,
		Overwrite:        false,
		Rename:           false,
		Curve:            "siec",
	})
	if err != nil {
		return err
	}
	manifestConsumer := func(ctx context.Context, info croc.SenderInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(info.FilesToTransfer) > transfer.MaxManifestEntries ||
			len(info.EmptyFoldersToTransfer) > transfer.MaxManifestEntries-len(info.FilesToTransfer) {
			return transfer.ErrResourceLimit
		}
		metadata := transfer.CrocReceiveMetadata{
			Files:            make([]transfer.CrocReceiveFile, len(info.FilesToTransfer)),
			EmptyDirectories: make([]string, len(info.EmptyFoldersToTransfer)),
			SendingText:      info.SendingText,
		}
		for i, file := range info.FilesToTransfer {
			metadata.Files[i] = transfer.CrocReceiveFile{
				Folder: file.FolderRemote, Name: file.Name, SizeBytes: file.Size,
				Mode: file.Mode, Symlink: file.Symlink != "", Archive: file.TempFile,
			}
		}
		for i, dir := range info.EmptyFoldersToTransfer {
			if dir.Name != "" || dir.Symlink != "" || dir.TempFile || dir.Size != 0 {
				return transfer.ErrUnsafeOffer
			}
			metadata.EmptyDirectories[i] = dir.FolderRemote
		}
		manifest, err := transfer.ProjectCrocReceiveManifest(metadata)
		if err != nil {
			return err
		}
		if err := consume(ctx, manifest); err != nil {
			return err
		}
		if request.ContinuePayload {
			return croc.ErrManifestContinue
		}
		return nil
	}
	if request.ContinuePayload {
		err = client.ReceiveManifestAndContinue(manifestConsumer)
	} else {
		err = client.ReceiveManifest(manifestConsumer)
	}
	return err
}

func runSend(ctx context.Context, request startRequest) error {
	expected, err := transfer.InspectSendSourcesMatching(request.Paths, request.ManifestDigest)
	if err != nil {
		return err
	}
	files, folders, count, err := croc.GetFilesInfo(request.Paths, false, false, nil)
	if err != nil {
		return err
	}
	actual, err := projectCrocSendManifest(files, folders)
	if err != nil {
		return err
	}
	actualDigest, err := transfer.SendManifestDigest(actual)
	expectedDigest, expectedErr := transfer.SendManifestDigest(expected)
	if err != nil || expectedErr != nil || actualDigest != expectedDigest {
		return transfer.ErrSourceChanged
	}
	client, err := croc.NewCtx(ctx, croc.Options{
		IsSender: true, SharedSecret: request.Code,
		RelayAddress: request.RelayAddress, RelayPassword: request.RelayPassword,
		DisableLocal: true, DisableClipboard: true, NoUpdateCheck: true,
		Curve: "siec", NoPrompt: true, Quiet: true,
	})
	if err != nil {
		return err
	}
	return client.Send(files, folders, count)
}

func projectCrocSendManifest(files, emptyFolders []croc.FileInfo) ([]transfer.ManifestEntry, error) {
	entries := make(map[string]transfer.ManifestEntry)
	addParents := func(relative string) {
		for parent := path.Dir(relative); parent != "." && parent != "/"; parent = path.Dir(parent) {
			if _, ok := entries[parent]; !ok {
				entries[parent] = transfer.ManifestEntry{Path: parent, Directory: true}
			}
		}
	}
	for _, file := range files {
		if !file.Mode.IsRegular() || file.Symlink != "" || file.TempFile {
			return nil, transfer.ErrSourceUnavailable
		}
		relative := path.Join(strings.ReplaceAll(file.FolderRemote, `\`, "/"), file.Name)
		addParents(relative)
		if _, exists := entries[relative]; exists {
			return nil, transfer.ErrInvalidInput
		}
		entries[relative] = transfer.ManifestEntry{Path: relative, SizeBytes: file.Size}
	}
	for _, folder := range emptyFolders {
		if folder.Name != "" || folder.Symlink != "" || folder.TempFile {
			return nil, transfer.ErrSourceUnavailable
		}
		relative := path.Clean(strings.ReplaceAll(folder.FolderRemote, `\`, "/"))
		if relative == "." {
			return nil, transfer.ErrInvalidInput
		}
		addParents(relative)
		if existing, exists := entries[relative]; exists && !existing.Directory {
			return nil, transfer.ErrInvalidInput
		}
		entries[relative] = transfer.ManifestEntry{Path: relative, Directory: true}
	}
	result := make([]transfer.ManifestEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	if _, err := transfer.ValidateManifest(result); err != nil {
		return nil, err
	}
	return result, nil
}

// waitDecision is the only reader for the second frame. If the deadline wins
// against a blocked anonymous pipe read, run ends and main exits the process.
func waitDecision(ctx context.Context, in *bufio.Reader, offerID string) error {
	_, err := waitDecisionRequest(ctx, in, offerID)
	return err
}

func waitDecisionRequest(ctx context.Context, in *bufio.Reader, offerID string) (decisionRequest, error) {
	type decisionResult struct {
		request decisionRequest
		err     error
	}
	done := make(chan decisionResult, 1)
	go func() {
		var d decisionRequest
		switch {
		case readFrame(in, &d) != nil || d.Version != wireVersion || d.OfferID != offerID || d.Accept == nil:
			done <- decisionResult{err: errInvalidDecision}
		case !*d.Accept:
			done <- decisionResult{request: d, err: errOfferRejected}
		default:
			done <- decisionResult{request: d}
		}
	}()
	select {
	case <-ctx.Done():
		return decisionRequest{}, ctx.Err()
	case result := <-done:
		if ctx.Err() != nil {
			return decisionRequest{}, ctx.Err()
		}
		return result.request, result.err
	}
}

func awaitOfferDecision(ctx context.Context, in *bufio.Reader, out *json.Encoder, entries []transfer.ManifestEntry, continuePayload bool) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	offerID := hex.EncodeToString(nonce[:])
	decisionCtx, cancel := context.WithTimeout(ctx, transfer.OfferDecisionTimeout)
	defer cancel()
	if decisionCtx.Err() != nil {
		return decisionCtx.Err()
	}
	if err := out.Encode(manifestResult{Version: wireVersion, Status: "awaiting_decision", OfferID: offerID, Entries: entries}); err != nil {
		return errControlWrite
	}
	decision, err := waitDecisionRequest(decisionCtx, in, offerID)
	if err != nil {
		return err
	}
	if !continuePayload {
		if decision.PayloadDir != "" {
			return errInvalidDecision
		}
		return nil
	}
	if decision.PayloadDir == "" || strings.ContainsAny(decision.PayloadDir, "\x00\r\n") {
		return errInvalidDecision
	}
	payloadDir, err := filepath.Abs(decision.PayloadDir)
	if err != nil || !filepath.IsAbs(payloadDir) {
		return errInvalidDecision
	}
	info, err := os.Stat(payloadDir)
	if err != nil || !info.IsDir() {
		return errInvalidDecision
	}
	if err := os.Chdir(payloadDir); err != nil {
		return errInvalidDecision
	}
	return nil
}

func run(ctx context.Context, in io.Reader, out io.Writer) int {
	reader := bufio.NewReaderSize(in, requestLimit)
	writer := json.NewEncoder(out)
	result := manifestResult{Version: wireVersion, Status: "invalid_request"}
	request, err := readRequest(reader)
	if err == nil {
		if request.Mode == "send" {
			err = runSend(ctx, request)
		} else {
			err = captureManifest(ctx, request, func(ctx context.Context, entries []transfer.ManifestEntry) error {
				return awaitOfferDecision(ctx, reader, writer, entries, request.ContinuePayload)
			})
		}
		switch {
		case err == nil && request.Mode == "send":
			result.Status = "completed"
		case err == nil && request.ContinuePayload:
			result.Status = "completed"
		case errors.Is(err, context.DeadlineExceeded):
			result.Status = "timeout"
		case ctx.Err() != nil || errors.Is(err, context.Canceled):
			result.Status = "cancelled"
		case errors.Is(err, transfer.ErrSourceUnavailable):
			result.Status = "source_unavailable"
		case errors.Is(err, transfer.ErrSourceChanged):
			result.Status = "source_changed"
		case errors.Is(err, transfer.ErrInvalidInput):
			result.Status = "invalid_input"
		case errors.Is(err, croc.ErrManifestOnly):
			result.Status = "accepted_no_payload"
		case errors.Is(err, errOfferRejected):
			result.Status = "rejected"
		case errors.Is(err, errInvalidDecision):
			result.Status = "invalid_decision"
		case errors.Is(err, transfer.ErrUnsafeOffer):
			result.Status = "unsafe_offer"
		case errors.Is(err, transfer.ErrResourceLimit):
			result.Status = "resource_limit"
		default:
			result.Status = "metadata_failed"
		}
	}
	if err := writer.Encode(result); err != nil {
		return 1
	}
	switch result.Status {
	case "accepted_no_payload", "rejected", "completed":
		return 0
	default:
		return 1
	}
}

func main() {
	// Only structured frames use the inherited stdout pipe. Any terminal
	// output from upstream Go code stays outside the control stream.
	control := os.Stdout
	sink, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil || len(os.Args) != 1 {
		os.Exit(1)
	}
	os.Stdout, os.Stderr = sink, sink
	crocLog.SetOutput(io.Discard)

	ctx, cancel := context.WithTimeout(context.Background(), workerTimeout)
	timer := time.AfterFunc(workerTimeout+time.Second, func() { os.Exit(1) })
	code := run(ctx, os.Stdin, control)
	timer.Stop()
	cancel()
	sink.Close()
	os.Exit(code)
}
