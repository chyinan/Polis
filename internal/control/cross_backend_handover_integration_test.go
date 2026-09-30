// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

func TestCrossBackendHandoverFlowsThroughPauseResumeAndJobRunWithoutProviderTurn(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	companyID := fmt.Sprintf("control-cross-handover-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "control-cross-handover-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"control-handover","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"control-handover","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"control-handover","version":"1.0.0"}}}`)},
		{RelativePath: "repo/scripts/build.mjs", MediaType: "text/javascript", Content: []byte("process.stdout.write('handover ok')")},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "control-handover-input", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic cross-backend executor qualification evidence")
	evidenceUpload, err := intake.PrepareUpload("executor-qualification.md", "text/markdown", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "control-handover-qualification-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	windowsPolicy, _, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, windowsPolicyJSON, _, err := environment.CanonicalizeWindowsNodeEnvironmentPolicy(windowsPolicy)
	if err != nil {
		t.Fatal(err)
	}
	linuxPolicy, _, _, err := environment.BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, linuxPolicyJSON, _, err := environment.CanonicalizeLinuxNodeEnvironmentPolicy(linuxPolicy)
	if err != nil {
		t.Fatal(err)
	}
	windowsRevision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(windowsPolicyJSON), ToolchainSHA256: strings.Repeat("a", 64),
	}, "control-cross-handover-windows-environment")
	if err != nil {
		t.Fatal(err)
	}
	linuxRevision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.LinuxNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(linuxPolicyJSON), ToolchainSHA256: strings.Repeat("b", 64),
	}, "control-cross-handover-linux-environment")
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range []kernel.ProjectEnvironmentRevision{windowsRevision, linuxRevision} {
		if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
			RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approve fixed source for local handover test", RequestID: "control-cross-handover-policy-" + revision.RevisionID,
		}); err != nil {
			t.Fatal(err)
		}
		qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidenceInput, "control-handover-qualification-"+strings.ReplaceAll(revision.ProfileID, "@", "-"))
		preparationID := "control-handover-preparation-" + strings.ReplaceAll(revision.ProfileID, "@", "-")
		err = nil
		if err == nil {
			_, err = pool.Exec(ctx, `INSERT INTO environment_preparation_runs(company_id,run_id,revision_id,request_id,created_by)
			VALUES($1,$2,$3,$4,'test-owner')`, companyID, preparationID, revision.RevisionID, preparationID)
		}
		if err == nil {
			_, err = pool.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code,evidence_sha256)
			VALUES($1,$2,$3,'ready','fixture_ready',$4)`, companyID, "control-handover-prepared-"+strings.ReplaceAll(revision.ProfileID, "@", "-"), preparationID, strings.Repeat("d", 64))
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	const taskID = "control-cross-handover-task"
	const sourceSessionID = "control-cross-handover-source-session"
	if _, err = pool.Exec(ctx, `UPDATE missions SET state='active' WHERE company_id=$1 AND id=$2`, companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,'compat','ready')`, companyID, taskID, missionID, core.EmployeeBackendID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO worker_workspaces(company_id,task_id,digest,revision) VALUES($1,$2,$3,1)`, companyID, taskID, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if err = runtime.EnsureTaskInputManifest(ctx, scope, missionID, taskID, "control-cross-handover-manifest"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE tasks SET state='working' WHERE company_id=$1 AND id=$2", companyID, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state)
VALUES($1,$2,$3,$4,1,1,$5,'provider-fixture','active')`, companyID, sourceSessionID, core.EmployeeBackendID, taskID, runtime.Incarnation()); err != nil {
		t.Fatal(err)
	}

	windowSnapshot, err := runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, windowsRevision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	argvDigest, err := projectJobArgvDigest("scripts/build.mjs", nil)
	if err != nil {
		t.Fatal(err)
	}
	isolation, _ := environment.IsolationProfileForNodeProfile(windowsRevision.ProfileID)
	sourceJob, err := runtime.TXCreateJobRun(ctx, companyID, kernel.JobRunInput{
		TaskID: taskID, SessionID: sourceSessionID, EnvironmentRevisionID: windowsRevision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: windowSnapshot.Revision.SourceRevisionSHA256, ArgvSHA256: argvDigest,
		WorkingDirectorySHA256: digestBytes([]byte(windowSnapshot.Revision.ProjectRootRelative)),
		NetworkPolicySHA256:    digestBytes([]byte(isolation + ":deny_all")),
		TimeoutMS:              windowSnapshot.Policy.TimeoutMS, OutputLimitBytes: windowSnapshot.Policy.OutputLimitBytes, RequestID: "control-cross-handover-source-job",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{string(environment.JobStarting), string(environment.JobRunning), string(environment.JobExited)} {
		var exitCode *int
		if state == string(environment.JobExited) {
			code := 0
			exitCode = &code
		}
		if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{
			JobID: sourceJob.ID, State: state, Readiness: "not_applicable", ExitCode: exitCode, ReasonCode: "fixture_source_terminal",
			RequestID: "control-cross-handover-source-event-" + state,
		}); err != nil {
			t.Fatal(err)
		}
	}

	worker := &handoverLifecycleWorkerAdapter{pool: pool, readinessErr: errors.New("provider runtime must remain unused during project handover")}
	service := NewService(runtime, worker)
	defer service.Close()
	if _, err = service.PauseMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-pause"}); err != nil {
		t.Fatalf("pause Mission through Control: %v", err)
	}
	handover, err := service.CreateTaskEnvironmentHandover(ctx, companyID, taskID, CreateProjectEnvironmentHandoverRequest{
		SourceJobID: sourceJob.ID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "control-cross-handover-create",
	})
	if err != nil {
		t.Fatalf("create handover through Control: %v", err)
	}
	if _, err = service.ResumeMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-resume"}); err != nil {
		t.Fatalf("resume through the no-provider handover path: %v", err)
	}
	if worker.readinessCalls != 0 || worker.starts != 0 || worker.stops != 1 {
		t.Fatalf("provider adapter activity during local handover: readiness=%d starts=%d stops=%d", worker.readinessCalls, worker.starts, worker.stops)
	}

	executor := &handoverLifecycleProjectJobExecutor{processes: []runner.AppContainerProcess{
		&finishedProjectJobProcess{stdout: []byte("handover ok")},
		&cancelledProjectJobProcess{stopped: make(chan struct{})},
		&uncertainStopProjectJobProcess{stopped: make(chan struct{})},
		&cancelledProjectJobProcess{stopped: make(chan struct{})},
	}}
	service.SetProjectJobExecutor(executor)
	linuxSnapshot, err := runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, linuxRevision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	linuxIsolation, _ := environment.IsolationProfileForNodeProfile(linuxRevision.ProfileID)
	job, err := service.StartProjectJob(ctx, companyID, StartProjectJobRequest{
		TaskID: taskID, EnvironmentRevisionID: linuxRevision.RevisionID, HandoverID: handover.HandoverID,
		Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{}, RequestID: "control-cross-handover-target-job",
	})
	if err != nil {
		t.Fatalf("start target JobRun through Control without a provider WorkerSession: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for job.State != string(environment.JobExited) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		job, err = runtime.GetJobRun(ctx, companyID, job.JobID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if job.State != string(environment.JobExited) || job.SessionID == "" || job.SessionID == sourceSessionID || job.HandoverID != handover.HandoverID || executor.launches != 1 {
		t.Fatalf("target JobRun=%+v launches=%d", job, executor.launches)
	}
	var executionMode, sessionState, sessionProfile string
	if err = pool.QueryRow(ctx, `SELECT execution_mode,state,profile FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, job.SessionID).Scan(&executionMode, &sessionState, &sessionProfile); err != nil || executionMode != "project_job" || sessionState != "stopped" || sessionProfile != "project-job:"+environment.LinuxNodeNPMProfile {
		t.Fatalf("controlled job session mode=%q state=%q profile=%q err=%v", executionMode, sessionState, sessionProfile, err)
	}
	activeJob, err := service.StartProjectJob(ctx, companyID, StartProjectJobRequest{
		TaskID: taskID, EnvironmentRevisionID: linuxRevision.RevisionID, Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{},
		RequestID: "control-cross-handover-running-job",
	})
	if err != nil || activeJob.State != string(environment.JobRunning) {
		t.Fatalf("start same-profile follow-up JobRun before pause = %+v err=%v", activeJob, err)
	}
	paused, err := service.PauseMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-pause-with-running-job"})
	if err != nil || paused.ResultingState != "paused" {
		t.Fatalf("PauseMission with running project JobRun receipt=%+v err=%v", paused, err)
	}
	stoppedJob, err := runtime.GetJobRun(ctx, companyID, activeJob.JobID)
	if err != nil || stoppedJob.State != string(environment.JobCancelled) {
		t.Fatalf("PauseMission left project JobRun live: job=%+v err=%v", stoppedJob, err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, activeJob.SessionID).Scan(&sessionState); err != nil || sessionState != "stopped" {
		t.Fatalf("PauseMission did not stop the project_job session: state=%q err=%v", sessionState, err)
	}
	worker.readinessErr = nil
	if _, err = service.ResumeMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-resume-after-pause"}); err != nil {
		t.Fatalf("resume for the unresolved-stop fixture: %v", err)
	}
	unknownJob, err := service.StartProjectJob(ctx, companyID, StartProjectJobRequest{
		TaskID: taskID, EnvironmentRevisionID: linuxRevision.RevisionID, Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{},
		RequestID: "control-cross-handover-unknown-job",
	})
	if err != nil || unknownJob.State != string(environment.JobRunning) {
		t.Fatalf("start the unknown-stop JobRun = %+v err=%v", unknownJob, err)
	}
	if _, err = service.PauseMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-pause-unconfirmed"}); err == nil {
		t.Fatal("PauseMission reported success while project JobRun stop was unconfirmed")
	}
	unknownJob, err = runtime.GetJobRun(ctx, companyID, unknownJob.JobID)
	if err != nil || unknownJob.State != string(environment.JobOutcomeUnknown) {
		t.Fatalf("unconfirmed pause JobRun=%+v error=%v", unknownJob, err)
	}
	var missionState string
	if err = pool.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2", companyID, missionID).Scan(&missionState); err != nil || missionState != "active" {
		t.Fatalf("Mission state after unconfirmed project-job stop = %q error=%v, want active", missionState, err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, unknownJob.SessionID).Scan(&sessionState); err != nil || sessionState != "reconcile_required" {
		t.Fatalf("unconfirmed stop session state=%q err=%v, want reconcile_required", sessionState, err)
	}
	reconciledJob, err := service.StopProjectJob(ctx, companyID, unknownJob.JobID, StopProjectJobRequest{RequestID: "control-cross-handover-unknown-reconcile"})
	if err != nil || reconciledJob.State != string(environment.JobCancelled) {
		t.Fatalf("explicit reconciliation did not clear the unknown JobRun: job=%+v err=%v", reconciledJob, err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, unknownJob.SessionID).Scan(&sessionState); err != nil || sessionState != "stopped" {
		t.Fatalf("reconciled project_job session state=%q err=%v, want stopped", sessionState, err)
	}
	cancelledByMission, err := service.StartProjectJob(ctx, companyID, StartProjectJobRequest{
		TaskID: taskID, EnvironmentRevisionID: linuxRevision.RevisionID, Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{},
		RequestID: "control-cross-handover-cancel-mission-job",
	})
	if err != nil || cancelledByMission.State != string(environment.JobRunning) {
		t.Fatalf("start project JobRun before Mission cancellation = %+v err=%v", cancelledByMission, err)
	}
	cancelReceipt, err := service.CancelMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-cancel-mission"})
	if err != nil || cancelReceipt.ResultingState != "cancelled" {
		t.Fatalf("CancelMission with running project JobRun receipt=%+v err=%v", cancelReceipt, err)
	}
	cancelledByMission, err = runtime.GetJobRun(ctx, companyID, cancelledByMission.JobID)
	if err != nil || cancelledByMission.State != string(environment.JobCancelled) {
		t.Fatalf("CancelMission left project JobRun live: job=%+v err=%v", cancelledByMission, err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, cancelledByMission.SessionID).Scan(&sessionState); err != nil || sessionState != "stopped" {
		t.Fatalf("CancelMission did not stop the project_job session: state=%q err=%v", sessionState, err)
	}
	if err = pool.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2", companyID, missionID).Scan(&missionState); err != nil || missionState != "cancelled" {
		t.Fatalf("Mission state after confirmed project-job cancellation = %q error=%v, want cancelled", missionState, err)
	}
	replayService := NewService(runtime, nil)
	replayedCancel, err := replayService.CancelMission(ctx, companyID, MissionCommandRequest{MissionID: missionID, RequestID: "control-cross-handover-cancel-mission"})
	if err != nil || replayedCancel.ResultingState != "cancelled" || replayedCancel.TargetID != missionID {
		t.Fatalf("CancelMission receipt replay without a WorkerAdapter = %+v err=%v", replayedCancel, err)
	}
	if linuxSnapshot.Revision.RevisionID != linuxRevision.RevisionID || linuxIsolation == "" {
		t.Fatal("target Linux environment fixture was not valid")
	}
}

type handoverLifecycleWorkerAdapter struct {
	pool           *pgxpool.Pool
	readinessErr   error
	readinessCalls int
	starts         int
	stops          int
}

func (*handoverLifecycleWorkerAdapter) Mode() string { return "test" }
func (adapter *handoverLifecycleWorkerAdapter) Readiness(context.Context) error {
	adapter.readinessCalls++
	return adapter.readinessErr
}
func (*handoverLifecycleWorkerAdapter) ToolSurface() provider.ToolSurface {
	return provider.ToolSurface{}
}
func (adapter *handoverLifecycleWorkerAdapter) Start(context.Context, string, string) error {
	adapter.starts++
	return nil
}
func (adapter *handoverLifecycleWorkerAdapter) Stop(ctx context.Context, companyID, missionID string) error {
	adapter.stops++
	_, err := adapter.pool.Exec(ctx, `UPDATE worker_sessions s SET state='stopped',stop_receipt='control-handover-stop'
FROM tasks t WHERE s.company_id=$1 AND t.company_id=s.company_id AND t.id=s.task_id AND t.mission_id=$2 AND s.state<>'stopped'`, companyID, missionID)
	return err
}
func (*handoverLifecycleWorkerAdapter) Close() {}

type handoverLifecycleProjectJobExecutor struct {
	launches     int
	processes    []runner.AppContainerProcess
	stopAttempts int
}

func (*handoverLifecycleProjectJobExecutor) HasPreparedEnvironment(string) bool { return true }
func (*handoverLifecycleProjectJobExecutor) SupportsProjectJobProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile || profileID == environment.LinuxNodeNPMProfile
}
func (executor *handoverLifecycleProjectJobExecutor) LaunchProjectJob(_ context.Context, request ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	if request.ProfileID != environment.LinuxNodeNPMProfile {
		return nil, core.Denied
	}
	if executor.launches >= len(executor.processes) {
		return nil, errors.New("no project JobRun fixture is configured")
	}
	process := executor.processes[executor.launches]
	executor.launches++
	return process, nil
}
func (executor *handoverLifecycleProjectJobExecutor) StopProjectJob(_ context.Context, _, _ string) error {
	if executor.launches > 0 {
		if process, ok := executor.processes[executor.launches-1].(*cancelledProjectJobProcess); ok {
			process.stop()
		}
		if process, ok := executor.processes[executor.launches-1].(*uncertainStopProjectJobProcess); ok {
			executor.stopAttempts++
			if executor.stopAttempts == 1 {
				return errors.New("fixture did not confirm process-tree stop")
			}
			process.stop()
		}
	}
	return nil
}
func (*handoverLifecycleProjectJobExecutor) ForgetProjectJob(string) {}
