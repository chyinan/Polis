// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/intake"
)

func qualifyTestEnvironmentExecutor(t *testing.T, ctx context.Context, runtime *Kernel, companyID, revisionID string, evidenceInput MissionInputRevision, requestID string) MissionInputRevision {
	t.Helper()
	revision, err := runtime.GetProjectEnvironmentRevision(ctx, companyID, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, ok := runtime.CurrentEnvironmentExecutorFingerprint(revision.ProfileID)
	if !ok {
		t.Fatalf("current executor fingerprint is unavailable for %s", revision.ProfileID)
	}
	evidence := environment.EnvironmentExecutorQualificationEvidence{
		SchemaVersion: environment.EnvironmentExecutorQualificationEvidenceSchema, ProfileID: revision.ProfileID,
		ExecutorFingerprintSHA256: fingerprint.ExecutorSHA256, HostFingerprintSHA256: fingerprint.HostSHA256,
		IsolationPolicySHA256: fingerprint.IsolationPolicySHA256, ToolchainSHA256: revision.ToolchainSHA256,
	}
	for _, checkID := range environment.RequiredEnvironmentExecutorQualificationChecks(revision.ProfileID) {
		evidence.Checks = append(evidence.Checks, environment.EnvironmentExecutorQualificationCheck{
			ID: checkID, Status: "passed", EvidenceInputID: evidenceInput.InputID,
			EvidenceInputRevision: evidenceInput.Revision, EvidenceSHA256: evidenceInput.ContentDigest,
		})
	}
	evidenceBytes, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	preparedReport, err := intake.PrepareUpload("executor-qualification.json", "application/json", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	reportInput, err := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), evidenceInput.MissionID, "", fmt.Sprintf("executor-report-%d", time.Now().UnixNano()), preparedReport, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentExecutorQualification(ctx, companyID, EnvironmentExecutorQualificationInput{
		RevisionID: revisionID, Decision: "qualified", EvidenceInputID: reportInput.InputID, EvidenceInputRevision: reportInput.Revision,
		Rationale: "test owner reviewed synthetic host qualification evidence", RequestID: requestID,
	}); err != nil {
		t.Fatal(err)
	}
	return reportInput
}
