// pattern: Imperative Shell
package kernel

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
)

func TestCrossBackendHandoverRequiresPauseAndKnownTerminalSourceAndIsSingleUse(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	companyID := fmt.Sprintf("cross-handover-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "cross-handover-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = k.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"handover-fixture","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"handover-fixture","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"handover-fixture","version":"1.0.0"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := k.TXAddMissionInput(ctx, scope, missionID, "", "cross-handover-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic Linux executor qualification evidence")
	evidenceUpload, err := intake.PrepareUpload("executor-qualification.md", "text/markdown", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := k.TXAddMissionInput(ctx, scope, missionID, "", "cross-handover-qualification-evidence", evidenceUpload, evidenceBytes)
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
	windowsRevision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: windowsNodeHandoverProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(windowsPolicyJSON), ToolchainSHA256: strings.Repeat("a", 64),
	}, "cross-handover-register-windows")
	if err != nil {
		t.Fatal(err)
	}
	linuxRevision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: linuxNodeHandoverProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(linuxPolicyJSON), ToolchainSHA256: strings.Repeat("b", 64),
	}, "cross-handover-register-linux")
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range []ProjectEnvironmentRevision{windowsRevision, linuxRevision} {
		if _, err = k.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{
			RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approve exact project snapshot for handover test", RequestID: "cross-handover-policy-" + revision.RevisionID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := k.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, k, companyID, linuxRevision.RevisionID, evidenceInput, "cross-handover-linux-qualified")
	err = nil
	if err == nil {
		_, err = pool.Exec(ctx, `INSERT INTO environment_preparation_runs(company_id,run_id,revision_id,request_id,created_by)
VALUES($1,'cross-handover-linux-ready-run',$2,'cross-handover-linux-ready-run','test-owner')`, companyID, linuxRevision.RevisionID)
	}
	if err == nil {
		_, err = pool.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code,evidence_sha256)
VALUES($1,'cross-handover-linux-ready-event','cross-handover-linux-ready-run','ready','fixture_ready',$2)`, companyID, strings.Repeat("d", 64))
	}
	pool.Release()
	if err != nil {
		t.Fatal(err)
	}
	hasLinuxRecoveryWork, err := k.HasUnrestoredLinuxNodeHostWork(ctx)
	if err != nil || hasLinuxRecoveryWork {
		t.Fatalf("ready Linux environment created restart work: present=%t error=%v", hasLinuxRecoveryWork, err)
	}
	recoveryMission, err := k.TXCreateMissionGoal(ctx, k.LocalScope(companyID), "Linux cgroup recovery fixture", "Verify the Linux host recovery gate.", "cross-handover-linux-recovery-mission")
	if err != nil {
		t.Fatal(err)
	}
	const recoveryTaskID = "cross-handover-linux-recovery-task"
	const recoverySessionID = "cross-handover-linux-recovery-session"
	const recoveryJobID = "cross-handover-linux-recovery-job"
	if _, err = k.pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
VALUES($1,$2,$3,$4,'bootstrap_plan','working')`, companyID, recoveryTaskID, recoveryMission.ID, core.EmployeePlanningID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,stop_receipt)
VALUES($1,$2,$3,$4,1,1,'linux-recovery-fixture','linux-recovery-fixture','stopped','fixture-stop')`, companyID, recoverySessionID, core.EmployeePlanningID, recoveryTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'batch',$6,$7,$8,$9,'[]'::jsonb,1000,4096,$10)`, companyID, recoveryJobID, recoveryTaskID, recoverySessionID, linuxRevision.RevisionID, strings.Repeat("a", 64), strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), "cross-handover-linux-recovery-job-request"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code)
VALUES($1,'cross-handover-linux-recovery-running',$2,'running','not_applicable','fixture_running')`, companyID, recoveryJobID); err != nil {
		t.Fatal(err)
	}
	if hasLinuxRecoveryWork, err = k.HasUnrestoredLinuxNodeHostWork(ctx); err != nil || !hasLinuxRecoveryWork {
		t.Fatalf("running Linux JobRun did not require host recovery: present=%t error=%v", hasLinuxRecoveryWork, err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,exit_code,reason_code)
VALUES($1,'cross-handover-linux-recovery-exited',$2,'exited','not_applicable',0,'fixture_exited')`, companyID, recoveryJobID); err != nil {
		t.Fatal(err)
	}
	if hasLinuxRecoveryWork, err = k.HasUnrestoredLinuxNodeHostWork(ctx); err != nil || hasLinuxRecoveryWork {
		t.Fatalf("terminal Linux JobRun still required host recovery: present=%t error=%v", hasLinuxRecoveryWork, err)
	}

	const taskID = "cross-handover-task"
	const sourceJobID = "cross-handover-source-job"
	const sourceSessionID = "cross-handover-source-session"
	if err = insertCrossHandoverSourceTask(ctx, k.pool, companyID, missionID, taskID, sourceSessionID, sourceJobID, windowsRevision.RevisionID, "exited", k.incarnation); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='active' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: sourceJobID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "cross-handover-before-pause",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("handover while Mission active = %v, want %s", err, core.Denied)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='paused' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE worker_sessions SET state='active',stop_receipt=NULL WHERE company_id=$1 AND id=$2", companyID, sourceSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: sourceJobID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "cross-handover-with-live-worker",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("handover while a WorkerSession is active = %v, want %s", err, core.Denied)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE worker_sessions SET state='stopped',stop_receipt='fixture-stop' WHERE company_id=$1 AND id=$2", companyID, sourceSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mission_inputs(company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state)
VALUES($1,'cross-handover-uploading-input',$2,1,'cross-handover-uploading-request','upload','pending.txt','text/plain',1,$3,'uploading')`, companyID, missionID, strings.Repeat("9", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: sourceJobID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "cross-handover-while-uploading",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("handover during a Mission input upload = %v, want %s", err, core.Denied)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mission_inputs SET state='rejected',updated_at=clock_timestamp() WHERE company_id=$1 AND input_id='cross-handover-uploading-input' AND revision=1`, companyID); err != nil {
		t.Fatal(err)
	}
	handover, err := k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: sourceJobID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "cross-handover-create",
	})
	if err != nil || handover.HandoverID == "" || handover.RecordSHA256 == "" || handover.SourceProfileID != windowsNodeHandoverProfile || handover.TargetProfileID != linuxNodeHandoverProfile {
		t.Fatalf("create cross-backend handover = %+v, error=%v", handover, err)
	}
	replay, err := k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: sourceJobID, TargetEnvironmentRevision: linuxRevision.RevisionID, RequestID: "cross-handover-create",
	})
	if err != nil || replay.HandoverID != handover.HandoverID {
		t.Fatalf("idempotent handover replay = %+v, error=%v; original=%+v", replay, err, handover)
	}

	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='active' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	jobInput := JobRunInput{
		TaskID: taskID, EnvironmentRevisionID: linuxRevision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: linuxRevision.SourceRevisionSHA256, ArgvSHA256: strings.Repeat("1", 64),
		WorkingDirectorySHA256: strings.Repeat("2", 64), NetworkPolicySHA256: strings.Repeat("3", 64),
		EnvironmentAllowlist: []string{}, TimeoutMS: 1000, OutputLimitBytes: 4096, RequestID: "cross-handover-target-without-reference",
	}
	if _, err = k.TXCreateJobRun(ctx, companyID, jobInput); !errors.Is(err, core.Denied) {
		t.Fatalf("cross-profile JobRun without handover = %v, want %s", err, core.Denied)
	}
	jobInput.HandoverID = handover.HandoverID
	jobInput.RequestID = "cross-handover-target-start"
	job, err := k.TXCreateJobRun(ctx, companyID, jobInput)
	if err != nil {
		t.Fatalf("start target JobRun with handover: %v", err)
	}
	jobRecord, err := k.GetJobRun(ctx, companyID, job.ID)
	if err != nil || jobRecord.HandoverID != handover.HandoverID || jobRecord.SessionID == "" || jobRecord.SessionID == sourceSessionID {
		t.Fatalf("target JobRun handover binding=%+v error=%v", jobRecord, err)
	}
	var sessionMode, targetIncarnation string
	if err = k.pool.QueryRow(ctx, "SELECT execution_mode,incarnation FROM worker_sessions WHERE company_id=$1 AND id=$2", companyID, jobRecord.SessionID).Scan(&sessionMode, &targetIncarnation); err != nil || sessionMode != "project_job" || targetIncarnation != k.incarnation {
		t.Fatalf("target JobRun session mode=%q incarnation=%q err=%v", sessionMode, targetIncarnation, err)
	}
	jobInput.RequestID = "cross-handover-target-second-consumer"
	if _, err = k.TXCreateJobRun(ctx, companyID, jobInput); err == nil {
		t.Fatal("one handover authorized more than one target JobRun")
	}
	for _, event := range []JobRunEventInput{
		{JobID: job.ID, State: "starting", Readiness: "not_applicable", ReasonCode: "fixture_starting", RequestID: "cross-handover-target-starting"},
		{JobID: job.ID, State: "running", Readiness: "not_applicable", ReasonCode: "fixture_running", RequestID: "cross-handover-target-running"},
	} {
		if _, err = k.TXRecordJobRunEvent(ctx, companyID, event); err != nil {
			t.Fatal(err)
		}
	}
	exitCode := 0
	if _, err = k.TXRecordJobRunEvent(ctx, companyID, JobRunEventInput{JobID: job.ID, State: "exited", Readiness: "not_applicable", ExitCode: &exitCode, ReasonCode: "fixture_exited", RequestID: "cross-handover-target-exited"}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='paused' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO environment_preparation_runs(company_id,run_id,revision_id,request_id,created_by)
VALUES($1,'cross-handover-linux-recovery-prep',$2,'cross-handover-linux-recovery-prep','test-owner')`, companyID, linuxRevision.RevisionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code)
VALUES($1,'cross-handover-linux-recovery-prep-running','cross-handover-linux-recovery-prep','running','fixture_running')`, companyID); err != nil {
		t.Fatal(err)
	}
	if hasLinuxRecoveryWork, err = k.HasUnrestoredLinuxNodeHostWork(ctx); err != nil || !hasLinuxRecoveryWork {
		t.Fatalf("running Linux preparation did not require host recovery: present=%t error=%v", hasLinuxRecoveryWork, err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code)
VALUES($1,'cross-handover-linux-recovery-prep-failed','cross-handover-linux-recovery-prep','failed','fixture_failed')`, companyID); err != nil {
		t.Fatal(err)
	}
	if hasLinuxRecoveryWork, err = k.HasUnrestoredLinuxNodeHostWork(ctx); err != nil || hasLinuxRecoveryWork {
		t.Fatalf("terminal Linux work still required host recovery: present=%t error=%v", hasLinuxRecoveryWork, err)
	}
	if _, err = k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: job.ID, TargetEnvironmentRevision: windowsRevision.RevisionID, RequestID: "cross-handover-reverse-unqualified-target",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("handover to an unqualified target profile = %v, want %s", err, core.Denied)
	}
	if err = insertCrossHandoverAdditionalJob(ctx, k.pool, companyID, taskID, "cross-handover-unknown-session", "cross-handover-unknown-job", linuxRevision.RevisionID, linuxNodeHandoverProfile, "outcome_unknown"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='active' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state)
VALUES($1,'cross-handover-unknown-retry-session',$2,$3,4,1,'linux-unknown-retry-fixture',$4,'active')`, companyID, core.EmployeeBackendID, taskID, linuxNodeHandoverProfile); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateJobRun(ctx, companyID, JobRunInput{
		TaskID: taskID, SessionID: "cross-handover-unknown-retry-session", EnvironmentRevisionID: linuxRevision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: linuxRevision.SourceRevisionSHA256, ArgvSHA256: strings.Repeat("1", 64),
		WorkingDirectorySHA256: strings.Repeat("2", 64), NetworkPolicySHA256: strings.Repeat("3", 64),
		EnvironmentAllowlist: []string{}, TimeoutMS: 1000, OutputLimitBytes: 4096, RequestID: "cross-handover-same-profile-after-unknown",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("same-profile JobRun after an unknown outcome = %v, want %s", err, core.Denied)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE worker_sessions SET state='stopped',stop_receipt='fixture-unknown-stop' WHERE company_id=$1 AND id='cross-handover-unknown-retry-session'", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE missions SET state='paused' WHERE company_id=$1 AND id=$2", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateCrossBackendHandover(ctx, companyID, CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: "cross-handover-unknown-job", TargetEnvironmentRevision: windowsRevision.RevisionID, RequestID: "cross-handover-unknown-source",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("handover from outcome_unknown JobRun = %v, want %s", err, core.Denied)
	}
}

func insertCrossHandoverSourceTask(ctx context.Context, pool *pgxpool.Pool, companyID, missionID, taskID, sessionID, jobID, environmentRevisionID, finalState, runtimeIncarnation string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
VALUES($1,$2,$3,$4,'compat','working')`, companyID, taskID, missionID, core.EmployeeBackendID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,stop_receipt)
VALUES($1,$2,$3,$4,1,1,$5,'windows-node-npm@1','stopped','fixture-stop')`, companyID, sessionID, core.EmployeeBackendID, taskID, runtimeIncarnation); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest,revision) VALUES($1,$2,$3,1)", companyID, taskID, strings.Repeat("e", 64)); err != nil {
		return err
	}
	if err = bindMissionInputManifest(ctx, tx, Scope{company: companyID}, missionID, taskID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'batch',$6,$7,$8,$9,'[]'::jsonb,1000,4096,$2)`, companyID, jobID, taskID, sessionID, environmentRevisionID, strings.Repeat("a", 64), strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		return err
	}
	var exitCode any
	if finalState == "exited" {
		exitCode = 0
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,exit_code,reason_code)
VALUES($1,$2,$3,$4,'not_applicable',$5,'fixture_terminal')`, companyID, jobID+"-event", jobID, finalState, exitCode); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertCrossHandoverAdditionalJob(ctx context.Context, pool *pgxpool.Pool, companyID, taskID, sessionID, jobID, environmentRevisionID, profileID, finalState string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,stop_receipt)
		VALUES($1,$2,$3,$4,3,1,'linux-unknown-fixture',$5,'stopped','fixture-stop')`, companyID, sessionID, core.EmployeeBackendID, taskID, profileID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'batch',$6,$7,$8,$9,'[]'::jsonb,1000,4096,$2)`, companyID, jobID, taskID, sessionID, environmentRevisionID, strings.Repeat("a", 64), strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code)
VALUES($1,$2,$3,$4,'not_applicable','fixture_unknown')`, companyID, jobID+"-event", jobID, finalState); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
