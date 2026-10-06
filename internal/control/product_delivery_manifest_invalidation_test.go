// pattern: Functional Core
package control

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestInvalidateDurableDeliveryManifestRejectsMalformedRequestBeforeRuntime(t *testing.T) {
	service := &Service{}
	_, err := service.InvalidateDurableDeliveryManifest(context.Background(), "company-1", InvalidateDurableDeliveryManifestRequest{
		ArtifactID: "artifact-1", ExpectedManifestRevision: "0", State: "invalidated", Reason: "reason", RequestID: "request-1",
	})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("invalidation error=%v, want %v", err, core.Malformed)
	}
}
