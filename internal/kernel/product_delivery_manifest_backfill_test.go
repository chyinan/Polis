// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestBackfillProductDeliveryManifestsRejectsMalformedManagementRequest(t *testing.T) {
	_, err := (*Kernel)(nil).TXBackfillProductDeliveryManifests(context.Background(), Scope{}, "")
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("TXBackfillProductDeliveryManifests() error = %v, want %v", err, core.Malformed)
	}
}
