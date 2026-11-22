//go:build windows

package transfer

import (
	"os/exec"
	"testing"
	"time"
)

func TestWorkerJobObjectTerminatesProcessOnClose(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := attachWorkerJob(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("attachWorkerJob failed: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// Closing the job handle triggers JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
	job.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("worker process survived job handle close")
	}
}
