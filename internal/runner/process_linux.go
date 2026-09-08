// pattern: Imperative Shell
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Process struct {
	cmd     *exec.Cmd
	In      io.WriteCloser
	Out     io.ReadCloser
	Err     io.ReadCloser
	done    chan struct{}
	id      string
	waitErr error
	stopMu  sync.Mutex
	proof   StopProof
}

// StopProof cannot be fabricated by a model or constructed outside this package.
type StopProof struct {
	id      string
	pid     int
	stopped bool
}

func (p StopProof) For(id string) bool     { return p.stopped && p.id == id && p.pid > 0 }
func (p StopProof) Description() string    { return fmt.Sprintf("process-group:%d:waited", p.pid) }
func (p StopProof) PID() int               { return p.pid }
func (p *Process) Identity() (string, int) { return p.id, p.cmd.Process.Pid }
func Start(id string, argv, env []string) (*Process, error) {
	if id == "" || len(argv) == 0 {
		return nil, errors.New("invalid process identity")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if env != nil {
		cmd.Env = env
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	outR, outW, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	errR, errW, e := os.Pipe()
	if e != nil {
		outR.Close()
		outW.Close()
		return nil, e
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	if e = cmd.Start(); e != nil {
		in.Close()
		outR.Close()
		outW.Close()
		errR.Close()
		errW.Close()
		return nil, e
	}
	outW.Close()
	errW.Close()
	p := &Process{cmd: cmd, In: in, Out: outR, Err: errR, done: make(chan struct{}), id: id}
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
