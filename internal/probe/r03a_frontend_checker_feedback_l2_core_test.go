// pattern: Functional Core
package probe

import (
	"testing"

	"polis/internal/codex"
)

func TestFrontendRegistrationDiffIgnoresPolicyOnlyChange(t *testing.T) {
	old := codex.ToolSurfaceManifest{
		SurfaceID: "peer_frontend", ToolCount: 1, AggregateManifestDigest: "old-manifest", AggregateSchemaDigest: "old-schema", AggregateSchemaBytes: 10,
		ThreadStartPayloadDigest: "old-payload", ThreadStartPayloadBytes: 20, BusinessWritePolicy: "diagnostic_denied",
		CheckpointPolicyRevision: "checkpoint-effective-contract@2", ArtifactEligibilityPolicyRevision: "artifact-commit-qualification@2", ContractSupersessionPolicyRevision: "peer-contract-supersession@2", AcceptanceCheckerRevision: "peer-semantic-checker@2",
		Tools: []codex.ToolSurfaceEntry{{Name: "polis_workspace_check", SchemaDigest: "schema", SchemaBytes: 10, DescriptionDigest: "description", BindingIdentity: "binding", AuthorizationClass: "peer_frontend", RegistrationOrdinal: 1}},
	}
	current := old
	current.Tools = append([]codex.ToolSurfaceEntry(nil), old.Tools...)
	current.CheckpointPolicyRevision = "checkpoint-effective-contract@2"
	current.ArtifactEligibilityPolicyRevision = "artifact-commit-qualification@2"
	current.ContractSupersessionPolicyRevision = "peer-contract-supersession@2"
	current.AcceptanceCheckerRevision = "peer-semantic-checker@3"
	if diff := compareFrontendRegistrationSurface(old, current); !diff.Equivalent || len(diff.Changes) != 0 {
		t.Fatalf("policy-only change was treated as registration drift: %+v", diff)
	}
	policy := compareFrontendPolicyRevisions(old, current)
	if !policy.OnlyCheckerRevisionChanged || len(policy.Changes) != 1 || policy.Changes[0].Field != "acceptance_checker_revision" {
		t.Fatalf("checker policy revision was not isolated: %+v", policy)
	}
}

func TestFrontendRegistrationDiffDetectsSchemaOrBindingDrift(t *testing.T) {
	old := codex.ToolSurfaceManifest{SurfaceID: "peer_frontend", ToolCount: 1, AggregateManifestDigest: "old-manifest", AggregateSchemaDigest: "old-schema", AggregateSchemaBytes: 10, ThreadStartPayloadDigest: "old-payload", ThreadStartPayloadBytes: 20, BusinessWritePolicy: "diagnostic_denied", Tools: []codex.ToolSurfaceEntry{{Name: "polis_workspace_check", SchemaDigest: "schema", SchemaBytes: 10, DescriptionDigest: "description", BindingIdentity: "binding", AuthorizationClass: "peer_frontend", RegistrationOrdinal: 1}}}
	current := old
	current.Tools = append([]codex.ToolSurfaceEntry(nil), old.Tools...)
	current.Tools[0].SchemaDigest = "new-schema"
	current.Tools[0].BindingIdentity = "new-binding"
	diff := compareFrontendRegistrationSurface(old, current)
	if diff.Equivalent || len(diff.Changes) != 2 {
		t.Fatalf("schema/binding drift was not detected: %+v", diff)
	}
}

func TestFrontendCheckerFeedbackRegressionPasses(t *testing.T) {
	if cases := frontendCheckerFeedbackRegression(); !checkerFeedbackRegressionPassed(cases) {
		t.Fatalf("checker feedback regression failed: %+v", cases)
	}
}
