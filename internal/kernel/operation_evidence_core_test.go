// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateOperationEvidenceManifestBindsScopeAndArtifactReferences(t *testing.T) {
	manifest := OperationEvidenceManifest{
		SchemaVersion: OperationEvidenceManifestSchema, CompanyID: "company-1", MissionID: "mission-1", TaskID: "task-1", OperationID: "run-1",
		Artifacts: []OperationEvidenceArtifactRef{{ArtifactID: "artifact-1", Digest: strings.Repeat("a", 64), Kind: "screenshot"}},
	}
	if err := ValidateOperationEvidenceManifest(manifest, "company-1", "mission-1", "task-1", "run-1"); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*OperationEvidenceManifest){
		"cross company": func(value *OperationEvidenceManifest) { value.CompanyID = "company-2" },
		"bad kind":      func(value *OperationEvidenceManifest) { value.Artifacts[0].Kind = "cookie" },
		"duplicate":     func(value *OperationEvidenceManifest) { value.Artifacts = append(value.Artifacts, value.Artifacts[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := manifest
			candidate.Artifacts = append([]OperationEvidenceArtifactRef(nil), manifest.Artifacts...)
			mutate(&candidate)
			if err := ValidateOperationEvidenceManifest(candidate, "company-1", "mission-1", "task-1", "run-1"); err == nil {
				t.Fatal("invalid evidence manifest was accepted")
			}
		})
	}
}

func TestCanonicalOperationEvidenceJSONNormalizesPresentationBeforeHashing(t *testing.T) {
	first, err := CanonicalOperationEvidenceJSON([]byte(`{"b":2,"a":[1,true]}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalOperationEvidenceJSON([]byte("{\n  \"a\": [1, true], \"b\": 2\n}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("canonical evidence differs: %s vs %s", first, second)
	}
}

func TestValidateResearchOperationEvidenceEnvelopeBindsManifestDigestAndScope(t *testing.T) {
	manifest := OperationEvidenceManifest{
		SchemaVersion: OperationEvidenceManifestSchema, CompanyID: "company-1", MissionID: "mission-1", TaskID: "task-1", OperationID: "operation-1",
		Artifacts: []OperationEvidenceArtifactRef{{ArtifactID: "artifact-1", Digest: strings.Repeat("a", 64), Kind: "search_result"}},
	}
	manifestBytes, err := CanonicalOperationEvidenceJSON(mustMarshalOperationEvidenceManifest(manifest))
	if err != nil {
		t.Fatal(err)
	}
	envelopeBytes, err := json.Marshal(ResearchOperationEvidenceEnvelope{EvidenceManifestSHA256: digestCapabilityBytes(manifestBytes), EvidenceManifest: manifest, Payload: json.RawMessage(`{"title":"result"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateResearchOperationEvidenceEnvelope(envelopeBytes, "company-1", "mission-1", "task-1", "operation-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateResearchOperationEvidenceEnvelope(envelopeBytes, "company-1", "mission-1", "task-1", "other-operation"); err == nil {
		t.Fatal("cross-operation evidence envelope was accepted")
	}
}
