//go:build linux

// pattern: Imperative Shell
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"polis/internal/gotestprofile"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type FixedGoResult struct {
	Facts                                 gotestprofile.ExecutionFacts
	Started, LaunchObserved, ExitObserved bool
	StartError                            error
}

func CheckFixedGoControls(cgroup, scratch string, l gotestprofile.ResourceLimits) error {
	self, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(cgroup, "/sys/fs/cgroup/system.slice/polis-self-dev-go-job@") || !strings.HasSuffix(cgroup, ".service") || !strings.Contains(string(self), "0::"+strings.TrimPrefix(cgroup, "/sys/fs/cgroup")+"\n") {
		return errors.New("job_cgroup_mismatch")
	}
	checks := map[string]string{"cpu.max": fmt.Sprintf("%d %d", l.CPUQuotaMicros, l.CPUPeriodMicros), "memory.max": strconv.FormatUint(l.MemoryBytes, 10), "memory.swap.max": strconv.FormatUint(l.SwapBytes, 10), "pids.max": strconv.FormatUint(l.PIDs, 10)}
	for key, want := range checks {
		v, e := os.ReadFile(filepath.Join(cgroup, key))
		if e != nil || strings.TrimSpace(string(v)) != want {
			return errors.New("cgroup_limits_unavailable")
		}
	}
	if _, err = os.Stat(filepath.Join(cgroup, "cgroup.kill")); err != nil {
		return errors.New("cgroup_kill_unavailable")
	}
	st, e := os.Lstat(scratch)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
		return errors.New("scratch_not_private")
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(scratch, &fs) != nil || fs.Type != 0x01021994 || uint64(fs.Blocks)*uint64(fs.Bsize) != l.ScratchBytes || fs.Files != l.ScratchInodes {
		return errors.New("scratch_hard_limits_unavailable")
	}
	if fs.Flags&6 != 6 || fs.Flags&8 != 0 {
		return errors.New("scratch_execution_policy_incompatible")
	}
	return nil
}

type fixedGoLog struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
	cancel    context.CancelFunc
}

func (l *fixedGoLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	remaining := l.limit - len(l.data)
	if len(p) > remaining {
		l.data = append(l.data, p[:remaining]...)
		l.truncated = true
		l.cancel()
	} else {
		l.data = append(l.data, p...)
	}
	return n, nil
}
func (l *fixedGoLog) snapshot() ([]byte, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]byte(nil), l.data...), l.truncated
}
func eventCount(path, key string) uint64 {
	raw, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == key {
			n, _ := strconv.ParseUint(f[1], 10, 64)
			return n
		}
	}
	return 0
}

// RunFixedGo starts only the canonical fixture argv inside bwrap. It requires the
// current process to already be in the exact systemd resource-controlled job.
func RunFixedGo(ctx context.Context, bwrap, source, toolchain, deps, scratch, cgroup string, p gotestprofile.Manifest) FixedGoResult {
	var result FixedGoResult
	if _, _, e := gotestprofile.Canonicalize(p); e != nil {
		result.StartError = e
		return result
	}
	if e := CheckFixedGoControls(cgroup, scratch, p.Limits); e != nil {
		result.StartError = e
		return result
	}
	for _, path := range []string{bwrap, source, toolchain, deps, scratch} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			result.StartError = errors.New("invalid_fixed_path")
			return result
		}
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &fixedGoLog{limit: int(p.Limits.StdoutBytes), cancel: cancel}
	errlog := &fixedGoLog{limit: int(p.Limits.StderrBytes), cancel: cancel}
	infoRead, infoWrite, pipeErr := os.Pipe()
	if pipeErr != nil {
		result.StartError = pipeErr
		return result
	}
	defer infoRead.Close()
	defer infoWrite.Close()
	argv := []string{"--die-with-parent", "--unshare-user", "--unshare-pid", "--unshare-net", "--unshare-ipc", "--unshare-uts", "--new-session", "--cap-drop", "ALL", "--clearenv", "--ro-bind", source, "/src", "--ro-bind", toolchain, "/go", "--ro-bind", deps, "/deps/gomodcache", "--bind", scratch, "/scratch", "--proc", "/proc", "--dev", "/dev", "--chdir", "/src"}
	for _, item := range p.Environment {
		k, v, _ := strings.Cut(item, "=")
		argv = append(argv, "--setenv", k, v)
	}
	argv = append(argv, "--info-fd", "3")
	argv = append(argv, "--")
	argv = append(argv, p.Command...)
	cmd := exec.CommandContext(childCtx, bwrap, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	cmd.Stdout = out
	cmd.Stderr = errlog
	cmd.ExtraFiles = []*os.File{infoWrite}
	cmd.WaitDelay = time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return cmd.Process.Kill()
	}
	oom := eventCount(filepath.Join(cgroup, "memory.events"), "oom_kill")
	pids := eventCount(filepath.Join(cgroup, "pids.events"), "max")
	if e := cmd.Start(); e != nil {
		result.StartError = e
		return result
	}
	result.LaunchObserved = true
	_ = infoWrite.Close()
	_ = cmd.Wait()
	result.ExitObserved = cmd.ProcessState != nil
	f := &result.Facts
	f.CgroupControlsApplied = true
	f.ScratchQuotaApplied = true
	_ = infoRead.SetReadDeadline(time.Now().Add(time.Second))
	infoRaw, _ := io.ReadAll(io.LimitReader(infoRead, 4096))
	var sandboxInfo struct {
		ChildPID int `json:"child-pid"`
	}
	f.NetworkIsolationApplied = json.Unmarshal(infoRaw, &sandboxInfo) == nil && sandboxInfo.ChildPID > 0
	if !f.NetworkIsolationApplied {
		f.SetupFailure = "sandbox_initialization_failed"
	}
	f.Stdout, f.StdoutTruncated = out.snapshot()
	f.Stderr, f.StderrTruncated = errlog.snapshot()
	if f.NetworkIsolationApplied && len(f.Stdout) == 0 && len(f.Stderr) > 0 {
		f.SetupFailure = "go_tool_start_failed"
	}
	result.Started = f.NetworkIsolationApplied && f.SetupFailure == ""
	if result.ExitObserved && cmd.ProcessState.ExitCode() >= 0 {
		f.ExitCodeKnown = true
		f.ExitCode = cmd.ProcessState.ExitCode()
	}
	f.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	f.Cancelled = errors.Is(ctx.Err(), context.Canceled)
	f.ProcessTreeReaped = true
	raw, e := os.ReadFile(filepath.Join(cgroup, "cgroup.procs"))
	if e != nil {
		f.ProcessTreeReaped = false
	} else {
		for _, v := range strings.Fields(string(raw)) {
			pid, _ := strconv.Atoi(v)
			if pid != os.Getpid() {
				f.ProcessTreeReaped = false
			}
		}
	}
	if eventCount(filepath.Join(cgroup, "memory.events"), "oom_kill") > oom {
		f.ResourceLimit = "memory"
	} else if eventCount(filepath.Join(cgroup, "pids.events"), "max") > pids {
		f.ResourceLimit = "pids"
	} else {
		var fs syscall.Statfs_t
		if syscall.Statfs(scratch, &fs) == nil {
			if fs.Bavail == 0 {
				f.ResourceLimit = "scratch_bytes"
			} else if fs.Ffree == 0 {
				f.ResourceLimit = "scratch_inodes"
			}
		}
	}
	return result
}
