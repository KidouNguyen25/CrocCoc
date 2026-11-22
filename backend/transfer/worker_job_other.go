//go:build !windows

package transfer

import "os/exec"

type workerJob struct{}

func prepareWorkerCommand(*exec.Cmd) error { return nil }

func attachWorkerJob(*exec.Cmd) (*workerJob, error) {
	return &workerJob{}, nil
}

func (*workerJob) Terminate() {}

func (*workerJob) Close() {}
