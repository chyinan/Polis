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

func TestLinuxProfileDoesNotReuseWindowsExecutorQualification(t *testing.T) {
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
	companyID := fmt.Sprintf("linux-profile-control-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := fmt.Sprintf("linux-profile-mission-%d", time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "Repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"offline-demo","version":"1.0.0"}`)},
		{RelativePath: "Repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"offline-demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"offline-demo","version":"1.0.0"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "linux-profile-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic Linux host qualification evidence for executor separation")
	evidenceUpload, err := intake.PrepareUpload("executor-qualification.md", "text/markdown", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "linux-profile-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	toolchainDigest := strings.Repeat("d", 64)
	_, policyJSON, _, err := environment.BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(runtime, &recordingWorkerAdapter{})
	defer service.Close()
	windowsExecutor := &recordingEnvironmentPreparationExecutor{}
	service.SetProjectEnvironmentPreparationExecutor(windowsExecutor)
	revision, err := service.RegisterProjectEnvironment(ctx, companyID, RegisterProjectEnvironmentRequest{
		ProfileID: environment.LinuxNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: "1",
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: toolchainDigest, RequestID: "linux-profile-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideProjectEnvironmentPolicy(ctx, companyID, revision.RevisionID, EnvironmentPolicyDecisionRequest{
		Decision: "approved", Rationale: "approve only the offline Linux profile", RequestID: "linux-profile-approve",
	}); err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidenceInput, "linux-profile-qualification")
	run, err := service.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, EnsureEnvironmentRequest{RequestID: "linux-profile-ensure"})
	if err != nil || run.State != string(environment.PreparationBlockedUnqualified) || run.ReasonCode != "environment_executor_profile_unavailable" || windowsExecutor.calls != 0 {
		t.Fatalf("Linux profile used the Windows executor: run=%+v executorCalls=%d error=%v", run, windowsExecutor.calls, err)
	}
}
