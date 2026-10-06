// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ResearchOperationRecord struct {
	CompanyID     string          `json:"companyId"`
	OperationID   string          `json:"operationId"`
	MissionID     string          `json:"missionId"`
	TaskID        string          `json:"taskId"`
	SessionID     string          `json:"sessionId"`
	Kind          string          `json:"kind"`
	Query         string          `json:"query,omitempty"`
	TargetURL     string          `json:"targetUrl,omitempty"`
	RequestSHA256 string          `json:"requestSha256"`
	State         string          `json:"state"`
	ReasonCode    string          `json:"reasonCode"`
	Result        json.RawMessage `json:"result"`
	Actor         string          `json:"actor"`
	RequestID     string          `json:"requestId"`
	CreatedAt     string          `json:"createdAt"`
}

func (k *Kernel) TXRequestResearchOperation(ctx context.Context, binding Binding, input ResearchOperationRequest, requestID string) (ResearchOperationRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(requestID) {
		return ResearchOperationRecord{}, core.Malformed
	}
	normalized, err := normalizeResearchOperationRequest(input)
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	operationID := stableCapabilityID("research-operation", binding.scope.company, requestID)
	requestSHA256 := fingerprint(normalized)
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "research.operation.request", normalized, func(tx pgx.Tx) (Receipt, error) {
		var missionID string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id
FROM tasks t JOIN worker_sessions s ON s.company_id=t.company_id AND s.task_id=t.id
WHERE t.company_id=$1 AND t.id=$2 AND s.id=$3 AND s.state='active'`, binding.scope.company, binding.task, binding.session).Scan(&missionID); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var query, targetURL any
		if normalized.Kind == ResearchOperationKindSearch {
			query = normalized.Query
		} else {
			targetURL = normalized.TargetURL
		}
		if _, err := tx.Exec(ctx, `INSERT INTO research_operations(company_id,operation_id,mission_id,task_id,session_id,kind,query,target_url,request_sha256,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, binding.scope.company, operationID, missionID, binding.task, binding.session, normalized.Kind, query, targetURL, requestSHA256, requestID); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("research-operation-event", binding.scope.company, requestID)
		result := json.RawMessage(`{"availability":"unavailable","provider_egress":0}`)
		if _, err := tx.Exec(ctx, `INSERT INTO research_operation_events(company_id,event_id,operation_id,state,reason_code,result,actor,request_id)
VALUES($1,$2,$3,'unavailable',$4,$5::jsonb,$6,$7)`, binding.scope.company, eventID, operationID, ResearchOperationUnavailableReason, result, binding.employee, requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: operationID, Status: "unavailable"}, nil
	})
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	return k.GetResearchOperation(ctx, binding.scope.company, writeReceipt.ID)
}

func (k *Kernel) GetResearchOperation(ctx context.Context, companyID, operationID string) (ResearchOperationRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(operationID) {
		return ResearchOperationRecord{}, core.Malformed
	}
	return readResearchOperation(ctx, k.pool, companyID, operationID, "", "")
}

func (k *Kernel) ProductTaskResearchOperationResults(ctx context.Context, binding Binding, operationID string) (ResearchOperationRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(operationID) {
		return ResearchOperationRecord{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return ResearchOperationRecord{}, err
	}
	record, err := readResearchOperation(ctx, tx, binding.scope.company, operationID, binding.task, binding.session)
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ResearchOperationRecord{}, err
	}
	return record, nil
}

type researchOperationQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readResearchOperation(ctx context.Context, queryer researchOperationQueryer, companyID, operationID, taskID, sessionID string) (ResearchOperationRecord, error) {
	if (taskID == "") != (sessionID == "") {
		return ResearchOperationRecord{}, core.Malformed
	}
	var record ResearchOperationRecord
	var result []byte
	var createdAt time.Time
	query := `SELECT o.company_id,o.operation_id,o.mission_id,o.task_id,o.session_id,o.kind,COALESCE(o.query,''),COALESCE(o.target_url,''),o.request_sha256,e.state,e.reason_code,e.result,e.actor,e.request_id,o.created_at
FROM research_operations o JOIN LATERAL (SELECT state,reason_code,result,actor,request_id FROM research_operation_events WHERE company_id=o.company_id AND operation_id=o.operation_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE o.company_id=$1 AND o.operation_id=$2`
	args := []any{companyID, operationID}
	if taskID != "" {
		query += ` AND o.task_id=$3 AND o.session_id=$4`
		args = append(args, taskID, sessionID)
	}
	err := queryer.QueryRow(ctx, query, args...).Scan(&record.CompanyID, &record.OperationID, &record.MissionID, &record.TaskID, &record.SessionID, &record.Kind, &record.Query, &record.TargetURL, &record.RequestSHA256, &record.State, &record.ReasonCode, &result, &record.Actor, &record.RequestID, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchOperationRecord{}, core.OutOfScope
	}
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	if !json.Valid(result) {
		return ResearchOperationRecord{}, core.Integrity
	}
	record.Result = append(json.RawMessage(nil), result...)
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}
