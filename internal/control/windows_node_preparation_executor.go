// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

var registryProxyCredentialPattern = regexp.MustCompile(`(?i)http://[^\s/:@]+:[^\s/@]+@127\.0\.0\.1:([0-9]+)`)

const maxConcurrentWindowsNodePreparations = 2
const maxQueuedWindowsNodePreparations = 8

var errEnvironmentPreparationQueueFull = errors.New("Windows Node/npm preparation queue is full")

type WindowsNodeNPMPreparationExecutor struct {
	nodeSourcePath   string
	npmCLISourcePath string
	systemRoot       string
	workspaceRoot    string
	controlRoot      string
	preparedMu       sync.Mutex
	prepared         map[string]retainedEnvironmentSnapshot
	pendingCleanup   map[string]retainedEnvironmentSnapshot
	jobProcesses     map[string]retainedProjectJobProcess
	closed           bool
	preparations     sync.WaitGroup
	preparationSlots chan struct{}
	preparationQueue chan struct{}
	preparationClose chan struct{}
}

type retainedProjectJobProcess struct {
	companyID  string
	revisionID string
	snapshot   *environment.WindowsNodeAppContainerSnapshot
	process    runner.AppContainerProcess
}

type retainedEnvironmentSnapshot struct {
	companyID              string
	runID                  string
	snapshot               *environment.WindowsNodeAppContainerSnapshot
	process                runner.AppContainerProcess
	releasePreparationSlot func()
	invalidated            bool
}

func NewWindowsNodeNPMPreparationExecutor(nodeSourcePath, npmCLISourcePath, systemRoot, workspaceRoot string) (*WindowsNodeNPMPreparationExecutor, error) {
	if runtime.GOOS != "windows" || !filepath.IsAbs(nodeSourcePath) || !filepath.IsAbs(npmCLISourcePath) || !filepath.IsAbs(systemRoot) || !filepath.IsAbs(workspaceRoot) {
		return nil, errors.New("Windows Node/npm preparation requires absolute toolchain and bounded-workspace paths")
	}
	if !regularNoLink(nodeSourcePath) || !regularNoLink(npmCLISourcePath) {
		return nil, errors.New("Windows Node/npm preparation requires regular administrator-selected toolchain files")
	}
	if info, err := os.Stat(systemRoot); err != nil || !info.IsDir() {
		return nil, errors.New("Windows Node/npm preparation requires a valid Windows system root")
	}
	workspaceInfo, err := os.Lstat(workspaceRoot)
	if err != nil || !workspaceInfo.IsDir() || workspaceInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(environment.ErrWindowsNodeWorkspaceStorage, errors.New("workspace storage root must be an existing non-link directory"))
	}
	executable, err := os.Executable()
	if err != nil || !filepath.IsAbs(executable) {
		return nil, errors.New("Windows Node/npm preparation could not resolve the control executable path")
	}
	return &WindowsNodeNPMPreparationExecutor{
		nodeSourcePath: filepath.Clean(nodeSourcePath), npmCLISourcePath: filepath.Clean(npmCLISourcePath), systemRoot: filepath.Clean(systemRoot),
		workspaceRoot: filepath.Clean(workspaceRoot), controlRoot: filepath.Dir(executable),
	}, nil
}

func (executor *WindowsNodeNPMPreparationExecutor) PrepareProjectEnvironment(ctx context.Context, runID string, loadSnapshot ProjectEnvironmentSnapshotLoader) (EnvironmentPreparationResult, error) {
	if executor == nil || ctx == nil || loadSnapshot == nil {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_executor_input_invalid"}, errors.New("Windows Node/npm executor received invalid preparation input")
	}
	releaseSlot, finishPreparation, accepted, admissionErr := executor.beginPreparation(ctx)
	if !accepted {
		if ctx.Err() != nil {
			return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
		}
		if errors.Is(admissionErr, errEnvironmentPreparationQueueFull) {
			return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_executor_busy"}, admissionErr
		}
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_executor_closed"}, errors.New("Windows Node/npm preparation executor is closed")
	}
	defer finishPreparation()
	transferredSlot := false
	defer func() {
		if !transferredSlot {
			releaseSlot()
		}
	}()
	if err := ctx.Err(); err != nil {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
	}
	input, err := loadSnapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
		}
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_snapshot_unavailable"}, err
	}
	if input.Revision.ProfileID != environment.WindowsNodeNPMProfile || input.Policy.ProfileID != input.Revision.ProfileID || input.Policy.NetworkPolicy != "registry_allowlist" {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_executor_profile_mismatch"}, errors.New("Windows Node/npm executor received an unsupported profile")
	}
	if err := ctx.Err(); err != nil {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
	}
	identity, workspaceID, err := newWindowsEnvironmentExecutionIDs()
	if err != nil {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_execution_identity_unavailable"}, err
	}
	snapshot, err := environment.PrepareWindowsNodeNPMInstallSnapshot(identity, workspaceID, input.Files, input.Plan, input.Policy.RegistryHosts, input.Revision.ToolchainSHA256, executor.nodeSourcePath, executor.npmCLISourcePath, environment.WindowsNodeWorkspaceStorageConfig{
		WorkspaceRoot: executor.workspaceRoot, ControlRoot: executor.controlRoot, SystemRoot: executor.systemRoot,
		ExpectedIsolationPolicySHA256: input.AuthorizedIsolationPolicySHA256,
	})
	if err != nil {
		if snapshot != nil {
			if closeErr := retrySnapshotCleanup(snapshot); closeErr != nil {
				_ = executor.storePendingCleanup(retainedEnvironmentSnapshot{companyID: input.Revision.CompanyID, runID: runID, snapshot: snapshot})
				return EnvironmentPreparationResult{TerminalState: string(environment.PreparationOutcomeUnknown), ReasonCode: "environment_prepare_cleanup_incomplete"}, errors.Join(err, closeErr)
			}
		}
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "environment_snapshot_materialization_failed"}, err
	}
	owner := retainedEnvironmentSnapshot{companyID: input.Revision.CompanyID, runID: runID, snapshot: snapshot}
	if err = ctx.Err(); err != nil {
		result := EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}
		return executor.finishInstall(owner, result, nil, "")
	}
	process, launchErr := snapshot.LaunchNPMInstall(executor.systemRoot)
	if launchErr != nil {
		return executor.finishInstall(owner, EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: "dependency_install_launch_failed"}, launchErr, "")
	}
	owner.process = process
	if stdin := process.Stdin(); stdin != nil {
		_ = stdin.Close()
	}
	logLimit := input.Policy.OutputLimitBytes
	if logLimit > maxEnvironmentPreparationArtifactBytes {
		logLimit = maxEnvironmentPreparationArtifactBytes
	}
	streamLimit := (logLimit - len("stdout:\n") - len("\nstderr:\n")) / 2
	stdout := capturePreparationStream(process.Stdout(), streamLimit)
	stderr := capturePreparationStream(process.Stderr(), streamLimit)
	processID := process.PID()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(input.Policy.TimeoutMS)*time.Millisecond)
	defer cancel()
	exitCode, waitErr := process.Wait(runCtx)
	wasCancelled := false
	if waitErr != nil {
		proof, stopErr := process.Stop()
		stopConfirmed := stopErr == nil && proof.For(workspaceID) && proof.PID() == processID
		if stopConfirmed && (errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded)) {
			wasCancelled = true
			waitErr = nil
		} else if !stopConfirmed {
			stdout.abort()
			stderr.abort()
			var releaseOnce sync.Once
			owner.releasePreparationSlot = func() { releaseOnce.Do(releaseSlot) }
			transferredSlot = true
			result := EnvironmentPreparationResult{TerminalState: string(environment.PreparationOutcomeUnknown), ReasonCode: "dependency_install_stop_unconfirmed"}
			finished, finishErr := executor.finishInstall(owner, result, errors.Join(waitErr, stopErr, errors.New("npm install process-tree stop could not be proven")), "")
			executor.monitorPendingProcess(owner)
			return finished, finishErr
		}
	}
	stdoutResult := stdout.wait()
	stderrResult := stderr.wait()
	logs, logsTruncated := combinePreparationLogs(stdoutResult, stderrResult)
	result := EnvironmentPreparationResult{Logs: redactRegistryProxyCredentials(logs), LogsTruncated: logsTruncated}
	if waitErr != nil || stdoutResult.err != nil || stderrResult.err != nil {
		result.TerminalState = string(environment.PreparationOutcomeUnknown)
		result.ReasonCode = "dependency_install_outcome_unknown"
		return executor.finishInstall(owner, result, errors.Join(waitErr, stdoutResult.err, stderrResult.err), "")
	}
	if wasCancelled {
		result.TerminalState = string(environment.PreparationCancelled)
		result.ReasonCode = "environment_preparation_cancelled"
		return executor.finishInstall(owner, result, nil, "")
	}
	if exitCode != 0 {
		result.TerminalState = string(environment.PreparationFailed)
		result.ReasonCode = "dependency_install_failed"
		result.Evidence = environmentPreparationEvidence(input, exitCode)
		return executor.finishInstall(owner, result, nil, "")
	}
	result.TerminalState = string(environment.PreparationReady)
	result.ReasonCode = "dependency_install_ready"
	result.Evidence = environmentPreparationEvidence(input, exitCode)
	return executor.finishInstall(owner, result, nil, input.Revision.RevisionID)
}

func (executor *WindowsNodeNPMPreparationExecutor) finishInstall(owner retainedEnvironmentSnapshot, result EnvironmentPreparationResult, cause error, retainRevisionID string) (EnvironmentPreparationResult, error) {
	snapshot := owner.snapshot
	var revokeErr error
	for attempt := 0; attempt < 2; attempt++ {
		revokeErr = snapshot.RevokeRegistryEgress()
		if revokeErr == nil {
			break
		}
	}
	if revokeErr == nil && cause == nil && retainRevisionID != "" && result.TerminalState == string(environment.PreparationReady) {
		if retainErr := executor.storePreparedSnapshot(retainRevisionID, owner); retainErr == nil {
			return result, nil
		} else {
			cause = retainErr
			result.TerminalState = string(environment.PreparationOutcomeUnknown)
			result.ReasonCode = "prepared_environment_retention_failed"
		}
	}
	closeErr := retrySnapshotCleanup(snapshot)
	if revokeErr != nil || closeErr != nil {
		result.TerminalState = string(environment.PreparationOutcomeUnknown)
		result.ReasonCode = "registry_egress_revoke_or_cleanup_failed"
		cause = errors.Join(cause, revokeErr, closeErr)
		if closeErr != nil {
			if retainErr := executor.storePendingCleanup(owner); retainErr != nil {
				cause = errors.Join(cause, retainErr)
			}
		}
	}
	return result, cause
}

func (executor *WindowsNodeNPMPreparationExecutor) storePreparedSnapshot(revisionID string, owner retainedEnvironmentSnapshot) error {
	if executor == nil || revisionID == "" || owner.snapshot == nil || owner.companyID == "" || owner.runID == "" {
		return errors.New("prepared environment snapshot is incomplete")
	}
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.closed {
		return errors.New("Windows Node/npm preparation executor is closed")
	}
	if executor.prepared == nil {
		executor.prepared = make(map[string]retainedEnvironmentSnapshot)
	}
	if _, exists := executor.prepared[revisionID]; exists {
		return errors.New("prepared environment already exists for this revision")
	}
	executor.prepared[revisionID] = owner
	return nil
}

func (executor *WindowsNodeNPMPreparationExecutor) storePendingCleanup(owner retainedEnvironmentSnapshot) error {
	if executor == nil || owner.snapshot == nil {
		return errors.New("pending AppContainer cleanup is incomplete")
	}
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.pendingCleanup == nil {
		executor.pendingCleanup = make(map[string]retainedEnvironmentSnapshot)
	}
	executor.pendingCleanup[fmt.Sprintf("%p", owner.snapshot)] = owner
	return nil
}

func (executor *WindowsNodeNPMPreparationExecutor) preparedSnapshot(revisionID string) (*environment.WindowsNodeAppContainerSnapshot, bool) {
	if executor == nil {
		return nil, false
	}
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.closed {
		return nil, false
	}
	owner, exists := executor.prepared[revisionID]
	return owner.snapshot, exists
}

func (executor *WindowsNodeNPMPreparationExecutor) HasPreparedEnvironment(revisionID string) bool {
	_, exists := executor.preparedSnapshot(revisionID)
	return exists
}

func (*WindowsNodeNPMPreparationExecutor) SupportsProjectEnvironmentProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile
}

func (*WindowsNodeNPMPreparationExecutor) SupportsProjectJobProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile
}

func (executor *WindowsNodeNPMPreparationExecutor) LaunchProjectJob(ctx context.Context, request ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	if executor == nil || ctx == nil || request.ProfileID != environment.WindowsNodeNPMProfile || !core.ValidID(request.CompanyID) || !core.ValidID(request.JobID) || !core.ValidID(request.EnvironmentRevisionID) {
		return nil, core.Malformed
	}
	if request.ServiceProbe != nil {
		if !core.ValidID(request.ServiceID) {
			return nil, core.Malformed
		}
		if _, err := environment.ServiceProbeSpecSHA256(*request.ServiceProbe); err != nil {
			return nil, core.Malformed
		}
	} else if request.ServiceID != "" {
		return nil, core.Malformed
	}
	executor.preparedMu.Lock()
	if executor.closed {
		executor.preparedMu.Unlock()
		return nil, errors.New("Windows Node/npm preparation executor is closed")
	}
	owner, exists := executor.prepared[request.EnvironmentRevisionID]
	if !exists || owner.invalidated || owner.companyID != request.CompanyID || owner.snapshot == nil {
		executor.preparedMu.Unlock()
		return nil, core.Denied
	}
	if _, exists := executor.jobProcesses[request.JobID]; exists {
		executor.preparedMu.Unlock()
		return nil, core.Conflict
	}
	executor.preparations.Add(1)
	executor.preparedMu.Unlock()
	defer executor.preparations.Done()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var (
		process runner.AppContainerProcess
		err     error
	)
	if request.ServiceProbe != nil {
		listener := runner.AppContainerServiceListenerSpec{BindAddress: request.ServiceProbe.BindAddress, Port: request.ServiceProbe.Port}
		process, err = owner.snapshot.LaunchNodeProjectService(request.JobID, request.ScriptPath, listener, executor.systemRoot)
	} else {
		process, err = owner.snapshot.LaunchNodeProjectScript(request.JobID, request.ScriptPath, request.Args, executor.systemRoot)
	}
	if err != nil {
		return nil, err
	}
	executor.preparedMu.Lock()
	if executor.jobProcesses == nil {
		executor.jobProcesses = make(map[string]retainedProjectJobProcess)
	}
	if _, exists = executor.jobProcesses[request.JobID]; exists {
		executor.preparedMu.Unlock()
		stopErr := owner.snapshot.StopNodeProjectProcess(process, request.JobID)
		if stopErr != nil {
			monitorProjectJobProcessExit(process)
			return nil, errors.Join(core.Conflict, stopErr)
		}
		_, waitErr := process.Wait(context.Background())
		return nil, errors.Join(core.Conflict, waitErr)
	}
	executor.jobProcesses[request.JobID] = retainedProjectJobProcess{companyID: request.CompanyID, revisionID: request.EnvironmentRevisionID, snapshot: owner.snapshot, process: process}
	closed := executor.closed
	executor.preparedMu.Unlock()
	if closed || ctx.Err() != nil {
		stopErr := owner.snapshot.StopNodeProjectProcess(process, request.JobID)
		if stopErr != nil {
			executor.monitorUnconfirmedProjectJob(request.JobID, process)
			return process, errors.Join(ctx.Err(), errors.New("Windows Node/npm executor closed during project launch"), stopErr)
		}
		_, waitErr := process.Wait(context.Background())
		executor.ForgetProjectJob(request.JobID)
		if ctx.Err() != nil {
			return nil, errors.Join(ctx.Err(), waitErr)
		}
		return nil, errors.Join(errors.New("Windows Node/npm executor closed during project launch"), waitErr)
	}
	return process, nil
}

func (executor *WindowsNodeNPMPreparationExecutor) StopProjectJob(_ context.Context, companyID, jobID string) error {
	if executor == nil {
		return errors.New("Windows Node/npm preparation executor is unavailable")
	}
	executor.preparedMu.Lock()
	job, exists := executor.jobProcesses[jobID]
	executor.preparedMu.Unlock()
	if !exists || job.companyID != companyID || job.snapshot == nil || job.process == nil {
		return core.OutOfScope
	}
	stopErr := job.snapshot.StopNodeProjectProcess(job.process, jobID)
	if stopErr == nil {
		return nil
	}
	if exitState, ok := job.process.(interface{ HasExited() bool }); !ok || !exitState.HasExited() {
		return stopErr
	}
	reconcileCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	proof, reconcileErr := runner.ReconcileWindowsProcessTree(reconcileCtx, jobID)
	cancel()
	if reconcileErr != nil {
		return errors.Join(stopErr, reconcileErr)
	}
	confirmer, ok := job.process.(interface {
		ConfirmTreeStopped(runner.WindowsProcessTreeStopProof) error
	})
	if !ok {
		return errors.Join(stopErr, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	if confirmErr := confirmer.ConfirmTreeStopped(proof); confirmErr != nil {
		return errors.Join(stopErr, confirmErr)
	}
	executor.ForgetProjectJob(jobID)
	return nil
}

// ReconcileUnrestoredProjectJob confirms that a prior Windows Job Object is no
// longer held by the previous Control process. Job Objects use
// KILL_ON_JOB_CLOSE, and their handles are not inherited by the child, so an
// untracked process tree is not replayed after daemon restart.
func (executor *WindowsNodeNPMPreparationExecutor) ReconcileUnrestoredProjectJob(companyID, jobID, revisionID string) error {
	if executor == nil || runtime.GOOS != "windows" || !core.ValidID(companyID) || !core.ValidID(jobID) || !core.ValidID(revisionID) {
		return core.Malformed
	}
	executor.preparedMu.Lock()
	job, exists := executor.jobProcesses[jobID]
	executor.preparedMu.Unlock()
	if exists {
		if job.companyID != companyID || job.revisionID != revisionID || job.process == nil {
			return core.OutOfScope
		}
		exitState, supported := job.process.(interface{ HasExited() bool })
		if !supported || !exitState.HasExited() {
			return core.ConflictError{Reason: "tracked Windows Job Object process has not confirmed exit", CurrentState: string(environment.JobOutcomeUnknown)}
		}
		// HasExited reports the root process state. The separate cleanup wait
		// confirms the monitor closed the Job Object that owns descendants.
		cleanupWaiter, supported := job.process.(interface{ WaitForTreeCleanup(context.Context) error })
		if !supported {
			return core.ConflictError{Reason: "Windows Job Object cleanup cannot be confirmed", CurrentState: string(environment.JobOutcomeUnknown)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		waitErr := cleanupWaiter.WaitForTreeCleanup(ctx)
		cancel()
		if waitErr != nil {
			reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 5*time.Second)
			proof, reconcileErr := runner.ReconcileWindowsProcessTree(reconcileCtx, jobID)
			reconcileCancel()
			if reconcileErr != nil {
				return core.ConflictError{Reason: "Windows Job Object cleanup is not confirmed", CurrentState: string(environment.JobOutcomeUnknown)}
			}
			confirmer, ok := job.process.(interface {
				ConfirmTreeStopped(runner.WindowsProcessTreeStopProof) error
			})
			if !ok || confirmer.ConfirmTreeStopped(proof) != nil {
				return core.ConflictError{Reason: "Windows Job Object ownership release is not confirmed", CurrentState: string(environment.JobOutcomeUnknown)}
			}
			executor.ForgetProjectJob(jobID)
			return nil
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := runner.ReconcileWindowsProcessTree(ctx, jobID); err != nil {
		return core.ConflictError{Reason: "Windows Job Object stop is not confirmed", CurrentState: string(environment.JobOutcomeUnknown)}
	}
	return nil
}

func (executor *WindowsNodeNPMPreparationExecutor) ForgetProjectJob(jobID string) {
	if executor == nil {
		return
	}
	executor.preparedMu.Lock()
	delete(executor.jobProcesses, jobID)
	executor.preparedMu.Unlock()
}

func (executor *WindowsNodeNPMPreparationExecutor) monitorUnconfirmedProjectJob(jobID string, process runner.AppContainerProcess) {
	go func() {
		_, waitErr := process.Wait(context.Background())
		if waitErr == nil {
			executor.ForgetProjectJob(jobID)
		}
	}()
}

func monitorProjectJobProcessExit(process runner.AppContainerProcess) {
	go func() { _, _ = process.Wait(context.Background()) }()
}

func (executor *WindowsNodeNPMPreparationExecutor) PreparedRunsForShutdown() []preparedEnvironmentRunRef {
	if executor == nil {
		return nil
	}
	executor.preparedMu.Lock()
	executor.markClosedLocked()
	executor.preparedMu.Unlock()
	executor.preparations.Wait()
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	runs := make([]preparedEnvironmentRunRef, 0, len(executor.prepared))
	for revisionID, owner := range executor.prepared {
		if owner.invalidated {
			continue
		}
		owner.invalidated = true
		executor.prepared[revisionID] = owner
		runs = append(runs, preparedEnvironmentRunRef{companyID: owner.companyID, runID: owner.runID})
	}
	return runs
}

func (executor *WindowsNodeNPMPreparationExecutor) beginPreparation(ctx context.Context) (func(), func(), bool, error) {
	if executor == nil || ctx == nil || ctx.Err() != nil {
		return nil, nil, false, nil
	}
	executor.preparedMu.Lock()
	if executor.closed {
		executor.preparedMu.Unlock()
		return nil, nil, false, nil
	}
	if executor.preparationSlots == nil {
		executor.preparationSlots = make(chan struct{}, maxConcurrentWindowsNodePreparations)
	}
	if executor.preparationQueue == nil {
		executor.preparationQueue = make(chan struct{}, maxQueuedWindowsNodePreparations)
	}
	if executor.preparationClose == nil {
		executor.preparationClose = make(chan struct{})
	}
	slots, queue, closing := executor.preparationSlots, executor.preparationQueue, executor.preparationClose
	select {
	case queue <- struct{}{}:
	default:
		executor.preparedMu.Unlock()
		return nil, nil, false, errEnvironmentPreparationQueueFull
	}
	executor.preparations.Add(1)
	executor.preparedMu.Unlock()
	dequeue := func() { <-queue }
	select {
	case slots <- struct{}{}:
		dequeue()
		var slotOnce sync.Once
		releaseSlot := func() { slotOnce.Do(func() { <-slots }) }
		executor.preparedMu.Lock()
		closed := executor.closed
		executor.preparedMu.Unlock()
		if closed || ctx.Err() != nil {
			releaseSlot()
			executor.preparations.Done()
			return nil, nil, false, nil
		}
		var finishOnce sync.Once
		finish := func() { finishOnce.Do(executor.preparations.Done) }
		return releaseSlot, finish, true, nil
	case <-ctx.Done():
		dequeue()
		executor.preparations.Done()
		return nil, nil, false, nil
	case <-closing:
		dequeue()
		executor.preparations.Done()
		return nil, nil, false, nil
	}
}

func (executor *WindowsNodeNPMPreparationExecutor) markClosedLocked() {
	if executor.closed {
		return
	}
	executor.closed = true
	if executor.preparationClose != nil {
		close(executor.preparationClose)
	}
}

func (executor *WindowsNodeNPMPreparationExecutor) Close() error {
	if executor == nil {
		return nil
	}
	executor.preparedMu.Lock()
	executor.markClosedLocked()
	executor.preparedMu.Unlock()
	executor.preparations.Wait()
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	var closeErrors []error
	for jobID, job := range executor.jobProcesses {
		if stopErr := job.snapshot.StopNodeProjectProcess(job.process, jobID); stopErr != nil {
			closeErrors = append(closeErrors, stopErr)
			continue
		}
		if _, waitErr := job.process.Wait(context.Background()); waitErr != nil {
			closeErrors = append(closeErrors, waitErr)
			continue
		}
		delete(executor.jobProcesses, jobID)
	}
	for revisionID, owner := range executor.prepared {
		if err := owner.snapshot.Close(); err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		delete(executor.prepared, revisionID)
	}
	for cleanupID, owner := range executor.pendingCleanup {
		_ = owner.snapshot.RevokeRegistryEgress()
		if owner.process != nil {
			proof, stopErr := owner.process.Stop()
			if stopErr != nil || proof.PID() <= 0 || proof.PID() != owner.process.PID() {
				closeErrors = append(closeErrors, errors.Join(stopErr, errors.New("pending AppContainer process stop remains unconfirmed")))
				continue
			}
			_ = owner.snapshot.RevokeRegistryEgress()
		}
		if err := owner.snapshot.Close(); err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		delete(executor.pendingCleanup, cleanupID)
		if owner.releasePreparationSlot != nil {
			owner.releasePreparationSlot()
		}
	}
	return errors.Join(closeErrors...)
}

func (executor *WindowsNodeNPMPreparationExecutor) monitorPendingProcess(owner retainedEnvironmentSnapshot) <-chan struct{} {
	done := make(chan struct{})
	if owner.process == nil || owner.snapshot == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		_, _ = owner.process.Wait(context.Background())
		_ = owner.snapshot.RevokeRegistryEgress()
		if closeErr := retrySnapshotCleanup(owner.snapshot); closeErr != nil {
			return
		}
		executor.preparedMu.Lock()
		delete(executor.pendingCleanup, fmt.Sprintf("%p", owner.snapshot))
		executor.preparedMu.Unlock()
		if owner.releasePreparationSlot != nil {
			owner.releasePreparationSlot()
		}
	}()
	return done
}

type preparationStreamResult struct {
	name      string
	content   []byte
	truncated bool
	err       error
}

type preparationStreamCapture struct {
	reader io.ReadCloser
	result chan preparationStreamResult
	once   sync.Once
}

func capturePreparationStream(reader io.ReadCloser, limit int) *preparationStreamCapture {
	capture := &preparationStreamCapture{reader: reader, result: make(chan preparationStreamResult, 1)}
	go func() {
		capture.result <- drainPreparationStream(reader, limit)
		close(capture.result)
		capture.abort()
	}()
	return capture
}

func (capture *preparationStreamCapture) abort() {
	if capture == nil || capture.reader == nil {
		return
	}
	capture.once.Do(func() { _ = capture.reader.Close() })
}

func (capture *preparationStreamCapture) wait() preparationStreamResult {
	if capture == nil {
		return preparationStreamResult{}
	}
	result, ok := <-capture.result
	if !ok {
		return preparationStreamResult{}
	}
	return result
}

func drainPreparationStream(reader io.Reader, limit int) preparationStreamResult {
	result := preparationStreamResult{}
	if reader == nil {
		return result
	}
	buffer := make([]byte, 8192)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			available := limit - len(result.content)
			if available > count {
				available = count
			}
			if available > 0 {
				result.content = append(result.content, buffer[:available]...)
			}
			if available < count {
				result.truncated = true
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				result.err = err
			}
			return result
		}
	}
}

func combinePreparationLogs(stdout, stderr preparationStreamResult) ([]byte, bool) {
	parts := [][]byte{[]byte("stdout:\n"), stdout.content, []byte("\nstderr:\n"), stderr.content}
	var content []byte
	for _, part := range parts {
		content = append(content, part...)
	}
	return content, stdout.truncated || stderr.truncated
}

func redactRegistryProxyCredentials(logs []byte) []byte {
	return registryProxyCredentialPattern.ReplaceAll(logs, []byte("http://[redacted]@127.0.0.1:$1"))
}

func environmentPreparationEvidence(input kernel.ProjectEnvironmentExecutionSnapshot, exitCode int) []byte {
	evidence := struct {
		SchemaVersion         string `json:"schemaVersion"`
		EnvironmentRevisionID string `json:"environmentRevisionId"`
		SourceRevisionSHA256  string `json:"sourceRevisionSha256"`
		PackageJSONSHA256     string `json:"packageJsonSha256"`
		LockfileSHA256        string `json:"lockfileSha256"`
		PolicySHA256          string `json:"policySha256"`
		ToolchainSHA256       string `json:"toolchainSha256"`
		ExitCode              int    `json:"exitCode"`
	}{"windows-node-npm-preparation@1", input.Revision.RevisionID, input.Revision.SourceRevisionSHA256, input.Revision.PackageJSONSHA256, input.Revision.LockfileSHA256, input.Revision.PolicySHA256, input.Revision.ToolchainSHA256, exitCode}
	content, _ := json.Marshal(evidence)
	return content
}

func newWindowsEnvironmentExecutionIDs() (string, string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(random[:])
	return "polis-env-" + id, "env-" + id, nil
}

func regularNoLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

func retrySnapshotCleanup(snapshot *environment.WindowsNodeAppContainerSnapshot) error {
	if snapshot == nil {
		return nil
	}
	var closeErr error
	for attempt := 0; attempt < 3; attempt++ {
		closeErr = snapshot.Close()
		if closeErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return fmt.Errorf("AppContainer snapshot cleanup remains pending: %w", closeErr)
}
