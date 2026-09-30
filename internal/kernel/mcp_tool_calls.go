// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

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

func (k *Kernel) TXBeginStdioMCPToolCall(ctx context.Context, binding Binding, input StdioMCPToolCallIntentInput) (StdioMCPToolCallRecord, error) {
	if ctx == nil || validateStdioMCPToolCallIntent(input) != nil || !core.ValidID(input.CapabilityID) {
		return StdioMCPToolCallRecord{}, core.Malformed
	}
	intentID := stableCapabilityID("mcp-call", binding.scope.company, binding.session, input.ProviderCallID)
	var authorization StdioMCPToolAuthorization
	_, err := k.TXWrite(ctx, binding.scope, &binding, newID(), "capability.mcp.tool_call.dispatching", input, func(tx pgx.Tx) (Receipt, error) {
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, binding.scope.company, input.CapabilityID); err != nil {
			return Receipt{}, err
		}
		var authorizeErr error
		authorization, authorizeErr = authorizeStdioMCPToolCallInTransaction(ctx, tx, binding, input.CapabilityID, input.ToolName, input.ToolSchemaSHA256)
		if authorizeErr != nil {
			return Receipt{}, authorizeErr
		}
		argumentsDigest := digestCapabilityBytes(input.Arguments)
		if _, insertErr := tx.Exec(ctx, `INSERT INTO mcp_tool_call_intents(
company_id,intent_id,session_id,employee_id,capability_id,runtime_qualification_id,provider_call_id,
tool_name,tool_schema_sha256,arguments_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, binding.scope.company, intentID, binding.session, binding.employee,
			input.CapabilityID, authorization.RuntimeQualification.RuntimeQualificationID, input.ProviderCallID,
			input.ToolName, input.ToolSchemaSHA256, argumentsDigest); insertErr != nil {
			if isUniqueViolation(insertErr) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, insertErr
		}
		eventID := stableCapabilityID("mcp-call-dispatch", intentID)
		if _, insertErr := tx.Exec(ctx, `INSERT INTO mcp_tool_call_events(
company_id,event_id,intent_id,status,request_id) VALUES($1,$2,$3,'dispatching',$4)`, binding.scope.company, eventID, intentID, eventID); insertErr != nil {
			return Receipt{}, insertErr
		}
		return Receipt{ID: intentID, Status: "dispatching"}, nil
	})
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	record, err := k.GetStdioMCPToolCall(ctx, binding.scope.company, intentID)
	if err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	if err = validateStdioMCPToolCallStart(record, input, authorization, binding); err != nil {
		return StdioMCPToolCallRecord{}, err
	}
	return record, nil
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

func readStdioMCPToolCall(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, companyID, intentID string) (StdioMCPToolCallRecord, error) {
	var record StdioMCPToolCallRecord
	var resultJSON []byte
	err := db.QueryRow(ctx, `SELECT i.company_id,i.intent_id,i.session_id,i.employee_id,i.capability_id,
i.runtime_qualification_id,i.provider_call_id,i.tool_name,i.tool_schema_sha256,i.arguments_sha256,
 e.status,COALESCE(e.reason_code,''),e.result,COALESCE(e.result_sha256,''),i.created_at::text
FROM mcp_tool_call_intents i JOIN LATERAL (
 SELECT status,reason_code,result,result_sha256,created_at FROM mcp_tool_call_events
 WHERE company_id=i.company_id AND intent_id=i.intent_id ORDER BY event_seq DESC LIMIT 1
) e ON true WHERE i.company_id=$1 AND i.intent_id=$2`, companyID, intentID).Scan(
		&record.CompanyID, &record.IntentID, &record.SessionID, &record.EmployeeID, &record.CapabilityID,
		&record.RuntimeQualificationID, &record.ProviderCallID, &record.ToolName, &record.ToolSchemaSHA256,
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
