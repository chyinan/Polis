// pattern: Imperative Shell

package kernel

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type CapabilityRevocationOwnerReviewInput struct {
	RevocationID string
	Rationale    string
	RequestID    string
}

// TXReviewIncompleteCapabilityRevocation records that the installation owner
// reviewed an incomplete legacy inventory and accepts that its historical
// session set remains unknown. It never completes the inventory or marks it
// quiesced.
func (k *Kernel) TXReviewIncompleteCapabilityRevocation(ctx context.Context, companyID string, input CapabilityRevocationOwnerReviewInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RevocationID) || !core.ValidID(input.RequestID) ||
		strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	scope := k.LocalScope(companyID)
	return k.TXWrite(ctx, scope, nil, input.RequestID, "capability.revocation.owner_review", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		statuses, _, err := listCapabilityRevocationStatuses(ctx, tx, companyID)
		if err != nil {
			return Receipt{}, err
		}
		var target *CapabilityRevocationStatus
		for index := range statuses {
			if statuses[index].RevocationID == input.RevocationID {
				target = &statuses[index]
				break
			}
		}
		if target == nil {
			return Receipt{}, core.OutOfScope
		}
		if target.SessionInventoryComplete {
			return Receipt{}, core.ConflictError{Reason: "capability revocation session inventory is already complete", CurrentState: "complete"}
		}
		if target.OwnerReview != nil {
			return Receipt{}, core.ConflictError{Reason: "capability revocation inventory already has an owner review", CurrentState: target.OwnerReview.Disposition}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO capability_revocation_owner_reviews(
company_id,revocation_id,scope,capability_kind,capability_id,version_digest,qualification_id,employee_id,disposition,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'acknowledged_unresolved',$9,'installation-owner',$10)`,
			companyID, input.RevocationID, target.Scope, target.CapabilityKind, target.CapabilityID,
			target.VersionDigest, target.QualificationID, target.EmployeeID, strings.TrimSpace(input.Rationale), input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.RevocationID, Status: "acknowledged_unresolved"}, nil
	})
}
