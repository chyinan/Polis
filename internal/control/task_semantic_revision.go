// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) DecideTaskSemanticRevision(ctx context.Context, companyID string, request DecideTaskSemanticRevisionRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || !core.ValidID(companyID) || !core.ValidID(request.TaskID) || request.Revision < 1 || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXDecideTaskSemanticRevision(ctx, companyID, kernel.TaskSemanticRevisionDecisionInput{
		TaskID: request.TaskID, Revision: request.Revision, BindingSHA256: request.BindingSHA256,
		Decision: request.Decision, Rationale: request.Rationale, RequestID: request.RequestID,
	})
}
