// pattern: Imperative Shell
package control

import (
	"context"
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
)

func TestReconcileEnvironmentPreparationsAfterRestartClosesUnknownWindowsJobRuns(t *testing.T) {
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
	companyID := fmt.Sprintf("windows-job-recovery-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "windows-job-recovery-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"recovery-fixture","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"recovery-fixture","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"recovery-fixture","version":"1.0.0"}}}`)},
		{RelativePath: "repo/build.mjs", MediaType: "text/javascript", Content: []byte("process.exit(0)")},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "windows-job-recovery-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	policy, _, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, policyManifest, _, err := environment.CanonicalizeWindowsNodeEnvironmentPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: policyManifest, ToolchainSHA256: strings.Repeat("e", 64),
	}, "windows-job-recovery-environment")
	if err != nil {
		t.Fatal(err)
	}
	taskID, sessionID, jobID := "windows-job-recovery-task", "windows-job-recovery-session", "windows-job-recovery-run"
	if _, err = pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
		VALUES($1,$2,$3,$4,'compat','working')`, companyID, taskID, missionID, core.EmployeeBackendID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state)
VALUES($1,$2,$3,$4,1,1,'windows-job-recovery-incarnation','offline-model/medium','active')`, companyID, sessionID, core.EmployeeBackendID, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'batch',$6,$7,$8,$9,'[]'::jsonb,60000,4096,'windows-job-recovery-run')`, companyID, jobID, taskID, sessionID, revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code)
VALUES($1,'windows-job-recovery-unknown',$2,'outcome_unknown','not_applicable','job_process_not_restored')`, companyID, jobID); err != nil {
		t.Fatal(err)
	}
	reconciler := &startupRecoveryProjectJobExecutor{recordingProjectJobExecutor: &recordingProjectJobExecutor{}}
	service := NewService(runtime, &recordingWorkerAdapter{})
	defer service.Close()
	service.SetProjectJobExecutor(reconciler)
	if _, err = service.ReconcileEnvironmentPreparationsAfterRestart(ctx); err != nil {
		t.Fatal(err)
	}
	job, err := runtime.GetJobRun(ctx, companyID, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != string(environment.JobCancelled) || reconciler.reconciliations < 1 {
		t.Fatalf("startup recovery job=%+v reconciliations=%d", job, reconciler.reconciliations)
	}
}

type startupRecoveryProjectJobExecutor struct {
	*recordingProjectJobExecutor
	reconciliations int
}

func (executor *startupRecoveryProjectJobExecutor) ReconcileUnrestoredProjectJob(companyID, jobID, revisionID string) error {
	executor.reconciliations++
	return executor.recordingProjectJobExecutor.ReconcileUnrestoredProjectJob(companyID, jobID, revisionID)
}
