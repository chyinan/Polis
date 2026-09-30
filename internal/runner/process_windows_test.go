//go:build windows

// pattern: Imperative Shell

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"syscall"
)

func TestWindowsJobObjectStopsDescendantsWhenRootExits(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidPath := filepath.Join(t.TempDir(), "child.pid")
	process, err := Start("job-tree-test", []string{executable}, []string{
		"POLIS_RUNNER_HELPER=parent",
		"POLIS_RUNNER_CHILD_PID_FILE=" + pidPath,
	})
	if err != nil {
		t.Fatalf("start contained helper: %v", err)
	}
	if err := process.WaitError(); err != nil {
		t.Fatalf("root helper exited with error: %v", err)
	}
	var childPID uint32
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		contents, readErr := os.ReadFile(pidPath)
		if readErr == nil {
			parsed, parseErr := strconv.ParseUint(string(contents), 10, 32)
			if parseErr != nil {
				t.Fatalf("child PID receipt is malformed: %q", contents)
			}
			childPID = uint32(parsed)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("root helper did not report its child PID")
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, childPID)
	if err != nil {
		t.Fatalf("open child process after root exit: %v", err)
	}
	defer windows.CloseHandle(child)
	result, err := windows.WaitForSingleObject(child, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if result != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant survived root job close: wait result %#x", result)
	}
}

func TestWindowsNilProcessEnvironmentCreatesExplicitEmptyBlock(t *testing.T) {
	block, err := windowsProcessEnvironmentBlock(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(block) != 2 || block[0] != 0 || block[1] != 0 {
		t.Fatalf("nil environment block=%v, want an explicit double-null block", block)
	}
}

func TestWindowsStartWithNilEnvironmentDoesNotInheritParentVariables(t *testing.T) {
	const secretName = "POLIS_PARENT_PROCESS_SECRET_TEST"
	t.Setenv(secretName, "must-not-cross-process-boundary")
	output, err := Run([]string{"cmd.exe", "/d", "/c", "echo %" + secretName + "%"}, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("run child with explicitly empty environment: %v", err)
	}
	if strings.Contains(string(output), "must-not-cross-process-boundary") {
		t.Fatal("Windows child inherited a parent environment secret")
	}
}

func TestWindowsRunIncludesStopFailureWhenTimeoutExpires(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinWriter.Close()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutWriter.Close()
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stderrWriter.Close()
	identity := "run-timeout-cleanup-failure-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	job := createInvalidPolicyJobForTest(t, identity)
	defer windows.CloseHandle(job)
	process := &Process{
		In: stdinReader, Out: stdoutReader, Err: stderrReader,
		done: make(chan struct{}), id: identity, cleanupErr: errors.New("injected unknown tree cleanup"),
	}
	close(process.done)
	start := func(string, []string, []string) (*Process, error) { return process, nil }
	_, runErr := runWithProcessStarter(start, []string{"unused"}, nil, time.Millisecond)
	if runErr == nil || !strings.Contains(runErr.Error(), "process time limit exceeded") {
		t.Fatalf("timeout error=%v, want the primary timeout", runErr)
	}
	if !errors.Is(runErr, ErrProcessTreeStopUnconfirmed) {
		t.Fatalf("timeout error omitted process-tree cleanup failure: %v", runErr)
	}
}

func TestWindowsRunIncludesStopFailureWhenOutputLimitIsExceeded(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinWriter.Close()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutWriter.Close()
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stderrWriter.Close()
	identity := "run-output-cleanup-failure-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	job := createInvalidPolicyJobForTest(t, identity)
	defer windows.CloseHandle(job)
	process := &Process{
		In: stdinReader, Out: stdoutReader, Err: stderrReader,
		done: make(chan struct{}), id: identity, cleanupErr: errors.New("injected unknown tree cleanup"),
	}
	close(process.done)
	written := make(chan error, 1)
	go func() {
		_, writeErr := stdoutWriter.Write(make([]byte, 32769))
		written <- writeErr
	}()
	start := func(string, []string, []string) (*Process, error) { return process, nil }
	_, runErr := runWithProcessStarter(start, []string{"unused"}, nil, 5*time.Second)
	select {
	case writeErr := <-written:
		if writeErr != nil {
			t.Fatalf("write bounded output fixture: %v", writeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("output fixture writer did not finish")
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "output limit exceeded") {
		t.Fatalf("output error=%v, want the primary output-limit error", runErr)
	}
	if !errors.Is(runErr, ErrProcessTreeStopUnconfirmed) {
		t.Fatalf("output error omitted process-tree cleanup failure: %v", runErr)
	}
}

func TestWindowsWaitErrorPreservesNonzeroExitAlongsideCleanupFailure(t *testing.T) {
	process := &Process{done: make(chan struct{}), cleanupErr: errors.New("injected unknown tree cleanup"), exitCode: 7}
	close(process.done)
	err := process.WaitError()
	if !errors.Is(err, ErrProcessTreeStopUnconfirmed) {
		t.Fatalf("wait error omitted cleanup failure: %v", err)
	}
	var exitErr ProcessExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode != 7 {
		t.Fatalf("wait error omitted nonzero exit code: %v", err)
	}
}

func createInvalidPolicyJobForTest(t *testing.T, identity string) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(workerProcessJobObjectName(identity))
	if err != nil {
		t.Fatal(err)
	}
	job, err := windows.CreateJobObject(nil, name)
	if err != nil {
		t.Fatalf("create invalid-policy fixture Job Object: %v", err)
	}
	return job
}

func TestWindowsReconcileNamedJobTerminatesItsActiveTree(t *testing.T) {
	identity := "reconcile-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	job, err := newWindowsNamedProcessJob(identity)
	if err != nil {
		t.Fatalf("create named process job: %v", err)
	}
	defer windows.CloseHandle(job)

	command := exec.Command(os.Args[0], "-test.run=^TestWindowsReconcileChildHelper$")
	command.Env = []string{"POLIS_RUNNER_HELPER=host-reconcile"}
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED}
	if err = command.Start(); err != nil {
		t.Fatalf("start suspended contained helper: %v", err)
	}
	waitStarted := false
	defer func() {
		if !waitStarted {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	if err = assignAndResumeWindowsProcess(job, uint32(command.Process.Pid)); err != nil {
		t.Fatalf("assign and resume contained helper: %v", err)
	}
	waitResult := make(chan error, 1)
	waitStarted = true
	go func() { waitResult <- command.Wait() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proof, reconcileErr := ReconcileWindowsProcessTree(ctx, identity)
	if reconcileErr != nil || !proof.For(identity) || proof.For(identity+"-other") {
		t.Fatalf("host reconciliation proof=%+v err=%v", proof, reconcileErr)
	}
	select {
	case waitErr := <-waitResult:
		if waitErr == nil {
			t.Fatal("contained helper exited successfully instead of being stopped")
		}
	case <-time.After(time.Second):
		t.Fatal("contained helper process did not exit after reconciliation")
	}
}

func TestWindowsWorkerProcessUsesSessionNamedJobForRestartReconciliation(t *testing.T) {
	identity := "worker-reconcile-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	process, err := Start(identity, []string{os.Args[0], "-test.run=^TestWindowsReconcileChildHelper$"}, []string{"POLIS_RUNNER_HELPER=host-reconcile"})
	if err != nil {
		t.Fatalf("start contained worker helper: %v", err)
	}
	defer process.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proof, err := ReconcileWindowsWorkerProcessTree(ctx, identity, process.PID())
	if err != nil || !proof.ForWorkerSession(identity, process.PID()) {
		t.Fatalf("worker-session recovery proof=%+v err=%v", proof, err)
	}
	_ = process.WaitError()
	process.mu.Lock()
	cleanupErr := process.cleanupErr
	process.mu.Unlock()
	if cleanupErr != nil {
		t.Fatalf("worker process monitor did not confirm the reaped tree: %v", cleanupErr)
	}
	if !process.HasExited() {
		t.Fatal("worker helper remained alive after host reconciliation")
	}
}

// TestMain holds this child in the host-reconcile mode until the Job Object is
// terminated; the test function itself is never reached in that mode.
func TestWindowsReconcileChildHelper(t *testing.T) {}
