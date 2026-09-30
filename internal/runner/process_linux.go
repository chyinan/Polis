// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Process struct {
	cmd                *exec.Cmd
	spec               ProcessLaunchSpec
	In                 io.WriteCloser
	Out                io.ReadCloser
	Err                io.ReadCloser
	done               chan struct{}
	id                 string
	waitErr            error
	stopMu             sync.Mutex
	proof              StopProof
	containmentProfile string
	containmentGroupID string
	containmentHostID  string
	containmentRootID  string
	containmentBootID  string
	workerCgroup       WorkerProcessCgroup
}

func (p StopProof) Description() string    { return processStopDescription("process-group", p.pid) }
func (p *Process) Identity() (string, int) { return p.id, p.cmd.Process.Pid }
func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
func (p *Process) ContainmentMetadata() ProcessContainmentMetadata {
	if p != nil && p.containmentProfile != "" {
		return ProcessContainmentMetadata{HostOS: "linux", Profile: p.containmentProfile, CgroupID: p.containmentGroupID,
			CgroupHostID: p.containmentHostID, CgroupRootID: p.containmentRootID, CgroupBootID: p.containmentBootID}
	}
	return CurrentProcessContainmentMetadata()
}
func CurrentProcessContainmentMetadata() ProcessContainmentMetadata {
	return ProcessContainmentMetadata{HostOS: "linux", Profile: "linux_process_group@1"}
}
func (p *Process) HasExited() bool {
	if p == nil {
		return true
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}
func Start(id string, argv, env []string) (*Process, error) {
	return StartWithWorkingDir(id, argv, env, "")
}

func StartWithWorkingDir(id string, argv, env []string, workingDir string) (*Process, error) {
	spec, err := NewProcessLaunchSpec(argv, env, workingDir, ProcessLaunchDirectories{RuntimeHome: workingDir + "/home", TempDirectory: workingDir + "/tmp"})
	if err != nil {
		return nil, err
	}
	return StartWithLaunchSpec(id, spec)
}

func StartWithLaunchSpec(id string, spec ProcessLaunchSpec) (*Process, error) {
	return startWithLaunchSpecCgroup(id, spec, -1)
}

func StartWithLaunchSpecInCgroup(id string, spec ProcessLaunchSpec, cgroupFD int) (*Process, error) {
	if cgroupFD < 0 {
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "cgroup descriptor is required"}
	}
	return startWithLaunchSpecCgroup(id, spec, cgroupFD)
}

func StartWithLaunchSpecInWorkerCgroup(id string, spec ProcessLaunchSpec, cgroup WorkerProcessCgroup) (*Process, error) {
	if cgroup == nil {
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "named WorkerSession cgroup is required"}
	}
	fd := cgroup.FileDescriptor()
	name := cgroup.Name()
	cgroupRoot := cgroup.CgroupRootPath()
	bubblewrapPath := cgroup.BubblewrapPath()
	hostIdentity := cgroup.HostIdentity()
	rootIdentity := cgroup.RootIdentity()
	bootID := cgroup.BootID()
	if fd < 0 || !validLinuxWorkerCgroupID(name) || !validLinuxWorkerCgroupHostID(hostIdentity) ||
		!validLinuxWorkerCgroupHostID(rootIdentity) || !validLinuxWorkerCgroupBootID(bootID) ||
		!validLinuxWorkerCgroupRootPath(cgroupRoot) || !filepath.IsAbs(bubblewrapPath) || filepath.Base(bubblewrapPath) != "bwrap" || !validLinuxWorkerCgroupSandboxArgv(spec.Argv, cgroupRoot, spec.RuntimeHome) {
		validationErr := &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "named WorkerSession cgroup is invalid"}
		if cleanupErr := cgroup.Cleanup(); cleanupErr != nil {
			return nil, errors.Join(validationErr, fmt.Errorf("WorkerSession cgroup cleanup is unresolved: %w", cleanupErr))
		}
		return nil, validationErr
	}
	spec.Argv = append([]string(nil), spec.Argv...)
	spec.Argv[0] = bubblewrapPath
	spec.Executable = bubblewrapPath
	process, err := startWithLaunchSpecCgroup(id, spec, fd)
	if err != nil {
		if cleanupErr := cgroup.Cleanup(); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("WorkerSession cgroup cleanup is unresolved: %w", cleanupErr))
		}
		return nil, err
	}
	process.containmentProfile = "linux_worker_cgroup_v2@1"
	process.containmentGroupID = name
	process.containmentHostID = hostIdentity
	process.containmentRootID = rootIdentity
	process.containmentBootID = bootID
	process.workerCgroup = cgroup
	return process, nil
}

func startWithLaunchSpecCgroup(id string, spec ProcessLaunchSpec, cgroupFD int) (*Process, error) {
	if id == "" {
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "process identity is required"}
	}
	cmd := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	if spec.WorkingDirectory != "" {
		if _, err := os.Stat(spec.WorkingDirectory); err != nil {
			return nil, NewLaunchFailure(LaunchPhaseWorkingDirectory, LaunchReasonCWDMissing, err, false, 0)
		}
		cmd.Dir = spec.WorkingDirectory
	}
	if spec.Environment != nil {
		cmd.Env = spec.Environment
	}
	cmd.SysProcAttr = linuxProcessSysProcAttr(cgroupFD)
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, ClassifyPipeSetupFailure(e)
	}
	outR, outW, e := os.Pipe()
	if e != nil {
		in.Close()
		return nil, ClassifyPipeSetupFailure(e)
	}
	errR, errW, e := os.Pipe()
	if e != nil {
		outR.Close()
		outW.Close()
		return nil, ClassifyPipeSetupFailure(e)
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	if e = cmd.Start(); e != nil {
		in.Close()
		outR.Close()
		outW.Close()
		errR.Close()
		errW.Close()
		return nil, ClassifyProcessCreateFailure(e, spec.WorkingDirectory)
	}
	outW.Close()
	errW.Close()
	containmentProfile := "linux_process_group@1"
	if cgroupFD >= 0 {
		containmentProfile = "linux_cgroup_process_group@1"
	}
	p := &Process{cmd: cmd, spec: spec, In: in, Out: outR, Err: errR, done: make(chan struct{}), id: id, containmentProfile: containmentProfile}
	go func() { p.waitErr = cmd.Wait(); close(p.done) }()
	return p, nil
}

func linuxProcessSysProcAttr(cgroupFD int) *syscall.SysProcAttr {
	attributes := &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if cgroupFD >= 0 {
		attributes.UseCgroupFD = true
		attributes.CgroupFD = cgroupFD
	}
	return attributes
}
func (p *Process) Stop() (StopProof, error) {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if p.proof.stopped {
		return p.proof, nil
	}
	if p.workerCgroup != nil {
		if err := p.workerCgroup.Cleanup(); err != nil {
			return StopProof{}, errors.Join(ErrProcessTreeStopUnconfirmed, err)
		}
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
			return StopProof{}, errors.New("process termination unconfirmed after WorkerSession cgroup cleanup")
		}
		p.In.Close()
		p.proof = StopProof{p.id, p.cmd.Process.Pid, true}
		return p.proof, nil
	}
	// os.Process uses Go's process handle/pidfd; do not signal a possibly reused PID.
	e := p.cmd.Process.Kill()
	if e != nil && !errors.Is(e, os.ErrProcessDone) {
		return StopProof{}, e
	}
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		return StopProof{}, errors.New("process termination unconfirmed")
	}
	reconcileCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	members, err := linuxProcessGroupMemberCount(reconcileCtx, "/proc", p.cmd.Process.Pid)
	cancel()
	if err != nil {
		return StopProof{}, errors.Join(ErrProcessTreeStopUnconfirmed, err)
	}
	if members > 0 {
		return StopProof{}, fmt.Errorf("%w: Linux process group still contains %d process(es)", ErrProcessTreeStopUnconfirmed, members)
	}
	p.In.Close()
	// Stop proof for this profile is limited to the Linux process group. A
	// caller needing stronger descendant containment must use a private PID namespace.
	p.proof = StopProof{p.id, p.cmd.Process.Pid, true}
	return p.proof, nil
}

func (p *Process) WaitError() error {
	if p == nil {
		return nil
	}
	<-p.done
	return p.waitErr
}
