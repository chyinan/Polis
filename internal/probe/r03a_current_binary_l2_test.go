// pattern: Imperative Shell
package probe

import (
	"strings"
	"testing"
)

func TestCurrentBinaryL2BindingIncludesL1RuntimeSurfaceAndPolicies(t *testing.T) {
	surface, _, _, err := buildT21ToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	binding := currentBinaryL2Binding{
		SchemaVersion:                      r03aCurrentBinaryL2BindingSchema,
		L1Fingerprint:                      strings.Repeat("a", 64),
		ControlledRuntimeManifestDigest:    strings.Repeat("b", 64),
		CodexVersion:                       "codex-cli 0.154.0-alpha.6.2",
		CodexBinarySHA256:                  strings.Repeat("c", 64),
		CodeModeHostSHA256:                 strings.Repeat("d", 64),
		ToolManifestDigest:                 surface.AggregateManifestDigest,
		AggregateSchemaDigest:              surface.AggregateSchemaDigest,
		AggregateSchemaBytes:               surface.AggregateSchemaBytes,
		ToolCount:                          surface.ToolCount,
		CheckpointPolicyRevision:           surface.CheckpointPolicyRevision,
		ArtifactEligibilityPolicyRevision:  surface.ArtifactEligibilityPolicyRevision,
		ContractSupersessionPolicyRevision: surface.ContractSupersessionPolicyRevision,
		AcceptanceCheckerRevision:          surface.AcceptanceCheckerRevision,
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("binding should validate: %v", err)
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprint) != 64 {
		t.Fatalf("unexpected fingerprint %q", fingerprint)
	}
}

func TestCurrentBinaryL2BindingRejectsMissingPolicyRevision(t *testing.T) {
	binding := currentBinaryL2Binding{
		SchemaVersion:                   r03aCurrentBinaryL2BindingSchema,
		L1Fingerprint:                   strings.Repeat("a", 64),
		ControlledRuntimeManifestDigest: strings.Repeat("b", 64),
		CodexVersion:                    "codex-cli 0.154.0-alpha.6.2",
		CodexBinarySHA256:               strings.Repeat("c", 64),
		CodeModeHostSHA256:              strings.Repeat("d", 64),
		ToolManifestDigest:              strings.Repeat("e", 64),
		AggregateSchemaDigest:           strings.Repeat("f", 64),
		AggregateSchemaBytes:            1,
		ToolCount:                       11,
	}
	if err := binding.Validate(); err == nil {
		t.Fatal("missing policy revision was accepted")
	}
}
