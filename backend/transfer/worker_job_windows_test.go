//go:build windows

package transfer

import (
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

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
	// The shell alone is insufficient: wait for the actual descendant ping.exe
	// and hold its process handle so PID reuse cannot cause a false pass.
	var childPID uint32
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		childPID, err = findWorkerJobTestChild(uint32(cmd.Process.Pid))
		if err != nil {
			break
		}
		if childPID != 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil || childPID == 0 {
		job.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("ping.exe child was not observed before job close: pid=%d, err=%v", childPID, err)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, childPID)
	if err != nil {
		job.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("open ping.exe child handle: %v", err)
	}
	defer windows.CloseHandle(child)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// Closing the job handle triggers JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	// for the entire tree, not just cmd.exe.
	job.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("worker process survived job handle close")
	}
	waitResult, err := windows.WaitForSingleObject(child, 5000)
	if err != nil || waitResult != windows.WAIT_OBJECT_0 {
		t.Fatalf("ping.exe descendant survived job close: result=%d, err=%v", waitResult, err)
	}
}

func findWorkerJobTestChild(parentPID uint32) (uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err = windows.Process32First(snapshot, &entry); err != nil {
		return 0, err
	}
	for {
		if entry.ParentProcessID == parentPID && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "ping.exe") {
			return entry.ProcessID, nil
		}
		if err = windows.Process32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return 0, nil
			}
			return 0, err
		}
	}
}
