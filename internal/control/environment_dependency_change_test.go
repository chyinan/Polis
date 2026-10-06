// pattern: Functional Core
package control

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestDependencyChangeServiceRejectsMalformedRequestBeforeRuntime(t *testing.T) {
	service := &Service{}
	_, err := service.ProposeDependencyChange(context.Background(), "company-1", ProposeDependencyChangeRequest{RequestID: "request-1"})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("proposal error=%v, want malformed", err)
	}
	_, err = service.DecideDependencyChange(context.Background(), "company-1", DecideDependencyChangeRequest{ProposalID: "proposal-1", Decision: "approved", RequestID: "request-2"})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("decision error=%v, want malformed", err)
	}
}
