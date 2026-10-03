package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

type InstallationOwnerService interface {
	ListObservedProviderAccounts(ctx context.Context) (kernel.ObservedProviderAccountList, error)
}

func (s *Service) ListObservedProviderAccounts(ctx context.Context) (kernel.ObservedProviderAccountList, error) {
	if s == nil || s.runtime == nil || ctx == nil {
		return kernel.ObservedProviderAccountList{}, core.Malformed
	}
	return s.runtime.ListObservedProviderAccounts(ctx, s.runtime.LocalInstallationOwnerScope(), 100)
}

var _ InstallationOwnerService = (*Service)(nil)
