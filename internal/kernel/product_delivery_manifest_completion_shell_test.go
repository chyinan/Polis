// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestCompleteProductDeliveryManifestRejectsMalformedCommandBeforeDatabaseAccess(t *testing.T) {
	_, err := (*Kernel)(nil).CompleteProductDeliveryManifest(context.Background(), "company-1", ProductDeliveryManifestCompletionCommand{
		ArtifactID:               "artifact-1",
		ExpectedManifestRevision: 0,
		RequestID:                "delivery-complete-1",
	})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("CompleteProductDeliveryManifest() error = %v, want %v", err, core.Malformed)
	}
}
