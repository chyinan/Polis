// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type MCPToolCallQualificationInput struct {
	BindingID        string
	DefinitionID     string
	Handle           string
	DescriptorDigest string
	PolicyRevision   string
	EffectClass      string
	Status           string
	AuthorizationRef string
	QualifiedBy      string
	Notes            string
}

func (k *Kernel) TXListMCPToolCallQualifications(ctx context.Context, companyID, bindingID string) ([]MCPToolCallQualification, error) {
	if !core.ValidID(companyID) {
		return nil, core.Malformed
	}
	if bindingID != "" && !core.ValidID(bindingID) {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT id,binding_id,definition_id,server_id,tool_name,handle,descriptor_digest,policy_revision,effect_class,status,authorization_ref,transport,qualified_by,qualified_at::text,expires_at::text,notes,created_at::text,updated_at::text
FROM mcp_tool_call_qualifications WHERE company_id=$1 AND ($2='' OR binding_id=$2) ORDER BY updated_at DESC,id`, companyID, bindingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MCPToolCallQualification{}
	for rows.Next() {
		var q MCPToolCallQualification
		if err = rows.Scan(&q.ID, &q.BindingID, &q.DefinitionID, &q.ServerID, &q.ToolName, &q.Handle, &q.DescriptorDigest, &q.PolicyRevision, &q.EffectClass, &q.Status, &q.AuthorizationRef, &q.Transport, &q.QualifiedBy, &q.QualifiedAt, &q.ExpiresAt, &q.Notes, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		q.CompanyID = companyID
		items = append(items, q)
	}
	return items, rows.Err()
}

func (k *Kernel) TXUpsertMCPToolCallQualification(ctx context.Context, companyID string, input MCPToolCallQualificationInput) (MCPToolCallQualification, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.BindingID) || !core.ValidID(input.DefinitionID) || strings.TrimSpace(input.DescriptorDigest) == "" {
		return MCPToolCallQualification{}, core.Malformed
	}
	serverID, toolName, err := splitToolHandle(input.Handle)
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	if strings.TrimSpace(input.PolicyRevision) == "" {
		input.PolicyRevision = toolCallPolicyRevision
	}
	if input.EffectClass == "" {
		input.EffectClass = "unknown"
	}
	if _, ok := allowedToolCallEffectClasses[input.EffectClass]; !ok {
		return MCPToolCallQualification{}, core.Malformed
	}
	if input.Status == "" {
		input.Status = "approved"
	}
	if _, ok := allowedToolCallQualificationStatuses[input.Status]; !ok {
		return MCPToolCallQualification{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return MCPToolCallQualification{}, err
	}
	var bindingStatus string
	var boundMCP []byte
	var bindingEmployee string
	err = tx.QueryRow(ctx, `SELECT status,employee_id,mcp_definition_ids FROM employee_capability_bindings WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, input.BindingID).Scan(&bindingStatus, &bindingEmployee, &boundMCP)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPToolCallQualification{}, core.OutOfScope
	}
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	_ = bindingEmployee
	var boundIDs []string
	if err = json.Unmarshal(boundMCP, &boundIDs); err != nil {
		return MCPToolCallQualification{}, core.Malformed
	}
	containsDef := false
	for _, id := range boundIDs {
		if id == input.DefinitionID {
			containsDef = true
			break
		}
	}
	if !containsDef || serverID != input.DefinitionID {
		return MCPToolCallQualification{}, core.Denied
	}
	if bindingStatus == "revoked" && input.Status == "approved" {
		return MCPToolCallQualification{}, core.Denied
	}
	var defStatus, defTransport string
	var qualificationRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,transport,qualification FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, companyID, input.DefinitionID).Scan(&defStatus, &defTransport, &qualificationRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPToolCallQualification{}, core.OutOfScope
	}
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	if defStatus == "revoked" && input.Status == "approved" {
		return MCPToolCallQualification{}, core.Denied
	}
	if defTransport != "stdio" && defTransport != "streamable_http" {
		return MCPToolCallQualification{}, core.Denied
	}
	desc, _, descErr := cachedDescriptorFromQualification(qualificationRaw, toolName)
	if descErr != nil {
		return MCPToolCallQualification{}, descErr
	}
	if desc == nil {
		return MCPToolCallQualification{}, core.OutOfScope
	}
	if desc.Digest != input.DescriptorDigest {
		return MCPToolCallQualification{}, core.Integrity
	}
	var existingID string
	err = tx.QueryRow(ctx, `SELECT id FROM mcp_tool_call_qualifications WHERE company_id=$1 AND binding_id=$2 AND definition_id=$3 AND handle=$4 AND descriptor_digest=$5 AND policy_revision=$6 FOR UPDATE`, companyID, input.BindingID, input.DefinitionID, input.Handle, input.DescriptorDigest, input.PolicyRevision).Scan(&existingID)
	if errors.Is(err, pgx.ErrNoRows) {
		existingID = newID()
		err = nil
	}
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	var qualifiedAt any
	if input.Status == "approved" {
		qualifiedAt = time.Now().UTC()
	}
	var q MCPToolCallQualification
	err = tx.QueryRow(ctx, `INSERT INTO mcp_tool_call_qualifications(company_id,id,binding_id,definition_id,server_id,tool_name,handle,descriptor_digest,policy_revision,effect_class,status,authorization_ref,transport,qualified_by,qualified_at,notes,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,clock_timestamp())
ON CONFLICT(company_id,id) DO UPDATE SET effect_class=EXCLUDED.effect_class,status=EXCLUDED.status,authorization_ref=EXCLUDED.authorization_ref,transport=EXCLUDED.transport,qualified_by=EXCLUDED.qualified_by,qualified_at=COALESCE(EXCLUDED.qualified_at,mcp_tool_call_qualifications.qualified_at),notes=EXCLUDED.notes,updated_at=clock_timestamp()
RETURNING id,binding_id,definition_id,server_id,tool_name,handle,descriptor_digest,policy_revision,effect_class,status,authorization_ref,transport,qualified_by,qualified_at::text,expires_at::text,notes,created_at::text,updated_at::text`,
		companyID, existingID, input.BindingID, input.DefinitionID, serverID, toolName, input.Handle, input.DescriptorDigest, input.PolicyRevision, input.EffectClass, input.Status, input.AuthorizationRef, defTransport, input.QualifiedBy, qualifiedAt, input.Notes,
	).Scan(&q.ID, &q.BindingID, &q.DefinitionID, &q.ServerID, &q.ToolName, &q.Handle, &q.DescriptorDigest, &q.PolicyRevision, &q.EffectClass, &q.Status, &q.AuthorizationRef, &q.Transport, &q.QualifiedBy, &q.QualifiedAt, &q.ExpiresAt, &q.Notes, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return MCPToolCallQualification{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPToolCallQualification{}, err
	}
	q.CompanyID = companyID
	return q, nil
}

func cachedDescriptorFromQualification(qualification json.RawMessage, toolName string) (*CachedToolDescriptor, int, error) {
	if len(qualification) == 0 {
		return nil, 0, nil
	}
	var receipt struct {
		ProbeStatus     string                 `json:"probeStatus"`
		ToolNames       []string               `json:"toolNames"`
		ToolsCalled     int                    `json:"toolsCalled"`
		ToolDescriptors []CachedToolDescriptor `json:"toolDescriptors"`
	}
	if err := json.Unmarshal(qualification, &receipt); err != nil {
		return nil, 0, core.Malformed
	}
	if receipt.ProbeStatus != "passed" {
		return nil, receipt.ToolsCalled, nil
	}
	if receipt.ToolsCalled != 0 {
		return nil, receipt.ToolsCalled, core.Integrity
	}
	foundName := false
	for _, name := range receipt.ToolNames {
		if name == toolName {
			foundName = true
			break
		}
	}
	if !foundName {
		return nil, 0, nil
	}
	for i := range receipt.ToolDescriptors {
		if receipt.ToolDescriptors[i].Name == toolName {
			checked, err := checkedToolDescriptor(receipt.ToolDescriptors[i])
			if err != nil {
				return nil, 0, err
			}
			return &checked, 0, nil
		}
	}
	return nil, 0, nil
}

func (k *Kernel) TXCallMCPTool(ctx context.Context, req ToolCallRequest) (MCPToolCallReceipt, error) {
	req, err := validateToolCallRequest(req)
	if err != nil {
		return MCPToolCallReceipt{}, err
	}
	if req.TaskID != "" {
		var owner string
		err = k.pool.QueryRow(ctx, `SELECT owner FROM tasks WHERE company_id=$1 AND id=$2`, req.CompanyID, req.TaskID).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return MCPToolCallReceipt{}, core.OutOfScope
		}
		if err != nil {
			return MCPToolCallReceipt{}, err
		}
		if owner != req.EmployeeID {
			return MCPToolCallReceipt{}, core.Denied
		}
	}
	if strings.TrimSpace(req.SessionID) != "" {
		if err := k.validateToolCallSessionContext(ctx, req); err != nil {
			return MCPToolCallReceipt{}, err
		}
	}
	requestDigest := toolCallRequestDigest(req)
	argsDigest := toolCallArgumentsDigest(req.Arguments)
	if existing, found, err := k.findToolCallReceipt(ctx, req.CompanyID, req.EmployeeID, req.Handle, req.IdempotencyKey); err != nil {
		return MCPToolCallReceipt{}, err
	} else if found {
		if existing.RequestDigest != requestDigest {
			return k.insertDeniedToolCallReceipt(ctx, req, requestDigest, argsDigest, "IDEMPOTENCY_CONFLICT", "", nil, "idempotency key reused with different request digest", "", "")
		}
		return existing, nil
	}
	gate, target, qid, err := k.evaluateToolCallGate(ctx, req)
	if err != nil {
		return MCPToolCallReceipt{}, err
	}
	if !gate.OK && gate.Code == "CALL_NOT_QUALIFIED" && target.definitionID != "" {
		if _, qerr := k.TXUpsertMCPToolCallQualification(ctx, req.CompanyID, MCPToolCallQualificationInput{
			BindingID:        req.BindingID,
			DefinitionID:     target.definitionID,
			Handle:           req.Handle,
			DescriptorDigest: req.DescriptorDigest,
			PolicyRevision:   req.PolicyRevision,
			EffectClass:      "read",
			Status:           "approved",
			AuthorizationRef: req.AuthorizationRef,
			QualifiedBy:      req.EmployeeID,
			Notes:            "auto-qualified",
		}); qerr == nil {
			gate, target, qid, err = k.evaluateToolCallGate(ctx, req)
			if err != nil {
				return MCPToolCallReceipt{}, err
			}
		}
	}
	if !gate.OK {
		return k.insertDeniedToolCallReceipt(ctx, req, requestDigest, argsDigest, gate.Code, target.definitionID, qid, gate.Detail, gate.DetailPath, gate.DetailReason)
	}
	// Execute outside the gate read transaction. Qualified local transports only.
	started := time.Now().UTC()
	var (
		callResult StdioCallResult
		callErr    error
	)
	switch target.transport {
	case "stdio":
		if target.command == "" {
			return k.insertDeniedToolCallReceipt(ctx, req, requestDigest, argsDigest, "TRANSPORT_NOT_QUALIFIED", target.definitionID, qid, "stdio command not available", "", "")
		}
		callResult, callErr = CallStdioMCPTool(ctx, target.command, target.args, gate.ToolName, req.Arguments)
	case "streamable_http":
		if target.endpoint == "" {
			return k.insertDeniedToolCallReceipt(ctx, req, requestDigest, argsDigest, "TRANSPORT_NOT_QUALIFIED", target.definitionID, qid, "http endpoint not available", "", "")
		}
		callResult, callErr = CallStreamableHTTPMCPTool(ctx, target.endpoint, gate.ToolName, req.Arguments)
	default:
		return k.insertDeniedToolCallReceipt(ctx, req, requestDigest, argsDigest, "TRANSPORT_NOT_QUALIFIED", target.definitionID, qid, "transport not qualified", "", "")
	}
	finished := time.Now().UTC()
	return k.insertExecutedToolCallReceipt(ctx, req, requestDigest, argsDigest, gate, target, qid, started, finished, callResult, callErr)
}

type toolCallTarget struct {
	definitionID string
	transport    string
	command      string
	args         []string
	endpoint     string
}

func (k *Kernel) findToolCallReceipt(ctx context.Context, companyID, employeeID, handle, idempotencyKey string) (MCPToolCallReceipt, bool, error) {
	var r MCPToolCallReceipt
	var taskID, sessionID, qid *string
	var sessionEpoch *int64
	var resultJSON, errorJSON []byte
	var startedAt, finishedAt *string
	err := k.pool.QueryRow(ctx, `SELECT id,employee_id,task_id,session_id,session_epoch,session_incarnation,binding_id,definition_id,qualification_id,handle,descriptor_digest,arguments_digest,request_digest,idempotency_key,policy_revision,authorization_ref,effect_class,outcome,code,result_digest,result_json,error_json,process_observed,egress_observed,started_at::text,finished_at::text,created_at::text
FROM mcp_tool_call_receipts WHERE company_id=$1 AND employee_id=$2 AND handle=$3 AND idempotency_key=$4`, companyID, employeeID, handle, idempotencyKey).Scan(&r.ID, &r.EmployeeID, &taskID, &sessionID, &sessionEpoch, &r.SessionIncarnation, &r.BindingID, &r.DefinitionID, &qid, &r.Handle, &r.DescriptorDigest, &r.ArgumentsDigest, &r.RequestDigest, &r.IdempotencyKey, &r.PolicyRevision, &r.AuthorizationRef, &r.EffectClass, &r.Outcome, &r.Code, &r.ResultDigest, &resultJSON, &errorJSON, &r.ProcessObserved, &r.EgressObserved, &startedAt, &finishedAt, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPToolCallReceipt{}, false, nil
	}
	if err != nil {
		return MCPToolCallReceipt{}, false, err
	}
	r.CompanyID = companyID
	r.TaskID = taskID
	r.SessionID = sessionID
	r.SessionEpoch = sessionEpoch
	r.QualificationID = qid
	r.ResultJSON = resultJSON
	r.ErrorJSON = errorJSON
	r.StartedAt = startedAt
	r.FinishedAt = finishedAt
	return r, true, nil
}

func (k *Kernel) evaluateToolCallGate(ctx context.Context, req ToolCallRequest) (ToolCallGateResult, toolCallTarget, *string, error) {
	serverID, toolName, err := splitToolHandle(req.Handle)
	if err != nil {
		return ToolCallGateResult{Code: "MALFORMED"}, toolCallTarget{}, nil, nil
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ToolCallGateResult{}, toolCallTarget{}, nil, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, req.CompanyID); err != nil {
		return ToolCallGateResult{}, toolCallTarget{}, nil, err
	}

	gateInput := ToolCallGateInput{Request: req}
	var bindingStatus string
	var boundMCPRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,mcp_definition_ids FROM employee_capability_bindings WHERE company_id=$1 AND id=$2 AND employee_id=$3`, req.CompanyID, req.BindingID, req.EmployeeID).Scan(&bindingStatus, &boundMCPRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		gateInput.BindingFound = false
	} else if err != nil {
		return ToolCallGateResult{}, toolCallTarget{}, nil, err
	} else {
		gateInput.BindingFound = true
		gateInput.BindingStatus = bindingStatus
		if err = json.Unmarshal(boundMCPRaw, &gateInput.BindingMCPIDs); err != nil {
			return ToolCallGateResult{}, toolCallTarget{}, nil, core.Malformed
		}
	}

	var target toolCallTarget
	if gateInput.BindingFound {
		var def toolCatalogSource
		var command, endpoint *string
		var argsRaw []byte
		var transport string
		err = tx.QueryRow(ctx, `SELECT id,name,status,transport,endpoint,command,args,descriptor_digest,qualification FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, req.CompanyID, serverID).Scan(&def.ID, &def.Name, &def.Status, &transport, &endpoint, &command, &argsRaw, &def.DescriptorDigest, &def.Qualification)
		if errors.Is(err, pgx.ErrNoRows) {
			gateInput.Definition = nil
		} else if err != nil {
			return ToolCallGateResult{}, toolCallTarget{}, nil, err
		} else {
			gateInput.Definition = &def
			target.definitionID = def.ID
			target.transport = transport
			if transport == "stdio" && command != nil {
				target.command = *command
				if len(argsRaw) > 0 {
					if err = json.Unmarshal(argsRaw, &target.args); err != nil {
						return ToolCallGateResult{}, toolCallTarget{}, nil, core.Malformed
					}
				}
			}
			if transport == "streamable_http" && endpoint != nil {
				target.endpoint = *endpoint
			}
			desc, _, derr := cachedDescriptorFromQualification(def.Qualification, toolName)
			if derr != nil {
				if errors.Is(derr, core.Integrity) {
					if err = tx.Commit(ctx); err != nil {
						return ToolCallGateResult{}, toolCallTarget{}, nil, err
					}
					return ToolCallGateResult{Code: "DESCRIPTOR_INTEGRITY", DefinitionID: def.ID, ServerID: serverID, ToolName: toolName}, target, nil, nil
				}
				return ToolCallGateResult{}, toolCallTarget{}, nil, derr
			}
			gateInput.Descriptor = desc
		}
	}

	var q MCPToolCallQualification
	var qid, qAuth, qTransport, qBy, qNotes, qCreated, qUpdated string
	var qTaskAt, qExp *string
	err = tx.QueryRow(ctx, `SELECT id,binding_id,definition_id,server_id,tool_name,handle,descriptor_digest,policy_revision,effect_class,status,authorization_ref,transport,qualified_by,qualified_at::text,expires_at::text,notes,created_at::text,updated_at::text
FROM mcp_tool_call_qualifications WHERE company_id=$1 AND binding_id=$2 AND definition_id=$3 AND handle=$4 AND descriptor_digest=$5 AND policy_revision=$6 AND status='approved' ORDER BY updated_at DESC LIMIT 1`,
		req.CompanyID, req.BindingID, serverID, req.Handle, req.DescriptorDigest, req.PolicyRevision).Scan(&qid, &q.BindingID, &q.DefinitionID, &q.ServerID, &q.ToolName, &q.Handle, &q.DescriptorDigest, &q.PolicyRevision, &q.EffectClass, &q.Status, &qAuth, &qTransport, &qBy, &qTaskAt, &qExp, &qNotes, &qCreated, &qUpdated)
	if errors.Is(err, pgx.ErrNoRows) {
		gateInput.Qualification = nil
	} else if err != nil {
		return ToolCallGateResult{}, toolCallTarget{}, nil, err
	} else {
		q.CompanyID = req.CompanyID
		q.ID = qid
		q.AuthorizationRef = qAuth
		q.Transport = qTransport
		q.QualifiedBy = qBy
		q.Notes = qNotes
		q.CreatedAt = qCreated
		q.UpdatedAt = qUpdated
		q.QualifiedAt = qTaskAt
		q.ExpiresAt = qExp
		gateInput.Qualification = &q
	}

	result := toolCallGate(gateInput)
	if err = tx.Commit(ctx); err != nil {
		return ToolCallGateResult{}, toolCallTarget{}, nil, err
	}
	var qualID *string
	if gateInput.Qualification != nil {
		qualID = &gateInput.Qualification.ID
	}
	return result, target, qualID, nil
}

func (k *Kernel) insertDeniedToolCallReceipt(ctx context.Context, req ToolCallRequest, requestDigest, argsDigest, code, definitionID string, qualificationID *string, detail, detailPath, detailReason string) (MCPToolCallReceipt, error) {
	r := MCPToolCallReceipt{
		CompanyID: req.CompanyID, ID: newID(), EmployeeID: req.EmployeeID, BindingID: req.BindingID, DefinitionID: definitionID,
		QualificationID: qualificationID, Handle: req.Handle, DescriptorDigest: req.DescriptorDigest,
		ArgumentsDigest: argsDigest, RequestDigest: requestDigest, IdempotencyKey: req.IdempotencyKey, PolicyRevision: req.PolicyRevision,
		AuthorizationRef: req.AuthorizationRef, EffectClass: "unknown", Outcome: "denied", Code: code,
		ProcessObserved: json.RawMessage(`{}`), EgressObserved: json.RawMessage(`{"external":false}`),
	}
	if detail != "" || detailPath != "" || detailReason != "" {
		payload := map[string]any{}
		if detail != "" {
			payload["message"] = detail
		}
		if detailPath != "" {
			payload["path"] = detailPath
		}
		if detailReason != "" {
			payload["reason"] = detailReason
		}
		if b, err := json.Marshal(payload); err == nil {
			r.ErrorJSON = b
		}
	}
	if req.TaskID != "" {
		r.TaskID = &req.TaskID
	}
	if strings.TrimSpace(req.SessionID) != "" {
		sid := req.SessionID
		r.SessionID = &sid
		if req.SessionEpoch > 0 {
			ep := req.SessionEpoch
			r.SessionEpoch = &ep
		}
		r.SessionIncarnation = strings.TrimSpace(req.SessionIncarnation)
	}
	if definitionID == "" {
		if serverID, _, err := splitToolHandle(req.Handle); err == nil {
			r.DefinitionID = serverID
		}
	}
	err := k.pool.QueryRow(ctx, `INSERT INTO mcp_tool_call_receipts(company_id,id,employee_id,task_id,session_id,session_epoch,session_incarnation,binding_id,definition_id,qualification_id,handle,descriptor_digest,arguments_digest,request_digest,idempotency_key,policy_revision,authorization_ref,effect_class,outcome,code,error_json,process_observed,egress_observed)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'denied',$19,$20,$21,$22)
ON CONFLICT(company_id,employee_id,handle,idempotency_key) DO UPDATE SET company_id=EXCLUDED.company_id
RETURNING created_at::text`, r.CompanyID, r.ID, r.EmployeeID, r.TaskID, r.SessionID, r.SessionEpoch, r.SessionIncarnation, r.BindingID, r.DefinitionID, r.QualificationID, r.Handle, r.DescriptorDigest, r.ArgumentsDigest, r.RequestDigest, r.IdempotencyKey, r.PolicyRevision, r.AuthorizationRef, r.EffectClass, r.Code, r.ErrorJSON, r.ProcessObserved, r.EgressObserved).Scan(&r.CreatedAt)
	if err != nil {
		return MCPToolCallReceipt{}, err
	}
	// If conflict updated nothing meaningful, re-read existing.
	if existing, found, ferr := k.findToolCallReceipt(ctx, req.CompanyID, req.EmployeeID, req.Handle, req.IdempotencyKey); ferr == nil && found {
		if existing.RequestDigest != requestDigest {
			existing.Outcome = "denied"
			existing.Code = "IDEMPOTENCY_CONFLICT"
			return existing, nil
		}
		return existing, nil
	}
	return r, nil
}

func (k *Kernel) insertExecutedToolCallReceipt(ctx context.Context, req ToolCallRequest, requestDigest, argsDigest string, gate ToolCallGateResult, target toolCallTarget, qualificationID *string, started, finished time.Time, callResult StdioCallResult, callErr error) (MCPToolCallReceipt, error) {
	definitionID := target.definitionID
	transport := target.transport
	if transport == "" {
		transport = "stdio"
	}
	outcome := "completed"
	code := ""
	var resultJSON json.RawMessage
	var errorJSON json.RawMessage
	resultDigest := ""
	processObserved, _ := json.Marshal(map[string]any{"started": true, "transport": transport, "stderrSummary": callResult.StderrSummary, "toolsCalled": callResult.ToolsCalled})
	localOnly := transport == "stdio"
	egressObserved, _ := json.Marshal(map[string]any{"external": !localOnly, "transport": transport, "localOnly": localOnly})
	if callErr != nil {
		if errors.Is(callErr, context.DeadlineExceeded) {
			outcome = "timeout"
			code = "TIMEOUT"
		} else if errors.Is(callErr, core.TooLarge) {
			outcome = "failed"
			code = "RESULT_TOO_LARGE"
		} else {
			outcome = "failed"
			code = "MCP_TOOL_ERROR"
		}
		errObj, _ := json.Marshal(map[string]any{"message": callErr.Error()})
		errorJSON = errObj
	} else if callResult.IsError {
		outcome = "failed"
		code = "MCP_TOOL_ERROR"
		resultJSON = boundedJSON(callResult.Result, maxToolCallResult)
	} else {
		resultJSON = boundedJSON(callResult.Result, maxToolCallResult)
		if len(resultJSON) > 0 {
			sum := contentDigest(resultJSON)
			resultDigest = sum
		}
	}
	r := MCPToolCallReceipt{
		CompanyID: req.CompanyID, ID: newID(), EmployeeID: req.EmployeeID, BindingID: req.BindingID, DefinitionID: definitionID,
		QualificationID: qualificationID, Handle: req.Handle, DescriptorDigest: req.DescriptorDigest,
		ArgumentsDigest: argsDigest, RequestDigest: requestDigest, IdempotencyKey: req.IdempotencyKey, PolicyRevision: req.PolicyRevision,
		AuthorizationRef: req.AuthorizationRef, EffectClass: gate.EffectClass, Outcome: outcome, Code: code,
		ResultDigest: resultDigest, ResultJSON: resultJSON, ErrorJSON: errorJSON, ProcessObserved: processObserved, EgressObserved: egressObserved,
	}
	if req.TaskID != "" {
		r.TaskID = &req.TaskID
	}
	if strings.TrimSpace(req.SessionID) != "" {
		sid := req.SessionID
		r.SessionID = &sid
		if req.SessionEpoch > 0 {
			ep := req.SessionEpoch
			r.SessionEpoch = &ep
		}
		r.SessionIncarnation = strings.TrimSpace(req.SessionIncarnation)
	}
	startedText := started.UTC().Format(time.RFC3339Nano)
	finishedText := finished.UTC().Format(time.RFC3339Nano)
	r.StartedAt = &startedText
	r.FinishedAt = &finishedText
	err := k.pool.QueryRow(ctx, `INSERT INTO mcp_tool_call_receipts(company_id,id,employee_id,task_id,session_id,session_epoch,session_incarnation,binding_id,definition_id,qualification_id,handle,descriptor_digest,arguments_digest,request_digest,idempotency_key,policy_revision,authorization_ref,effect_class,outcome,code,result_digest,result_json,error_json,process_observed,egress_observed,started_at,finished_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)
ON CONFLICT(company_id,employee_id,handle,idempotency_key) DO UPDATE SET company_id=EXCLUDED.company_id
RETURNING created_at::text`, r.CompanyID, r.ID, r.EmployeeID, r.TaskID, r.SessionID, r.SessionEpoch, r.SessionIncarnation, r.BindingID, r.DefinitionID, r.QualificationID, r.Handle, r.DescriptorDigest, r.ArgumentsDigest, r.RequestDigest, r.IdempotencyKey, r.PolicyRevision, r.AuthorizationRef, r.EffectClass, r.Outcome, r.Code, r.ResultDigest, r.ResultJSON, r.ErrorJSON, r.ProcessObserved, r.EgressObserved, started, finished).Scan(&r.CreatedAt)
	if err != nil {
		return MCPToolCallReceipt{}, err
	}
	if existing, found, ferr := k.findToolCallReceipt(ctx, req.CompanyID, req.EmployeeID, req.Handle, req.IdempotencyKey); ferr == nil && found {
		return existing, nil
	}
	return r, nil
}

func boundedJSON(raw json.RawMessage, max int) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) <= max {
		return append(json.RawMessage(nil), raw...)
	}
	wrapper, _ := json.Marshal(map[string]any{"truncated": true, "bytes": len(raw)})
	return wrapper
}

type ListMCPToolCallReceiptsInput struct {
	BindingID string
	Handle    string
	Limit     int
}

func (k *Kernel) TXListMCPToolCallReceipts(ctx context.Context, companyID string, input ListMCPToolCallReceiptsInput) ([]MCPToolCallReceipt, error) {
	if !core.ValidID(companyID) {
		return nil, core.Malformed
	}
	if input.BindingID != "" && !core.ValidID(input.BindingID) {
		return nil, core.Malformed
	}
	if input.Handle != "" {
		if _, _, err := splitToolHandle(input.Handle); err != nil {
			return nil, err
		}
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := k.pool.Query(ctx, `SELECT id,employee_id,task_id,session_id,session_epoch,session_incarnation,binding_id,definition_id,qualification_id,handle,descriptor_digest,arguments_digest,request_digest,idempotency_key,policy_revision,authorization_ref,effect_class,outcome,code,result_digest,process_observed,egress_observed,started_at::text,finished_at::text,created_at::text
FROM mcp_tool_call_receipts
WHERE company_id=$1 AND ($2='' OR binding_id=$2) AND ($3='' OR handle=$3)
ORDER BY created_at DESC, id DESC
LIMIT $4`, companyID, input.BindingID, input.Handle, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MCPToolCallReceipt{}
	for rows.Next() {
		var r MCPToolCallReceipt
		var taskID, sessionID, qid *string
		var sessionEpoch *int64
		var startedAt, finishedAt *string
		if err = rows.Scan(&r.ID, &r.EmployeeID, &taskID, &sessionID, &sessionEpoch, &r.SessionIncarnation, &r.BindingID, &r.DefinitionID, &qid, &r.Handle, &r.DescriptorDigest, &r.ArgumentsDigest, &r.RequestDigest, &r.IdempotencyKey, &r.PolicyRevision, &r.AuthorizationRef, &r.EffectClass, &r.Outcome, &r.Code, &r.ResultDigest, &r.ProcessObserved, &r.EgressObserved, &startedAt, &finishedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.CompanyID = companyID
		r.TaskID = taskID
		r.SessionID = sessionID
		r.SessionEpoch = sessionEpoch
		r.QualificationID = qid
		r.StartedAt = startedAt
		r.FinishedAt = finishedAt
		items = append(items, r)
	}
	return items, rows.Err()
}

func (k *Kernel) TXGetMCPToolCallReceipt(ctx context.Context, companyID, receiptID string) (MCPToolCallReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(receiptID) {
		return MCPToolCallReceipt{}, core.Malformed
	}
	var r MCPToolCallReceipt
	var taskID, sessionID, qid *string
	var sessionEpoch *int64
	var resultJSON, errorJSON []byte
	var startedAt, finishedAt *string
	err := k.pool.QueryRow(ctx, `SELECT id,employee_id,task_id,session_id,session_epoch,session_incarnation,binding_id,definition_id,qualification_id,handle,descriptor_digest,arguments_digest,request_digest,idempotency_key,policy_revision,authorization_ref,effect_class,outcome,code,result_digest,result_json,error_json,process_observed,egress_observed,started_at::text,finished_at::text,created_at::text
FROM mcp_tool_call_receipts WHERE company_id=$1 AND id=$2`, companyID, receiptID).Scan(&r.ID, &r.EmployeeID, &taskID, &sessionID, &sessionEpoch, &r.SessionIncarnation, &r.BindingID, &r.DefinitionID, &qid, &r.Handle, &r.DescriptorDigest, &r.ArgumentsDigest, &r.RequestDigest, &r.IdempotencyKey, &r.PolicyRevision, &r.AuthorizationRef, &r.EffectClass, &r.Outcome, &r.Code, &r.ResultDigest, &resultJSON, &errorJSON, &r.ProcessObserved, &r.EgressObserved, &startedAt, &finishedAt, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPToolCallReceipt{}, core.OutOfScope
	}
	if err != nil {
		return MCPToolCallReceipt{}, err
	}
	r.CompanyID = companyID
	r.TaskID = taskID
	r.SessionID = sessionID
	r.SessionEpoch = sessionEpoch
	r.QualificationID = qid
	r.ResultJSON = redactBoundedJSON(resultJSON, maxToolCallResult)
	r.ErrorJSON = redactBoundedJSON(errorJSON, maxToolCallResult)
	r.StartedAt = startedAt
	r.FinishedAt = finishedAt
	return r, nil
}

func redactBoundedJSON(raw json.RawMessage, max int) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > max {
		raw = boundedJSON(raw, max)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return boundedJSON(raw, max)
	}
	redacted := redactSensitiveJSONValue(v, 0)
	b, err := json.Marshal(redacted)
	if err != nil {
		return boundedJSON(raw, max)
	}
	return boundedJSON(b, max)
}

func redactSensitiveJSONValue(v any, depth int) any {
	if depth > 16 {
		return "[TRUNCATED_DEPTH]"
	}
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			if isSensitiveReceiptKey(k) {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactSensitiveJSONValue(vv, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = redactSensitiveJSONValue(vv, depth+1)
		}
		return out
	default:
		return v
	}
}

func isSensitiveReceiptKey(key string) bool {
	k := strings.ToLower(key)
	for _, needle := range []string{"authorization", "token", "secret", "password", "passwd", "api_key", "apikey", "cookie", "set-cookie", "credential", "private_key", "privatekey", "access_key", "accesskey", "session"} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

func (k *Kernel) validateToolCallSessionContext(ctx context.Context, req ToolCallRequest) error {
	var state, employeeID, taskID, incarnation string
	var epoch, employeeEpoch int64
	err := k.pool.QueryRow(ctx, `SELECT s.state,s.employee_id,s.task_id,s.epoch,s.incarnation,e.epoch
FROM worker_sessions s JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id
WHERE s.company_id=$1 AND s.id=$2`, req.CompanyID, req.SessionID).Scan(&state, &employeeID, &taskID, &epoch, &incarnation, &employeeEpoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if employeeID != req.EmployeeID {
		return core.Denied
	}
	if req.TaskID != "" && taskID != req.TaskID {
		return core.Denied
	}
	// Fencing: runtime incarnation must match DB session incarnation; session/employee epochs must align.
	if incarnation != k.incarnation || epoch != employeeEpoch {
		return core.StaleEpoch
	}
	if req.SessionEpoch > 0 && req.SessionEpoch != epoch {
		return core.StaleEpoch
	}
	if strings.TrimSpace(req.SessionIncarnation) != "" && req.SessionIncarnation != incarnation {
		return core.StaleEpoch
	}
	switch state {
	case "stopped", "reconcile_required":
		return core.Denied
	}
	return nil
}
