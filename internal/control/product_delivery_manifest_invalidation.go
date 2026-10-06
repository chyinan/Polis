// pattern: Imperative Shell
package control

import (
	"context"
	"strings"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) InvalidateDurableDeliveryManifest(ctx context.Context, companyID string, request InvalidateDurableDeliveryManifestRequest) (InvalidateDurableDeliveryManifestReceipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(request.ArtifactID) || validateRequestID(request.RequestID) != nil || (request.State != "invalidated" && request.State != "withdrawn") {
		return InvalidateDurableDeliveryManifestReceipt{}, core.Malformed
	}
	manifestRevision, err := parsePositiveDeliveryRevision(request.ExpectedManifestRevision)
	if err != nil {
		return InvalidateDurableDeliveryManifestReceipt{}, err
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || len(reason) > 4096 {
		return InvalidateDurableDeliveryManifestReceipt{}, core.Malformed
	}
	receipt, err := s.runtime.InvalidateProductDeliveryManifest(ctx, companyID, kernel.ProductDeliveryManifestInvalidationCommand{
		ArtifactID: request.ArtifactID, ExpectedManifestRevision: manifestRevision, State: request.State, Reason: reason, RequestID: request.RequestID,
	})
	if err != nil {
		return InvalidateDurableDeliveryManifestReceipt{}, err
	}
	return InvalidateDurableDeliveryManifestReceipt{
		RequestID: receipt.RequestID, CompanyID: receipt.CompanyID, DeliveryID: receipt.DeliveryID,
		ManifestRevision: receipt.ManifestRevision, DispositionRevision: receipt.DispositionRevision,
		State: receipt.State, Actor: receipt.Actor, Reason: receipt.Reason, CreatedAt: receipt.CreatedAt,
	}, nil
}

var _ DurableDeliveryManifestLifecycleService = (*Service)(nil)
