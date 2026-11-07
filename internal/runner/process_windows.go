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

func (p StopProof) Description() string {
	return processStopDescription("windows-process-handle", p.pid)
}
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
	spec, err := NewProcessLaunchSpec(argv, env, workingDir, ProcessLaunchDirectories{RuntimeHome: workingDir + "\\home", TempDirectory: workingDir + "\\tmp"})
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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, ClassifyPipeSetupFailure(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, ClassifyPipeSetupFailure(err)
	}
	errOut, err := cmd.StderrPipe()
	if err != nil {
		in.Close()
		out.Close()
		return nil, ClassifyPipeSetupFailure(err)
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		errOut.Close()
		return nil, ClassifyProcessCreateFailure(err, spec.WorkingDirectory)
	}
	process := &Process{cmd: cmd, spec: spec, In: in, Out: out, Err: errOut, done: make(chan struct{}), id: id}
	go func() {
		process.waitErr = cmd.Wait()
		close(process.done)
	}()
	return process, nil
}

func (p *Process) Stop() (StopProof, error) {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if p.proof.stopped {
		return p.proof, nil
	}
	if p.cmd.Process == nil {
		return StopProof{}, errors.New("process handle is unavailable")
	}
	err := p.cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return StopProof{}, err
	}
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		return StopProof{}, errors.New("process termination unconfirmed")
	}
	p.In.Close()
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
	p, err := Start("bounded-check", argv, env)
	if err != nil {
		return nil, err
	}
	defer p.Stop()
	defer p.Out.Close()
	defer p.Err.Close()
	p.In.Close()
	type result struct {
		b   []byte
		err error
	}
	results := make(chan result, 2)
	for _, reader := range []io.Reader{p.Out, p.Err} {
		go func(reader io.Reader) {
			b, err := io.ReadAll(io.LimitReader(reader, 32769))
			if len(b) > 32768 {
				err = errors.New("output limit exceeded")
				p.Stop()
			}
			results <- result{b: b, err: err}
		}(reader)
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()
	var all []byte
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			all = append(all, result.b...)
			if result.err != nil {
				return all, result.err
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
