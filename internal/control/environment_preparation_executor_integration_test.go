// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/kernel"
)

func TestEnvironmentPreparationExecutorRunsOnlyAfterQualificationAndReplaysReadyState(t *testing.T) {
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
	companyID := fmt.Sprintf("environment-executor-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "environment-executor-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "environment-executor-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic host qualification evidence for the executor integration test")
	evidenceUpload, err := intake.PrepareUpload("executor-qualification.md", "text/markdown", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "environment-executor-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	toolchainDigest := strings.Repeat("e", 64)
	_, policyJSON, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(runtime, &recordingWorkerAdapter{})
	defer service.Close()
	executor := &recordingEnvironmentPreparationExecutor{}
	service.SetProjectEnvironmentPreparationExecutor(executor)
	revision, err := service.RegisterProjectEnvironment(ctx, companyID, RegisterProjectEnvironmentRequest{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: "1",
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: toolchainDigest, RequestID: "environment-executor-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideProjectEnvironmentPolicy(ctx, companyID, revision.RevisionID, EnvironmentPolicyDecisionRequest{
		Decision: "approved", Rationale: "reviewed explicit source and lockfile digest", RequestID: "environment-executor-approve",
	}); err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidenceInput, "environment-executor-qualify")
	if _, err = runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, revision.RevisionID); err != nil {
		t.Fatalf("load verified execution snapshot before ensure: %v", err)
	}

	request := EnsureEnvironmentRequest{RequestID: "environment-executor-ensure"}
	run, err := service.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, request)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != string(environment.PreparationReady) || executor.calls != 1 {
		t.Fatalf("preparation result=%+v executor calls=%d, want ready and one execution", run, executor.calls)
	}
	if executor.snapshot.Revision.RevisionID != revision.RevisionID || len(executor.snapshot.Files) != 2 || executor.snapshot.Plan.LockfileSHA256 != revision.LockfileSHA256 {
		t.Fatalf("executor did not receive the verified immutable source snapshot: %+v", executor.snapshot)
	}
	replayed, err := service.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, request)
	if err != nil || replayed.RunID != run.RunID || replayed.State != string(environment.PreparationReady) || executor.calls != 1 {
		t.Fatalf("ready request replay=%+v calls=%d err=%v, want same ready run without re-execution", replayed, executor.calls, err)
	}
	if err = service.Close(); err != nil {
		t.Fatalf("close prepared environment service: %v", err)
	}
	closedRun, err := runtime.GetEnvironmentPreparationRun(ctx, companyID, run.RunID)
	if err != nil || closedRun.State != string(environment.PreparationOutcomeUnknown) {
		t.Fatalf("service close did not invalidate the memory-only prepared profile: %+v err=%v", closedRun, err)
	}
	executor.shutdownInvalidated = true

	restartedService := NewService(runtime, &recordingWorkerAdapter{})
	defer restartedService.Close()
	restartedExecutor := &recordingEnvironmentPreparationExecutor{}
	restartedService.SetProjectEnvironmentPreparationExecutor(restartedExecutor)
	restartedRequest := EnsureEnvironmentRequest{RequestID: "environment-executor-ensure-after-close"}
	restartedPrepared, err := restartedService.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, restartedRequest)
	if err != nil || restartedPrepared.State != string(environment.PreparationReady) || restartedExecutor.calls != 1 {
		t.Fatalf("fresh preparation after profile close=%+v executor calls=%d err=%v", restartedPrepared, restartedExecutor.calls, err)
	}
	crashRecoveryService := NewService(runtime, &recordingWorkerAdapter{})
	defer crashRecoveryService.Close()
	invalidated, err := crashRecoveryService.ReconcileEnvironmentPreparationsAfterRestart(ctx)
	if err != nil || invalidated < 1 {
		t.Fatalf("restart reconciliation invalidated=%d err=%v, want the ready profile invalidated", invalidated, err)
	}
	recoveredRun, err := runtime.GetEnvironmentPreparationRun(ctx, companyID, restartedPrepared.RunID)
	if err != nil || recoveredRun.State != string(environment.PreparationOutcomeUnknown) {
		t.Fatalf("restart reconciliation left a stale ready profile: %+v err=%v", recoveredRun, err)
	}
	restartedExecutor.shutdownInvalidated = true
	replayedAfterRestart, err := crashRecoveryService.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, restartedRequest)
	if err != nil || replayedAfterRestart.RunID != restartedPrepared.RunID || replayedAfterRestart.State != string(environment.PreparationOutcomeUnknown) {
		t.Fatalf("replayed request after restart reconciliation=%+v err=%v", replayedAfterRestart, err)
	}
	restoredExecutor := &recordingEnvironmentPreparationExecutor{}
	crashRecoveryService.SetProjectEnvironmentPreparationExecutor(restoredExecutor)
	freshRequest := EnsureEnvironmentRequest{RequestID: "environment-executor-rematerialize-after-restart"}
	rematerialized, err := crashRecoveryService.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, freshRequest)
	if err != nil || rematerialized.RunID == restartedPrepared.RunID || rematerialized.State != string(environment.PreparationReady) || restoredExecutor.calls != 1 {
		t.Fatalf("new request did not re-prepare the approved environment: run=%+v calls=%d err=%v", rematerialized, restoredExecutor.calls, err)
	}
	if restoredExecutor.snapshot.Revision.RevisionID != revision.RevisionID || len(restoredExecutor.snapshot.Files) != 2 || restoredExecutor.snapshot.Plan.LockfileSHA256 != revision.LockfileSHA256 {
		t.Fatalf("re-preparation did not restore the verified immutable source snapshot: %+v", restoredExecutor.snapshot)
	}
}

type recordingEnvironmentPreparationExecutor struct {
	calls               int
	snapshot            kernel.ProjectEnvironmentExecutionSnapshot
	companyID           string
	runID               string
	shutdownInvalidated bool
}

func (executor *recordingEnvironmentPreparationExecutor) PrepareProjectEnvironment(ctx context.Context, runID string, loadSnapshot ProjectEnvironmentSnapshotLoader) (EnvironmentPreparationResult, error) {
	snapshot, err := loadSnapshot(ctx)
	if err != nil {
		return EnvironmentPreparationResult{}, err
	}
	executor.calls++
	executor.snapshot = snapshot
	executor.companyID = snapshot.Revision.CompanyID
	executor.runID = runID
	return EnvironmentPreparationResult{TerminalState: string(environment.PreparationReady), Evidence: []byte("offline qualification fixture"), Logs: []byte("npm ci fixture complete")}, nil
}

func (executor *recordingEnvironmentPreparationExecutor) HasPreparedEnvironment(revisionID string) bool {
	return revisionID == executor.snapshot.Revision.RevisionID && !executor.shutdownInvalidated
}

func (*recordingEnvironmentPreparationExecutor) SupportsProjectEnvironmentProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile
}

func (executor *recordingEnvironmentPreparationExecutor) PreparedRunsForShutdown() []preparedEnvironmentRunRef {
	if executor.runID == "" || executor.shutdownInvalidated {
		return nil
	}
	executor.shutdownInvalidated = true
	return []preparedEnvironmentRunRef{{companyID: executor.companyID, runID: executor.runID}}
}

func (executor *recordingEnvironmentPreparationExecutor) Close() error { return nil }
