// pattern: Functional Core
package workbench

import (
	"bytes"
	"strings"
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

func TestValidateDurableDeliveryFeedbackBacklogEventBindsDeliveryScope(t *testing.T) {
	event := DurableDeliveryFeedbackBacklogEventView{
		EventID: "event-1", DeliveryID: "artifact-1", ManifestRevision: "2", DispositionRevision: "1",
		MissionID: "mission-1", TaskID: "task-1", ArtifactID: "artifact-1", Status: "open", Actor: "system",
		Reason: "please revise", RequestID: "request-1", CreatedAt: "2026-10-06T00:00:00Z",
	}
	if !validateDurableDeliveryFeedbackBacklogEvent(event, "artifact-1", "mission-1", "task-1", "2") {
		t.Fatal("valid scoped feedback backlog event was rejected")
	}
	event.TaskID = "task-other"
	if validateDurableDeliveryFeedbackBacklogEvent(event, "artifact-1", "mission-1", "task-1", "2") {
		t.Fatal("cross-task feedback backlog event was accepted")
	}
}

func TestCanonicalDurableDeliveryManifestAllowsIncompleteAssemblingEvidence(t *testing.T) {
	sections := []DurableDeliveryManifestSection{
		{Key: "source_inputs", State: "unavailable", Detail: "not captured"},
		{Key: "environment_build", State: "unavailable", Detail: "not captured"},
		{Key: "file_inventory", State: "available", Detail: "artifact"},
		{Key: "run_instructions", State: "unavailable", Detail: "not captured"},
		{Key: "verification", State: "unavailable", Detail: "not inferred"},
		{Key: "limitations", State: "unavailable", Detail: "not reviewed"},
		{Key: "license_source", State: "unavailable", Detail: "not captured"},
		{Key: "feedback", State: "not_requested", Detail: "not requested"},
	}
	_, _, err := canonicalDurableDeliveryManifest(DurableDeliveryManifestView{
		SchemaVersion: DurableDeliveryManifestSchema, DeliveryID: "artifact-1", Revision: "1", CompanyID: "company-1", MissionID: "mission-1", TaskID: "task-1", ArtifactID: "artifact-1", State: "assembling",
		Artifact: DurableDeliveryManifestArtifact{FileName: "artifact.bin", ByteSize: "1", SHA256: "a" + strings.Repeat("a", 63)}, Sections: sections, CreatedAt: "2026-10-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("assembling manifest with unavailable evidence was rejected: %v", err)
	}
}
