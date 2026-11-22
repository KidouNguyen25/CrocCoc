//go:build windows

package transfer

import (
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWorkerJobObjectTerminatesProcessOnClose(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >nul")
	if err := prepareWorkerCommand(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	thread, err := openWorkerThread(uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	suspendThread := windows.NewLazySystemDLL("kernel32.dll").NewProc("SuspendThread")
	previousSuspendCount, _, callErr := suspendThread.Call(uintptr(thread))
	if previousSuspendCount == ^uintptr(0) {
		_ = windows.CloseHandle(thread)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("SuspendThread failed: %v", callErr)
	}
	resumedCount, resumeErr := windows.ResumeThread(thread)
	_ = windows.CloseHandle(thread)
	if resumeErr != nil || previousSuspendCount != 1 || resumedCount != 2 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("worker thread was not created suspended: count before suspend=%d, after resume=%d, err=%v", previousSuspendCount, resumedCount, resumeErr)
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
