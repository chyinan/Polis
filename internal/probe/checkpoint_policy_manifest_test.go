// pattern: Functional Core
package probe

import (
	"polis/internal/codex"
	"polis/internal/core"
	"testing"
)

func TestPolicyRevisionAndInteractionContractStaleQualification(t *testing.T) {
	current, _, _, err := buildT21ToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	if current.CheckpointPolicyRevision != core.CheckpointPolicyRevision || current.ArtifactEligibilityPolicyRevision != core.ArtifactEligibilityPolicyRevision || current.ContractSupersessionPolicyRevision != core.PeerContractSupersessionPolicyRevision {
		t.Fatal("semantic policy revisions absent")
	}
	if current.ToolCount != 11 || current.AggregateSchemaBytes <= 0 || current.AggregateManifestDigest == "" {
		t.Fatal("current public tool schema is invalid")
	}
	for _, field := range []string{"checkpoint", "artifact", "checker", "supersession"} {
		old := current
		switch field {
		case "checkpoint":
			old.CheckpointPolicyRevision = "previous"
		case "artifact":
			old.ArtifactEligibilityPolicyRevision = "previous"
		case "checker":
			old.AcceptanceCheckerRevision = "previous"
		case "supersession":
			old.ContractSupersessionPolicyRevision = "previous"
		}
		if sameR03AT2ToolSurface(current, old) {
			t.Fatalf("%s policy drift did not stale L2", field)
		}
		a := codex.CanonicalManifestV7{ToolSurface: current}
		b := a
		b.ToolSurface = old
		if a.Base.Combination.CurrentFingerprintV7(a) == b.Base.Combination.CurrentFingerprintV7(b) {
			t.Fatal("policy excluded from execution fingerprint")
		}
	}
}
