//go:build croc_no_tailcat

// This worker is a loopback-only protocol spike, not part of the desktop binaries.
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
	Version       int    `json:"version"`
	Code          string `json:"code"`
	RelayAddress  string `json:"relayAddress"`
	RelayPassword string `json:"relayPassword"`
	ContinuePayload bool   `json:"continuePayload,omitempty"`
}

type manifestResult struct {
	Version int                      `json:"version"`
	Status  string                   `json:"status"`
	OfferID string                   `json:"offerID,omitempty"`
	Entries []transfer.ManifestEntry `json:"entries,omitempty"`
}

type decisionRequest struct {
	Version int    `json:"version"`
	OfferID string `json:"offerID"`
	Accept  *bool  `json:"accept"`
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
		!isLoopbackRelay(request.RelayAddress) {
		return startRequest{}, errInvalidRequest
	}
	return request, nil
}

func isLoopbackRelay(address string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || addr.Zone() != "" || !addr.IsLoopback() {
		return false
	}
	p, err := strconv.ParseUint(port, 10, 16)
	return err == nil && p != 0
}

// captureManifest holds the authenticated receive callback open while consume
// handles the offer. The pinned Croc hook still stops before payload I/O.
func captureManifest(ctx context.Context, request startRequest, consume func(context.Context, []transfer.ManifestEntry) error) error {
	client, err := croc.NewCtx(ctx, croc.Options{
		SharedSecret:     request.Code,
		RelayAddress:     request.RelayAddress,
		RelayPassword:    request.RelayPassword,
		DisableLocal:     true,
		DisableClipboard: true,
		NoUpdateCheck:    true,
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

// waitDecision is the only reader for the second frame. If the deadline wins
// against a blocked anonymous pipe read, run ends and main exits the process.
func waitDecision(ctx context.Context, in *bufio.Reader, offerID string) error {
	done := make(chan error, 1)
	go func() {
		var d decisionRequest
		switch {
		case readFrame(in, &d) != nil || d.Version != wireVersion || d.OfferID != offerID || d.Accept == nil:
			done <- errInvalidDecision
		case !*d.Accept:
			done <- errOfferRejected
		default:
			done <- nil
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
}

func awaitOfferDecision(ctx context.Context, in *bufio.Reader, out *json.Encoder, entries []transfer.ManifestEntry) error {
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
	return waitDecision(decisionCtx, in, offerID)
}

func run(ctx context.Context, in io.Reader, out io.Writer) int {
	reader := bufio.NewReaderSize(in, requestLimit)
	writer := json.NewEncoder(out)
	result := manifestResult{Version: wireVersion, Status: "invalid_request"}
	request, err := readRequest(reader)
	if err == nil {
		err = captureManifest(ctx, request, func(ctx context.Context, entries []transfer.ManifestEntry) error {
			return awaitOfferDecision(ctx, reader, writer, entries)
		})
		switch {
		case err == nil && request.ContinuePayload:
			result.Status = "completed"
		case ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			result.Status = "cancelled"
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
