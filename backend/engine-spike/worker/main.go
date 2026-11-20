//go:build croc_no_tailcat

// This worker is a loopback-only protocol spike, not part of the desktop binaries.
package main

import (
	"bytes"
	"context"
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
	wireVersion   = 1
	requestLimit  = 8192
	workerTimeout = 2 * time.Minute
)

type startRequest struct {
	Version       int    `json:"version"`
	Code          string `json:"code"`
	RelayAddress  string `json:"relayAddress"`
	RelayPassword string `json:"relayPassword"`
}

type manifestResult struct {
	Version int                      `json:"version"`
	Status  string                   `json:"status"`
	Entries []transfer.ManifestEntry `json:"entries,omitempty"`
}

var errInvalidRequest = errors.New("invalid worker request")

func readRequest(r io.Reader) (startRequest, error) {
	data, err := io.ReadAll(io.LimitReader(r, requestLimit+1))
	if err != nil || len(data) > requestLimit {
		return startRequest{}, errInvalidRequest
	}
	var request startRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return startRequest{}, errInvalidRequest
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return startRequest{}, errInvalidRequest
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

func captureManifest(ctx context.Context, request startRequest) ([]transfer.ManifestEntry, error) {
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
		return nil, err
	}
	var manifest []transfer.ManifestEntry
	err = client.ReceiveManifest(func(ctx context.Context, info croc.SenderInfo) error {
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
		var err error
		manifest, err = transfer.ProjectCrocReceiveManifest(metadata)
		return err
	})
	if errors.Is(err, croc.ErrManifestOnly) && len(manifest) > 0 {
		return manifest, nil
	}
	if err == nil {
		err = errors.New("missing metadata-only stop")
	}
	return nil, err
}

func run(ctx context.Context, in io.Reader, out io.Writer) int {
	result := manifestResult{Version: wireVersion, Status: "invalid_request"}
	request, err := readRequest(in)
	if err == nil {
		result.Entries, err = captureManifest(ctx, request)
		switch {
		case err == nil:
			result.Status = "manifest_only"
		case errors.Is(err, transfer.ErrUnsafeOffer):
			result.Status = "unsafe_offer"
		case errors.Is(err, transfer.ErrResourceLimit):
			result.Status = "resource_limit"
		case ctx.Err() != nil:
			result.Status = "cancelled"
		default:
			result.Status = "metadata_failed"
		}
	}
	if err := json.NewEncoder(out).Encode(result); err != nil || result.Status != "manifest_only" {
		return 1
	}
	return 0
}

func main() {
	// Only the structured result uses the inherited stdout pipe. Any terminal
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
