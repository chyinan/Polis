// pattern: Functional Core
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestInvalidateProductDeliveryManifestRejectsMalformedCommandBeforeDatabaseAccess(t *testing.T) {
	_, err := (*Kernel)(nil).InvalidateProductDeliveryManifest(context.Background(), "company-1", ProductDeliveryManifestInvalidationCommand{ArtifactID: "artifact-1", ExpectedManifestRevision: 0, State: "invalidated", Reason: "reason", RequestID: "request-1"})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("InvalidateProductDeliveryManifest() error=%v, want %v", err, core.Malformed)
	}
}

func TestValidateProductDeliveryManifestInvalidationCommand(t *testing.T) {
	valid := ProductDeliveryManifestInvalidationCommand{ArtifactID: "artifact-1", ExpectedManifestRevision: 2, State: "invalidated", Reason: "security review", RequestID: "delivery-invalidate-1"}
	if err := validateProductDeliveryManifestInvalidationCommand(valid); err != nil {
		t.Fatalf("valid invalidation command rejected: %v", err)
	}
	for name, command := range map[string]ProductDeliveryManifestInvalidationCommand{
		"zero revision":   {ArtifactID: "artifact-1", ExpectedManifestRevision: 0, State: "invalidated", Reason: "reason", RequestID: "request-1"},
		"invalid state":   {ArtifactID: "artifact-1", ExpectedManifestRevision: 2, State: "accepted", Reason: "reason", RequestID: "request-1"},
		"empty reason":    {ArtifactID: "artifact-1", ExpectedManifestRevision: 2, State: "withdrawn", Reason: " ", RequestID: "request-1"},
		"invalid request": {ArtifactID: "artifact-1", ExpectedManifestRevision: 2, State: "withdrawn", Reason: "reason", RequestID: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProductDeliveryManifestInvalidationCommand(command); !errors.Is(err, core.Malformed) {
				t.Fatalf("validation error=%v, want malformed", err)
			}
		})
	}
}
