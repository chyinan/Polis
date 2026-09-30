// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/runner"
)

func TestWindowsWorkerHostStopReceiptReplayRequiresExactEvidence(t *testing.T) {
	metadata := runner.ProcessContainmentMetadata{HostOS: "windows", Profile: "windows_worker_job_object@1"}
	observation := windowsWorkerHostStopObservation{HostOS: "windows", Profile: metadata.Profile, TreeStopped: true}
	if !windowsWorkerHostStopReceiptMatches("stopped", "windows-worker-job-zero:0", 0, metadata, observation) {
		t.Fatal("exact durable Windows host-stop evidence was not accepted")
	}
	if windowsWorkerHostStopReceiptMatches("stopping", "windows-worker-job-zero:0", 0, metadata, observation) {
		t.Fatal("nonterminal WorkerSession was accepted as reconciled")
	}
	if windowsWorkerHostStopReceiptMatches("stopped", "windows-worker-job-zero:1", 0, metadata, observation) {
		t.Fatal("stop receipt/PID mismatch was accepted")
	}
	if windowsWorkerHostStopReceiptMatches("stopped", "windows-worker-job-zero:0", 0, runner.ProcessContainmentMetadata{HostOS: "linux", Profile: "linux_process_group@1"}, observation) {
		t.Fatal("Linux containment metadata was accepted as Windows host evidence")
	}
	observation.TreeStopped = false
	if windowsWorkerHostStopReceiptMatches("stopped", "windows-worker-job-zero:0", 0, metadata, observation) {
		t.Fatal("host observation without tree-stop proof was accepted")
	}
}

func TestWorkerHostContainmentBindingScopesRecoveryCandidates(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "worker-host-recovery-"+newID())
	must(t, err)
	task, err := k.TXCreateProbe(ctx, scope, "worker-host-recovery-mission")
	must(t, err)
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "offline-process-recovery")
	must(t, err)
	metadata := runner.CurrentProcessContainmentMetadata()
	if err = k.TXBindWorkerProcessHost(ctx, binding, metadata); err != nil {
		t.Fatalf("bind host containment before process creation: %v", err)
	}
	candidates, err := k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("restoring WorkerSession candidates=%+v error=%v", candidates, err)
	}
	restoringFound := false
	for _, item := range candidates {
		if item.CompanyID == scope.company && item.SessionID == binding.session && item.State == "restoring" {
			restoringFound = true
			break
		}
	}
	if !restoringFound {
		t.Fatalf("newly bound restoring WorkerSession was omitted from host recovery candidates: %+v", candidates)
	}
	expectedMetadata := metadata
	if metadata.HostOS == "linux" {
		expectedMetadata = runner.ProcessContainmentMetadata{
			HostOS: "linux", Profile: "linux_worker_cgroup_v2@1", CgroupID: "polis-worker-aaaaaaaaaaaaaaaaaaaaaaaa",
			CgroupHostID: strings.Repeat("b", 64), CgroupRootID: strings.Repeat("c", 64), CgroupBootID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		}
		if err = k.TXBindWorkerProcessHost(ctx, binding, expectedMetadata); err != nil {
			t.Fatalf("upgrade the prelaunch Linux process-group placeholder to its exact Worker cgroup: %v", err)
		}
		hasLinuxRecoveryWork, recoveryErr := k.HasUnrestoredLinuxNodeHostWork(ctx)
		if recoveryErr != nil || !hasLinuxRecoveryWork {
			t.Fatalf("unrestored Linux Worker cgroup did not require its startup recovery root: present=%t error=%v", hasLinuxRecoveryWork, recoveryErr)
		}
	}
	wrongMetadata := runner.ProcessContainmentMetadata{HostOS: "windows", Profile: "windows_worker_job_object@1"}
	if metadata.HostOS == "windows" {
		wrongMetadata = runner.ProcessContainmentMetadata{HostOS: "linux", Profile: "linux_process_group@1"}
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, wrongMetadata); err != core.Conflict {
		t.Fatalf("containment profile drift error=%v, want conflict", err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE worker_sessions SET state='reconcile_required' WHERE company_id=$1 AND id=$2`, scope.company, binding.session); err != nil {
		t.Fatal(err)
	}
	candidates, err = k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("host recovery candidates=%+v error=%v", candidates, err)
	}
	var candidate WorkerProcessRecoveryCandidate
	found := false
	for _, item := range candidates {
		if item.CompanyID == scope.company && item.SessionID == binding.session {
			candidate, found = item, true
			break
		}
	}
	if !found {
		t.Fatalf("bound WorkerSession was omitted from host recovery candidates: %+v", candidates)
	}
	if candidate.SessionID != binding.session || candidate.PID != 0 || candidate.Containment != expectedMetadata {
		t.Fatalf("candidate lost immutable session host binding: %+v", candidate)
	}
	if err = k.TXConfirmWindowsWorkerTreeStopped(ctx, candidate, runner.WindowsProcessTreeStopProof{}); err != core.Denied {
		t.Fatalf("fabricated host-stop proof error=%v, want %s", err, core.Denied)
	}
	if candidate.Containment.Profile == "linux_worker_cgroup_v2@1" {
		if err = k.TXConfirmLinuxWorkerCgroupStopped(ctx, candidate, environment.LinuxWorkerCgroupStopProof{}); err != core.Denied {
			t.Fatalf("fabricated Linux cgroup stop proof error=%v, want %s", err, core.Denied)
		}
	}
	var state string
	if err = k.pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, scope.company, binding.session).Scan(&state); err != nil || state != "reconcile_required" {
		t.Fatalf("invalid host proof changed session state to %q, err=%v", state, err)
	}
}

func TestTaskWorkspaceCASRehydratesForNewSessionAfterKernelRestart(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	first, err := Open(ctx, dsn, blobRoot)
	must(t, err)
	companyID := "workspace-restart-" + newID()
	scope, err := first.TXCreateCompany(ctx, companyID)
	must(t, err)
	task, err := first.TXCreateProbe(ctx, scope, "workspace-restart-mission-"+newID())
	must(t, err)
	content := `{"files":[{"path":"README.md","content":"saved task workspace"}]}`
	digest, err := putBlob(blobRoot, companyID, []byte(content))
	must(t, err)
	if _, err = first.pool.Exec(ctx, `UPDATE worker_workspaces SET digest=$3,revision=revision+1 WHERE company_id=$1 AND task_id=$2`, companyID, task.ID, digest); err != nil {
		t.Fatal(err)
	}
	firstBinding, err := first.TXNewWorker(ctx, scope, task.ID, "offline-workspace-restart-before")
	must(t, err)
	firstHandover, err := first.Handover(ctx, firstBinding)
	must(t, err)
	if firstHandover.Workspace.Digest != digest || firstHandover.Workspace.Revision != 2 || firstHandover.Workspace.Content != content {
		t.Fatalf("initial Worker handover did not read the CAS workspace: %+v", firstHandover.Workspace)
	}
	if err = first.TXFinalizeWorkerBeforeProcess(ctx, firstBinding, "workspace persistence fixture did not start a process", "workspace-restart-first-no-process"); err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := Open(ctx, dsn, blobRoot)
	must(t, err)
	defer second.Close()
	secondScope := second.LocalScope(companyID)
	secondBinding, err := second.TXNewWorker(ctx, secondScope, task.ID, "offline-workspace-restart-after")
	must(t, err)
	secondHandover, err := second.Handover(ctx, secondBinding)
	must(t, err)
	if secondHandover.Workspace.Digest != digest || secondHandover.Workspace.Revision != 2 || secondHandover.Workspace.Content != content {
		t.Fatalf("fresh Worker session did not rehydrate the durable CAS workspace: %+v", secondHandover.Workspace)
	}
	if err = second.TXFinalizeWorkerBeforeProcess(ctx, secondBinding, "workspace persistence fixture did not start a process", "workspace-restart-second-no-process"); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyLinuxWorkerSessionProcessGroupRecoveryRemainsUnresolved(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux procfs reconciliation required")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "linux-worker-recovery-"+newID())
	must(t, err)
	task, err := k.TXCreateProbe(ctx, scope, "linux-worker-recovery-mission")
	must(t, err)
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "offline-linux-worker-recovery")
	must(t, err)
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "exec /bin/sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, binding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, binding, "offline-linux-worker-recovery"); err != nil {
		t.Fatal(err)
	}
	if err = k.txResetFakeState(ctx); err != nil {
		t.Fatal(err)
	}
	if summary, reconcileErr := k.ReconcileLinuxWorkerSessions(ctx); reconcileErr == nil || summary.Candidates < 1 || summary.Stopped != 0 || summary.Unresolved != summary.Candidates {
		t.Fatalf("live process group was not left unresolved: summary=%+v err=%v", summary, reconcileErr)
	}
	if _, err = process.Stop(); err != nil {
		t.Fatalf("stop fake WorkerSession process: %v", err)
	}
	if summary, reconcileErr := k.ReconcileLinuxWorkerSessions(ctx); reconcileErr == nil || summary.Candidates < 1 || summary.Stopped != 0 || summary.Unresolved != summary.Candidates {
		t.Fatalf("legacy process-group session was incorrectly treated as a full Worker tree: summary=%+v err=%v", summary, reconcileErr)
	}
	var sessionState, taskState, missionState string
	if err = k.pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, scope.company, binding.SessionID()).Scan(&sessionState); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM tasks WHERE company_id=$1 AND id=$2`, scope.company, task.ID).Scan(&taskState); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM missions WHERE company_id=$1 AND id=$2`, scope.company, task.Mission).Scan(&missionState); err != nil {
		t.Fatal(err)
	}
	if sessionState != "reconcile_required" || taskState != "working" || missionState != "active" {
		t.Fatalf("legacy process-group refusal changed work state: session=%s task=%s mission=%s", sessionState, taskState, missionState)
	}
}

func TestTXFinalizeWorkerBeforeProcessIsIdempotentForSameFailure(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "worker-finalize-retry-"+newID())
	must(t, err)
	task, err := k.TXCreateProbe(ctx, scope, "worker-finalize-retry-mission")
	must(t, err)
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "offline-start-failure")
	must(t, err)
	const reason = "provider process was not created"
	const requestID = "worker-finalize-retry-same-failure"
	if err = k.TXFinalizeWorkerBeforeProcess(ctx, binding, reason, requestID); err != nil {
		t.Fatal(err)
	}
	if err = k.TXFinalizeWorkerBeforeProcess(ctx, binding, reason, requestID); err != nil {
		t.Fatalf("same no-process failure was not idempotent: %v", err)
	}
}
