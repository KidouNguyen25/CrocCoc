//go:build windows

package transfer

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type workerJob struct {
	handle windows.Handle
}

func prepareWorkerCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return ErrWorkerUnavailable
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	return nil
}

// attachWorkerJob binds the started worker process to a kill-on-close Windows
// Job Object before its primary thread runs, so crashes or cancellation
// terminate the entire process tree. Fails closed if setup or resume fails.
func attachWorkerJob(cmd *exec.Cmd) (*workerJob, error) {
	if cmd == nil || cmd.Process == nil {
		return nil, ErrWorkerUnavailable
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, ErrWorkerUnavailable
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, ErrWorkerUnavailable
	}
	procHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, ErrWorkerUnavailable
	}
	assignErr := windows.AssignProcessToJobObject(job, procHandle)
	_ = windows.CloseHandle(procHandle)
	if assignErr != nil {
		_ = windows.CloseHandle(job)
		return nil, ErrWorkerUnavailable
	}
	thread, err := openWorkerThread(uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, ErrWorkerUnavailable
	}
	previousSuspendCount, err := windows.ResumeThread(thread)
	_ = windows.CloseHandle(thread)
	if err != nil || previousSuspendCount != 1 {
		_ = windows.CloseHandle(job)
		return nil, ErrWorkerUnavailable
	}
	return &workerJob{handle: job}, nil
}

func openWorkerThread(processID uint32) (windows.Handle, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return 0, err
	}
	for {
		if entry.OwnerProcessID == processID {
			return windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		}
		if err := windows.Thread32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return 0, ErrWorkerUnavailable
			}
			return 0, err
		}
	}
}

func (j *workerJob) Terminate() {
	if j == nil || j.handle == 0 {
		return
	}
	_ = windows.TerminateJobObject(j.handle, 1)
}

func (j *workerJob) Close() {
	if j == nil || j.handle == 0 {
		return
	}
	_ = windows.CloseHandle(j.handle)
	j.handle = 0
}
