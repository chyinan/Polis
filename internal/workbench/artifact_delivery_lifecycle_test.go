// pattern: Functional Core
package workbench

import (
	"bytes"
	"testing"
)

func TestCanonicalDurableDeliveryManifestDoesNotHTMLEscapeEvidence(t *testing.T) {
	manifest := DurableDeliveryManifestView{
		SchemaVersion: DurableDeliveryManifestSchema,
		DeliveryID:    "artifact-1",
		Revision:      "2",
		CompanyID:     "company-1",
		MissionID:     "mission-1",
		TaskID:        "task-1",
		ArtifactID:    "artifact-1",
		State:         "ready",
		Artifact:      DurableDeliveryManifestArtifact{FileName: "artifact.bin", ByteSize: "1", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Sections: []DurableDeliveryManifestSection{
			{Key: "source_inputs", State: "available", Detail: "<source>&"},
			{Key: "environment_build", State: "available", Detail: "build"},
			{Key: "file_inventory", State: "available", Detail: "file"},
			{Key: "run_instructions", State: "available", Detail: "run"},
			{Key: "verification", State: "available", Detail: "verify"},
			{Key: "limitations", State: "available", Detail: "limit"},
			{Key: "license_source", State: "available", Detail: "license"},
			{Key: "feedback", State: "not_requested", Detail: "feedback"},
		},
		CreatedAt: "2026-10-06T00:00:00Z",
	}

	encoded, _, err := canonicalDurableDeliveryManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte("<source>&")) {
		t.Fatalf("canonical manifest HTML-escaped evidence detail: %s", encoded)
	}
}
