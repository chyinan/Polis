// pattern: Functional Core
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestBuildReadyProductDeliveryManifestBindsAllEvidence(t *testing.T) {
	input := readyProductDeliveryManifestInput{
		CompanyID:          "company-1",
		MissionID:          "mission-1",
		TaskID:             "task-1",
		ArtifactID:         "artifact-1",
		Revision:           2,
		ArtifactBytes:      128,
		ArtifactSHA256:     stringsRepeatForTest('a', 64),
		VerificationDetail: "checkpoint=checkpoint-1; validation=check-1",
		CreatedAt:          time.Date(2026, 10, 6, 1, 2, 3, 4, time.UTC),
		Evidence: readyProductDeliveryEvidence{
			SourceInputs:     deliveryManifestEvidence{Reference: "task_input_manifests/task-1/1", Digest: stringsRepeatForTest('b', 64), Detail: "fixed input manifest"},
			EnvironmentBuild: deliveryManifestEvidence{Reference: "task_validation_bindings/task-1", Digest: stringsRepeatForTest('c', 64), Detail: "runner=product@1"},
			RunInstructions:  deliveryManifestEvidence{Reference: "task_plan/task-1", Digest: stringsRepeatForTest('d', 64), Detail: "go test ./..."},
			Limitations:      deliveryManifestEvidence{Reference: "polis.delivery.limitations@1", Digest: stringsRepeatForTest('e', 64), Detail: "local deterministic qualification only"},
			LicenseSource:    deliveryManifestEvidence{Reference: "polis.delivery.license-source@1", Digest: stringsRepeatForTest('f', 64), Detail: "source attribution is included in the evidence record"},
		},
	}

	manifest, raw, digest, err := buildReadyProductDeliveryManifest(input)
	if err != nil {
		t.Fatalf("buildReadyProductDeliveryManifest() error = %v", err)
	}
	if manifest.State != "ready" || manifest.Revision != "2" {
		t.Fatalf("manifest identity = state %q revision %q, want ready/2", manifest.State, manifest.Revision)
	}
	if len(manifest.Sections) != 8 {
		t.Fatalf("section count = %d, want 8", len(manifest.Sections))
	}
	for index, section := range manifest.Sections {
		if index < 7 && section.State != "available" {
			t.Fatalf("section %q state = %q, want available", section.Key, section.State)
		}
	}
	if manifest.Sections[7].State != "not_requested" {
		t.Fatalf("feedback state = %q, want not_requested", manifest.Sections[7].State)
	}
	for _, detail := range []string{
		input.Evidence.SourceInputs.Detail,
		input.Evidence.EnvironmentBuild.Detail,
		input.Evidence.RunInstructions.Detail,
		input.Evidence.Limitations.Detail,
		input.Evidence.LicenseSource.Detail,
	} {
		if !bytes.Contains(raw, []byte(detail)) {
			t.Fatalf("canonical manifest does not contain evidence detail %q", detail)
		}
	}
	checksum := sha256.Sum256(raw)
	if digest != hex.EncodeToString(checksum[:]) {
		t.Fatalf("digest = %q, want sha256(raw) = %q", digest, hex.EncodeToString(checksum[:]))
	}
}

func TestBuildReadyProductDeliveryManifestRejectsMissingEvidence(t *testing.T) {
	input := readyProductDeliveryManifestInput{
		CompanyID:          "company-1",
		MissionID:          "mission-1",
		TaskID:             "task-1",
		ArtifactID:         "artifact-1",
		Revision:           2,
		ArtifactBytes:      128,
		ArtifactSHA256:     stringsRepeatForTest('a', 64),
		VerificationDetail: "checkpoint=checkpoint-1; validation=check-1",
		CreatedAt:          time.Date(2026, 10, 6, 1, 2, 3, 4, time.UTC),
		Evidence: readyProductDeliveryEvidence{
			SourceInputs:     deliveryManifestEvidence{Reference: "task_input_manifests/task-1/1", Digest: stringsRepeatForTest('b', 64), Detail: "fixed input manifest"},
			EnvironmentBuild: deliveryManifestEvidence{Reference: "task_validation_bindings/task-1", Digest: stringsRepeatForTest('c', 64), Detail: "runner=product@1"},
			RunInstructions:  deliveryManifestEvidence{Reference: "task_plan/task-1", Digest: stringsRepeatForTest('d', 64), Detail: "go test ./..."},
			Limitations:      deliveryManifestEvidence{Reference: "polis.delivery.limitations@1", Digest: stringsRepeatForTest('e', 64), Detail: "local deterministic qualification only"},
			LicenseSource:    deliveryManifestEvidence{Reference: "polis.delivery.license-source@1", Digest: stringsRepeatForTest('f', 64)},
		},
	}

	if _, _, _, err := buildReadyProductDeliveryManifest(input); err == nil {
		t.Fatal("buildReadyProductDeliveryManifest() error = nil, want missing license/source evidence rejection")
	}
}

func stringsRepeatForTest(value byte, count int) string {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
