// pattern: Imperative Shell
package control

import (
	"context"
	"time"

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

type RevokeMemoryRecordRequest struct {
	ExpectedRevision      int64  `json:"expectedRevision"`
	ExpectedContentSHA256 string `json:"expectedContentSha256"`
	ReasonCode            string `json:"reasonCode"`
	RequestID             string `json:"requestId"`
	Confirm               bool   `json:"confirm"`
}

type MemoryRevalidationService interface {
	GetMemoryTaskStatus(ctx context.Context, companyID, taskID string) (kernel.MemoryTaskStatus, error)
	GetMemoryTaskRevalidationPreview(ctx context.Context, companyID, taskID, dependencyID, correctionID string) (kernel.MemoryTaskRevalidationPreview, error)
	RevalidateMemoryTask(ctx context.Context, companyID, taskID string, request RevalidateMemoryTaskRequest) (kernel.Receipt, error)
}

type MemoryRevocationService interface {
	GetMemoryRecordRevocationPreview(ctx context.Context, companyID, recordID string) (kernel.MemoryRecordRevocationPreview, error)
	RevokeMemoryRecord(ctx context.Context, companyID, recordID string, request RevokeMemoryRecordRequest) (kernel.Receipt, error)
}

type MemoryCorrectionQueueService interface {
	ListMemoryCorrections(ctx context.Context, companyID string) (kernel.MemoryCorrectionQueue, error)
}

type ProposeMemoryCorrectionRequest struct {
	WorkerSessionID string                       `json:"workerSessionId"`
	CorrectionID    string                       `json:"correctionId"`
	RecordID        string                       `json:"recordId"`
	BaseRevision    int64                        `json:"baseRevision"`
	Content         string                       `json:"content"`
	ObservedAt      time.Time                    `json:"observedAt,omitempty"`
	Source          kernel.MemorySourceReference `json:"source"`
	Reason          string                       `json:"reason"`
	RequestID       string                       `json:"requestId"`
}

type ReviewMemoryCorrectionRequest struct {
	WorkerSessionID string `json:"workerSessionId"`
	Decision        string `json:"decision"`
	Reason          string `json:"reason"`
	RequestID       string `json:"requestId"`
}

type MemoryCorrectionCommandService interface {
	ProposeMemoryCorrection(ctx context.Context, companyID string, request ProposeMemoryCorrectionRequest) (kernel.Receipt, error)
	ReviewMemoryCorrection(ctx context.Context, companyID, correctionID string, request ReviewMemoryCorrectionRequest) (kernel.Receipt, error)
}

func (s *Service) ListMemoryCorrections(ctx context.Context, companyID string) (kernel.MemoryCorrectionQueue, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) {
		return kernel.MemoryCorrectionQueue{}, core.Malformed
	}
	return s.runtime.ListMemoryCorrections(ctx, s.runtime.LocalScope(companyID), 100)
}

func (s *Service) ProposeMemoryCorrection(ctx context.Context, companyID string, request ProposeMemoryCorrectionRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) ||
		!core.ValidID(request.WorkerSessionID) || !core.ValidID(request.CorrectionID) || !core.ValidID(request.RecordID) ||
		request.BaseRevision < 1 || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXProposeMemoryCorrectionForWorkerSession(ctx, s.runtime.LocalScope(companyID), request.WorkerSessionID, kernel.MemoryCorrectionInput{
		CorrectionID: request.CorrectionID, RecordID: request.RecordID, BaseRevision: request.BaseRevision,
		Content: request.Content, ObservedAt: request.ObservedAt, Source: request.Source, Reason: request.Reason,
	}, request.RequestID)
}

func (s *Service) ReviewMemoryCorrection(ctx context.Context, companyID, correctionID string, request ReviewMemoryCorrectionRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(correctionID) ||
		!core.ValidID(request.WorkerSessionID) || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXReviewMemoryCorrectionForWorkerSession(ctx, s.runtime.LocalScope(companyID), request.WorkerSessionID, kernel.MemoryCorrectionReviewInput{
		CorrectionID: correctionID, Decision: request.Decision, Reason: request.Reason,
	}, request.RequestID)
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

func (s *Service) RevokeMemoryRecord(ctx context.Context, companyID, recordID string, request RevokeMemoryRecordRequest) (kernel.Receipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(recordID) ||
		request.ExpectedRevision < 1 || len(request.ExpectedContentSHA256) != 64 || validateRequestID(request.RequestID) != nil || !request.Confirm ||
		(request.ReasonCode != "incorrect" && request.ReasonCode != "sensitive" && request.ReasonCode != "requested" && request.ReasonCode != "other") {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXRevokeMemoryRecord(ctx, s.runtime.LocalScope(companyID), kernel.MemoryRecordRevocationInput{
		RecordID: recordID, ExpectedRevision: request.ExpectedRevision,
		ExpectedContentSHA256: request.ExpectedContentSHA256, ReasonCode: request.ReasonCode,
	}, request.RequestID)
}

func (s *Service) GetMemoryRecordRevocationPreview(ctx context.Context, companyID, recordID string) (kernel.MemoryRecordRevocationPreview, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(recordID) {
		return kernel.MemoryRecordRevocationPreview{}, core.Malformed
	}
	return s.runtime.GetMemoryRecordRevocationPreview(ctx, s.runtime.LocalScope(companyID), recordID)
}

var _ MemoryRevalidationService = (*Service)(nil)
var _ MemoryRevocationService = (*Service)(nil)
var _ MemoryCorrectionQueueService = (*Service)(nil)
var _ MemoryCorrectionCommandService = (*Service)(nil)
