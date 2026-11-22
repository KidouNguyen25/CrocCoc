//go:build windows

package transfer

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type workerJob struct {
	handle windows.Handle
}

// attachWorkerJob binds the started worker process to a kill-on-close Windows
// Job Object so crashes or cancellation terminate the entire process tree.
// Fails closed if the job cannot be created or assigned.
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
	return &workerJob{handle: job}, nil
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
