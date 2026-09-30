// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"

	"polis/internal/environment"
	"polis/internal/runner"
)

type linuxNodeProcess struct {
	process   *runner.Process
	identity  string
	waitOnce  sync.Once
	waitDone  chan struct{}
	waitErr   error
	cgroup    environment.LinuxNodeResourceCgroup
	cgroupMu  sync.Mutex
	cgroupErr error
}

func startLinuxNodeProcess(identity string, argv []string) (runner.AppContainerProcess, error) {
	return startLinuxNodeProcessWithCgroup(identity, argv, nil)
}

func startLinuxNodeProcessWithCgroup(identity string, argv []string, cgroup environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error) {
	spec, err := runner.NewProcessLaunchSpec(argv, []string{}, "", runner.ProcessLaunchDirectories{})
	if err != nil {
		return nil, err
	}
	var process *runner.Process
	if cgroup == nil {
		process, err = runner.StartWithLaunchSpec(identity, spec)
	} else {
		fd := cgroup.FileDescriptor()
		if fd < 0 {
			return nil, environment.ErrLinuxNodeCgroupUnavailable
		}
		process, err = runner.StartWithLaunchSpecInCgroup(identity, spec, fd)
	}
	if err != nil {
		return nil, err
	}
	return &linuxNodeProcess{process: process, identity: identity, waitDone: make(chan struct{}), cgroup: cgroup}, nil
}

func (process *linuxNodeProcess) PID() int { return process.process.PID() }

func (process *linuxNodeProcess) Stdin() io.WriteCloser { return process.process.In }

func (process *linuxNodeProcess) Stdout() io.ReadCloser { return process.process.Out }

func (process *linuxNodeProcess) Stderr() io.ReadCloser { return process.process.Err }

func (process *linuxNodeProcess) Wait(ctx context.Context) (int, error) {
	if process == nil || process.process == nil || ctx == nil {
		return 0, errors.New("Linux Node process handle is unavailable")
	}
	process.waitOnce.Do(func() {
		go func() {
			process.waitErr = process.process.WaitError()
			close(process.waitDone)
		}()
	})
	select {
	case <-process.waitDone:
		if err := process.cleanupCgroup(); err != nil {
			return 0, err
		}
		if process.waitErr == nil {
			return 0, nil
		}
		var exitError *exec.ExitError
		if errors.As(process.waitErr, &exitError) {
			return exitError.ExitCode(), nil
		}
		return -1, process.waitErr
	case <-ctx.Done():
		proof, stopErr := process.Stop()
		if stopErr != nil || !proof.For(process.identity) || proof.PID() != process.PID() {
			return 0, errors.Join(ctx.Err(), stopErr, runner.ErrAppContainerProcessStopUnconfirmed)
		}
		<-process.waitDone
		if err := process.cleanupCgroup(); err != nil {
			return 0, errors.Join(ctx.Err(), err)
		}
		return 0, ctx.Err()
	}
}

func (process *linuxNodeProcess) cleanupCgroup() error {
	if process == nil || process.cgroup == nil {
		return nil
	}
	process.cgroupMu.Lock()
	defer process.cgroupMu.Unlock()
	if err := process.cgroup.Cleanup(); err != nil {
		process.cgroupErr = errors.Join(environment.ErrLinuxNodeCgroupCleanupUnconfirmed, err)
		return process.cgroupErr
	}
	process.cgroupErr = nil
	return nil
}

func (process *linuxNodeProcess) CgroupCleanupPending() bool {
	if process == nil {
		return false
	}
	process.cgroupMu.Lock()
	defer process.cgroupMu.Unlock()
	return process.cgroupErr != nil
}

func (process *linuxNodeProcess) Stop() (runner.StopProof, error) {
	if process == nil || process.process == nil {
		return runner.StopProof{}, runner.ErrAppContainerUnavailable
	}
	proof, err := process.process.Stop()
	if err != nil || !proof.For(process.identity) || proof.PID() != process.PID() {
		return proof, errors.Join(err, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	return proof, nil
}
