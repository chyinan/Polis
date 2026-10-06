// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type GenericActionIntentRecord struct {
	CompanyID      string `json:"companyId"`
	IntentID       string `json:"intentId"`
	MissionID      string `json:"missionId"`
	TaskID         string `json:"taskId"`
	SessionID      string `json:"sessionId"`
	ActionKind     string `json:"actionKind"`
	ResourceKey    string `json:"resourceKey"`
	TargetSHA256   string `json:"targetSha256"`
	InputSHA256    string `json:"inputSha256"`
	IdempotencyKey string `json:"idempotencyKey"`
	State          string `json:"state"`
	ReasonCode     string `json:"reasonCode"`
	Actor          string `json:"actor"`
	RequestID      string `json:"requestId"`
	CreatedAt      string `json:"createdAt"`
}

func (k *Kernel) TXRecordGenericActionIntent(ctx context.Context, binding Binding, input GenericActionIntentRequest, requestID string) (GenericActionIntentRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(requestID) {
		return GenericActionIntentRecord{}, core.Malformed
	}
	normalized, err := normalizeGenericActionIntentRequest(input)
	if err != nil {
		return GenericActionIntentRecord{}, err
	}
	intentID := stableCapabilityID("generic-action-intent", binding.scope.company, requestID)
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "generic.action_intent.record", normalized, func(tx pgx.Tx) (Receipt, error) {
		var existingIntentID string
		if err := tx.QueryRow(ctx, `SELECT intent_id FROM generic_action_intents WHERE company_id=$1 AND idempotency_key=$2 FOR SHARE`, binding.scope.company, normalized.IdempotencyKey).Scan(&existingIntentID); err == nil {
			return Receipt{}, core.Conflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		var missionID string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id
FROM tasks t JOIN worker_sessions s ON s.company_id=t.company_id AND s.task_id=t.id
WHERE t.company_id=$1 AND t.id=$2 AND s.id=$3 AND s.state='active'`, binding.scope.company, binding.task, binding.session).Scan(&missionID); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO generic_action_intents(company_id,intent_id,mission_id,task_id,session_id,action_kind,resource_key,target_sha256,input_sha256,idempotency_key)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, binding.scope.company, intentID, missionID, binding.task, binding.session, normalized.ActionKind, normalized.ResourceKey, normalized.TargetSHA256, normalized.InputSHA256, normalized.IdempotencyKey); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("generic-action-intent-event", binding.scope.company, requestID)
		if _, err := tx.Exec(ctx, `INSERT INTO generic_action_intent_events(company_id,event_id,intent_id,state,reason_code,actor,request_id)
VALUES($1,$2,$3,'denied',$4,$5,$6)`, binding.scope.company, eventID, intentID, GenericActionIntentDeniedReason, binding.employee, requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: intentID, Status: "denied"}, nil
	})
	if err != nil {
		return GenericActionIntentRecord{}, err
	}
	return k.GetGenericActionIntent(ctx, binding.scope.company, writeReceipt.ID)
}

func (k *Kernel) GetGenericActionIntent(ctx context.Context, companyID, intentID string) (GenericActionIntentRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(intentID) {
		return GenericActionIntentRecord{}, core.Malformed
	}
	return readGenericActionIntent(ctx, k.pool, companyID, intentID, "", "")
}

func (k *Kernel) ProductTaskGenericActionIntent(ctx context.Context, binding Binding, intentID string) (GenericActionIntentRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(intentID) {
		return GenericActionIntentRecord{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return GenericActionIntentRecord{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return GenericActionIntentRecord{}, err
	}
	record, err := readGenericActionIntent(ctx, tx, binding.scope.company, intentID, binding.task, binding.session)
	if err != nil {
		return GenericActionIntentRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return GenericActionIntentRecord{}, err
	}
	return record, nil
}

type genericActionIntentQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readGenericActionIntent(ctx context.Context, queryer genericActionIntentQueryer, companyID, intentID, taskID, sessionID string) (GenericActionIntentRecord, error) {
	if (taskID == "") != (sessionID == "") {
		return GenericActionIntentRecord{}, core.Malformed
	}
	var record GenericActionIntentRecord
	var createdAt time.Time
	query := `SELECT i.company_id,i.intent_id,i.mission_id,i.task_id,i.session_id,i.action_kind,i.resource_key,i.target_sha256,i.input_sha256,i.idempotency_key,e.state,e.reason_code,e.actor,e.request_id,i.created_at
FROM generic_action_intents i JOIN LATERAL (SELECT state,reason_code,actor,request_id FROM generic_action_intent_events WHERE company_id=i.company_id AND intent_id=i.intent_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE i.company_id=$1 AND i.intent_id=$2`
	args := []any{companyID, intentID}
	if taskID != "" {
		query += ` AND i.task_id=$3 AND i.session_id=$4`
		args = append(args, taskID, sessionID)
	}
	err := queryer.QueryRow(ctx, query, args...).Scan(&record.CompanyID, &record.IntentID, &record.MissionID, &record.TaskID, &record.SessionID, &record.ActionKind, &record.ResourceKey, &record.TargetSHA256, &record.InputSHA256, &record.IdempotencyKey, &record.State, &record.ReasonCode, &record.Actor, &record.RequestID, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenericActionIntentRecord{}, core.OutOfScope
	}
	if err != nil {
		return GenericActionIntentRecord{}, err
	}
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}
