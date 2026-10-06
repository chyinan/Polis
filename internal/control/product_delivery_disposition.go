// pattern: Imperative Shell
package control

import (
	"context"
	"strconv"
	"strings"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) RecordDurableDeliveryDisposition(ctx context.Context, companyID string, request RecordDurableDeliveryDispositionRequest) (DurableDeliveryDispositionReceipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(request.ArtifactID) ||
		(request.State != "accepted" && request.State != "changes_requested") || validateRequestID(request.RequestID) != nil {
		return DurableDeliveryDispositionReceipt{}, core.Malformed
	}
	manifestRevision, err := parsePositiveDeliveryRevision(request.ExpectedManifestRevision)
	if err != nil {
		return DurableDeliveryDispositionReceipt{}, err
	}
	dispositionRevision, err := parsePositiveDeliveryRevision(request.ExpectedDispositionRevision)
	if err != nil {
		return DurableDeliveryDispositionReceipt{}, err
	}
	reason := strings.TrimSpace(request.Reason)
	if len(reason) == 0 || len(reason) > 4096 {
		return DurableDeliveryDispositionReceipt{}, core.Malformed
	}

	receipt, err := s.runtime.RecordProductDeliveryDisposition(ctx, companyID, kernel.ProductDeliveryDispositionCommand{
		ArtifactID:                  request.ArtifactID,
		ExpectedManifestRevision:    manifestRevision,
		ExpectedDispositionRevision: dispositionRevision,
		State:                       request.State,
		Reason:                      reason,
		RequestID:                   request.RequestID,
	})
	if err != nil {
		return DurableDeliveryDispositionReceipt{}, err
	}
	return DurableDeliveryDispositionReceipt{
		RequestID:           receipt.RequestID,
		CompanyID:           receipt.CompanyID,
		DeliveryID:          receipt.DeliveryID,
		ManifestRevision:    receipt.ManifestRevision,
		DispositionRevision: receipt.DispositionRevision,
		State:               receipt.State,
		Actor:               receipt.Actor,
		Reason:              receipt.Reason,
		CreatedAt:           receipt.CreatedAt,
	}, nil
}

func parsePositiveDeliveryRevision(value string) (int64, error) {
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision <= 0 || strconv.FormatInt(revision, 10) != value {
		return 0, core.Malformed
	}
	return revision, nil
}

var _ DurableDeliveryDispositionService = (*Service)(nil)
