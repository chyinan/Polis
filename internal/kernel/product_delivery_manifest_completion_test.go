// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateProductDeliveryManifestCompletionCommandAcceptsBoundedEvidence(t *testing.T) {
	command := ProductDeliveryManifestCompletionCommand{
		ArtifactID:               "artifact-1",
		ExpectedManifestRevision: 1,
		RequestID:                "delivery-complete-1",
		Evidence: readyProductDeliveryEvidence{
			SourceInputs:     deliveryManifestEvidence{Reference: "task_input_manifests/task-1/1", Digest: stringsRepeatForTest('a', 64), Detail: "source"},
			EnvironmentBuild: deliveryManifestEvidence{Reference: "task_validation_bindings/task-1", Digest: stringsRepeatForTest('b', 64), Detail: "build"},
			RunInstructions:  deliveryManifestEvidence{Reference: "task_plan/task-1", Digest: stringsRepeatForTest('c', 64), Detail: "run"},
			Limitations:      deliveryManifestEvidence{Reference: "polis.delivery.limitations@1", Digest: completionEvidenceDigestForTest("limitations"), Detail: "limitations"},
			LicenseSource:    deliveryManifestEvidence{Reference: "polis.delivery.license-source@1", Digest: completionEvidenceDigestForTest("license/source"), Detail: "license/source"},
		},
	}

	if err := validateProductDeliveryManifestCompletionCommand(command); err != nil {
		t.Fatalf("validateProductDeliveryManifestCompletionCommand() error = %v", err)
	}
}

func completionEvidenceDigestForTest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestValidateProductDeliveryManifestCompletionCommandRejectsRevisionZero(t *testing.T) {
	command := ProductDeliveryManifestCompletionCommand{
		ArtifactID:               "artifact-1",
		ExpectedManifestRevision: 0,
		RequestID:                "delivery-complete-1",
	}

	if err := validateProductDeliveryManifestCompletionCommand(command); err == nil {
		t.Fatal("validateProductDeliveryManifestCompletionCommand() error = nil, want malformed revision")
	}
}
