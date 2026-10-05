package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

type InstallationOwnerService interface {
	ListObservedProviderAccounts(ctx context.Context) (kernel.ObservedProviderAccountList, error)
}

type InstallationWorkerSlotService interface {
	GetInstallationWorkerSlotPolicy(ctx context.Context) (kernel.InstallationWorkerSlotPolicy, error)
	SetInstallationWorkerSlotPolicy(ctx context.Context, request InstallationWorkerSlotPolicyRequest) (kernel.InstallationWorkerSlotPolicy, error)
}

type InstallationWorkerSlotPolicyRequest struct {
	RequestID        string `json:"requestId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	MaxActiveSlots   int64  `json:"maxActiveSlots"`
	ProtectedSlots   int64  `json:"protectedSlots"`
}

func (s *Service) ListObservedProviderAccounts(ctx context.Context) (kernel.ObservedProviderAccountList, error) {
	if s == nil || s.runtime == nil || ctx == nil {
		return kernel.ObservedProviderAccountList{}, core.Malformed
	}
	return s.runtime.ListObservedProviderAccounts(ctx, s.runtime.LocalInstallationOwnerScope(), 100)
}

func (s *Service) GetInstallationWorkerSlotPolicy(ctx context.Context) (kernel.InstallationWorkerSlotPolicy, error) {
	if s == nil || s.runtime == nil || ctx == nil {
		return kernel.InstallationWorkerSlotPolicy{}, core.Malformed
	}
	return s.runtime.GetInstallationWorkerSlotPolicy(ctx, s.runtime.LocalInstallationOwnerScope())
}

func (s *Service) SetInstallationWorkerSlotPolicy(ctx context.Context, request InstallationWorkerSlotPolicyRequest) (kernel.InstallationWorkerSlotPolicy, error) {
	if s == nil || s.runtime == nil || ctx == nil {
		return kernel.InstallationWorkerSlotPolicy{}, core.Malformed
	}
	return s.runtime.TXSetInstallationWorkerSlotPolicy(ctx, s.runtime.LocalInstallationOwnerScope(), kernel.InstallationWorkerSlotPolicyInput{
		RequestID:        request.RequestID,
		ExpectedRevision: request.ExpectedRevision,
		MaxActiveSlots:   request.MaxActiveSlots,
		ProtectedSlots:   request.ProtectedSlots,
	})
}

var _ InstallationOwnerService = (*Service)(nil)
var _ InstallationWorkerSlotService = (*Service)(nil)
