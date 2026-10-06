// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) CompleteDurableDeliveryManifest(ctx context.Context, companyID string, request CompleteDurableDeliveryManifestRequest) (CompleteDurableDeliveryManifestReceipt, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(request.ArtifactID) || validateRequestID(request.RequestID) != nil {
		return CompleteDurableDeliveryManifestReceipt{}, core.Malformed
	}
	manifestRevision, err := parsePositiveDeliveryRevision(request.ExpectedManifestRevision)
	if err != nil {
		return CompleteDurableDeliveryManifestReceipt{}, err
	}
	receipt, err := s.runtime.CompleteProductDeliveryManifest(ctx, companyID, kernel.ProductDeliveryManifestCompletionCommand{
		ArtifactID:               request.ArtifactID,
		ExpectedManifestRevision: manifestRevision,
		RequestID:                request.RequestID,
		Evidence: kernel.ReadyProductDeliveryEvidence{
			SourceInputs:     kernel.DeliveryManifestEvidence{Reference: request.SourceInputs.Reference, Digest: request.SourceInputs.Digest, Detail: request.SourceInputs.Detail},
			EnvironmentBuild: kernel.DeliveryManifestEvidence{Reference: request.EnvironmentBuild.Reference, Digest: request.EnvironmentBuild.Digest, Detail: request.EnvironmentBuild.Detail},
			RunInstructions:  kernel.DeliveryManifestEvidence{Reference: request.RunInstructions.Reference, Digest: request.RunInstructions.Digest, Detail: request.RunInstructions.Detail},
			Limitations:      kernel.DeliveryManifestEvidence{Reference: request.Limitations.Reference, Digest: request.Limitations.Digest, Detail: request.Limitations.Detail},
			LicenseSource:    kernel.DeliveryManifestEvidence{Reference: request.LicenseSource.Reference, Digest: request.LicenseSource.Digest, Detail: request.LicenseSource.Detail},
		},
	})
	if err != nil {
		return CompleteDurableDeliveryManifestReceipt{}, err
	}
	return CompleteDurableDeliveryManifestReceipt{
		RequestID:           receipt.RequestID,
		CompanyID:           receipt.CompanyID,
		DeliveryID:          receipt.DeliveryID,
		ManifestRevision:    receipt.ManifestRevision,
		DispositionRevision: receipt.DispositionRevision,
		State:               receipt.State,
		Actor:               receipt.Actor,
		CreatedAt:           receipt.CreatedAt,
	}, nil
}

var _ DurableDeliveryManifestCompletionService = (*Service)(nil)
