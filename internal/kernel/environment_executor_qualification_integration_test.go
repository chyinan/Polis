// pattern: Imperative Shell
package kernel

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/intake"
)

func TestEnvironmentExecutorQualificationRequiresCurrentCASBoundEvidence(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("executor-qualification-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "executor-qualification-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"qualification-fixture","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"qualification-fixture","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"qualification-fixture","version":"1.0.0"}}}`)},
		{RelativePath: "repo/build.mjs", MediaType: "text/javascript", Content: []byte("process.exit(0)")},
	})
	if err != nil {
		t.Fatal(err)
	}
	projectInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "executor-qualification-project", directory.Upload, directory.Archive)
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
	toolchainSHA256 := strings.Repeat("e", 64)
	revision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: projectInput.InputID, SourceInputRevision: projectInput.Revision,
		PolicyManifest: policyManifest, ToolchainSHA256: toolchainSHA256,
	}, "executor-qualification-register")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{
		RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approve the fixed project policy", RequestID: "executor-qualification-policy",
	}); err != nil {
		t.Fatal(err)
	}
	blocked, err := runtime.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "executor-qualification-before")
	if err != nil || blocked.State != string(environment.PreparationBlockedUnqualified) {
		t.Fatalf("preparation without executor qualification = %+v error=%v", blocked, err)
	}

	evidence := []byte("host verification report: isolated process, policy and recovery checks reviewed")
	preparedEvidence, err := intake.PrepareUpload("windows-executor-report.md", "text/markdown", evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "executor-qualification-evidence", preparedEvidence, evidence)
	if err != nil {
		t.Fatal(err)
	}
	reportInput := qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidenceInput, "executor-qualification-approve")
	identity, ok := runtime.CurrentEnvironmentExecutorFingerprint(environment.WindowsNodeNPMProfile)
	if !ok {
		t.Fatal("current Windows executor identity was not captured")
	}
	var executorSHA256, hostSHA256, policySHA256, recordedToolchain, evidenceSHA256, recordedEvidenceID string
	var recordedEvidenceRevision int64
	if err = runtime.pool.QueryRow(ctx, `SELECT executor_fingerprint_sha256,host_fingerprint_sha256,isolation_policy_sha256,toolchain_sha256,evidence_sha256,evidence_input_id,evidence_input_revision
FROM environment_executor_qualification_events WHERE company_id=$1 AND profile_id=$2 AND decision='qualified'`, companyID, environment.WindowsNodeNPMProfile).Scan(
		&executorSHA256, &hostSHA256, &policySHA256, &recordedToolchain, &evidenceSHA256, &recordedEvidenceID, &recordedEvidenceRevision); err != nil {
		t.Fatal(err)
	}
	if executorSHA256 != identity.ExecutorSHA256 || hostSHA256 != identity.HostSHA256 || policySHA256 != identity.IsolationPolicySHA256 || recordedToolchain != toolchainSHA256 || evidenceSHA256 != reportInput.ContentDigest || recordedEvidenceID != reportInput.InputID || recordedEvidenceRevision != reportInput.Revision {
		t.Fatalf("executor qualification identity/evidence binding mismatch: executor=%q host=%q policy=%q toolchain=%q evidence=%q input=%q@%d", executorSHA256, hostSHA256, policySHA256, recordedToolchain, evidenceSHA256, recordedEvidenceID, recordedEvidenceRevision)
	}
	qualified, err := runtime.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "executor-qualification-after")
	if err != nil || qualified.State != string(environment.PreparationAccepted) {
		t.Fatalf("preparation with current executor qualification = %+v error=%v", qualified, err)
	}
	if _, err = runtime.TXRecordEnvironmentExecutorQualification(ctx, companyID, EnvironmentExecutorQualificationInput{
		RevisionID: revision.RevisionID, Decision: "revoked", EvidenceInputID: reportInput.InputID, EvidenceInputRevision: reportInput.Revision,
		Rationale: "owner revoked the executor qualification", RequestID: "executor-qualification-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	revoked, err := runtime.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "executor-qualification-after-revoke")
	if err != nil || revoked.State != string(environment.PreparationBlockedUnqualified) {
		t.Fatalf("preparation after executor qualification revocation = %+v error=%v", revoked, err)
	}
}
