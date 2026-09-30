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

func TestEnvironmentLifecycleControlCommandsPersistGatedStatus(t *testing.T) {
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
	companyID := fmt.Sprintf("environment-control-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "environment-control-mission-" + fmt.Sprint(time.Now().UnixNano())
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
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "environment-control-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
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
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
		RequestID: "environment-control-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if revision.SourceInputID != source.InputID || revision.SourceRevisionSHA256 != source.ContentDigest {
		t.Fatalf("control registration did not bind the authoritative input: %+v", revision)
	}
	if _, err = service.DecideProjectEnvironmentPolicy(ctx, companyID, revision.RevisionID, EnvironmentPolicyDecisionRequest{
		Decision: "approved", Rationale: "reviewed explicit source and lockfile digest", RequestID: "environment-control-approve",
	}); err != nil {
		t.Fatal(err)
	}
	run, err := service.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, EnsureEnvironmentRequest{RequestID: "environment-control-ensure"})
	if err != nil {
		t.Fatal(err)
	}
	if run.State != string(environment.PreparationBlockedUnqualified) || run.ReasonCode != "environment_executor_not_qualified" {
		t.Fatalf("environment control ensure returned an unsafe state: %+v", run)
	}
	if executor.calls != 0 {
		t.Fatalf("unqualified environment reached the executor %d times", executor.calls)
	}
}
