// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
	"polis/internal/environment"
)

func TestDependencyChangeCommandsRejectMalformedInputBeforeDatabaseAccess(t *testing.T) {
	proposal := environment.DependencyChangeProposal{MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"left-pad": "1.3.0"}, Rationale: "reason"}
	if _, err := (*Kernel)(nil).TXProposeDependencyChange(context.Background(), Scope{company: "company-1"}, proposal, "request-1"); !errors.Is(err, core.StaleEpoch) && !errors.Is(err, core.Malformed) {
		t.Fatalf("nil-kernel proposal error=%v, want an early validation error", err)
	}
	if _, err := (*Kernel)(nil).DecideDependencyChange(context.Background(), Scope{company: "company-1"}, DependencyChangeDecisionInput{ProposalID: "proposal-1", Decision: "approved", Rationale: "approved", RequestID: "request-2"}); !errors.Is(err, core.StaleEpoch) && !errors.Is(err, core.Malformed) {
		t.Fatalf("nil-kernel decision error=%v, want an early validation error", err)
	}
}
