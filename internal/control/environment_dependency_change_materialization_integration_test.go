// pattern: Imperative Shell
package control

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
	"polis/internal/kernel"
	"polis/internal/pgfixture"
)

type recordingDependencyChangeExecutor struct {
	calls            int
	lastProposal     environment.DependencyChangeProposal
	lastManifest     []byte
	returnError      error
	resolvedLockfile []byte
}

func (executor *recordingDependencyChangeExecutor) PrepareProjectEnvironment(context.Context, string, ProjectEnvironmentSnapshotLoader) (EnvironmentPreparationResult, error) {
	return EnvironmentPreparationResult{}, errors.New("preparation was not expected in the dependency materialization test")
}

func (executor *recordingDependencyChangeExecutor) ResolveProjectDependencyChange(_ context.Context, _ string, snapshot kernel.ProjectEnvironmentExecutionSnapshot, proposal environment.DependencyChangeProposal, packageJSON []byte) (DependencyChangeResolutionResult, error) {
	executor.calls++
	executor.lastProposal = proposal
	executor.lastManifest = append([]byte(nil), packageJSON...)
	if executor.returnError != nil {
		return DependencyChangeResolutionResult{}, executor.returnError
	}
	if len(executor.resolvedLockfile) == 0 {
		return DependencyChangeResolutionResult{}, errors.New("fake resolver fixture has no lockfile output")
	}
	if snapshot.Revision.RevisionID != proposal.BaseRevisionID {
		return DependencyChangeResolutionResult{}, core.OutOfScope
	}
	return DependencyChangeResolutionResult{Lockfile: append([]byte(nil), executor.resolvedLockfile...), Logs: []byte("fake resolver; no network")}, nil
}

func TestMaterializeDependencyChangeUsesQualifiedResolverAndPersistsSuccessor(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("dependency-materialize-control-%d", time.Now().UnixNano())
	scope, err := createPGTestCompany(runtime, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "dependency-materialize-control-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = createPGTestMission(runtime, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialize-control-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	_, policyJSON, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
	}, "dependency-materialize-control-base")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approved bounded base environment", RequestID: "dependency-materialize-control-policy"}); err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("disposable qualified executor fixture")
	evidenceUpload, err := intake.PrepareUpload("qualification.txt", "text/plain", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialize-control-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidence, "dependency-materialize-control-qualify")

	proposal := ProposeDependencyChangeRequest{
		MissionID: missionID, BaseRevisionID: revision.RevisionID, RegistryHosts: []string{"registry.npmjs.org"},
		Dependencies: map[string]string{"new-package": "3.0.0"}, Rationale: "approved controlled dependency", RequestID: "dependency-materialize-control-proposal",
	}
	record, err := runtimeService(runtime).ProposeDependencyChange(ctx, companyID, proposal)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := runtimeService(runtime).DecideDependencyChange(ctx, companyID, DecideDependencyChangeRequest{ProposalID: record.ProposalID, Decision: "approved", Rationale: "reviewed exact package and registry", RequestID: "dependency-materialize-control-approve"})
	if err != nil || decision.State != "approved" {
		t.Fatalf("approval=%+v err=%v", decision, err)
	}

	resolver := &recordingDependencyChangeExecutor{resolvedLockfile: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0","new-package":"3.0.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="},"node_modules/new-package":{"version":"3.0.0","resolved":"https://registry.npmjs.org/new-package/-/new-package-3.0.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)}
	service := NewService(runtime, &recordingWorkerAdapter{})
	defer service.Close()
	materializeRequest := MaterializeDependencyChangeRequest{ProposalID: record.ProposalID, RequestID: "dependency-materialize-control-run"}
	if _, err = service.MaterializeDependencyChange(ctx, companyID, materializeRequest); !errors.Is(err, core.Denied) {
		t.Fatalf("materialization without configured resolver error=%v, want denied", err)
	}
	if _, err = runtime.DependencyChangeMaterializationByRequest(ctx, scope, materializeRequest.RequestID); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("missing-resolver path created an attempt: %v", err)
	}
	service.SetProjectEnvironmentPreparationExecutor(resolver)
	materialized, err := service.MaterializeDependencyChange(ctx, companyID, materializeRequest)
	if err != nil || materialized.State != "materialized" || materialized.SuccessorRevisionID == "" || materialized.ResolverLogsSHA256 == "" || resolver.calls != 1 {
		t.Fatalf("materialization=%+v calls=%d err=%v", materialized, resolver.calls, err)
	}
	if resolver.lastProposal.BaseRevisionID != revision.RevisionID || resolver.lastProposal.Dependencies["new-package"] != "3.0.0" || resolver.lastManifest == nil {
		t.Fatalf("resolver did not receive approved proposal and exact manifest: proposal=%+v manifest=%s", resolver.lastProposal, resolver.lastManifest)
	}
	derivedRevision, err := runtime.GetProjectEnvironmentRevision(ctx, companyID, materialized.SuccessorRevisionID)
	if err != nil || derivedRevision.SourceInputID != materialized.DerivedInputID || derivedRevision.PackageJSONSHA256 == revision.PackageJSONSHA256 || derivedRevision.LockfileSHA256 == revision.LockfileSHA256 {
		t.Fatalf("successor revision=%+v err=%v", derivedRevision, err)
	}
	baseState, err := runtime.GetProjectEnvironmentRevision(ctx, companyID, revision.RevisionID)
	if err != nil || baseState.PackageJSONSHA256 != revision.PackageJSONSHA256 || baseState.LockfileSHA256 != revision.LockfileSHA256 {
		t.Fatalf("base revision was mutated: %+v err=%v", baseState, err)
	}
	replayed, err := service.MaterializeDependencyChange(ctx, companyID, materializeRequest)
	if err != nil || replayed.MaterializationID != materialized.MaterializationID || resolver.calls != 1 {
		t.Fatalf("same request replay=%+v calls=%d err=%v", replayed, resolver.calls, err)
	}
	proposals, err := runtime.ListDependencyChangeProposals(ctx, scope, missionID)
	if err != nil || len(proposals) != 1 || proposals[0].Materialization == nil || proposals[0].Materialization.State != "materialized" || proposals[0].Materialization.ResolverLogsSHA256 == "" {
		t.Fatalf("owner proposal projection=%+v err=%v", proposals, err)
	}
}

func TestMaterializeDependencyChangeDoesNotReplayResolverUnknownOutcome(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("dependency-materialize-unknown-%d", time.Now().UnixNano())
	scope, err := createPGTestCompany(runtime, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "dependency-materialize-unknown-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = createPGTestMission(runtime, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialize-unknown-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	_, policyJSON, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision, PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64)}, "dependency-materialize-unknown-base")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approved bounded base environment", RequestID: "dependency-materialize-unknown-policy"}); err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("disposable qualified executor fixture")
	evidenceUpload, err := intake.PrepareUpload("qualification.txt", "text/plain", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "dependency-materialize-unknown-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidence, "dependency-materialize-unknown-qualify")
	service := runtimeService(runtime)
	defer service.Close()
	proposal, err := service.ProposeDependencyChange(ctx, companyID, ProposeDependencyChangeRequest{MissionID: missionID, BaseRevisionID: revision.RevisionID, RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"new-package": "1.2.3"}, Rationale: "approved unknown test", RequestID: "dependency-materialize-unknown-proposal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideDependencyChange(ctx, companyID, DecideDependencyChangeRequest{ProposalID: proposal.ProposalID, Decision: "approved", Rationale: "approve for test", RequestID: "dependency-materialize-unknown-approve"}); err != nil {
		t.Fatal(err)
	}
	resolver := &recordingDependencyChangeExecutor{returnError: errors.New("simulated unknown resolver outcome")}
	service.SetProjectEnvironmentPreparationExecutor(resolver)
	request := MaterializeDependencyChangeRequest{ProposalID: proposal.ProposalID, RequestID: "dependency-materialize-unknown-run"}
	first, err := service.MaterializeDependencyChange(ctx, companyID, request)
	if err != nil || first.State != "outcome_unknown" || resolver.calls != 1 {
		t.Fatalf("first unknown result=%+v calls=%d err=%v", first, resolver.calls, err)
	}
	replayed, err := service.MaterializeDependencyChange(ctx, companyID, request)
	if err != nil || replayed.State != "outcome_unknown" || replayed.MaterializationID != first.MaterializationID || resolver.calls != 1 {
		t.Fatalf("unknown replay=%+v calls=%d err=%v", replayed, resolver.calls, err)
	}
	if _, err = service.MaterializeDependencyChange(ctx, companyID, MaterializeDependencyChangeRequest{ProposalID: proposal.ProposalID, RequestID: "dependency-materialize-unknown-new-run"}); !errors.Is(err, core.Conflict) {
		t.Fatalf("distinct request replayed unknown resolver outcome: %v", err)
	}
}

func runtimeService(runtime *kernel.Kernel) *Service {
	return NewService(runtime, &recordingWorkerAdapter{})
}
