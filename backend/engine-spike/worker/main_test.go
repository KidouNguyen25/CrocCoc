//go:build croc_no_tailcat

package main

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestStartRequestOnlyAcceptsOneBoundedLoopbackFrame(t *testing.T) {
	valid := startRequest{Version: wireVersion, Code: "synthetic-code-fixture", RelayAddress: "127.0.0.1:2345", RelayPassword: "fixture"}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := readRequest(bytes.NewReader(data)); err != nil || got != valid {
		t.Fatal("valid request rejected")
	}
	for _, bad := range []string{
		string(data) + "{}",
		strings.Replace(string(data), "127.0.0.1", "localhost", 1),
		strings.Replace(string(data), "127.0.0.1", "192.0.2.1", 1),
		strings.Replace(string(data), "2345", "0", 1),
		strings.Repeat(" ", requestLimit+1),
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

func TestWorkerLoopbackProjectsManifestAcrossPipes(t *testing.T) {
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
	frame, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = bytes.NewReader(frame)
	output, err := cmd.Output()
	if err != nil {
		status := "no structured status"
		var reply manifestResult
		if json.Unmarshal(output, &reply) == nil {
			status = reply.Status
		}
		t.Fatalf("test worker failed: %v (%s)", err, status)
	}
	var result manifestResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal("worker stdout was not one typed result")
	}
	want := []transfer.ManifestEntry{
		{Path: "Album", Directory: true},
		{Path: "Album/Empty", Directory: true},
		{Path: "Album/ảnh.txt", SizeBytes: 6},
	}
	if result.Version != wireVersion || result.Status != "manifest_only" || !reflect.DeepEqual(result.Entries, want) ||
		bytes.Contains(output, []byte(request.Code)) || bytes.Contains(output, []byte(request.RelayPassword)) {
		t.Fatal("worker did not return the projected fixture without credentials")
	}
	items, err := os.ReadDir(dest)
	if err != nil || len(items) != 0 {
		t.Fatal("worker wrote to its destination")
	}
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
