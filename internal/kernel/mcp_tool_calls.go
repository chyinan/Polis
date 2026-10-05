// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/mcptransport"
)

type StdioMCPToolCallIntentInput struct {
	ProviderCallID   string          `json:"providerCallId"`
	CapabilityID     string          `json:"capabilityId"`
	ToolName         string          `json:"toolName"`
	ToolSchemaSHA256 string          `json:"toolSchemaSha256"`
	Arguments        json.RawMessage `json:"arguments"`
}

type StdioMCPToolCallRecord struct {
	CompanyID              string                        `json:"companyId"`
	IntentID               string                        `json:"intentId"`
	SessionID              string                        `json:"sessionId"`
	EmployeeID             string                        `json:"employeeId"`
	CapabilityID           string                        `json:"capabilityId"`
	RuntimeQualificationID string                        `json:"runtimeQualificationId"`
	DispatchPermitID       string                        `json:"dispatchPermitId,omitempty"`
	AttemptID              string                        `json:"attemptId,omitempty"`
	TaskID                 string                        `json:"taskId,omitempty"`
	WorkerGeneration       int64                         `json:"workerGeneration,omitempty"`
	EmployeeEpoch          int64                         `json:"employeeEpoch,omitempty"`
	GrantRevision          int64                         `json:"grantRevision,omitempty"`
	TargetSHA256           string                        `json:"targetSha256,omitempty"`
	InputSHA256            string                        `json:"inputSha256,omitempty"`
	ProviderCallID         string                        `json:"providerCallId"`
	ToolName               string                        `json:"toolName"`
	ToolSchemaSHA256       string                        `json:"toolSchemaSha256"`
	ArgumentsSHA256        string                        `json:"argumentsSha256"`
	Status                 string                        `json:"status"`
	ReasonCode             string                        `json:"reasonCode,omitempty"`
	Result                 *mcptransport.StdioToolResult `json:"result,omitempty"`
	ResultSHA256           string                        `json:"resultSha256,omitempty"`
	CreatedAt              string                        `json:"createdAt"`
}

type StdioMCPToolDispatchPermit struct {
	CompanyID                 string `json:"companyId"`
	PermitID                  string `json:"permitId"`
	ActionID                  string `json:"actionId"`
	IntentID                  string `json:"intentId"`
	AttemptID                 string `json:"attemptId"`
	SessionID                 string `json:"sessionId"`
	EmployeeID                string `json:"employeeId"`
	TaskID                    string `json:"taskId"`
	WorkerGeneration          int64  `json:"workerGeneration"`
	EmployeeEpoch             int64  `json:"employeeEpoch"`
	GrantRevision             int64  `json:"grantRevision"`
	CapabilityID              string `json:"capabilityId"`
	CapabilityVersion         string `json:"capabilityVersion"`
	CapabilityQualificationID string `json:"capabilityQualificationId"`
	RuntimeQualificationID    string `json:"runtimeQualificationId"`
	Transport                 string `json:"transport"`
	ProviderCallID            string `json:"providerCallId"`
	ToolName                  string `json:"toolName"`
	ToolSchemaSHA256          string `json:"toolSchemaSha256"`
	TargetSHA256              string `json:"targetSha256"`
	InputSHA256               string `json:"inputSha256"`
	CreatedAt                 string `json:"createdAt"`
	ExpiresAt                 string `json:"expiresAt"`
	Status                    string `json:"status"`
}

type StdioMCPToolDispatchPermitConsumption struct {
	PermitID string
	Call     StdioMCPToolCallIntentInput
}

const stdioMCPToolDispatchPermitLifetime = 15 * time.Second

// TXBeginStdioMCPToolCall now issues a short-lived permit only. The intent
// enters the in-flight call ledger later, atomically with permit consumption.
func (k *Kernel) TXBeginStdioMCPToolCall(ctx context.Context, binding Binding, input StdioMCPToolCallIntentInput) (StdioMCPToolDispatchPermit, error) {
	if ctx == nil || validateStdioMCPToolCallIntent(input) != nil || !core.ValidID(input.CapabilityID) {
		return StdioMCPToolDispatchPermit{}, core.Malformed
	}
	intentID := stableCapabilityID("mcp-call", binding.scope.company, binding.session, input.ProviderCallID)
	permitID := stableCapabilityID("mcp-permit", binding.scope.company, binding.session, input.ProviderCallID)
	attemptID := stableCapabilityID("mcp-attempt", binding.scope.company, binding.session, input.ProviderCallID)
	var authorization StdioMCPToolAuthorization
	_, err := k.TXWrite(ctx, binding.scope, &binding, permitID, "capability.mcp.tool_call.permit.issued", input, func(tx pgx.Tx) (Receipt, error) {
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, binding.scope.company, input.CapabilityID); err != nil {
			return Receipt{}, err
		}
		var authorizeErr error
		authorization, authorizeErr = authorizeStdioMCPToolCallInTransaction(ctx, tx, binding, input.CapabilityID, input.ToolName, input.ToolSchemaSHA256)
		if authorizeErr != nil {
			return Receipt{}, authorizeErr
		}
		argumentsDigest := digestCapabilityBytes(input.Arguments)
		var taskID string
		var workerGeneration, workerEpoch int64
		if err := tx.QueryRow(ctx, `SELECT task_id,generation,epoch FROM worker_sessions
WHERE company_id=$1 AND id=$2 AND employee_id=$3 FOR SHARE`, binding.scope.company, binding.session, binding.employee).
			Scan(&taskID, &workerGeneration, &workerEpoch); err != nil {
			return Receipt{}, err
		}
		if workerEpoch != binding.epoch || authorization.GrantRevision < 1 || authorization.TargetSHA256 == "" {
			return Receipt{}, core.StaleEpoch
		}
		if _, insertErr := tx.Exec(ctx, `WITH issue_clock AS (SELECT clock_timestamp() AS created_at)
INSERT INTO mcp_tool_dispatch_permits(company_id,permit_id,action_id,attempt_id,session_id,employee_id,task_id,worker_generation,
employee_epoch,grant_revision,capability_id,capability_version_digest,capability_qualification_id,runtime_qualification_id,transport,
provider_call_id,tool_name,tool_schema_sha256,target_sha256,input_sha256,created_at,expires_at)
SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,issue_clock.created_at,
issue_clock.created_at+make_interval(secs => $21::double precision) FROM issue_clock`, binding.scope.company, permitID, intentID, attemptID,
			binding.session, binding.employee, taskID, workerGeneration, workerEpoch, authorization.GrantRevision, input.CapabilityID,
			authorization.RuntimeQualification.VersionDigest, authorization.RuntimeQualification.CapabilityQualificationID,
			authorization.RuntimeQualification.RuntimeQualificationID, authorization.Transport, input.ProviderCallID, input.ToolName,
			input.ToolSchemaSHA256, authorization.TargetSHA256, argumentsDigest, stdioMCPToolDispatchPermitLifetime.Seconds()); insertErr != nil {
			if isUniqueViolation(insertErr) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, insertErr
		}
		eventID := stableCapabilityID("mcp-permit-issued", permitID)
		if _, insertErr := tx.Exec(ctx, `INSERT INTO mcp_tool_dispatch_permit_events(company_id,event_id,permit_id,status,request_id)
VALUES($1,$2,$3,'issued',$4)`, binding.scope.company, eventID, permitID, eventID); insertErr != nil {
			return Receipt{}, insertErr
		}
		return Receipt{ID: permitID, Status: "issued"}, nil
	})
	if err != nil {
		return StdioMCPToolDispatchPermit{}, err
	}
	permit, err := k.GetStdioMCPToolDispatchPermit(ctx, binding.scope.company, permitID)
	if err != nil {
		return StdioMCPToolDispatchPermit{}, err
	}
	if err = validateStdioMCPToolDispatchPermit(permit, input, authorization, binding); err != nil {
		return StdioMCPToolDispatchPermit{}, err
	}
	return permit, nil
}

// TXConsumeStdioMCPToolCallPermit is the only path that admits an external
// MCP request. Permit consumption and the immutable dispatching intent commit
// together under the same Company lifecycle row used by revocation.
func (k *Kernel) TXConsumeStdioMCPToolCallPermit(ctx context.Context, binding Binding, input StdioMCPToolDispatchPermitConsumption) (StdioMCPToolCallRecord, error) {
	if ctx == nil || !core.ValidID(input.PermitID) || validateStdioMCPToolCallIntent(input.Call) != nil {
		return StdioMCPToolCallRecord{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, binding.scope, &binding); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	permit, err := readStdioMCPToolDispatchPermitInTransaction(ctx, tx, binding.scope.company, input.PermitID, true)
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if err = lockCapabilityMCPServerInTransaction(ctx, tx, binding.scope.company, permit.CapabilityID); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if permit.Status != "issued" || permit.SessionID != binding.session || permit.EmployeeID != binding.employee || permit.TaskID != binding.task ||
		permit.EmployeeEpoch != binding.epoch || permit.ProviderCallID != input.Call.ProviderCallID || permit.CapabilityID != input.Call.CapabilityID ||
		permit.ToolName != input.Call.ToolName || permit.ToolSchemaSHA256 != input.Call.ToolSchemaSHA256 ||
		permit.InputSHA256 != digestCapabilityBytes(input.Call.Arguments) {
		return StdioMCPToolCallRecord{}, core.Denied
	}
	var authorization StdioMCPToolAuthorization
	authorization, err = authorizeStdioMCPToolCallInTransaction(ctx, tx, binding, permit.CapabilityID, permit.ToolName, permit.ToolSchemaSHA256)
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if authorization.GrantRevision != permit.GrantRevision || authorization.TargetSHA256 != permit.TargetSHA256 ||
		authorization.RuntimeQualification.RuntimeQualificationID != permit.RuntimeQualificationID {
		return StdioMCPToolCallRecord{}, core.Denied
	}
	var validExpiry bool
	if err = tx.QueryRow(ctx, `SELECT expires_at>clock_timestamp() FROM mcp_tool_dispatch_permits
WHERE company_id=$1 AND permit_id=$2`, binding.scope.company, input.PermitID).Scan(&validExpiry); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if !validExpiry {
		return StdioMCPToolCallRecord{}, core.Denied
	}
	consumedEventID := stableCapabilityID("mcp-permit-consumed", input.PermitID)
	if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_dispatch_permit_events(company_id,event_id,permit_id,status,request_id)
VALUES($1,$2,$3,'consumed',$4)`, binding.scope.company, consumedEventID, input.PermitID, consumedEventID); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_call_intents(company_id,intent_id,session_id,employee_id,capability_id,runtime_qualification_id,
provider_call_id,tool_name,tool_schema_sha256,arguments_sha256,dispatch_permit_id,attempt_id,task_id,worker_generation,employee_epoch,
grant_revision,target_sha256,input_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, binding.scope.company, permit.ActionID, permit.SessionID,
		permit.EmployeeID, permit.CapabilityID, permit.RuntimeQualificationID, permit.ProviderCallID, permit.ToolName, permit.ToolSchemaSHA256,
		permit.InputSHA256, permit.PermitID, permit.AttemptID, permit.TaskID, permit.WorkerGeneration, permit.EmployeeEpoch,
		permit.GrantRevision, permit.TargetSHA256, permit.InputSHA256); err != nil {
		if isUniqueViolation(err) {
			return StdioMCPToolCallRecord{}, core.Conflict
		}
		return StdioMCPToolCallRecord{}, err
	}
	dispatchEventID := stableCapabilityID("mcp-call-dispatch", permit.ActionID)
	if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_call_events(company_id,event_id,intent_id,status,request_id)
VALUES($1,$2,$3,'dispatching',$4)`, binding.scope.company, dispatchEventID, permit.ActionID, dispatchEventID); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if err = appendEvent(ctx, tx, binding.scope, "capability.mcp.tool_call.dispatching", map[string]any{
		"intent_id": permit.ActionID, "dispatch_permit_id": permit.PermitID, "attempt_id": permit.AttemptID,
		"employee_epoch": permit.EmployeeEpoch, "grant_revision": permit.GrantRevision,
		"target_sha256": permit.TargetSHA256, "input_sha256": permit.InputSHA256,
	}); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	return k.GetStdioMCPToolCall(ctx, binding.scope.company, permit.ActionID)
}

func (k *Kernel) TXCompleteStdioMCPToolCall(ctx context.Context, binding Binding, intentID string, result mcptransport.StdioToolResult) error {
	if ctx == nil || !core.ValidID(intentID) {
		return core.Malformed
	}
	var encoded []byte
	var resultDigest string
	// The result is checked against the immutable intent inside the transaction;
	// this preliminary bound prevents large payloads from reaching the database.
	encoded, resultDigest, err := validateStdioMCPToolResult(StdioMCPToolCallRecord{
		ToolName: result.ToolName, ToolSchemaSHA256: result.ToolSchemaSHA256,
	}, result)
	if err != nil {
		return core.Malformed
	}
	input := struct {
		IntentID     string
		ResultSHA256 string
	}{IntentID: intentID, ResultSHA256: resultDigest}
	_, err = k.TXWrite(ctx, binding.scope, &binding, stableCapabilityID("mcp-call-result", intentID), "capability.mcp.tool_call.completed", input, func(tx pgx.Tx) (Receipt, error) {
		record, err := readStdioMCPToolCallInTransaction(ctx, tx, binding.scope.company, intentID)
		if err != nil {
			return Receipt{}, err
		}
		if err = validateStdioMCPToolCallOwner(record, binding); err != nil {
			return Receipt{}, err
		}
		if _, _, err = validateStdioMCPToolResult(record, result); err != nil {
			return Receipt{}, core.Denied
		}
		if record.Status == "completed" {
			if record.ResultSHA256 != resultDigest {
				return Receipt{}, core.Conflict
			}
			return Receipt{ID: intentID, Status: "completed"}, nil
		}
		if record.Status != "dispatching" {
			return Receipt{}, core.Denied
		}
		eventID := stableCapabilityID("mcp-call-completed", intentID)
		if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_call_events(
company_id,event_id,intent_id,status,result,result_sha256,request_id)
VALUES($1,$2,$3,'completed',$4,$5,$6)`, binding.scope.company, eventID, intentID, encoded, resultDigest, eventID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: intentID, Status: "completed"}, nil
	})
	return err
}

func (k *Kernel) TXMarkStdioMCPToolCallsUnknownForSession(ctx context.Context, companyID, sessionID, reasonCode string) (int, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(sessionID) || !validMCPUnknownReason(reasonCode) {
		return 0, core.Malformed
	}
	count := 0
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, newID(), "capability.mcp.tool_call.outcome_unknown", struct {
		SessionID  string
		ReasonCode string
	}{sessionID, reasonCode}, func(tx pgx.Tx) (Receipt, error) {
		var sessionState string
		if err := tx.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, sessionID).Scan(&sessionState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if sessionState != "reconcile_required" && sessionState != "stopped" {
			return Receipt{}, core.Denied
		}
		rows, err := tx.Query(ctx, `SELECT i.intent_id FROM mcp_tool_call_intents i
JOIN LATERAL (SELECT status FROM mcp_tool_call_events e WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY event_seq DESC LIMIT 1) current ON true
WHERE i.company_id=$1 AND i.session_id=$2 AND current.status='dispatching'
ORDER BY i.intent_id FOR UPDATE OF i`, companyID, sessionID)
		if err != nil {
			return Receipt{}, err
		}
		intentIDs := make([]string, 0)
		for rows.Next() {
			var intentID string
			if err = rows.Scan(&intentID); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			intentIDs = append(intentIDs, intentID)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return Receipt{}, err
		}
		for _, intentID := range intentIDs {
			eventID := stableCapabilityID("mcp-call-unknown", intentID)
			if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_call_events(
company_id,event_id,intent_id,status,reason_code,request_id)
VALUES($1,$2,$3,'outcome_unknown',$4,$5)`, companyID, eventID, intentID, reasonCode, eventID); err != nil {
				return Receipt{}, err
			}
		}
		permitRows, err := tx.Query(ctx, `SELECT p.permit_id FROM mcp_tool_dispatch_permits p
WHERE p.company_id=$1 AND p.session_id=$2
 AND (SELECT e.status FROM mcp_tool_dispatch_permit_events e WHERE e.company_id=p.company_id AND e.permit_id=p.permit_id
      ORDER BY e.event_seq DESC LIMIT 1)='issued'
ORDER BY p.permit_id FOR UPDATE OF p`, companyID, sessionID)
		if err != nil {
			return Receipt{}, err
		}
		permitIDs := make([]string, 0)
		for permitRows.Next() {
			var permitID string
			if err = permitRows.Scan(&permitID); err != nil {
				permitRows.Close()
				return Receipt{}, err
			}
			permitIDs = append(permitIDs, permitID)
		}
		permitRows.Close()
		if err = permitRows.Err(); err != nil {
			return Receipt{}, err
		}
		for _, permitID := range permitIDs {
			eventID := stableCapabilityID("mcp-permit-expired-stop", permitID)
			if _, err = tx.Exec(ctx, `INSERT INTO mcp_tool_dispatch_permit_events(company_id,event_id,permit_id,status,reason_code,request_id)
VALUES($1,$2,$3,'expired','worker_stopped_before_dispatch',$4)`, companyID, eventID, permitID, eventID); err != nil {
				return Receipt{}, err
			}
		}
		count = len(intentIDs)
		return Receipt{ID: sessionID, Status: "outcome_unknown"}, nil
	})
	return count, err
}

func (k *Kernel) GetStdioMCPToolCall(ctx context.Context, companyID, intentID string) (StdioMCPToolCallRecord, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(intentID) {
		return StdioMCPToolCallRecord{}, core.Malformed
	}
	return readStdioMCPToolCall(ctx, k.pool, companyID, intentID)
}

func (k *Kernel) GetStdioMCPToolDispatchPermit(ctx context.Context, companyID, permitID string) (StdioMCPToolDispatchPermit, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(permitID) {
		return StdioMCPToolDispatchPermit{}, core.Malformed
	}
	return readStdioMCPToolDispatchPermit(ctx, k.pool, companyID, permitID, false)
}

func readStdioMCPToolDispatchPermit(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, companyID, permitID string, lock bool) (StdioMCPToolDispatchPermit, error) {
	return readStdioMCPToolDispatchPermitInTransaction(ctx, db, companyID, permitID, lock)
}

func readStdioMCPToolDispatchPermitInTransaction(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, companyID, permitID string, lock bool) (StdioMCPToolDispatchPermit, error) {
	var permit StdioMCPToolDispatchPermit
	query := `SELECT p.company_id,p.permit_id,p.action_id,p.attempt_id,p.session_id,p.employee_id,p.task_id,p.worker_generation,
p.employee_epoch,p.grant_revision,p.capability_id,p.capability_version_digest,p.capability_qualification_id,p.runtime_qualification_id,
p.transport,p.provider_call_id,p.tool_name,p.tool_schema_sha256,p.target_sha256,p.input_sha256,p.created_at::text,p.expires_at::text,current.status
FROM mcp_tool_dispatch_permits p
JOIN LATERAL (SELECT e.status FROM mcp_tool_dispatch_permit_events e
 WHERE e.company_id=p.company_id AND e.permit_id=p.permit_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
WHERE p.company_id=$1 AND p.permit_id=$2`
	if lock {
		query += ` FOR UPDATE OF p`
	}
	err := db.QueryRow(ctx, query, companyID, permitID).Scan(
		&permit.CompanyID, &permit.PermitID, &permit.ActionID, &permit.AttemptID, &permit.SessionID, &permit.EmployeeID,
		&permit.TaskID, &permit.WorkerGeneration, &permit.EmployeeEpoch, &permit.GrantRevision, &permit.CapabilityID,
		&permit.CapabilityVersion, &permit.CapabilityQualificationID, &permit.RuntimeQualificationID, &permit.Transport,
		&permit.ProviderCallID, &permit.ToolName, &permit.ToolSchemaSHA256, &permit.TargetSHA256, &permit.InputSHA256,
		&permit.CreatedAt, &permit.ExpiresAt, &permit.Status)
	permit.IntentID = permit.ActionID
	if errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPToolDispatchPermit{}, core.OutOfScope
	}
	return permit, err
}

func readStdioMCPToolCall(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, companyID, intentID string) (StdioMCPToolCallRecord, error) {
	var record StdioMCPToolCallRecord
	var resultJSON []byte
	err := db.QueryRow(ctx, `SELECT i.company_id,i.intent_id,i.session_id,i.employee_id,i.capability_id,
i.runtime_qualification_id,COALESCE(i.dispatch_permit_id,''),COALESCE(i.attempt_id,''),COALESCE(i.task_id,''),COALESCE(i.worker_generation,0),
COALESCE(i.employee_epoch,0),COALESCE(i.grant_revision,0),COALESCE(i.target_sha256,''),COALESCE(i.input_sha256,''),
i.provider_call_id,i.tool_name,i.tool_schema_sha256,i.arguments_sha256,
 e.status,COALESCE(e.reason_code,''),e.result,COALESCE(e.result_sha256,''),i.created_at::text
FROM mcp_tool_call_intents i JOIN LATERAL (
 SELECT status,reason_code,result,result_sha256,created_at FROM mcp_tool_call_events
 WHERE company_id=i.company_id AND intent_id=i.intent_id ORDER BY event_seq DESC LIMIT 1
) e ON true WHERE i.company_id=$1 AND i.intent_id=$2`, companyID, intentID).Scan(
		&record.CompanyID, &record.IntentID, &record.SessionID, &record.EmployeeID, &record.CapabilityID,
		&record.RuntimeQualificationID, &record.DispatchPermitID, &record.AttemptID, &record.TaskID, &record.WorkerGeneration,
		&record.EmployeeEpoch, &record.GrantRevision, &record.TargetSHA256, &record.InputSHA256,
		&record.ProviderCallID, &record.ToolName, &record.ToolSchemaSHA256,
		&record.ArgumentsSHA256, &record.Status, &record.ReasonCode, &resultJSON, &record.ResultSHA256, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPToolCallRecord{}, core.OutOfScope
	}
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if len(resultJSON) > 0 {
		var result mcptransport.StdioToolResult
		if err = json.Unmarshal(resultJSON, &result); err != nil {
			return StdioMCPToolCallRecord{}, core.Integrity
		}
		record.Result = &result
	}
	return record, nil
}

func readStdioMCPToolCallInTransaction(ctx context.Context, tx pgx.Tx, companyID, intentID string) (StdioMCPToolCallRecord, error) {
	return readStdioMCPToolCall(ctx, tx, companyID, intentID)
}

func validMCPUnknownReason(value string) bool {
	return value == "worker_interrupted_during_call" || value == "worker_session_stopped" || value == "owner_process_lost" || value == "tool_response_not_persisted" || value == "schema_drift"
}

var _ = errors.Is
