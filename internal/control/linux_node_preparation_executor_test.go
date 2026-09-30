// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestLinuxNodeExecutorRequiresDelegatedCgroupV2Root(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux cgroup validation requires Linux")
	}
	paths := linuxNodeExecutorFixture(t)
	if _, err := NewLinuxNodeNPMPreparationExecutor(paths); err == nil || !errors.Is(err, environment.ErrLinuxNodeCgroupUnavailable) {
		t.Fatalf("ordinary fixture directory was accepted as a cgroup root: %v", err)
	}
}

func TestForgetProjectJobKeepsCleanupConfirmationForReconciliation(t *testing.T) {
	group := &linuxNodeTestCgroup{failUntil: 1}
	process := &linuxNodeCgroupTestProcess{AppContainerProcess: &linuxNodeTestProcess{}, cgroup: group}
	executor := &LinuxNodeNPMPreparationExecutor{
		jobProcesses: map[string]linuxNodeProjectJob{
			"job-1": {companyID: "company-1", revisionID: "revision-1", workspaceID: "workspace-1", identity: "job-1", process: process},
		},
	}
	if _, err := process.Wait(context.Background()); !errors.Is(err, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		t.Fatalf("initial cleanup error=%v, want unconfirmed", err)
	}
	executor.MarkProjectJobCgroupCleanupUnconfirmed("company-1", "job-1")
	executor.ForgetProjectJob("job-1")
	deadline := time.After(2 * time.Second)
	for {
		err := executor.ReconcilePendingProjectJob("company-1", "job-1")
		if err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("cleanup confirmation was not retained for reconciliation: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
	if group.cleanupAttempts != 2 {
		t.Fatalf("cgroup cleanup attempts=%d, want 2", group.cleanupAttempts)
	}
}

func TestLinuxNodeFailedLaunchRetainsCgroupForExplicitCleanupRetry(t *testing.T) {
	group := &linuxNodeTestCgroup{failUntil: 2}
	executor := &LinuxNodeNPMPreparationExecutor{
		resourceGroups: linuxNodeTestCgroupManager{group: group},
		launch: func(string, []string, environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error) {
			return nil, errors.New("fixture process creation failed")
		},
	}
	process, pendingGroup, err := executor.launchProjectProcess("linux-node-job-1", "job-1", "company-1", "workspace-1", []string{"bwrap"})
	if process != nil || pendingGroup != group || !errors.Is(err, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		t.Fatalf("failed launch=(%v,%v,%v), want no process and retained cleanup group", process, pendingGroup, err)
	}
	if stopErr := executor.StopProjectJob(context.Background(), "company-1", "job-1"); !errors.Is(stopErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		t.Fatalf("first cleanup retry error=%v, want still unconfirmed", stopErr)
	}
	if stopErr := executor.StopProjectJob(context.Background(), "company-1", "job-1"); stopErr != nil {
		t.Fatalf("second cleanup retry error=%v", stopErr)
	}
	executor.preparedMu.Lock()
	_, retained := executor.pendingResourceGroups["job-1"]
	executor.preparedMu.Unlock()
	if retained || group.cleanupAttempts != 3 {
		t.Fatalf("pending group retained=%t cleanupAttempts=%d", retained, group.cleanupAttempts)
	}
}

func TestLinuxNodeJobRefusesWorkspaceWithUnconfirmedPendingCgroup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux executor lifecycle fixtures require Linux")
	}
	paths := linuxNodeExecutorFixture(t)
	toolchainDigest, err := environment.LinuxNodeToolchainSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := "linux-workspace"
	workspace := filepath.Join(paths.WorkspaceRoot, workspaceID)
	if err = os.MkdirAll(filepath.Join(workspace, "Repo", "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(workspace, ".npm-cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(workspace, "Repo", "scripts", "build.mjs"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := environment.NodeNPMProjectPlan{ProfileID: environment.LinuxNodeNPMProfile, SourceKind: environment.NodeSnapshotFilesSource, ProjectRoot: "Repo", InstallPolicy: environment.LinuxNodeNPMInstallPolicy}
	pendingGroup := &linuxNodeTestCgroup{failUntil: 2}
	launchCount := 0
	executor := &LinuxNodeNPMPreparationExecutor{
		paths:          paths,
		resourceGroups: linuxNodeTestCgroupManager{},
		prepared: map[string]linuxNodeWorkspace{
			"revision-1": {companyID: "company-1", revisionID: "revision-1", workspaceID: workspaceID, workspaceRoot: workspace, toolchainSHA256: toolchainDigest, plan: plan},
		},
		pendingResourceGroups: map[string]linuxNodePendingCgroup{
			"old-job": {companyID: "company-1", workspaceID: workspaceID, group: pendingGroup},
		},
		launch: func(_ string, _ []string, cgroup environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error) {
			launchCount++
			return &linuxNodeCgroupTestProcess{AppContainerProcess: &linuxNodeTestProcess{}, cgroup: cgroup}, nil
		},
	}
	_, err = executor.LaunchProjectJob(context.Background(), ProjectJobExecutionRequest{
		CompanyID: "company-1", JobID: "new-job", ProfileID: environment.LinuxNodeNPMProfile,
		EnvironmentRevisionID: "revision-1", ScriptPath: "scripts/build.mjs",
	})
	if !errors.Is(err, core.Conflict) || launchCount != 0 {
		t.Fatalf("job launch error=%v launchCount=%d, want workspace conflict before process launch", err, launchCount)
	}
}

func TestLinuxNodeJobAtomicallyReservesWorkspaceDuringLaunch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux executor lifecycle fixtures require Linux")
	}
	paths := linuxNodeExecutorFixture(t)
	toolchainDigest, err := environment.LinuxNodeToolchainSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := "linux-workspace"
	workspace := filepath.Join(paths.WorkspaceRoot, workspaceID)
	if err = os.MkdirAll(filepath.Join(workspace, "Repo", "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(workspace, ".npm-cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(workspace, "Repo", "scripts", "build.mjs"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := environment.NodeNPMProjectPlan{ProfileID: environment.LinuxNodeNPMProfile, SourceKind: environment.NodeSnapshotFilesSource, ProjectRoot: "Repo", InstallPolicy: environment.LinuxNodeNPMInstallPolicy}
	launchEntered := make(chan string, 2)
	releaseFirst := make(chan struct{})
	var launchMu sync.Mutex
	launchCalls := 0
	executor := &LinuxNodeNPMPreparationExecutor{
		paths: paths,
		prepared: map[string]linuxNodeWorkspace{
			"revision-1": {companyID: "company-1", revisionID: "revision-1", workspaceID: workspaceID, workspaceRoot: workspace, toolchainSHA256: toolchainDigest, plan: plan},
		},
		launch: func(identity string, _ []string, _ environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error) {
			launchMu.Lock()
			launchCalls++
			callNumber := launchCalls
			launchMu.Unlock()
			launchEntered <- identity
			if callNumber == 1 {
				<-releaseFirst
			}
			return &linuxNodeTestProcess{}, nil
		},
	}

	type launchResult struct {
		process runner.AppContainerProcess
		err     error
	}
	firstDone := make(chan launchResult, 1)
	go func() {
		process, launchErr := executor.LaunchProjectJob(context.Background(), ProjectJobExecutionRequest{
			CompanyID: "company-1", JobID: "first-job", ProfileID: environment.LinuxNodeNPMProfile,
			EnvironmentRevisionID: "revision-1", ScriptPath: "scripts/build.mjs",
		})
		firstDone <- launchResult{process: process, err: launchErr}
	}()
	select {
	case <-launchEntered:
	case <-time.After(2 * time.Second):
		close(releaseFirst)
		t.Fatal("first JobRun did not reach its process launcher")
	}
	secondDone := make(chan launchResult, 1)
	go func() {
		process, launchErr := executor.LaunchProjectJob(context.Background(), ProjectJobExecutionRequest{
			CompanyID: "company-1", JobID: "second-job", ProfileID: environment.LinuxNodeNPMProfile,
			EnvironmentRevisionID: "revision-1", ScriptPath: "scripts/build.mjs",
		})
		secondDone <- launchResult{process: process, err: launchErr}
	}()
	select {
	case result := <-secondDone:
		if result.process != nil || !errors.Is(result.err, core.Conflict) {
			close(releaseFirst)
			<-firstDone
			t.Fatalf("second launch=(%v,%v), want workspace conflict without process", result.process, result.err)
		}
	case <-time.After(2 * time.Second):
		close(releaseFirst)
		<-firstDone
		t.Fatal("second JobRun did not resolve while the first launch was blocked")
	}
	close(releaseFirst)
	first := <-firstDone
	if first.process == nil || first.err != nil {
		t.Fatalf("first launch=(%v,%v), want process without error", first.process, first.err)
	}
	launchMu.Lock()
	defer launchMu.Unlock()
	if launchCalls != 1 {
		t.Fatalf("process launch calls=%d, want exactly one", launchCalls)
	}
}

func TestLinuxNodePreparationRequiresDedicatedWorkspaceFilesystemBeforeLoadingSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux filesystem qualification fixtures require Linux")
	}
	paths := linuxNodeExecutorFixture(t)
	executor, err := newLinuxNodeNPMPreparationExecutor(paths, linuxNodeTestCgroupManager{})
	if err != nil {
		t.Fatal(err)
	}
	executor.enforceWorkspaceDiskLimit = true
	loaderCalled := false
	result, err := executor.PrepareProjectEnvironment(context.Background(), "disk-bound-run", func(context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error) {
		loaderCalled = true
		return kernel.ProjectEnvironmentExecutionSnapshot{}, nil
	})
	if !errors.Is(err, environment.ErrLinuxNodeWorkspaceDiskLimitUnavailable) || loaderCalled || result.ReasonCode != "environment_workspace_disk_bound_unavailable" {
		t.Fatalf("workspace disk bound result=(%+v,%v) loaderCalled=%t", result, err, loaderCalled)
	}
}

func TestLinuxNodePreparationExecutorUsesOfflineCacheAndRetainsVerifiedWorkspace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process fixtures require Linux")
	}
	paths := linuxNodeExecutorFixture(t)
	toolchainDigest, err := environment.LinuxNodeToolchainSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	files := []environment.ProjectSourceFile{
		{RelativePath: "Repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "Repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
		{RelativePath: "Repo/build.mjs", MediaType: "text/javascript", Content: []byte("console.log('fixture')")},
	}
	plan, err := environment.InspectLinuxNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	policy, _, policyDigest, err := environment.BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 30_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := newLinuxNodeNPMPreparationExecutor(paths, linuxNodeTestCgroupManager{})
	if err != nil {
		t.Fatal(err)
	}
	var gotArgv []string
	launchCount := 0
	executor.launch = func(_ string, argv []string, cgroup environment.LinuxNodeResourceCgroup) (runner.AppContainerProcess, error) {
		if cgroup == nil || cgroup.FileDescriptor() != 42 {
			t.Fatalf("Linux launch omitted its bounded cgroup: %v", cgroup)
		}
		launchCount++
		gotArgv = append([]string(nil), argv...)
		return &linuxNodeCgroupTestProcess{AppContainerProcess: &linuxNodeTestProcess{stdout: []byte("npm offline success\n")}, cgroup: cgroup}, nil
	}
	input := kernel.ProjectEnvironmentExecutionSnapshot{
		Revision: kernel.ProjectEnvironmentRevision{
			CompanyID: "linux-executor-company", RevisionID: "linux-executor-revision", ProfileID: environment.LinuxNodeNPMProfile,
			ToolchainSHA256: toolchainDigest, SourceRevisionSHA256: repeatControlDigest('a'),
			PackageJSONSHA256: plan.PackageJSONSHA256, LockfileSHA256: plan.LockfileSHA256, PolicySHA256: policyDigest,
		},
		Policy: policy, Plan: plan, Files: files,
	}
	result, err := executor.PrepareProjectEnvironment(context.Background(), "linux-executor-run", func(context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error) {
		return input, nil
	})
	if err != nil || result.TerminalState != string(environment.PreparationReady) || len(result.Evidence) == 0 {
		t.Fatalf("Linux preparation=%+v error=%v", result, err)
	}
	if !containsLinuxExecutorArg(gotArgv, "--unshare-all") || !containsLinuxExecutorArg(gotArgv, "--offline") || containsLinuxExecutorArg(gotArgv, "--share-net") {
		t.Fatalf("Linux preparation argv did not stay offline: %v", gotArgv)
	}
	if !executor.HasPreparedEnvironment(input.Revision.RevisionID) {
		t.Fatal("verified Linux workspace was not retained for project jobs")
	}
	executor.preparedMu.Lock()
	owner := executor.prepared[input.Revision.RevisionID]
	executor.preparedMu.Unlock()
	copiedCache, err := os.ReadFile(filepath.Join(owner.workspaceRoot, ".npm-cache", "_cacache", "index-v5"))
	if err != nil || string(copiedCache) != "offline fixture cache" {
		t.Fatalf("workspace npm cache=%q error=%v", copiedCache, err)
	}
	if err = os.WriteFile(filepath.Join(paths.NPMCacheRoot, "_cacache", "index-v5"), []byte("changed offline fixture cache"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = executor.LaunchProjectJob(context.Background(), ProjectJobExecutionRequest{
		CompanyID: input.Revision.CompanyID, JobID: "linux-node-job", ProfileID: environment.LinuxNodeNPMProfile,
		EnvironmentRevisionID: input.Revision.RevisionID, ScriptPath: "build.mjs",
	}); err == nil || launchCount != 1 {
		t.Fatalf("Linux JobRun launched after offline cache drift: launches=%d err=%v", launchCount, err)
	}
	if err = os.WriteFile(filepath.Join(paths.NPMCacheRoot, "_cacache", "index-v5"), []byte("offline fixture cache"), 0600); err != nil {
		t.Fatal(err)
	}
	jobProcess, err := executor.LaunchProjectJob(context.Background(), ProjectJobExecutionRequest{
		CompanyID: input.Revision.CompanyID, JobID: "linux-node-job", ProfileID: environment.LinuxNodeNPMProfile,
		EnvironmentRevisionID: input.Revision.RevisionID, ScriptPath: "build.mjs", Args: []string{"--check"},
	})
	if err != nil || jobProcess == nil || launchCount != 2 || !containsLinuxExecutorArg(gotArgv, "/workspace/Repo/build.mjs") || !containsLinuxExecutorArg(gotArgv, "--check") {
		t.Fatalf("Linux batch JobRun argv=%v launches=%d error=%v", gotArgv, launchCount, err)
	}
	if _, err = jobProcess.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	executor.ForgetProjectJob("linux-node-job")
	if err = executor.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(owner.workspaceRoot); !os.IsNotExist(err) {
		t.Fatalf("executor shutdown kept temporary workspace: %v", err)
	}
}

func TestLinuxNodeProcessReportsExitAndConfirmsCancellation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process fixtures require Linux")
	}
	process, err := startLinuxNodeProcess("linux-node-exit-test", []string{"/bin/sh", "-c", "exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	_ = process.Stdin().Close()
	exitCode, waitErr := process.Wait(context.Background())
	if waitErr != nil || exitCode != 7 {
		t.Fatalf("Linux process exit=(%d,%v), want (7,nil)", exitCode, waitErr)
	}

	process, err = startLinuxNodeProcess("linux-node-cancel-test", []string{"/bin/sleep", "30"})
	if err != nil {
		t.Fatal(err)
	}
	_ = process.Stdin().Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, waitErr = process.Wait(ctx); !errors.Is(waitErr, context.DeadlineExceeded) {
		t.Fatalf("cancelled Linux process wait error=%v", waitErr)
	}
	proof, stopErr := process.Stop()
	if stopErr != nil || !proof.For("linux-node-cancel-test") || proof.PID() != process.PID() {
		t.Fatalf("Linux process stop proof pid=%d expected=%d error=%v", proof.PID(), process.PID(), stopErr)
	}
}

func TestLinuxNodeExecutorKeepsWorkspaceUntilUnconfirmedProcessExits(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process fixtures require Linux")
	}
	workspaceRoot := t.TempDir()
	workspaceID := "linux-workspace"
	workspace := filepath.Join(workspaceRoot, workspaceID)
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	process := &linuxNodeUnconfirmedProcess{done: make(chan struct{})}
	executor := &LinuxNodeNPMPreparationExecutor{
		paths:        environment.LinuxNodeSandboxPaths{RuntimeRoot: t.TempDir(), WorkspaceRoot: workspaceRoot},
		prepared:     map[string]linuxNodeWorkspace{"revision-1": {companyID: "company-1", revisionID: "revision-1", workspaceID: workspaceID, workspaceRoot: workspace}},
		jobProcesses: map[string]linuxNodeProjectJob{"job-1": {companyID: "company-1", revisionID: "revision-1", workspaceID: workspaceID, identity: "linux-workspace-job-1", process: process}},
	}
	if err := executor.Close(); !errors.Is(err, runner.ErrAppContainerProcessStopUnconfirmed) {
		t.Fatalf("executor close error=%v, expected an unconfirmed process stop", err)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("executor removed workspace while process outcome was unresolved: %v", err)
	}
	close(process.done)
	deadline := time.After(2 * time.Second)
	for {
		_, err := os.Stat(workspace)
		if os.IsNotExist(err) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("executor kept workspace after process exit: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestLinuxNodePendingProcessRetainsWorkspaceWhenCleanupFails(t *testing.T) {
	workspaceID := "linux-workspace"
	owner := linuxNodeWorkspace{companyID: "company-1", runID: "run-1", revisionID: "revision-1", identity: "linux-workspace", workspaceID: workspaceID}
	cleanupAttempts := 0
	cleanupComplete := make(chan struct{})
	slotReleased := make(chan struct{})
	process := &linuxNodeTestProcess{}
	executor := &LinuxNodeNPMPreparationExecutor{
		paths:        environment.LinuxNodeSandboxPaths{WorkspaceRoot: t.TempDir()},
		pending:      map[string]linuxNodeWorkspace{workspaceID: owner},
		jobProcesses: map[string]linuxNodeProjectJob{"pending-" + workspaceID: {companyID: owner.companyID, revisionID: owner.revisionID, workspaceID: workspaceID, identity: owner.identity, process: process}},
		removeWorkspace: func(string, string) error {
			cleanupAttempts++
			if cleanupAttempts == 11 {
				close(cleanupComplete)
			}
			return errors.New("fixture removal denied")
		},
	}
	executor.monitorPendingProcess(owner, process, func() { close(slotReleased) })
	select {
	case <-cleanupComplete:
	case <-time.After(2 * time.Second):
		t.Fatal("pending cleanup retry sequence did not finish")
	}
	select {
	case <-slotReleased:
		t.Fatal("executor released admission before workspace cleanup succeeded")
	default:
	}
	executor.preparedMu.Lock()
	_, pending := executor.pending[workspaceID]
	_, processTracked := executor.jobProcesses["pending-"+workspaceID]
	executor.preparedMu.Unlock()
	if !pending || !processTracked {
		t.Fatalf("failed cleanup discarded workspace ownership: pending=%t processTracked=%t", pending, processTracked)
	}
}

func TestLinuxNodePendingProcessRetainsAdmissionUntilCgroupCleanupSucceeds(t *testing.T) {
	workspaceID := "linux-workspace"
	owner := linuxNodeWorkspace{companyID: "company-1", runID: "run-1", revisionID: "revision-1", identity: "linux-workspace", workspaceID: workspaceID}
	group := &linuxNodeTestCgroup{failUntil: 2}
	process := &linuxNodeCgroupTestProcess{AppContainerProcess: &linuxNodeTestProcess{}, cgroup: group}
	cleanupComplete := make(chan struct{})
	slotReleased := make(chan struct{})
	removedWorkspace := false
	executor := &LinuxNodeNPMPreparationExecutor{
		paths:        environment.LinuxNodeSandboxPaths{WorkspaceRoot: t.TempDir()},
		pending:      map[string]linuxNodeWorkspace{workspaceID: owner},
		jobProcesses: map[string]linuxNodeProjectJob{"pending-" + workspaceID: {companyID: owner.companyID, revisionID: owner.revisionID, workspaceID: workspaceID, identity: owner.identity, process: process}},
		removeWorkspace: func(string, string) error {
			removedWorkspace = true
			return nil
		},
	}
	go func() {
		executor.monitorPendingProcess(owner, process, func() {
			close(slotReleased)
			close(cleanupComplete)
		})
	}()
	select {
	case <-cleanupComplete:
	case <-time.After(2 * time.Second):
		t.Fatal("pending process did not release after cgroup cleanup succeeded")
	}
	if group.cleanupAttempts != 3 || !removedWorkspace {
		t.Fatalf("cgroup cleanup attempts=%d workspaceRemoved=%t", group.cleanupAttempts, removedWorkspace)
	}
	executor.preparedMu.Lock()
	_, pending := executor.pending[workspaceID]
	_, processTracked := executor.jobProcesses["pending-"+workspaceID]
	executor.preparedMu.Unlock()
	if pending || processTracked {
		t.Fatalf("successfully cleaned process remained tracked: pending=%t processTracked=%t", pending, processTracked)
	}
	select {
	case <-slotReleased:
	default:
		t.Fatal("preparation slot was not released after cleanup succeeded")
	}
}

func TestLinuxNodeExecutorCloseRetainsWorkspaceWhenPendingCgroupIsUnconfirmed(t *testing.T) {
	workspaceRoot := t.TempDir()
	workspaceID := "workspace-close-test"
	workspace := filepath.Join(workspaceRoot, workspaceID)
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	owner := linuxNodeWorkspace{companyID: "company-1", runID: "run-1", revisionID: "revision-1", workspaceID: workspaceID, workspaceRoot: workspace}
	group := &linuxNodeTestCgroup{failUntil: 1}
	executor := &LinuxNodeNPMPreparationExecutor{
		paths:    environment.LinuxNodeSandboxPaths{WorkspaceRoot: workspaceRoot},
		prepared: map[string]linuxNodeWorkspace{"revision-1": owner},
		pending:  map[string]linuxNodeWorkspace{workspaceID: owner},
		pendingResourceGroups: map[string]linuxNodePendingCgroup{
			"job-1": {companyID: "company-1", workspaceID: workspaceID, group: group},
		},
	}
	if err := executor.Close(); !errors.Is(err, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		t.Fatalf("executor close error=%v, want cgroup cleanup uncertainty", err)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("executor removed workspace while cgroup cleanup was unresolved: %v", err)
	}
	executor.preparedMu.Lock()
	_, prepared := executor.prepared["revision-1"]
	_, pending := executor.pending[workspaceID]
	_, cgroupTracked := executor.pendingResourceGroups["job-1"]
	executor.preparedMu.Unlock()
	if !prepared || !pending || !cgroupTracked {
		t.Fatalf("close discarded unresolved ownership: prepared=%t pending=%t cgroup=%t", prepared, pending, cgroupTracked)
	}
}

func linuxNodeExecutorFixture(t *testing.T) environment.LinuxNodeSandboxPaths {
	t.Helper()
	root := t.TempDir()
	image := filepath.Join(root, "image")
	workspaceRoot := filepath.Join(root, "workspaces")
	cache := filepath.Join(root, "npm-cache")
	cgroupRoot := filepath.Join(root, "delegated-cgroups")
	for _, directory := range []string{
		filepath.Join(image, "usr", "bin"), filepath.Join(image, "usr", "lib", "node_modules", "npm", "bin"), workspaceRoot,
		filepath.Join(cache, "_cacache"), cgroupRoot,
	} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, mountpoint := range []string{"proc", "dev", "tmp", "workspace"} {
		if err := os.Mkdir(filepath.Join(image, mountpoint), 0700); err != nil {
			t.Fatal(err)
		}
	}
	bwrap := filepath.Join(root, "bwrap")
	node := filepath.Join(image, "usr", "bin", "node")
	npm := filepath.Join(image, "usr", "lib", "node_modules", "npm", "bin", "npm-cli.js")
	for _, file := range []struct {
		path string
		mode os.FileMode
		data []byte
	}{{bwrap, 0700, []byte("fixture bwrap")}, {node, 0700, []byte("fixture node")}, {npm, 0600, []byte("fixture npm")}, {filepath.Join(cache, "_cacache", "index-v5"), 0600, []byte("offline fixture cache")}} {
		if err := os.WriteFile(file.path, file.data, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	return environment.LinuxNodeSandboxPaths{
		BubblewrapPath: bwrap, RuntimeRoot: root, SystemImageRoot: image, WorkspaceRoot: workspaceRoot,
		NPMCacheRoot: cache, CgroupRoot: cgroupRoot, NodeExecutable: "/usr/bin/node", NPMCLIScript: "/usr/lib/node_modules/npm/bin/npm-cli.js",
	}
}

type linuxNodeTestProcess struct {
	stdout []byte
}

type linuxNodeTestCgroupManager struct{ group *linuxNodeTestCgroup }

func (manager linuxNodeTestCgroupManager) Create(string) (environment.LinuxNodeResourceCgroup, error) {
	if manager.group != nil {
		return manager.group, nil
	}
	return &linuxNodeTestCgroup{}, nil
}

type linuxNodeTestCgroup struct {
	cleanupAttempts int
	failUntil       int
}

func (*linuxNodeTestCgroup) FileDescriptor() int { return 42 }
func (group *linuxNodeTestCgroup) Cleanup() error {
	group.cleanupAttempts++
	if group.cleanupAttempts <= group.failUntil {
		return environment.ErrLinuxNodeCgroupCleanupUnconfirmed
	}
	return nil
}

type linuxNodeCgroupTestProcess struct {
	runner.AppContainerProcess
	cgroup        environment.LinuxNodeResourceCgroup
	cgroupMu      sync.Mutex
	cgroupPending bool
}

func (process *linuxNodeCgroupTestProcess) Wait(ctx context.Context) (int, error) {
	exitCode, err := process.AppContainerProcess.Wait(ctx)
	if process.cgroup != nil {
		cleanupErr := process.cgroup.Cleanup()
		process.cgroupMu.Lock()
		process.cgroupPending = errors.Is(cleanupErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed)
		process.cgroupMu.Unlock()
		err = errors.Join(err, cleanupErr)
	}
	return exitCode, err
}

func (process *linuxNodeCgroupTestProcess) CgroupCleanupPending() bool {
	process.cgroupMu.Lock()
	defer process.cgroupMu.Unlock()
	return process.cgroupPending
}

func (*linuxNodeTestProcess) PID() int { return 23456 }
func (*linuxNodeTestProcess) Stdin() io.WriteCloser {
	return &linuxNodeTestWriteCloser{Writer: io.Discard}
}
func (process *linuxNodeTestProcess) Stdout() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(process.stdout))
}
func (*linuxNodeTestProcess) Stderr() io.ReadCloser             { return io.NopCloser(bytes.NewReader(nil)) }
func (*linuxNodeTestProcess) Wait(context.Context) (int, error) { return 0, nil }
func (*linuxNodeTestProcess) Stop() (runner.StopProof, error)   { return runner.StopProof{}, nil }

type linuxNodeTestWriteCloser struct{ io.Writer }

func (*linuxNodeTestWriteCloser) Close() error { return nil }

type linuxNodeUnconfirmedProcess struct{ done chan struct{} }

func (*linuxNodeUnconfirmedProcess) PID() int { return 34567 }
func (*linuxNodeUnconfirmedProcess) Stdin() io.WriteCloser {
	return &linuxNodeTestWriteCloser{Writer: io.Discard}
}
func (*linuxNodeUnconfirmedProcess) Stdout() io.ReadCloser { return io.NopCloser(bytes.NewReader(nil)) }
func (*linuxNodeUnconfirmedProcess) Stderr() io.ReadCloser { return io.NopCloser(bytes.NewReader(nil)) }
func (process *linuxNodeUnconfirmedProcess) Wait(context.Context) (int, error) {
	<-process.done
	return 0, nil
}
func (*linuxNodeUnconfirmedProcess) Stop() (runner.StopProof, error) { return runner.StopProof{}, nil }

func containsLinuxExecutorArg(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

func repeatControlDigest(character byte) string {
	digest := make([]byte, 64)
	for index := range digest {
		digest[index] = character
	}
	return string(digest)
}
