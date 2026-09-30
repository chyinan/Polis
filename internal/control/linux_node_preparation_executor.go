// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

const maxConcurrentLinuxNodePreparations = 2
const maxQueuedLinuxNodePreparations = 8

var errLinuxNodePreparationQueueFull = errors.New("Linux Node/npm preparation queue is full")

type LinuxNodeNPMPreparationExecutor struct {
	paths                       environment.LinuxNodeSandboxPaths
	launch                      func(string, []string, environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error)
	resourceGroups              environment.LinuxNodeCgroupManager
	ownsResourceGroups          bool
	removeWorkspace             func(string, string) error
	preparedMu                  sync.Mutex
	prepared                    map[string]linuxNodeWorkspace
	pending                     map[string]linuxNodeWorkspace
	jobProcesses                map[string]linuxNodeProjectJob
	launchReservations          map[string]string
	pendingResourceGroups       map[string]linuxNodePendingCgroup
	unconfirmedJobCgroupCleanup map[string]string
	confirmedJobCgroupCleanup   map[string]string
	closed                      bool
	preparations                sync.WaitGroup
	slots                       chan struct{}
	queue                       chan struct{}
	closing                     chan struct{}
	enforceWorkspaceDiskLimit   bool
}

type linuxNodeWorkspace struct {
	companyID       string
	runID           string
	revisionID      string
	identity        string
	workspaceID     string
	workspaceRoot   string
	toolchainSHA256 string
	plan            environment.NodeNPMProjectPlan
	invalidated     bool
	cgroupGroup     environment.LinuxNodeResourceCgroup
}

type linuxNodeProjectJob struct {
	companyID   string
	revisionID  string
	workspaceID string
	identity    string
	process     runner.AppContainerProcess
}

type linuxNodePendingCgroup struct {
	companyID   string
	workspaceID string
	group       environment.LinuxNodeResourceCgroup
}

func NewLinuxNodeNPMPreparationExecutor(paths environment.LinuxNodeSandboxPaths) (*LinuxNodeNPMPreparationExecutor, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Linux Node/npm preparation is unavailable on %s", runtime.GOOS)
	}
	if err := environment.ValidateLinuxNodeSandboxPaths(paths); err != nil {
		return nil, fmt.Errorf("Linux Node/npm preparation configuration is invalid: %w", err)
	}
	resourceGroups, err := environment.NewLinuxNodeCgroupV2Manager(paths.RuntimeRoot, paths.CgroupRoot, paths.InstanceIdentity, environment.DefaultLinuxNodeResourceLimits())
	if err != nil {
		return nil, fmt.Errorf("Linux Node/npm cgroup resource controls are not qualified: %w", err)
	}
	if err = environment.VerifyLinuxNodeWorkspaceFilesystem(paths); err != nil {
		if closer, ok := resourceGroups.(environment.LinuxNodeCgroupManagerCloser); ok {
			_ = closer.Close()
		}
		return nil, fmt.Errorf("Linux Node/npm workspace disk bound is not enforced: %w", err)
	}
	executor, err := newLinuxNodeNPMPreparationExecutor(paths, resourceGroups)
	if err != nil {
		if closer, ok := resourceGroups.(environment.LinuxNodeCgroupManagerCloser); ok {
			_ = closer.Close()
		}
		return nil, err
	}
	executor.enforceWorkspaceDiskLimit = true
	executor.ownsResourceGroups = true
	return executor, nil
}

func NewLinuxNodeNPMPreparationExecutorWithCgroupManager(paths environment.LinuxNodeSandboxPaths, resourceGroups environment.LinuxNodeCgroupManager) (*LinuxNodeNPMPreparationExecutor, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Linux Node/npm preparation is unavailable on %s", runtime.GOOS)
	}
	identity, ok := resourceGroups.(environment.LinuxNodeCgroupManagerIdentity)
	if !ok || identity.RuntimeRootPath() != filepath.Clean(paths.RuntimeRoot) || identity.CgroupRootPath() != filepath.Clean(paths.CgroupRoot) || identity.InstanceIdentity() != paths.InstanceIdentity {
		return nil, fmt.Errorf("Linux Node/npm cgroup lease does not match the configured runtime and cgroup roots: %w", environment.ErrLinuxNodeCgroupUnavailable)
	}
	if err := environment.VerifyLinuxNodeWorkspaceFilesystem(paths); err != nil {
		return nil, fmt.Errorf("Linux Node/npm workspace disk bound is not enforced: %w", err)
	}
	executor, err := newLinuxNodeNPMPreparationExecutor(paths, resourceGroups)
	if err != nil {
		return nil, err
	}
	executor.enforceWorkspaceDiskLimit = true
	return executor, nil
}

func newLinuxNodeNPMPreparationExecutor(paths environment.LinuxNodeSandboxPaths, resourceGroups environment.LinuxNodeCgroupManager) (*LinuxNodeNPMPreparationExecutor, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Linux Node/npm preparation is unavailable on %s", runtime.GOOS)
	}
	if resourceGroups == nil {
		return nil, fmt.Errorf("Linux Node/npm cgroup resource controls are required: %w", environment.ErrLinuxNodeCgroupUnavailable)
	}
	if err := environment.ValidateLinuxNodeSandboxPaths(paths); err != nil {
		return nil, fmt.Errorf("Linux Node/npm preparation configuration is invalid: %w", err)
	}
	if _, err := environment.LinuxNodeToolchainSHA256(paths); err != nil {
		return nil, fmt.Errorf("Linux Node/npm toolchain fingerprint is invalid: %w", err)
	}
	return &LinuxNodeNPMPreparationExecutor{paths: paths, launch: startLinuxNodeProcessWithCgroup, resourceGroups: resourceGroups, jobProcesses: make(map[string]linuxNodeProjectJob)}, nil
}

func (executor *LinuxNodeNPMPreparationExecutor) SupportsProjectEnvironmentProfile(profileID string) bool {
	return executor != nil && profileID == environment.LinuxNodeNPMProfile
}

func (executor *LinuxNodeNPMPreparationExecutor) SupportsProjectJobProfile(profileID string) bool {
	return executor != nil && profileID == environment.LinuxNodeNPMProfile
}

func (executor *LinuxNodeNPMPreparationExecutor) HasPreparedEnvironment(revisionID string) bool {
	if executor == nil {
		return false
	}
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	owner, ok := executor.prepared[revisionID]
	return !executor.closed && ok && !owner.invalidated
}

func (executor *LinuxNodeNPMPreparationExecutor) PrepareProjectEnvironment(ctx context.Context, runID string, loadSnapshot ProjectEnvironmentSnapshotLoader) (EnvironmentPreparationResult, error) {
	if executor == nil || ctx == nil || loadSnapshot == nil {
		return linuxNodePreparationFailure("environment_executor_input_invalid"), errors.New("Linux Node/npm executor received invalid preparation input")
	}
	releaseSlot, finish, accepted, admissionErr := executor.beginPreparation(ctx)
	if !accepted {
		if ctx.Err() != nil {
			return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
		}
		if errors.Is(admissionErr, errLinuxNodePreparationQueueFull) {
			return linuxNodePreparationFailure("environment_executor_busy"), admissionErr
		}
		return linuxNodePreparationFailure("environment_executor_closed"), errors.New("Linux Node/npm executor is closed")
	}
	defer finish()
	transferredSlot := false
	defer func() {
		if !transferredSlot {
			releaseSlot()
		}
	}()
	if err := ctx.Err(); err != nil {
		return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
	}
	if err := executor.verifyWorkspaceDiskLimit(); err != nil {
		return linuxNodePreparationFailure("environment_workspace_disk_bound_unavailable"), err
	}
	input, err := loadSnapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil
		}
		return linuxNodePreparationFailure("environment_snapshot_unavailable"), err
	}
	if input.Revision.ProfileID != environment.LinuxNodeNPMProfile || input.Policy.ProfileID != input.Revision.ProfileID || input.Policy.NetworkPolicy != "deny_all" || input.Plan.InstallPolicy != environment.LinuxNodeNPMInstallPolicy {
		return linuxNodePreparationFailure("environment_executor_profile_mismatch"), errors.New("Linux Node/npm executor received an unsupported profile or policy")
	}
	toolchainDigest, err := environment.LinuxNodeToolchainSHA256(executor.paths)
	if err != nil || toolchainDigest != input.Revision.ToolchainSHA256 {
		return linuxNodePreparationFailure("environment_toolchain_fingerprint_mismatch"), errors.Join(err, core.Integrity)
	}
	identity, workspaceID, err := newLinuxEnvironmentExecutionIDs()
	if err != nil {
		return linuxNodePreparationFailure("environment_execution_identity_unavailable"), err
	}
	materialized, err := environment.MaterializeNodeNPMProjectFiles(executor.paths.WorkspaceRoot, workspaceID, input.Files, input.Plan, input.Policy.RegistryHosts)
	if err != nil {
		return linuxNodePreparationFailure("environment_snapshot_materialization_failed"), err
	}
	cacheWorkspace := filepath.Join(materialized.WorkspaceRoot, ".npm-cache")
	if err = os.Mkdir(cacheWorkspace, 0700); err == nil {
		err = environment.MaterializeLinuxNodeNPMCache(executor.paths.NPMCacheRoot, cacheWorkspace)
	}
	if err != nil {
		cleanupErr := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, workspaceID)
		if cleanupErr != nil {
			owner := linuxNodeWorkspace{companyID: input.Revision.CompanyID, runID: runID, revisionID: input.Revision.RevisionID, identity: identity, workspaceID: workspaceID, workspaceRoot: materialized.WorkspaceRoot, toolchainSHA256: toolchainDigest, plan: input.Plan}
			pendingErr := executor.storePending(owner)
			if pendingErr == nil {
				transferredSlot = true
				executor.monitorPendingWorkspace(owner, releaseSlot)
			}
			return linuxNodePreparationFailure("environment_prepare_cleanup_incomplete"), errors.Join(err, cleanupErr)
		}
		return linuxNodePreparationFailure("environment_offline_cache_materialization_failed"), err
	}
	owner := linuxNodeWorkspace{
		companyID: input.Revision.CompanyID, runID: runID, revisionID: input.Revision.RevisionID,
		identity: identity, workspaceID: workspaceID, workspaceRoot: materialized.WorkspaceRoot,
		toolchainSHA256: toolchainDigest, plan: input.Plan,
	}
	if err = ctx.Err(); err != nil {
		return executor.finishLinuxPreparation(owner, EnvironmentPreparationResult{TerminalState: string(environment.PreparationCancelled), ReasonCode: "environment_preparation_cancelled"}, nil, "", releaseSlot, &transferredSlot)
	}
	paths := executor.paths
	paths.WorkspaceRoot = materialized.WorkspaceRoot
	argv, err := environment.BuildLinuxNodeNPMInstallArgv(paths, input.Plan)
	if err != nil {
		return executor.finishLinuxPreparation(owner, linuxNodePreparationFailure("dependency_install_plan_invalid"), err, "", releaseSlot, &transferredSlot)
	}
	process, pendingGroup, launchErr := executor.launchProjectProcess(identity, "prep-"+runID, input.Revision.CompanyID, owner.workspaceID, argv)
	if launchErr != nil {
		if pendingGroup != nil {
			owner.cgroupGroup = pendingGroup
			result := EnvironmentPreparationResult{TerminalState: string(environment.PreparationOutcomeUnknown), ReasonCode: "dependency_install_cgroup_cleanup_unconfirmed"}
			pendingErr := executor.storePending(owner)
			if pendingErr == nil {
				transferredSlot = true
				executor.monitorPendingWorkspace(owner, releaseSlot)
			}
			return result, errors.Join(launchErr, pendingErr)
		}
		return executor.finishLinuxPreparation(owner, linuxNodePreparationFailure("dependency_install_launch_failed"), launchErr, "", releaseSlot, &transferredSlot)
	}
	if stdin := process.Stdin(); stdin != nil {
		_ = stdin.Close()
	}
	streamLimit := (maxEnvironmentPreparationArtifactBytes - len("stdout:\n") - len("\nstderr:\n")) / 2
	stdout, stderr := capturePreparationStream(process.Stdout(), streamLimit), capturePreparationStream(process.Stderr(), streamLimit)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(input.Policy.TimeoutMS)*time.Millisecond)
	exitCode, waitErr := process.Wait(runCtx)
	wasCancelled := errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded)
	if waitErr != nil && !wasCancelled && errors.Is(waitErr, runner.ErrAppContainerProcessStopUnconfirmed) {
		stdout.abort()
		stderr.abort()
		if pendingErr := executor.storePendingProcess(owner, process); pendingErr != nil {
			cancel()
			return linuxNodePreparationFailure("dependency_install_stop_unconfirmed"), errors.Join(waitErr, pendingErr)
		}
		result := EnvironmentPreparationResult{TerminalState: string(environment.PreparationOutcomeUnknown), ReasonCode: "dependency_install_stop_unconfirmed"}
		cancel()
		transferredSlot = true
		executor.monitorPendingProcess(owner, process, releaseSlot)
		return result, errors.Join(waitErr, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	if errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		stdout.abort()
		stderr.abort()
		if pendingErr := executor.storePendingProcess(owner, process); pendingErr != nil {
			cancel()
			return linuxNodePreparationFailure("dependency_install_cgroup_cleanup_unconfirmed"), errors.Join(waitErr, pendingErr)
		}
		result := EnvironmentPreparationResult{TerminalState: string(environment.PreparationOutcomeUnknown), ReasonCode: "dependency_install_cgroup_cleanup_unconfirmed"}
		cancel()
		transferredSlot = true
		executor.monitorPendingProcess(owner, process, releaseSlot)
		return result, waitErr
	}
	cancel()
	stdoutResult, stderrResult := stdout.wait(), stderr.wait()
	logs, truncated := combinePreparationLogs(stdoutResult, stderrResult)
	result := EnvironmentPreparationResult{Logs: logs, LogsTruncated: truncated}
	if waitErr != nil || stdoutResult.err != nil || stderrResult.err != nil {
		if wasCancelled {
			result.TerminalState, result.ReasonCode = string(environment.PreparationCancelled), "environment_preparation_cancelled"
			return executor.finishLinuxPreparation(owner, result, nil, "", releaseSlot, &transferredSlot)
		}
		result.TerminalState, result.ReasonCode = string(environment.PreparationOutcomeUnknown), "dependency_install_outcome_unknown"
		return executor.finishLinuxPreparation(owner, result, errors.Join(waitErr, stdoutResult.err, stderrResult.err), "", releaseSlot, &transferredSlot)
	}
	if exitCode != 0 {
		result.TerminalState, result.ReasonCode = string(environment.PreparationFailed), "dependency_install_failed"
		result.Evidence = linuxNodePreparationEvidence(input, exitCode)
		return executor.finishLinuxPreparation(owner, result, nil, "", releaseSlot, &transferredSlot)
	}
	result.TerminalState, result.ReasonCode = string(environment.PreparationReady), "dependency_install_ready"
	result.Evidence = linuxNodePreparationEvidence(input, exitCode)
	return executor.finishLinuxPreparation(owner, result, nil, input.Revision.RevisionID, releaseSlot, &transferredSlot)
}

func (executor *LinuxNodeNPMPreparationExecutor) finishLinuxPreparation(owner linuxNodeWorkspace, result EnvironmentPreparationResult, cause error, retainRevisionID string, releaseSlot func(), transferredSlot *bool) (EnvironmentPreparationResult, error) {
	if cause == nil && retainRevisionID != "" && result.TerminalState == string(environment.PreparationReady) {
		if err := executor.storePrepared(retainRevisionID, owner); err == nil {
			return result, nil
		} else {
			cause = err
			result.TerminalState, result.ReasonCode = string(environment.PreparationOutcomeUnknown), "prepared_environment_retention_failed"
		}
	}
	cleanupErr := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, owner.workspaceID)
	if cleanupErr != nil {
		if pendingErr := executor.storePending(owner); pendingErr != nil {
			cause = errors.Join(cause, pendingErr)
		}
		result.TerminalState, result.ReasonCode = string(environment.PreparationOutcomeUnknown), "environment_workspace_cleanup_failed"
		if transferredSlot != nil {
			*transferredSlot = true
			executor.monitorPendingWorkspace(owner, releaseSlot)
		}
		return result, errors.Join(cause, cleanupErr)
	}
	return result, cause
}

func (executor *LinuxNodeNPMPreparationExecutor) storePrepared(revisionID string, owner linuxNodeWorkspace) error {
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.closed || revisionID == "" || owner.workspaceRoot == "" || owner.companyID == "" || owner.runID == "" {
		return errors.New("prepared Linux environment is incomplete or executor is closed")
	}
	if executor.prepared == nil {
		executor.prepared = make(map[string]linuxNodeWorkspace)
	}
	if _, exists := executor.prepared[revisionID]; exists {
		return core.Conflict
	}
	executor.prepared[revisionID] = owner
	return nil
}

func (executor *LinuxNodeNPMPreparationExecutor) storePending(owner linuxNodeWorkspace) error {
	if executor == nil || owner.workspaceRoot == "" || owner.workspaceID == "" {
		return errors.New("pending Linux workspace cleanup is incomplete")
	}
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.pending == nil {
		executor.pending = make(map[string]linuxNodeWorkspace)
	}
	executor.pending[owner.workspaceID] = owner
	return nil
}

func (executor *LinuxNodeNPMPreparationExecutor) storePendingProcess(owner linuxNodeWorkspace, process runner.AppContainerProcess) error {
	if process == nil {
		return errors.New("pending Linux install process is missing")
	}
	if err := executor.storePending(owner); err != nil {
		return err
	}
	executor.preparedMu.Lock()
	executor.jobProcesses["pending-"+owner.workspaceID] = linuxNodeProjectJob{companyID: owner.companyID, revisionID: owner.revisionID, workspaceID: owner.workspaceID, identity: owner.identity, process: process}
	executor.preparedMu.Unlock()
	return nil
}

func (executor *LinuxNodeNPMPreparationExecutor) monitorPendingProcess(owner linuxNodeWorkspace, process runner.AppContainerProcess, releaseSlot func()) {
	go func() {
		resourceClean := false
		for attempt := 0; attempt < 10; attempt++ {
			_, waitErr := process.Wait(context.Background())
			if !errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
				resourceClean = true
				break
			}
			time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
		}
		if !resourceClean {
			return
		}
		if err := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, owner.workspaceID); err != nil {
			executor.monitorPendingWorkspace(owner, releaseSlot)
			return
		}
		executor.preparedMu.Lock()
		delete(executor.pending, owner.workspaceID)
		delete(executor.jobProcesses, "pending-"+owner.workspaceID)
		executor.preparedMu.Unlock()
		releaseSlot()
	}()
}

func (executor *LinuxNodeNPMPreparationExecutor) monitorPendingWorkspace(owner linuxNodeWorkspace, releaseSlot func()) {
	go func() {
		for attempt := 0; attempt < 10; attempt++ {
			if owner.cgroupGroup != nil {
				if err := owner.cgroupGroup.Cleanup(); err != nil {
					time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
					continue
				}
				executor.forgetPendingResourceGroup("prep-"+owner.runID, owner.cgroupGroup)
			}
			if executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, owner.workspaceID) == nil {
				executor.preparedMu.Lock()
				delete(executor.pending, owner.workspaceID)
				delete(executor.jobProcesses, "pending-"+owner.workspaceID)
				executor.preparedMu.Unlock()
				releaseSlot()
				return
			}
			time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
		}
	}()
}

func (executor *LinuxNodeNPMPreparationExecutor) removeWorkspaceAt(root, workspaceID string) error {
	if err := executor.verifyWorkspaceDiskLimit(); err != nil {
		return err
	}
	if executor.removeWorkspace != nil {
		return executor.removeWorkspace(root, workspaceID)
	}
	return removeLinuxNodeWorkspace(root, workspaceID)
}

func (executor *LinuxNodeNPMPreparationExecutor) launchProjectProcess(identity, cleanupKey, companyID, workspaceID string, argv []string) (runner.AppContainerProcess, environment.LinuxNodeResourceCgroup, error) {
	if err := executor.verifyWorkspaceDiskLimit(); err != nil {
		return nil, nil, err
	}
	if executor.resourceGroups == nil {
		process, err := executor.launch(identity, argv, nil)
		return process, nil, err
	}
	group, err := executor.resourceGroups.Create(identity)
	if err != nil {
		return nil, nil, err
	}
	process, err := executor.launch(identity, argv, group)
	if err == nil && process == nil {
		err = errors.New("Linux Node/npm launch returned no process handle")
	}
	if err == nil {
		return process, nil, nil
	}
	cleanupErr := group.Cleanup()
	if cleanupErr == nil {
		return nil, nil, err
	}
	executor.retainPendingResourceGroup(cleanupKey, companyID, workspaceID, group)
	return nil, group, errors.Join(err, environment.ErrLinuxNodeCgroupCleanupUnconfirmed, cleanupErr)
}

func (executor *LinuxNodeNPMPreparationExecutor) verifyWorkspaceDiskLimit() error {
	if executor == nil || !executor.enforceWorkspaceDiskLimit {
		return nil
	}
	return environment.VerifyLinuxNodeWorkspaceFilesystem(executor.paths)
}

func (executor *LinuxNodeNPMPreparationExecutor) retainPendingResourceGroup(key, companyID, workspaceID string, group environment.LinuxNodeResourceCgroup) {
	if executor == nil || key == "" || companyID == "" || workspaceID == "" || group == nil {
		return
	}
	executor.preparedMu.Lock()
	if executor.pendingResourceGroups == nil {
		executor.pendingResourceGroups = make(map[string]linuxNodePendingCgroup)
	}
	executor.pendingResourceGroups[key] = linuxNodePendingCgroup{companyID: companyID, workspaceID: workspaceID, group: group}
	executor.preparedMu.Unlock()
}

func (executor *LinuxNodeNPMPreparationExecutor) forgetPendingResourceGroup(key string, group environment.LinuxNodeResourceCgroup) {
	executor.preparedMu.Lock()
	if current, ok := executor.pendingResourceGroups[key]; ok && current.group == group {
		delete(executor.pendingResourceGroups, key)
	}
	executor.preparedMu.Unlock()
}

func (executor *LinuxNodeNPMPreparationExecutor) LaunchProjectJob(ctx context.Context, request ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	if executor == nil || ctx == nil || request.ProfileID != environment.LinuxNodeNPMProfile || !core.ValidID(request.CompanyID) || !core.ValidID(request.JobID) || !core.ValidID(request.EnvironmentRevisionID) {
		return nil, core.Malformed
	}
	if request.ServiceID != "" || request.ServiceProbe != nil {
		return nil, core.Denied
	}
	executor.preparedMu.Lock()
	if executor.closed {
		executor.preparedMu.Unlock()
		return nil, errors.New("Linux Node/npm executor is closed")
	}
	owner, exists := executor.prepared[request.EnvironmentRevisionID]
	if !exists || owner.invalidated || owner.companyID != request.CompanyID {
		executor.preparedMu.Unlock()
		return nil, core.Denied
	}
	for _, job := range executor.jobProcesses {
		if job.workspaceID == owner.workspaceID {
			executor.preparedMu.Unlock()
			return nil, core.Conflict
		}
	}
	for _, pending := range executor.pendingResourceGroups {
		if pending.workspaceID == owner.workspaceID {
			executor.preparedMu.Unlock()
			return nil, core.Conflict
		}
	}
	for workspaceID, jobID := range executor.launchReservations {
		if workspaceID == owner.workspaceID || jobID == request.JobID {
			executor.preparedMu.Unlock()
			return nil, core.Conflict
		}
	}
	if _, exists := executor.jobProcesses[request.JobID]; exists {
		executor.preparedMu.Unlock()
		return nil, core.Conflict
	}
	if _, exists := executor.pendingResourceGroups[request.JobID]; exists {
		executor.preparedMu.Unlock()
		return nil, core.Conflict
	}
	if executor.launchReservations == nil {
		executor.launchReservations = make(map[string]string)
	}
	executor.launchReservations[owner.workspaceID] = request.JobID
	executor.preparations.Add(1)
	executor.preparedMu.Unlock()
	defer func() {
		executor.preparedMu.Lock()
		if executor.launchReservations[owner.workspaceID] == request.JobID {
			delete(executor.launchReservations, owner.workspaceID)
		}
		executor.preparedMu.Unlock()
		executor.preparations.Done()
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest, err := environment.LinuxNodeToolchainSHA256(executor.paths)
	if err != nil || digest != owner.toolchainSHA256 {
		return nil, errors.Join(core.Integrity, err)
	}
	paths := executor.paths
	paths.WorkspaceRoot = owner.workspaceRoot
	argv, err := environment.BuildLinuxNodeProjectScriptArgv(paths, owner.plan, request.ScriptPath, request.Args)
	if err != nil {
		return nil, err
	}
	identity := owner.workspaceID + "-" + request.JobID
	process, pendingGroup, err := executor.launchProjectProcess(identity, request.JobID, request.CompanyID, owner.workspaceID, argv)
	if err != nil {
		if pendingGroup != nil {
			return nil, errors.Join(err, environment.ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		return nil, err
	}
	executor.preparedMu.Lock()
	if executor.jobProcesses == nil {
		executor.jobProcesses = make(map[string]linuxNodeProjectJob)
	}
	if _, exists := executor.jobProcesses[request.JobID]; exists || executor.closed || ctx.Err() != nil {
		executor.preparedMu.Unlock()
		stopProof, stopErr := process.Stop()
		if stopErr == nil && stopProof.For(identity) && stopProof.PID() == process.PID() {
			_, waitErr := process.Wait(context.Background())
			if waitErr != nil {
				if retainErr := executor.retainUnconfirmedProjectJob(owner, request.JobID, identity, process); retainErr != nil {
					return process, errors.Join(ctx.Err(), core.Conflict, waitErr, retainErr)
				}
				return process, errors.Join(ctx.Err(), core.Conflict, waitErr)
			}
			return nil, errors.Join(ctx.Err(), core.Conflict)
		}
		if retainErr := executor.retainUnconfirmedProjectJob(owner, request.JobID, identity, process); retainErr != nil {
			return process, errors.Join(ctx.Err(), runner.ErrAppContainerProcessStopUnconfirmed, retainErr)
		}
		return process, errors.Join(ctx.Err(), runner.ErrAppContainerProcessStopUnconfirmed)
	}
	executor.jobProcesses[request.JobID] = linuxNodeProjectJob{companyID: request.CompanyID, revisionID: request.EnvironmentRevisionID, workspaceID: owner.workspaceID, identity: identity, process: process}
	executor.preparedMu.Unlock()
	return process, nil
}

func (executor *LinuxNodeNPMPreparationExecutor) retainUnconfirmedProjectJob(owner linuxNodeWorkspace, jobID, identity string, process runner.AppContainerProcess) error {
	if executor == nil || !core.ValidID(jobID) || !core.ValidID(owner.workspaceID) || process == nil {
		return core.Malformed
	}
	owner.invalidated = true
	executor.preparedMu.Lock()
	defer executor.preparedMu.Unlock()
	if executor.prepared == nil {
		executor.prepared = make(map[string]linuxNodeWorkspace)
	}
	if executor.pending == nil {
		executor.pending = make(map[string]linuxNodeWorkspace)
	}
	if executor.jobProcesses == nil {
		executor.jobProcesses = make(map[string]linuxNodeProjectJob)
	}
	executor.prepared[owner.revisionID] = owner
	executor.pending[owner.workspaceID] = owner
	executor.jobProcesses[jobID] = linuxNodeProjectJob{
		companyID: owner.companyID, revisionID: owner.revisionID, workspaceID: owner.workspaceID,
		identity: identity, process: process,
	}
	return nil
}

func (executor *LinuxNodeNPMPreparationExecutor) StopProjectJob(_ context.Context, companyID, jobID string) error {
	if executor == nil {
		return errors.New("Linux Node/npm executor is unavailable")
	}
	executor.preparedMu.Lock()
	job, exists := executor.jobProcesses[jobID]
	pending, pendingExists := executor.pendingResourceGroups[jobID]
	executor.preparedMu.Unlock()
	if !exists && pendingExists && pending.companyID == companyID {
		if err := pending.group.Cleanup(); err != nil {
			return errors.Join(environment.ErrLinuxNodeCgroupCleanupUnconfirmed, err)
		}
		executor.forgetPendingResourceGroup(jobID, pending.group)
		return nil
	}
	if !exists || job.companyID != companyID || job.process == nil {
		return core.OutOfScope
	}
	proof, err := job.process.Stop()
	if err != nil || !proof.For(job.identity) || proof.PID() != job.process.PID() {
		return errors.Join(err, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	return nil
}

func (executor *LinuxNodeNPMPreparationExecutor) ReconcilePendingProjectJob(companyID, jobID string) error {
	if executor == nil || !core.ValidID(companyID) || !core.ValidID(jobID) {
		return core.Malformed
	}
	executor.preparedMu.Lock()
	pending, pendingExists := executor.pendingResourceGroups[jobID]
	if pendingExists && pending.companyID == companyID {
		executor.preparedMu.Unlock()
		if err := pending.group.Cleanup(); err != nil {
			return errors.Join(environment.ErrLinuxNodeCgroupCleanupUnconfirmed, err)
		}
		executor.forgetPendingResourceGroup(jobID, pending.group)
		return nil
	}
	if company, confirmed := executor.confirmedJobCgroupCleanup[jobID]; confirmed && company == companyID {
		delete(executor.confirmedJobCgroupCleanup, jobID)
		executor.preparedMu.Unlock()
		return nil
	}
	executor.preparedMu.Unlock()
	return core.OutOfScope
}

func (executor *LinuxNodeNPMPreparationExecutor) ForgetProjectJob(jobID string) {
	if executor == nil {
		return
	}
	executor.preparedMu.Lock()
	job, exists := executor.jobProcesses[jobID]
	if exists {
		if pending, ok := job.process.(interface{ CgroupCleanupPending() bool }); ok && pending.CgroupCleanupPending() {
			if companyID := executor.unconfirmedJobCgroupCleanup[jobID]; companyID == "" {
				if executor.unconfirmedJobCgroupCleanup == nil {
					executor.unconfirmedJobCgroupCleanup = make(map[string]string)
				}
				executor.unconfirmedJobCgroupCleanup[jobID] = job.companyID
			}
			executor.preparedMu.Unlock()
			go executor.retryProjectJobCgroupCleanup(jobID, job)
			return
		}
	}
	if companyID := executor.unconfirmedJobCgroupCleanup[jobID]; companyID != "" {
		if executor.confirmedJobCgroupCleanup == nil {
			executor.confirmedJobCgroupCleanup = make(map[string]string)
		}
		executor.confirmedJobCgroupCleanup[jobID] = companyID
		delete(executor.unconfirmedJobCgroupCleanup, jobID)
	}
	delete(executor.jobProcesses, jobID)
	executor.preparedMu.Unlock()
}

func (executor *LinuxNodeNPMPreparationExecutor) MarkProjectJobCgroupCleanupUnconfirmed(companyID, jobID string) {
	if executor == nil || !core.ValidID(companyID) || !core.ValidID(jobID) {
		return
	}
	executor.preparedMu.Lock()
	if executor.unconfirmedJobCgroupCleanup == nil {
		executor.unconfirmedJobCgroupCleanup = make(map[string]string)
	}
	executor.unconfirmedJobCgroupCleanup[jobID] = companyID
	executor.preparedMu.Unlock()
}

func (executor *LinuxNodeNPMPreparationExecutor) retryProjectJobCgroupCleanup(jobID string, job linuxNodeProjectJob) {
	for attempt := 0; attempt < 10; attempt++ {
		_, _ = job.process.Wait(context.Background())
		if pending, ok := job.process.(interface{ CgroupCleanupPending() bool }); !ok || !pending.CgroupCleanupPending() {
			executor.preparedMu.Lock()
			if current, exists := executor.jobProcesses[jobID]; exists && current.process == job.process {
				delete(executor.jobProcesses, jobID)
			}
			if companyID := executor.unconfirmedJobCgroupCleanup[jobID]; companyID != "" {
				if executor.confirmedJobCgroupCleanup == nil {
					executor.confirmedJobCgroupCleanup = make(map[string]string)
				}
				executor.confirmedJobCgroupCleanup[jobID] = companyID
				delete(executor.unconfirmedJobCgroupCleanup, jobID)
			}
			executor.preparedMu.Unlock()
			return
		}
		time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
	}
}

func (executor *LinuxNodeNPMPreparationExecutor) PreparedRunsForShutdown() []preparedEnvironmentRunRef {
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

func (executor *LinuxNodeNPMPreparationExecutor) beginPreparation(ctx context.Context) (func(), func(), bool, error) {
	executor.preparedMu.Lock()
	if executor.closed || ctx.Err() != nil {
		executor.preparedMu.Unlock()
		return nil, nil, false, nil
	}
	if executor.slots == nil {
		executor.slots = make(chan struct{}, maxConcurrentLinuxNodePreparations)
		executor.queue = make(chan struct{}, maxQueuedLinuxNodePreparations)
		executor.closing = make(chan struct{})
	}
	slots, queue, closing := executor.slots, executor.queue, executor.closing
	select {
	case queue <- struct{}{}:
	default:
		executor.preparedMu.Unlock()
		return nil, nil, false, errLinuxNodePreparationQueueFull
	}
	executor.preparations.Add(1)
	executor.preparedMu.Unlock()
	dequeue := func() { <-queue }
	select {
	case slots <- struct{}{}:
		dequeue()
		var slotOnce sync.Once
		release := func() { slotOnce.Do(func() { <-slots }) }
		executor.preparedMu.Lock()
		closed := executor.closed
		executor.preparedMu.Unlock()
		if closed || ctx.Err() != nil {
			release()
			executor.preparations.Done()
			return nil, nil, false, nil
		}
		var finishOnce sync.Once
		finish := func() { finishOnce.Do(executor.preparations.Done) }
		return release, finish, true, nil
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

func (executor *LinuxNodeNPMPreparationExecutor) markClosedLocked() {
	if executor.closed {
		return
	}
	executor.closed = true
	if executor.closing != nil {
		close(executor.closing)
	}
}

func (executor *LinuxNodeNPMPreparationExecutor) Close() error {
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
	unsafeWorkspaces := make(map[string]struct{})
	uncertainJobs := make(map[string]linuxNodeProjectJob)
	for jobID, job := range executor.jobProcesses {
		proof, stopErr := job.process.Stop()
		if stopErr != nil || !proof.For(job.identity) || proof.PID() != job.process.PID() {
			closeErrors = append(closeErrors, errors.Join(stopErr, runner.ErrAppContainerProcessStopUnconfirmed))
			unsafeWorkspaces[job.workspaceID] = struct{}{}
			uncertainJobs[jobID] = job
			continue
		}
		_, waitErr := job.process.Wait(context.Background())
		if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
			closeErrors = append(closeErrors, waitErr)
		}
		if errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
			unsafeWorkspaces[job.workspaceID] = struct{}{}
			uncertainJobs[jobID] = job
			continue
		}
		delete(executor.jobProcesses, jobID)
	}
	for key, pending := range executor.pendingResourceGroups {
		if err := pending.group.Cleanup(); err != nil {
			closeErrors = append(closeErrors, errors.Join(environment.ErrLinuxNodeCgroupCleanupUnconfirmed, err))
			if pending.workspaceID != "" {
				unsafeWorkspaces[pending.workspaceID] = struct{}{}
			}
			continue
		}
		delete(executor.pendingResourceGroups, key)
	}
	for revisionID, owner := range executor.prepared {
		if _, unsafe := unsafeWorkspaces[owner.workspaceID]; unsafe {
			continue
		}
		if err := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, owner.workspaceID); err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		delete(executor.prepared, revisionID)
	}
	for workspaceID, owner := range executor.pending {
		if _, unsafe := unsafeWorkspaces[workspaceID]; unsafe {
			continue
		}
		if err := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, workspaceID); err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		delete(executor.pending, workspaceID)
		_ = owner
	}
	for jobID, job := range uncertainJobs {
		go executor.cleanupWorkspaceAfterProcessExit(jobID, job)
	}
	if executor.ownsResourceGroups {
		if closer, ok := executor.resourceGroups.(environment.LinuxNodeCgroupManagerCloser); ok {
			if err := closer.Close(); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
	}
	return errors.Join(closeErrors...)
}

func (executor *LinuxNodeNPMPreparationExecutor) cleanupWorkspaceAfterProcessExit(jobID string, job linuxNodeProjectJob) {
	resourceClean := false
	for attempt := 0; attempt < 10; attempt++ {
		_, waitErr := job.process.Wait(context.Background())
		if !errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
			resourceClean = true
			break
		}
		time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
	}
	if !resourceClean {
		return
	}
	if err := executor.removeWorkspaceAt(executor.paths.WorkspaceRoot, job.workspaceID); err != nil {
		return
	}
	executor.preparedMu.Lock()
	delete(executor.jobProcesses, jobID)
	for revisionID, owner := range executor.prepared {
		if owner.workspaceID == job.workspaceID {
			delete(executor.prepared, revisionID)
		}
	}
	delete(executor.pending, job.workspaceID)
	executor.preparedMu.Unlock()
}

func removeLinuxNodeWorkspace(root, workspaceID string) error {
	if !core.ValidID(workspaceID) || !filepath.IsAbs(root) || !linuxNodePathRoot(root) {
		return errors.New("Linux workspace cleanup path is invalid")
	}
	target := filepath.Join(filepath.Clean(root), workspaceID)
	relative, err := filepath.Rel(filepath.Clean(root), target)
	if err != nil || relative != workspaceID {
		return errors.New("Linux workspace escaped its managed root")
	}
	if err = os.RemoveAll(target); err != nil {
		return err
	}
	if _, err = os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, errors.New("Linux workspace removal could not be confirmed"))
	}
	return nil
}

func linuxNodePathRoot(root string) bool {
	if !filepath.IsAbs(root) {
		return false
	}
	for current := filepath.Clean(root); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
	}
}

func linuxNodePreparationFailure(reason string) EnvironmentPreparationResult {
	return EnvironmentPreparationResult{TerminalState: string(environment.PreparationFailed), ReasonCode: reason}
}

func linuxNodePreparationEvidence(input kernel.ProjectEnvironmentExecutionSnapshot, exitCode int) []byte {
	evidence := struct {
		SchemaVersion         string `json:"schemaVersion"`
		EnvironmentRevisionID string `json:"environmentRevisionId"`
		SourceRevisionSHA256  string `json:"sourceRevisionSha256"`
		PackageJSONSHA256     string `json:"packageJsonSha256"`
		LockfileSHA256        string `json:"lockfileSha256"`
		PolicySHA256          string `json:"policySha256"`
		ToolchainSHA256       string `json:"toolchainSha256"`
		IsolationProfile      string `json:"isolationProfile"`
		ExitCode              int    `json:"exitCode"`
	}{"linux-node-npm-preparation@1", input.Revision.RevisionID, input.Revision.SourceRevisionSHA256, input.Revision.PackageJSONSHA256, input.Revision.LockfileSHA256, input.Revision.PolicySHA256, input.Revision.ToolchainSHA256, environment.LinuxNodeIsolationProfile, exitCode}
	content, _ := json.Marshal(evidence)
	return content
}

func newLinuxEnvironmentExecutionIDs() (string, string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", "", err
	}
	identifier := hex.EncodeToString(random[:])
	return "polis-linux-env-" + identifier, "env-linux-" + identifier, nil
}
