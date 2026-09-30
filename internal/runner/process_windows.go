// pattern: Imperative Shell

package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Process struct {
	spec       ProcessLaunchSpec
	In         io.WriteCloser
	Out        io.ReadCloser
	Err        io.ReadCloser
	done       chan struct{}
	id         string
	pid        int
	process    windows.Handle
	job        windows.Handle
	waitErr    error
	cleanupErr error
	exitCode   uint32
	mu         sync.Mutex
	stopMu     sync.Mutex
	proof      StopProof
}

func (p StopProof) Description() string {
	return processStopDescription("windows-job-object-tree", p.pid)
}
func (p *Process) Identity() (string, int) { return p.id, p.pid }
func (p *Process) PID() int {
	if p == nil {
		return 0
	}
	return p.pid
}
func (*Process) ContainmentMetadata() ProcessContainmentMetadata {
	return CurrentProcessContainmentMetadata()
}
func CurrentProcessContainmentMetadata() ProcessContainmentMetadata {
	return ProcessContainmentMetadata{HostOS: "windows", Profile: "windows_worker_job_object@1"}
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
	if id == "" || len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "process identity is required"}
	}
	job, err := newWindowsWorkerProcessJob(id)
	if err != nil {
		return nil, &LaunchFailure{Phase: LaunchPhaseProcessCreate, ReasonCode: LaunchReasonProcessNotCreated, SafeMessage: "unable to create process containment job"}
	}
	return startWindowsWorkerProcess(id, spec, job)
}

func startWindowsWorkerProcess(id string, spec ProcessLaunchSpec, job windows.Handle) (*Process, error) {
	closeJob := func() { _ = windows.CloseHandle(job) }
	executable, err := exec.LookPath(spec.Argv[0])
	if err != nil {
		closeJob()
		return nil, ClassifyProcessCreateFailure(err, spec.WorkingDirectory)
	}
	if !filepath.IsAbs(executable) {
		executable, err = filepath.Abs(executable)
		if err != nil {
			closeJob()
			return nil, ClassifyProcessCreateFailure(err, spec.WorkingDirectory)
		}
	}
	if spec.WorkingDirectory != "" {
		info, statErr := os.Stat(spec.WorkingDirectory)
		if statErr != nil || !info.IsDir() {
			closeJob()
			return nil, NewLaunchFailure(LaunchPhaseWorkingDirectory, LaunchReasonCWDMissing, statErr, false, 0)
		}
	}
	commandLine, err := windowsCommandLine(spec.Argv)
	if err != nil {
		closeJob()
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "process command line is invalid", Cause: err}
	}
	executableUTF16, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		closeJob()
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "process executable path is invalid", Cause: err}
	}
	commandLineUTF16, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		closeJob()
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "process command line is invalid", Cause: err}
	}
	var workingDirectoryUTF16 *uint16
	if spec.WorkingDirectory != "" {
		workingDirectoryUTF16, err = windows.UTF16PtrFromString(filepath.Clean(spec.WorkingDirectory))
		if err != nil {
			closeJob()
			return nil, NewLaunchFailure(LaunchPhaseWorkingDirectory, LaunchReasonCWDMissing, err, false, 0)
		}
	}
	environmentBlock, err := windowsProcessEnvironmentBlock(spec.Environment)
	if err != nil {
		closeJob()
		return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonEnvironmentInvalid, SafeMessage: "process environment is invalid", Cause: err}
	}

	var childStdin, parentStdin, parentStdout, childStdout, parentStderr, childStderr windows.Handle
	cleanupPipes := func() {
		for _, handle := range []windows.Handle{childStdin, parentStdin, parentStdout, childStdout, parentStderr, childStderr} {
			if handle != 0 {
				_ = windows.CloseHandle(handle)
			}
		}
	}
	attributes := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	if err = windows.CreatePipe(&childStdin, &parentStdin, attributes, 0); err == nil {
		err = windows.CreatePipe(&parentStdout, &childStdout, attributes, 0)
	}
	if err == nil {
		err = windows.CreatePipe(&parentStderr, &childStderr, attributes, 0)
	}
	if err != nil {
		cleanupPipes()
		closeJob()
		return nil, ClassifyPipeSetupFailure(err)
	}
	for _, parentHandle := range []windows.Handle{parentStdin, parentStdout, parentStderr} {
		if err = windows.SetHandleInformation(parentHandle, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
			cleanupPipes()
			closeJob()
			return nil, ClassifyPipeSetupFailure(err)
		}
	}
	attributeList, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		cleanupPipes()
		closeJob()
		return nil, ClassifyPipeSetupFailure(err)
	}
	defer attributeList.Delete()
	childHandles := []windows.Handle{childStdin, childStdout, childStderr}
	jobHandles := []windows.Handle{job}
	var pinner runtime.Pinner
	pinner.Pin(&childHandles[0])
	pinner.Pin(&jobHandles[0])
	if err = attributeList.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&childHandles[0]), uintptr(len(childHandles))*unsafe.Sizeof(childHandles[0])); err == nil {
		err = attributeList.Update(procThreadAttributeJobList, unsafe.Pointer(&jobHandles[0]), unsafe.Sizeof(jobHandles[0]))
	}
	if err != nil {
		pinner.Unpin()
		cleanupPipes()
		closeJob()
		return nil, ClassifyPipeSetupFailure(err)
	}
	startup := windows.StartupInfoEx{}
	startup.StartupInfo.Cb = uint32(unsafe.Sizeof(startup))
	startup.StartupInfo.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
	startup.StartupInfo.ShowWindow = windows.SW_HIDE
	startup.StartupInfo.StdInput = childStdin
	startup.StartupInfo.StdOutput = childStdout
	startup.StartupInfo.StdErr = childStderr
	startup.ProcThreadAttributeList = attributeList.List()
	flags := uint32(windows.CREATE_SUSPENDED | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NO_WINDOW)
	var environmentPointer *uint16
	if len(environmentBlock) > 0 {
		pinner.Pin(&environmentBlock[0])
		environmentPointer = &environmentBlock[0]
	}
	var processInfo windows.ProcessInformation
	err = windows.CreateProcess(executableUTF16, commandLineUTF16, nil, nil, true, flags, environmentPointer, workingDirectoryUTF16, &startup.StartupInfo, &processInfo)
	pinner.Unpin()
	runtime.KeepAlive(childHandles)
	runtime.KeepAlive(jobHandles)
	runtime.KeepAlive(environmentBlock)
	if err != nil {
		cleanupPipes()
		closeJob()
		return nil, ClassifyProcessCreateFailure(err, spec.WorkingDirectory)
	}
	for _, handle := range []windows.Handle{childStdin, childStdout, childStderr} {
		_ = windows.CloseHandle(handle)
	}
	childStdin, childStdout, childStderr = 0, 0, 0
	previousSuspendCount, resumeErr := windows.ResumeThread(processInfo.Thread)
	_ = windows.CloseHandle(processInfo.Thread)
	if resumeErr != nil || previousSuspendCount != 1 {
		_ = windows.TerminateJobObject(job, 1)
		_, _ = windows.WaitForSingleObject(processInfo.Process, 3000)
		_ = windows.CloseHandle(processInfo.Process)
		cleanupPipes()
		closeJob()
		cause := resumeErr
		if cause == nil {
			cause = errors.New("suspended process main thread had an unexpected suspend count")
		}
		return nil, &LaunchFailure{Phase: LaunchPhaseProcessCreate, ReasonCode: LaunchReasonProcessNotCreated, ProcessCreated: true, PIDPresent: true, SafeMessage: "unable to resume contained process", Cause: cause}
	}
	process := &Process{
		spec: spec, In: os.NewFile(uintptr(parentStdin), "worker-stdin"),
		Out: os.NewFile(uintptr(parentStdout), "worker-stdout"), Err: os.NewFile(uintptr(parentStderr), "worker-stderr"),
		done: make(chan struct{}), id: id, pid: int(processInfo.ProcessId), process: processInfo.Process, job: job,
	}
	parentStdin, parentStdout, parentStderr = 0, 0, 0
	go process.monitor()
	return process, nil
}

func windowsCommandLine(argv []string) (string, error) {
	quoted := make([]string, len(argv))
	for index, argument := range argv {
		if strings.ContainsRune(argument, '\x00') {
			return "", errors.New("process argument contains a null byte")
		}
		quoted[index] = syscall.EscapeArg(argument)
	}
	return strings.Join(quoted, " "), nil
}

func windowsProcessEnvironmentBlock(environment []string) ([]uint16, error) {
	if environment == nil {
		return []uint16{0, 0}, nil
	}
	if len(environment) > 512 {
		return nil, errors.New("process environment has too many entries")
	}
	entries := append([]string(nil), environment...)
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name == "" || strings.HasPrefix(name, "=") || strings.ContainsRune(entry, '\x00') {
			return nil, errors.New("process environment entry is malformed")
		}
		key := strings.ToUpper(name)
		if _, duplicate := seen[key]; duplicate {
			return nil, errors.New("process environment contains duplicate names")
		}
		seen[key] = struct{}{}
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToUpper(entries[i]) < strings.ToUpper(entries[j]) })
	block := strings.Join(entries, "\x00") + "\x00\x00"
	encoded := utf16.Encode([]rune(block))
	if len(encoded) > 32767 {
		return nil, errors.New("process environment exceeds the Windows bound")
	}
	return encoded, nil
}

func workerProcessJobObjectName(identity string) string {
	digest := sha256.Sum256([]byte(identity))
	return "Global\\Polis-Worker-" + hex.EncodeToString(digest[:])
}

func newWindowsWorkerProcessJob(identity string) (windows.Handle, error) {
	if identity == "" {
		return 0, errors.New("worker process identity is required")
	}
	return newWindowsNamedJobObject(workerProcessJobObjectName(identity))
}

func ReconcileWindowsWorkerProcessTree(ctx context.Context, identity string, pid int) (WindowsProcessTreeStopProof, error) {
	if ctx == nil || identity == "" || pid < 0 {
		return WindowsProcessTreeStopProof{}, errors.New("worker process reconciliation identity is invalid")
	}
	return reconcileWindowsNamedProcessTree(ctx, workerProcessJobObjectName(identity), identity, pid, "worker_session")
}

func StartWithLaunchSpecInCgroup(id string, spec ProcessLaunchSpec, _ int) (*Process, error) {
	return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "Linux cgroup placement is unavailable on Windows"}
}

func StartWithLaunchSpecInWorkerCgroup(_ string, _ ProcessLaunchSpec, cgroup WorkerProcessCgroup) (*Process, error) {
	if cgroup != nil {
		if err := cgroup.Cleanup(); err != nil {
			return nil, errors.Join(&LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "Linux Worker cgroups are unavailable on Windows"}, err)
		}
	}
	return nil, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "Linux Worker cgroups are unavailable on Windows"}
}

func assignAndResumeWindowsProcess(job windows.Handle, processID uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION, false, processID)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	if err = windows.AssignProcessToJobObject(job, process); err != nil {
		return err
	}
	threadID, err := suspendedMainThreadID(processID)
	if err != nil {
		return err
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME|windows.THREAD_QUERY_INFORMATION, false, threadID)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(thread)
	previousSuspendCount, err := windows.ResumeThread(thread)
	if err != nil {
		return err
	}
	if previousSuspendCount != 1 {
		return errors.New("suspended process main thread had an unexpected suspend count")
	}
	return nil
}

func suspendedMainThreadID(processID uint32) (uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID == processID {
			return entry.ThreadID, nil
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return 0, err
	}
	return 0, errors.New("suspended process main thread was not found")
}

func (p *Process) Stop() (StopProof, error) {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if p.proof.stopped {
		return p.proof, nil
	}
	p.mu.Lock()
	job := p.job
	if job != 0 {
		err := windows.TerminateJobObject(job, 1)
		p.mu.Unlock()
		if err != nil {
			select {
			case <-p.done:
				return p.confirmStopWithHostFallback()
			default:
				return StopProof{}, err
			}
		}
	} else {
		p.mu.Unlock()
	}
	select {
	case <-p.done:
		return p.confirmStopWithHostFallback()
	case <-time.After(5 * time.Second):
		return StopProof{}, errors.New("process termination unconfirmed")
	}
}

func (p *Process) confirmStopWithHostFallback() (StopProof, error) {
	proof, err := p.confirmedStopProof()
	if err == nil {
		return proof, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	hostProof, reconcileErr := ReconcileWindowsWorkerProcessTree(ctx, p.id, p.pid)
	cancel()
	if reconcileErr != nil {
		return StopProof{}, errors.Join(err, reconcileErr)
	}
	if confirmErr := p.ConfirmHostTreeStopped(hostProof); confirmErr != nil {
		return StopProof{}, errors.Join(err, confirmErr)
	}
	return p.confirmedStopProof()
}

func (p *Process) monitor() {
	processHandle := p.process
	_, waitErr := windows.WaitForSingleObject(processHandle, windows.INFINITE)
	var exitCode uint32
	if waitErr == nil {
		waitErr = windows.GetExitCodeProcess(processHandle, &exitCode)
	}
	p.mu.Lock()
	jobHandle := p.job
	p.job = 0
	p.process = 0
	p.waitErr = waitErr
	p.exitCode = exitCode
	p.mu.Unlock()
	processCloseErr := windows.CloseHandle(processHandle)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	cleanupErr := errors.Join(processCloseErr, terminateAndWaitWindowsProcessTree(cleanupCtx, jobHandle))
	cancel()
	cleanupErr = errors.Join(cleanupErr, windows.CloseHandle(jobHandle))
	_ = p.In.Close()
	p.mu.Lock()
	p.cleanupErr = cleanupErr
	p.mu.Unlock()
	close(p.done)
}

func (p *Process) confirmedStopProof() (StopProof, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cleanupErr != nil {
		return StopProof{}, errors.Join(ErrProcessTreeStopUnconfirmed, p.cleanupErr)
	}
	p.proof = StopProof{id: p.id, pid: p.pid, stopped: true}
	return p.proof, nil
}

func (p *Process) ConfirmHostTreeStopped(proof WindowsProcessTreeStopProof) error {
	if p == nil || !proof.ForWorkerSession(p.id, p.pid) {
		return ErrProcessTreeStopUnconfirmed
	}
	select {
	case <-p.done:
	default:
		return ErrProcessTreeStopUnconfirmed
	}
	p.mu.Lock()
	p.cleanupErr = nil
	p.mu.Unlock()
	return nil
}

func (p *Process) WaitError() error {
	if p == nil {
		return nil
	}
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	var failures []error
	if p.waitErr != nil {
		failures = append(failures, p.waitErr)
	}
	if p.cleanupErr != nil {
		failures = append(failures, errors.Join(ErrProcessTreeStopUnconfirmed, p.cleanupErr))
	}
	if p.exitCode != 0 {
		failures = append(failures, ProcessExitError{ExitCode: p.exitCode})
	}
	return errors.Join(failures...)
}
