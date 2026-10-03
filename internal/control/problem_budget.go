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

type ProblemToolCallClosingReserveRequest struct {
	RequestID               string `json:"requestId"`
	ReservedToolCalls       int64  `json:"reservedToolCalls"`
	ExpectedToolCallLimit   int64  `json:"expectedToolCallLimit"`
	ExpectedBudgetRevision  int64  `json:"expectedBudgetRevision"`
	ExpectedReserveRevision int64  `json:"expectedReserveRevision"`
	Reason                  string `json:"reason"`
	Confirm                 bool   `json:"confirm"`
}

type TaskToolCallAllocationRequest struct {
	RequestID               string `json:"requestId"`
	AdditionalCalls         int64  `json:"additionalToolCalls"`
	ExpectedLimit           int64  `json:"expectedToolCallLimit"`
	ExpectedTaskRevision    int64  `json:"expectedTaskRevision"`
	ExpectedProblemRevision int64  `json:"expectedProblemRevision"`
	ExpectedReserveRevision int64  `json:"expectedReserveRevision"`
	Reason                  string `json:"reason"`
	Confirm                 bool   `json:"confirm"`
}

type TaskToolCallIncompleteClosureRequest struct {
	RequestID                string `json:"requestId"`
	ExpectedTaskLimit        *int64 `json:"expectedTaskToolCallLimit"`
	ExpectedTaskUsed         int64  `json:"expectedTaskToolCallsUsed"`
	ExpectedTaskRevision     int64  `json:"expectedTaskRevision"`
	ExpectedProblemLimit     *int64 `json:"expectedProblemToolCallLimit"`
	ExpectedProblemUsed      int64  `json:"expectedProblemToolCallsUsed"`
	ExpectedProblemRevision  int64  `json:"expectedProblemRevision"`
	ExpectedReserveCalls     int64  `json:"expectedClosingReserveToolCalls"`
	ExpectedReserveRemaining int64  `json:"expectedClosingReserveRemaining"`
	ExpectedReserveRevision  int64  `json:"expectedReserveRevision"`
	Reason                   string `json:"reason"`
	Confirm                  bool   `json:"confirm"`
}

type ProblemToolBudgetService interface {
	ListProblemToolCallBudgets(ctx context.Context, companyID string) (kernel.ProblemToolCallBudgetList, error)
	AllocateProblemToolCalls(ctx context.Context, companyID, problemKey string, request ProblemToolCallAllocationRequest) (kernel.Receipt, error)
	AllocateTaskToolCalls(ctx context.Context, companyID, problemKey, taskID string, request TaskToolCallAllocationRequest) (kernel.Receipt, error)
	CloseTaskToolBudgetIncomplete(ctx context.Context, companyID, problemKey, taskID string, request TaskToolCallIncompleteClosureRequest) (kernel.Receipt, error)
	SetProblemToolCallClosingReserve(ctx context.Context, companyID, problemKey string, request ProblemToolCallClosingReserveRequest) (kernel.Receipt, error)
}

func (s *Service) SetProblemToolCallClosingReserve(ctx context.Context, companyID, problemKey string, request ProblemToolCallClosingReserveRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !kernel.ValidProblemKey(problemKey) ||
		validateRequestID(request.RequestID) != nil || request.ReservedToolCalls < 0 || request.ExpectedToolCallLimit < 0 ||
		request.ExpectedBudgetRevision < 1 || request.ExpectedReserveRevision < 0 || !request.Confirm {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXSetProblemToolCallClosingReserve(ctx, s.runtime.LocalScope(companyID), problemKey, kernel.ProblemToolCallClosingReserveInput{
		RequestID: request.RequestID, ReservedCalls: request.ReservedToolCalls,
		ExpectedLimit: request.ExpectedToolCallLimit, ExpectedBudgetRevision: request.ExpectedBudgetRevision,
		ExpectedReserveRevision: request.ExpectedReserveRevision, Reason: request.Reason,
	})
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

func (s *Service) AllocateTaskToolCalls(ctx context.Context, companyID, problemKey, taskID string, request TaskToolCallAllocationRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !kernel.ValidProblemKey(problemKey) || !core.ValidID(taskID) ||
		validateRequestID(request.RequestID) != nil || request.AdditionalCalls < 1 || request.ExpectedLimit < 1 ||
		request.ExpectedTaskRevision < 1 || request.ExpectedProblemRevision < 1 || request.ExpectedReserveRevision < 0 || !request.Confirm {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXAllocateTaskToolCalls(ctx, s.runtime.LocalScope(companyID), problemKey, taskID, kernel.TaskToolCallAllocationInput{
		RequestID: request.RequestID, AdditionalCalls: request.AdditionalCalls, ExpectedLimit: request.ExpectedLimit,
		ExpectedTaskRevision: request.ExpectedTaskRevision, ExpectedProblemRevision: request.ExpectedProblemRevision,
		ExpectedReserveRevision: request.ExpectedReserveRevision, Reason: request.Reason,
	})
}

func (s *Service) CloseTaskToolBudgetIncomplete(ctx context.Context, companyID, problemKey, taskID string, request TaskToolCallIncompleteClosureRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !kernel.ValidProblemKey(problemKey) || !core.ValidID(taskID) ||
		validateRequestID(request.RequestID) != nil || request.ExpectedTaskUsed < 0 || request.ExpectedTaskRevision < 1 ||
		request.ExpectedProblemUsed < 0 || request.ExpectedProblemRevision < 1 || request.ExpectedReserveCalls < 0 ||
		request.ExpectedReserveRemaining < 0 || request.ExpectedReserveRemaining > request.ExpectedReserveCalls || request.ExpectedReserveRevision < 0 || !request.Confirm ||
		(request.ExpectedTaskLimit != nil && *request.ExpectedTaskLimit < 0) || (request.ExpectedProblemLimit != nil && *request.ExpectedProblemLimit < 0) {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXCloseTaskToolBudgetIncomplete(ctx, s.runtime.LocalScope(companyID), problemKey, taskID, kernel.TaskToolCallIncompleteClosureInput{
		RequestID: request.RequestID, ExpectedTaskLimit: request.ExpectedTaskLimit, ExpectedTaskUsed: request.ExpectedTaskUsed,
		ExpectedTaskRevision: request.ExpectedTaskRevision, ExpectedProblemLimit: request.ExpectedProblemLimit,
		ExpectedProblemUsed: request.ExpectedProblemUsed, ExpectedProblemRevision: request.ExpectedProblemRevision,
		ExpectedReserveCalls: request.ExpectedReserveCalls, ExpectedReserveRemaining: request.ExpectedReserveRemaining,
		ExpectedReserveRevision: request.ExpectedReserveRevision, Reason: request.Reason,
	})
}

var _ ProblemToolBudgetService = (*Service)(nil)
