package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

type ProblemToolCallAllocationRequest struct {
	RequestID        string `json:"requestId"`
	AdditionalCalls  int64  `json:"additionalToolCalls"`
	ExpectedLimit    int64  `json:"expectedToolCallLimit"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Reason           string `json:"reason"`
	Confirm          bool   `json:"confirm"`
}

type ProblemToolBudgetService interface {
	ListProblemToolCallBudgets(ctx context.Context, companyID string) (kernel.ProblemToolCallBudgetList, error)
	AllocateProblemToolCalls(ctx context.Context, companyID, problemKey string, request ProblemToolCallAllocationRequest) (kernel.Receipt, error)
}

func (s *Service) ListProblemToolCallBudgets(ctx context.Context, companyID string) (kernel.ProblemToolCallBudgetList, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) {
		return kernel.ProblemToolCallBudgetList{}, core.Malformed
	}
	return s.runtime.ListProblemToolCallBudgets(ctx, s.runtime.LocalScope(companyID), 100)
}

func (s *Service) AllocateProblemToolCalls(ctx context.Context, companyID, problemKey string, request ProblemToolCallAllocationRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !kernel.ValidProblemKey(problemKey) ||
		validateRequestID(request.RequestID) != nil || request.AdditionalCalls < 1 || request.ExpectedLimit < 1 ||
		request.ExpectedRevision < 1 || !request.Confirm {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXAllocateProblemToolCalls(ctx, s.runtime.LocalScope(companyID), problemKey, kernel.ProblemToolCallAllocationInput{
		RequestID: request.RequestID, AdditionalCalls: request.AdditionalCalls,
		ExpectedLimit: request.ExpectedLimit, ExpectedRevision: request.ExpectedRevision, Reason: request.Reason,
	})
}

var _ ProblemToolBudgetService = (*Service)(nil)
