//go:build croc_no_tailcat

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"croccoc/backend/transfer"
	"github.com/schollz/croc/v11/src/croc"
	"github.com/schollz/croc/v11/src/tcp"
)

func TestStartRequestAcceptsOneBoundedRelayFrame(t *testing.T) {
	valid := startRequest{Version: wireVersion, Code: "synthetic-code-fixture", RelayAddress: "relay.example.test:2345", RelayPassword: "fixture"}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := readRequest(bufio.NewReaderSize(bytes.NewReader(append(data, '\n')), requestLimit)); err != nil || !reflect.DeepEqual(got, valid) {
		t.Fatal("valid request rejected")
	}
	for _, bad := range []string{
		string(data),
		string(data) + "{}\n",
		strings.Replace(string(data), "relay.example.test", "bad/host", 1) + "\n",
		strings.Replace(string(data), "relay.example.test", "0.0.0.0", 1) + "\n",
		strings.Replace(string(data), "2345", "0", 1) + "\n",
		strings.Repeat(" ", requestLimit) + "\n",
	} {
		var result bytes.Buffer
		if code := run(t.Context(), strings.NewReader(bad), &result); code == 0 {
			t.Fatal("bad request allowed")
		}
		var reply manifestResult
		if err := json.Unmarshal(result.Bytes(), &reply); err != nil || reply.Status != "invalid_request" ||
			len(reply.Entries) != 0 || strings.Contains(result.String(), valid.Code) {
			t.Fatal("invalid request leaked data or did not fail closed")
		}
	}
}

// TestWorkerProcess runs main as a separate process with anonymous pipes,
// without placing credentials into its command line or environment.
func TestWorkerProcess(t *testing.T) {
	if os.Getenv("CROCCOC_TEST_WORKER") != "1" {
		return
	}
	os.Args = os.Args[:1]
	main()
}

func TestDecisionFramesFailClosed(t *testing.T) {
	const id = "fixture"
	for _, tc := range []struct {
		name, frame string
		want        error
	}{
		{"accept", `{"version":2,"offerID":"fixture","accept":true}` + "\n", nil},
		{"reject", `{"version":2,"offerID":"fixture","accept":false}` + "\n", errOfferRejected},
		{"missing decision", `{"version":2,"offerID":"fixture"}` + "\n", errInvalidDecision},
		{"null decision", `{"version":2,"offerID":"fixture","accept":null}` + "\n", errInvalidDecision},
		{"unknown property", `{"version":2,"offerID":"fixture","accept":true,"path":"other"}` + "\n", errInvalidDecision},
		{"stale offer", `{"version":2,"offerID":"other","accept":true}` + "\n", errInvalidDecision},
		{"old version", `{"version":1,"offerID":"fixture","accept":true}` + "\n", errInvalidDecision},
		{"oversized", strings.Repeat(" ", requestLimit) + "\n", errInvalidDecision},
		{"closed stdin", "", errInvalidDecision},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := waitDecision(t.Context(), bufio.NewReaderSize(strings.NewReader(tc.frame), requestLimit), id)
			if !errors.Is(err, tc.want) {
				t.Fatalf("decision = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDecisionTimeoutDoesNotWaitForPipeEOF(t *testing.T) {
	in, out := io.Pipe()
	defer in.Close()
	defer out.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- waitDecision(ctx, bufio.NewReaderSize(in, requestLimit), "fixture") }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("decision = %v, want deadline", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("decision remained blocked on the pipe")
	}
}

func TestProjectCrocSendManifestPreservesSelectedTree(t *testing.T) {
	files := []croc.FileInfo{{
		FolderRemote: "Album/nested/", Name: "photo.txt", Size: 5,
	}}
	folders := []croc.FileInfo{{FolderRemote: "Album/Empty/"}}
	got, err := projectCrocSendManifest(files, folders)
	if err != nil {
		t.Fatal(err)
	}
	want := []transfer.ManifestEntry{
		{Path: "Album", Directory: true},
		{Path: "Album/Empty", Directory: true},
		{Path: "Album/nested", Directory: true},
		{Path: "Album/nested/photo.txt", SizeBytes: 5},
	}
	gotDigest, err := transfer.SendManifestDigest(got)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest, err := transfer.SendManifestDigest(want)
	if err != nil || gotDigest != wantDigest {
		t.Fatalf("projected source manifest mismatch: %v", err)
	}
}

func TestProjectCrocSendManifestRejectsUnsafeEntries(t *testing.T) {
	if _, err := projectCrocSendManifest([]croc.FileInfo{{
		FolderRemote: "../outside/", Name: "secret.txt", Size: 1,
	}}, nil); !errors.Is(err, transfer.ErrUnsafeOffer) {
		t.Fatalf("unsafe source manifest accepted: %v", err)
	}
}

func TestWorkerLoopbackWaitsForDecisionWithoutStartingPayload(t *testing.T) {
	for _, tc := range []struct {
		name, status                                   string
		accept, wrongOffer, closePipe, continuePayload bool
	}{
		{name: "accepted", status: "accepted_no_payload", accept: true},
		{name: "accepted payload", status: "completed", accept: true, continuePayload: true},
		{name: "rejected", status: "rejected"},
		{name: "stale offer", status: "invalid_decision", accept: true, wrongOffer: true},
		{name: "pipe closed", status: "invalid_decision", closePipe: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testWorkerLoopback(t, tc.status, tc.accept, tc.wrongOffer, tc.closePipe, tc.continuePayload)
		})
	}
}

func testWorkerLoopback(t *testing.T, status string, accept, wrongOffer, closePipe, continuePayload bool) {
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	listen := func() net.Listener {
		t.Helper()
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
		return l
	}
	control, data := listen(), listen()
	port := data.Addr().(*net.TCPAddr).Port
	_, dataPort, err := net.SplitHostPort(data.Addr().String())
	if err != nil || port == 0 {
		t.Fatalf("invalid test relay port: %v", err)
	}
	request := startRequest{
		Version: wireVersion, Code: "point-frost-worker-fixture",
		RelayAddress: control.Addr().String(), RelayPassword: "local-test-relay",
		ContinuePayload: continuePayload,
	}
	go tcp.RunWithListener(data, request.RelayPassword, tcp.WithCtx(ctx), tcp.WithLogLevel("error"))
	go tcp.RunWithListener(control, request.RelayPassword, tcp.WithCtx(ctx), tcp.WithLogLevel("error"), tcp.WithBanner(dataPort))

	src := t.TempDir()
	album := filepath.Join(src, "Album")
	if err := os.MkdirAll(filepath.Join(album, "Empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "ảnh.txt"), []byte("sample"), 0600); err != nil {
		t.Fatal(err)
	}
	files, folders, count, err := croc.GetFilesInfo([]string{album}, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	originalStderr := os.Stderr
	sender, err := croc.NewCtx(ctx, croc.Options{
		SharedSecret: request.Code, RelayAddress: request.RelayAddress, RelayPassword: request.RelayPassword,
		RelayPorts: []string{dataPort}, IsSender: true, DisableLocal: true, DisableClipboard: true,
		Curve: "siec", Transport: croc.TransportRelay, Quiet: true,
	})
	quietStderr := os.Stderr
	t.Cleanup(func() {
		os.Stderr = originalStderr
		if quietStderr != originalStderr {
			quietStderr.Close()
		}
	})
	if err != nil {
		t.Fatal("create test sender")
	}
	senderDone := make(chan error, 1)
	go func() { senderDone <- sender.Send(files, folders, count) }()

	dest := t.TempDir()
	testExe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, testExe, "-test.run", "^TestWorkerProcess$")
	cmd.Dir = dest
	cmd.Env = append(os.Environ(), "CROCCOC_TEST_WORKER=1", "LOGGER=trace")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal("start test worker")
	}
	var waited bool
	t.Cleanup(func() {
		if !waited {
			cancel()
			_ = cmd.Wait()
		}
	})
	encoder := json.NewEncoder(in)
	if err := encoder.Encode(request); err != nil {
		t.Fatal("write worker start frame")
	}
	reader := bufio.NewReader(out)
	read := func() manifestResult {
		t.Helper()
		raw, err := reader.ReadBytes('\n')
		var result manifestResult
		if err != nil || json.Unmarshal(raw, &result) != nil || result.Version != wireVersion ||
			bytes.Contains(raw, []byte(request.Code)) || bytes.Contains(raw, []byte(request.RelayPassword)) {
			t.Fatal("worker did not return a typed frame without credentials")
		}
		return result
	}
	checkDestination := func(expectPayload bool) {
		t.Helper()
		items, err := os.ReadDir(dest)
		if err != nil {
			t.Fatal("cannot inspect worker destination")
		}
		if !expectPayload && len(items) != 0 {
			t.Fatal("worker wrote to its destination")
		}
		if expectPayload {
			data, err := os.ReadFile(filepath.Join(dest, "Album", "ảnh.txt"))
			if err != nil || string(data) != "sample" {
				t.Fatalf("worker did not receive exact payload: %q %v", data, err)
			}
			if info, err := os.Stat(filepath.Join(dest, "Album", "Empty")); err != nil || !info.IsDir() {
				t.Fatalf("worker did not receive empty directory: %v", err)
			}
		}
	}
	result := read()
	want := []transfer.ManifestEntry{
		{Path: "Album", Directory: true},
		{Path: "Album/Empty", Directory: true},
		{Path: "Album/ảnh.txt", SizeBytes: 6},
	}
	if result.Status != "awaiting_decision" || len(result.OfferID) != 32 || !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("expected projected offer, got %q", result.Status)
	}
	// Keep the pipe open and withhold a decision. An early terminal frame
	// must be caught before the client writes an accept or reject.
	pending := make(chan manifestResult, 1)
	readError := make(chan error, 1)
	go func() {
		raw, err := reader.ReadBytes('\n')
		var final manifestResult
		if err == nil {
			err = json.Unmarshal(raw, &final)
			if bytes.Contains(raw, []byte(request.Code)) || bytes.Contains(raw, []byte(request.RelayPassword)) {
				err = errors.New("terminal frame leaked credentials")
			}
		}
		if err != nil {
			readError <- err
			return
		}
		pending <- final
	}()
	select {
	case early := <-pending:
		t.Fatalf("worker finished before decision: %s", early.Status)
	case err := <-readError:
		t.Fatalf("worker output closed before decision: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	checkDestination(false)
	if closePipe {
		if err := in.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		offerID := result.OfferID
		if wrongOffer {
			offerID = "not the current offer"
		}
		decision := decisionRequest{Version: wireVersion, OfferID: offerID, Accept: &accept}
		if continuePayload && accept {
			decision.PayloadDir = dest
		}
		if err := encoder.Encode(decision); err != nil {
			t.Fatal("write worker decision")
		}
	}
	select {
	case final := <-pending:
		if final.Version != wireVersion || final.Status != status || final.OfferID != "" || len(final.Entries) != 0 {
			t.Fatalf("expected terminal %q, got %q", status, final.Status)
		}
	case err := <-readError:
		t.Fatalf("missing terminal frame: %v", err)
	case <-ctx.Done():
		t.Fatal("worker did not respond to the decision")
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("expected EOF after terminal frame, got %v", err)
	}
	err = cmd.Wait()
	waited = true
	if (err == nil) != (status != "invalid_decision") {
		t.Fatalf("unexpected worker exit for %q: %v", status, err)
	}
	checkDestination(continuePayload)
	if continuePayload {
		select {
		case sendErr := <-senderDone:
			if sendErr != nil {
				t.Fatalf("sender payload failed: %v", sendErr)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("test sender did not finish")
		}
	} else {
		cancel()
		select {
		case sendErr := <-senderDone:
			if sendErr == nil {
				t.Fatal("sender reported a successful payload transfer")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("test sender did not stop")
		}
	}
}
