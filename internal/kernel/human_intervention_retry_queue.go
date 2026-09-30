// pattern: Imperative Shell
package kernel

import (
	"context"

	"polis/internal/core"
	"polis/internal/qqnotify"
)

const maxHumanInterventionRetryBatch = 50

type DueHumanInterventionRetry struct {
	CompanyID      string
	InterventionID string
}

// TXReconcileExpiredHumanInterventionSends closes sender attempts whose short
// permit expired without a recorded receipt. Their outcome stays unknown and
// is never put back into the retry queue.
func (k *Kernel) TXReconcileExpiredHumanInterventionSends(ctx context.Context) (int, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH expired AS (
 SELECT company_id,id,intent_id FROM notification_deliveries
 WHERE state='sending' AND expires_at<=clock_timestamp()
 ORDER BY expires_at,company_id,id LIMIT $1 FOR UPDATE SKIP LOCKED
)
UPDATE notification_deliveries d
SET state='outcome_unknown',error_code='sender_interrupted_after_permit_expiry',retry_at=NULL
FROM expired e WHERE d.company_id=e.company_id AND d.id=e.id
RETURNING e.company_id,e.intent_id`, maxHumanInterventionRetryBatch)
	if err != nil {
		return 0, err
	}
	type expiredNotice struct{ companyID, intentID string }
	expired := make([]expiredNotice, 0)
	for rows.Next() {
		var notice expiredNotice
		if err = rows.Scan(&notice.companyID, &notice.intentID); err != nil {
			rows.Close()
			return 0, err
		}
		expired = append(expired, notice)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, notice := range expired {
		if _, err = tx.Exec(ctx, `UPDATE notification_intents SET state='outcome_unknown'
WHERE company_id=$1 AND id=$2 AND state='pending'`, notice.companyID, notice.intentID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(expired), nil
}

func (k *Kernel) ListDueHumanInterventionRetries(ctx context.Context, limit int) ([]DueHumanInterventionRetry, error) {
	if limit < 1 || limit > maxHumanInterventionRetryBatch {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT ni.company_id,hi.id
FROM notification_intents ni
JOIN human_interventions hi ON hi.company_id=ni.company_id AND hi.id=ni.intervention_id
JOIN LATERAL (
 SELECT state,attempt_count,retry_at,route_revision FROM notification_deliveries
 WHERE company_id=ni.company_id AND intent_id=ni.id
 ORDER BY attempt_count DESC,created_at DESC,id LIMIT 1
) latest ON true
JOIN notification_routes route ON route.company_id=hi.company_id AND route.adapter='qq_official'
 AND route.enabled AND route.status='ready' AND route.qualification_status='qualified'
 AND route.qualified_until>clock_timestamp() AND route.route_revision=latest.route_revision
JOIN qq_channel_qualifications qualification ON qualification.company_id=route.company_id
 AND qualification.route_id=route.id AND qualification.route_revision=route.route_revision
 AND qualification.target_openid=route.destination AND qualification.qualification_status='qualified'
 AND qualification.qualified_until>clock_timestamp()
WHERE ni.event_kind='human_intervention.open' AND ni.state='pending'
AND hi.state='open' AND hi.expires_at>clock_timestamp()
AND latest.state='retry_wait' AND latest.retry_at<=clock_timestamp()
AND latest.attempt_count<$2
ORDER BY latest.retry_at,ni.company_id,hi.id LIMIT $1`, limit, qqnotify.MaxNotificationAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	due := make([]DueHumanInterventionRetry, 0)
	for rows.Next() {
		var item DueHumanInterventionRetry
		if err = rows.Scan(&item.CompanyID, &item.InterventionID); err != nil {
			return nil, err
		}
		due = append(due, item)
	}
	return due, rows.Err()
}
