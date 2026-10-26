// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/pgfixture"
)

func TestDependencyChangeMaterializationJournalIsSingleUseAndLinksImmutableOutput(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("dependency-materialization-%d", time.Now().UnixNano())
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "dependency-materialization-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = createPGTestMission(k, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	baseFiles := []intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)},
	}
	directory, err := intake.PrepareDirectorySnapshot(baseFiles)
	if err != nil {
		t.Fatal(err)
	}
	source, err := k.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialization-base-input", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	_, policyJSON, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
	}, "dependency-materialization-base-revision")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{RevisionID: baseRevision.RevisionID, Decision: "approved", Rationale: "reviewed fixed registry and source", RequestID: "dependency-materialization-base-policy"}); err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic disposable resolver qualification fixture")
	evidencePrepared, err := intake.PrepareUpload("qualification.txt", "text/plain", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := k.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialization-qualification-input", evidencePrepared, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, k, companyID, baseRevision.RevisionID, evidence, "dependency-materialization-qualify")

	unknownProposal := dependencyChangeIntegrationProposal(missionID, baseRevision.RevisionID, "dependency-materialization-unknown-proposal", "2.0.0")
	unknownRecord, err := k.TXProposeDependencyChange(ctx, scope, unknownProposal, unknownProposal.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	unknownRecord, err = k.DecideDependencyChange(ctx, scope, DependencyChangeDecisionInput{ProposalID: unknownRecord.ProposalID, Decision: "approved", Rationale: "approve exact version", RequestID: "dependency-materialization-unknown-approve"})
	if err != nil || unknownRecord.State != "approved" {
		t.Fatalf("approve unknown-outcome fixture=%+v err=%v", unknownRecord, err)
	}
	first, started, err := k.TXBeginDependencyChangeMaterialization(ctx, scope, unknownRecord.ProposalID, "dependency-materialization-run-unknown")
	if err != nil || !started || first.State != "running" {
		t.Fatalf("begin attempt=%+v started=%t err=%v", first, started, err)
	}
	replayed, replayStarted, err := k.TXBeginDependencyChangeMaterialization(ctx, scope, unknownRecord.ProposalID, "dependency-materialization-run-unknown")
	if err != nil || replayStarted || replayed.MaterializationID != first.MaterializationID || replayed.State != "running" {
		t.Fatalf("same-request replay=%+v started=%t err=%v", replayed, replayStarted, err)
	}
	if _, _, err = k.TXBeginDependencyChangeMaterialization(ctx, scope, unknownRecord.ProposalID, "dependency-materialization-run-duplicate"); !errors.Is(err, core.Conflict) {
		t.Fatalf("concurrent distinct attempt error=%v, want conflict", err)
	}
	if _, err = k.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{RevisionID: baseRevision.RevisionID, Decision: "revoked", Rationale: "attempt to revoke during resolver lease", RequestID: "dependency-materialization-revoke-while-running"}); !errors.Is(err, core.Conflict) {
		t.Fatalf("base policy revoked while resolver was running: %v", err)
	}
	reconciled, err := k.TXReconcileUnrestoredDependencyChangeMaterializations(ctx, "dependency-materialization-startup-reconcile")
	if err != nil || reconciled != 1 {
		t.Fatalf("startup reconciliation count=%d err=%v", reconciled, err)
	}
	unknown, err := k.DependencyChangeMaterialization(ctx, scope, first.MaterializationID)
	if err != nil || unknown.State != "outcome_unknown" {
		t.Fatalf("reconciled attempt=%+v err=%v", unknown, err)
	}
	if _, _, err = k.TXBeginDependencyChangeMaterialization(ctx, scope, unknownRecord.ProposalID, "dependency-materialization-run-no-replay"); !errors.Is(err, core.Conflict) {
		t.Fatalf("unknown outcome was replayed: %v", err)
	}

	materializedProposal := dependencyChangeIntegrationProposal(missionID, baseRevision.RevisionID, "dependency-materialization-success-proposal", "3.0.0")
	materializedRecord, err := k.TXProposeDependencyChange(ctx, scope, materializedProposal, materializedProposal.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	materializedRecord, err = k.DecideDependencyChange(ctx, scope, DependencyChangeDecisionInput{ProposalID: materializedRecord.ProposalID, Decision: "approved", Rationale: "approve exact version", RequestID: "dependency-materialization-success-approve"})
	if err != nil || materializedRecord.State != "approved" {
		t.Fatalf("approve success fixture=%+v err=%v", materializedRecord, err)
	}
	attempt, started, err := k.TXBeginDependencyChangeMaterialization(ctx, scope, materializedRecord.ProposalID, "dependency-materialization-run-success")
	if err != nil || !started || attempt.State != "running" {
		t.Fatalf("begin success attempt=%+v started=%t err=%v", attempt, started, err)
	}
	baseSnapshot, err := k.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, baseRevision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err := environment.PrepareDependencyChangeProjectFiles(baseSnapshot.Files, baseSnapshot.Plan, materializedRecord.Proposal)
	if err != nil {
		t.Fatal(err)
	}
	lockfile := []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0","new-package":"3.0.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="},"node_modules/new-package":{"version":"3.0.0","resolved":"https://registry.npmjs.org/new-package/-/new-package-3.0.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)
	derivedFiles, derivedPlan, err := environment.ValidateDependencyChangeMaterialization(baseSnapshot.Files, baseSnapshot.Plan, materializedRecord.Proposal, manifest, lockfile, baseSnapshot.Policy.RegistryHosts)
	if err != nil {
		t.Fatal(err)
	}
	directoryFiles := make([]intake.DirectoryInputFile, len(derivedFiles))
	for index, file := range derivedFiles {
		directoryFiles[index] = intake.DirectoryInputFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: file.Content}
	}
	derivedDirectory, err := intake.PrepareDirectorySnapshot(directoryFiles)
	if err != nil {
		t.Fatal(err)
	}
	inputRequestID := "dependency-materialization-input-success"
	derivedInput, err := k.TXAddMissionInput(ctx, scope, missionID, "", inputRequestID, derivedDirectory.Upload, derivedDirectory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	successor, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: derivedInput.InputID, SourceInputRevision: derivedInput.Revision,
		PolicyManifest: json.RawMessage(baseRevision.PolicyManifest), ToolchainSHA256: baseRevision.ToolchainSHA256,
		PackageJSONSHA256: derivedPlan.PackageJSONSHA256, LockfileSHA256: derivedPlan.LockfileSHA256,
	}, "dependency-materialization-successor-register")
	if err != nil {
		t.Fatal(err)
	}
	completed, err := k.TXFinishDependencyChangeMaterialization(ctx, scope, DependencyChangeMaterializationFinish{
		MaterializationID: attempt.MaterializationID, State: "materialized", DerivedInputID: derivedInput.InputID,
		DerivedInputRevision: derivedInput.Revision, SuccessorRevisionID: successor.RevisionID,
		PackageJSONSHA256: derivedPlan.PackageJSONSHA256, LockfileSHA256: derivedPlan.LockfileSHA256,
	})
	if err != nil || completed.State != "materialized" || completed.SuccessorRevisionID != successor.RevisionID {
		t.Fatalf("complete materialization=%+v err=%v", completed, err)
	}
	if _, _, err = k.TXBeginDependencyChangeMaterialization(ctx, scope, materializedRecord.ProposalID, "dependency-materialization-run-after-success"); !errors.Is(err, core.Conflict) {
		t.Fatalf("materialized proposal was attempted again: %v", err)
	}
}

func dependencyChangeIntegrationProposal(missionID, baseRevisionID, requestID, version string) environment.DependencyChangeProposal {
	return environment.DependencyChangeProposal{
		MissionID: missionID, BaseRevisionID: baseRevisionID, RequestID: requestID,
		RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"new-package": version},
		Rationale: "disposable integration proposal",
	}
}
