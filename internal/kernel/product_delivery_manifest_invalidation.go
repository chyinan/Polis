// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ProductDeliveryManifestInvalidationReceipt struct {
	RequestID           string `json:"requestId"`
	CompanyID           string `json:"companyId"`
	DeliveryID          string `json:"deliveryId"`
	ManifestRevision    string `json:"manifestRevision"`
	DispositionRevision string `json:"dispositionRevision"`
	State               string `json:"state"`
	Actor               string `json:"actor"`
	Reason              string `json:"reason"`
	CreatedAt           string `json:"createdAt"`
}

const productDeliveryManifestInvalidationActor = "installation-owner"

func (k *Kernel) InvalidateProductDeliveryManifest(ctx context.Context, companyID string, command ProductDeliveryManifestInvalidationCommand) (ProductDeliveryManifestInvalidationReceipt, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || validateProductDeliveryManifestInvalidationCommand(command) != nil {
		return ProductDeliveryManifestInvalidationReceipt{}, core.Malformed
	}
	command.Reason = strings.TrimSpace(command.Reason)
	writeReceipt, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, command.RequestID, "delivery.manifest.invalidate", command, func(tx pgx.Tx) (Receipt, error) {
		var storedRevision int64
		var missionID, taskID, artifactID, currentState, rawManifest, storedSHA256 string
		if err := tx.QueryRow(ctx, `SELECT revision,mission_id,task_id,artifact_id,state,manifest::text,manifest_sha256
FROM delivery_manifest_revisions
WHERE company_id=$1 AND delivery_id=$2 AND artifact_id=$2
ORDER BY revision DESC LIMIT 1 FOR UPDATE`, companyID, command.ArtifactID).Scan(&storedRevision, &missionID, &taskID, &artifactID, &currentState, &rawManifest, &storedSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if artifactID != command.ArtifactID || storedRevision != command.ExpectedManifestRevision {
			return Receipt{}, core.Conflict
		}
		if currentState != "assembling" && currentState != "ready" {
			return Receipt{}, core.ConflictError{Reason: "delivery manifest is already terminal", CurrentState: currentState}
		}
		manifest, err := validateStoredProductDeliveryManifest([]byte(rawManifest), storedSHA256, companyID, missionID, taskID, artifactID, storedRevision, currentState)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		var createdAt time.Time
		if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&createdAt); err != nil {
			return Receipt{}, err
		}
		newRevision := storedRevision + 1
		manifest.Revision = strconv.FormatInt(newRevision, 10)
		manifest.State = command.State
		manifest.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		manifestBytes, err := marshalDurableProductDeliveryManifest(manifest)
		if err != nil {
			return Receipt{}, err
		}
		manifestSHA256 := sha256Hex(manifestBytes)
		if _, err = tx.Exec(ctx, `INSERT INTO delivery_manifest_revisions(
company_id,delivery_id,revision,mission_id,task_id,artifact_id,state,manifest,manifest_sha256,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, companyID, artifactID, newRevision, missionID, taskID, artifactID, command.State, manifestBytes, manifestSHA256, createdAt); err != nil {
			return Receipt{}, err
		}
		dispositionRequestID := "delivery-lifecycle-" + fingerprint(command.RequestID)[:48]
		if _, err = tx.Exec(ctx, `INSERT INTO delivery_user_dispositions(
company_id,delivery_id,revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at)
VALUES($1,$2,1,$3,'not_requested',$4,$5,$6,NULL,$7)`, companyID, artifactID, newRevision, productDeliveryManifestInvalidationActor,
			command.Reason, dispositionRequestID, createdAt); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: artifactID, Status: command.State, Revision: newRevision}, nil
	})
	if err != nil {
		return ProductDeliveryManifestInvalidationReceipt{}, err
	}
	var result ProductDeliveryManifestInvalidationReceipt
	var createdAt time.Time
	dispositionRequestID := "delivery-lifecycle-" + fingerprint(command.RequestID)[:48]
	err = k.pool.QueryRow(ctx, `SELECT request_id,company_id,delivery_id,manifest_revision::text,revision::text,state,actor,reason,created_at
FROM delivery_user_dispositions WHERE company_id=$1 AND request_id=$2`, companyID, dispositionRequestID).Scan(
		&result.RequestID, &result.CompanyID, &result.DeliveryID, &result.ManifestRevision, &result.DispositionRevision, &result.State, &result.Actor, &result.Reason, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductDeliveryManifestInvalidationReceipt{}, core.Integrity
	}
	if err != nil {
		return ProductDeliveryManifestInvalidationReceipt{}, err
	}
	if writeReceipt.ID != command.ArtifactID || writeReceipt.Status != command.State || writeReceipt.Revision <= command.ExpectedManifestRevision ||
		result.RequestID != dispositionRequestID || result.CompanyID != companyID || result.DeliveryID != command.ArtifactID ||
		result.ManifestRevision != strconv.FormatInt(writeReceipt.Revision, 10) || result.State != "not_requested" || result.Actor != productDeliveryManifestInvalidationActor || result.Reason != command.Reason {
		return ProductDeliveryManifestInvalidationReceipt{}, core.Integrity
	}
	result.RequestID = command.RequestID
	result.State = command.State
	result.Actor = productDeliveryManifestInvalidationActor
	result.Reason = command.Reason
	result.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return result, nil
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
