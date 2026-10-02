// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

type RevalidateMemoryTaskRequest struct {
	DependencyID  string `json:"dependencyId"`
	CorrectionID  string `json:"correctionId"`
	ContextSHA256 string `json:"contextSha256"`
	Reason        string `json:"reason"`
	RequestID     string `json:"requestId"`
}

type MemoryRevalidationService interface {
	GetMemoryTaskStatus(ctx context.Context, companyID, taskID string) (kernel.MemoryTaskStatus, error)
	GetMemoryTaskRevalidationPreview(ctx context.Context, companyID, taskID, dependencyID, correctionID string) (kernel.MemoryTaskRevalidationPreview, error)
	RevalidateMemoryTask(ctx context.Context, companyID, taskID string, request RevalidateMemoryTaskRequest) (kernel.Receipt, error)
}

func (s *Service) GetMemoryTaskStatus(ctx context.Context, companyID, taskID string) (kernel.MemoryTaskStatus, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(taskID) {
		return kernel.MemoryTaskStatus{}, core.Malformed
	}
	return s.runtime.GetMemoryTaskStatus(ctx, s.runtime.LocalScope(companyID), taskID)
}

func (s *Service) GetMemoryTaskRevalidationPreview(ctx context.Context, companyID, taskID, dependencyID, correctionID string) (kernel.MemoryTaskRevalidationPreview, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(taskID) ||
		!core.ValidID(dependencyID) || !core.ValidID(correctionID) {
		return kernel.MemoryTaskRevalidationPreview{}, core.Malformed
	}
	return s.runtime.GetMemoryTaskRevalidationPreview(ctx, s.runtime.LocalScope(companyID), taskID, dependencyID, correctionID)
}

func (s *Service) RevalidateMemoryTask(ctx context.Context, companyID, taskID string, request RevalidateMemoryTaskRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(taskID) ||
		!core.ValidID(request.DependencyID) || !core.ValidID(request.CorrectionID) ||
		!core.ValidID(request.ContextSHA256) || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXRevalidateMemoryTask(ctx, s.runtime.LocalScope(companyID), kernel.MemoryTaskRevalidationInput{
		TaskID: taskID, DependencyID: request.DependencyID, CorrectionID: request.CorrectionID,
		ContextSHA256: request.ContextSHA256, Reason: request.Reason,
	}, request.RequestID)
}

var _ MemoryRevalidationService = (*Service)(nil)
