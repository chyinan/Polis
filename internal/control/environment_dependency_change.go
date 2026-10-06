// pattern: Imperative Shell
package control

import (
	"context"
	"strings"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
)

func (s *Service) ProposeDependencyChange(ctx context.Context, companyID string, request ProposeDependencyChangeRequest) (kernel.DependencyChangeProposalRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || validateRequestID(request.RequestID) != nil {
		return kernel.DependencyChangeProposalRecord{}, core.Malformed
	}
	return s.runtime.TXProposeDependencyChange(ctx, s.runtime.LocalScope(companyID), environment.DependencyChangeProposal{
		MissionID: request.MissionID, BaseRevisionID: request.BaseRevisionID, RequestID: request.RequestID,
		RegistryHosts: request.RegistryHosts, Dependencies: request.Dependencies, Rationale: strings.TrimSpace(request.Rationale),
	}, request.RequestID)
}

func (s *Service) DecideDependencyChange(ctx context.Context, companyID string, request DecideDependencyChangeRequest) (kernel.DependencyChangeProposalRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(request.ProposalID) || validateRequestID(request.RequestID) != nil || (request.Decision != "approved" && request.Decision != "rejected") {
		return kernel.DependencyChangeProposalRecord{}, core.Malformed
	}
	return s.runtime.DecideDependencyChange(ctx, s.runtime.LocalScope(companyID), kernel.DependencyChangeDecisionInput{
		ProposalID: request.ProposalID, Decision: request.Decision, Rationale: strings.TrimSpace(request.Rationale), RequestID: request.RequestID,
	})
}

var _ DependencyChangeService = (*Service)(nil)
