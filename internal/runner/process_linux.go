// pattern: Imperative Shell
package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Process struct {
	cmd     *exec.Cmd
	spec    ProcessLaunchSpec
	In      io.WriteCloser
	Out     io.ReadCloser
	Err     io.ReadCloser
	done    chan struct{}
	id      string
	waitErr error
	stopMu  sync.Mutex
	proof   StopProof
}

func (p StopProof) Description() string    { return processStopDescription("process-group", p.pid) }
func (p *Process) Identity() (string, int) { return p.id, p.cmd.Process.Pid }
func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
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
	p := &Process{cmd: cmd, spec: spec, In: in, Out: outR, Err: errR, done: make(chan struct{}), id: id}
	go func() { p.waitErr = cmd.Wait(); close(p.done) }()
	return p, nil
}
func (p *Process) Stop() (StopProof, error) {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if p.proof.stopped {
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
	p.In.Close()
	// The production child is bubblewrap with a private PID namespace and
	// --die-with-parent. Its namespace init death also destroys escaped descendants.
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

func Run(argv, env []string, limit time.Duration) ([]byte, error) {
	p, e := Start("bounded-check", argv, env)
	if e != nil {
		return nil, e
	}
	defer p.Stop()
	defer p.Out.Close()
	defer p.Err.Close()
	p.In.Close()
	type result struct {
		b []byte
		e error
	}
	ch := make(chan result, 2)
	for _, r := range []io.Reader{p.Out, p.Err} {
		go func(r io.Reader) {
			b, e := io.ReadAll(io.LimitReader(r, 32769))
			if len(b) > 32768 {
				e = errors.New("output limit exceeded")
				p.Stop()
			}
			ch <- result{b, e}
		}(r)
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()
	var all []byte
	for i := 0; i < 2; i++ {
		select {
		case r := <-ch:
			all = append(all, r.b...)
			if r.e != nil {
				return all, r.e
			}
		case <-timer.C:
			return all, errors.New("process time limit exceeded")
		}
	}
	select {
	case <-p.done:
		return all, p.waitErr
	case <-timer.C:
		return all, errors.New("process time limit exceeded")
	}
}
