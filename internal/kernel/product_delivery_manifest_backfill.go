// pattern: Imperative Shell
package kernel

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ProductDeliveryManifestBackfillResult struct {
	CompanyID string `json:"companyId"`
	Created   int64  `json:"created"`
}

const historicalDeliveryBackfillBatchLimit = 128

func (k *Kernel) TXBackfillProductDeliveryManifests(ctx context.Context, scope Scope, requestID string) (ProductDeliveryManifestBackfillResult, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(requestID) {
		return ProductDeliveryManifestBackfillResult{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, requestID, "delivery.manifest.backfill", scope.company, func(tx pgx.Tx) (Receipt, error) {
		var createdAt time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&createdAt); err != nil {
			return Receipt{}, err
		}
		rows, err := tx.Query(ctx, `SELECT a.id,t.mission_id,a.task_id,a.bytes,a.digest
FROM artifacts a
JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND a.artifact_kind='deliverable' AND a.state='ready'
  AND NOT EXISTS(SELECT 1 FROM delivery_manifest_revisions r WHERE r.company_id=a.company_id AND r.delivery_id=a.id)
ORDER BY a.id LIMIT $2`, scope.company, historicalDeliveryBackfillBatchLimit)
		if err != nil {
			return Receipt{}, err
		}
		type historicalArtifact struct {
			id, missionID, taskID, digest string
			bytes                         int64
		}
		items := make([]historicalArtifact, 0, historicalDeliveryBackfillBatchLimit)
		for rows.Next() {
			var item historicalArtifact
			if err = rows.Scan(&item.id, &item.missionID, &item.taskID, &item.bytes, &item.digest); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			items = append(items, item)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return Receipt{}, err
		}
		rows.Close()
		var created int64
		for _, item := range items {
			if item.bytes <= 0 || item.bytes > 64*1024*1024 {
				return Receipt{}, core.Integrity
			}
			_, manifestBytes, manifestSHA256, buildErr := buildHistoricalProductDeliveryManifest(historicalProductDeliveryManifestInput{
				CompanyID: scope.company, MissionID: item.missionID, TaskID: item.taskID, ArtifactID: item.id,
				ArtifactBytes: int(item.bytes), ArtifactSHA256: item.digest, CreatedAt: createdAt,
			})
			if buildErr != nil {
				return Receipt{}, buildErr
			}
			result, execErr := tx.Exec(ctx, `INSERT INTO delivery_manifest_revisions(
company_id,delivery_id,revision,mission_id,task_id,artifact_id,state,manifest,manifest_sha256,created_at)
VALUES($1,$2,1,$3,$4,$2,'assembling',$5,$6,$7)
ON CONFLICT(company_id,delivery_id,revision) DO NOTHING`, scope.company, item.id, item.missionID, item.taskID, manifestBytes, manifestSHA256, createdAt)
			if execErr != nil {
				return Receipt{}, execErr
			}
			if result.RowsAffected() == 0 {
				continue
			}
			dispositionRequestID := "backfill-" + fingerprint(item.id)[:48]
			if _, execErr = tx.Exec(ctx, `INSERT INTO delivery_user_dispositions(
company_id,delivery_id,revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at)
VALUES($1,$2,1,1,'not_requested','system',$3,$4,NULL,$5)
ON CONFLICT(company_id,request_id) DO NOTHING`, scope.company, item.id,
				"Historical backfill does not request user feedback; download or preview is not acceptance.", dispositionRequestID, createdAt); execErr != nil {
				return Receipt{}, execErr
			}
			var dispositionState, dispositionActor, dispositionReason string
			if err = tx.QueryRow(ctx, `SELECT state,actor,reason FROM delivery_user_dispositions WHERE company_id=$1 AND delivery_id=$2 AND manifest_revision=1 AND revision=1 AND request_id=$3`, scope.company, item.id, dispositionRequestID).Scan(&dispositionState, &dispositionActor, &dispositionReason); err != nil {
				return Receipt{}, err
			}
			if dispositionState != "not_requested" || dispositionActor != "system" || dispositionReason != "Historical backfill does not request user feedback; download or preview is not acceptance." {
				return Receipt{}, core.Integrity
			}
			created++
		}
		return Receipt{ID: scope.company, Status: "backfilled", Revision: created}, nil
	})
	if err != nil {
		return ProductDeliveryManifestBackfillResult{}, err
	}
	return ProductDeliveryManifestBackfillResult{CompanyID: scope.company, Created: receipt.Revision}, nil
}
