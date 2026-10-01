// pattern: Imperative Shell

package kernel

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const (
	maxCapabilityRevocationStatuses = 64
	maxRevocationInventoryItems     = 64
)

// CapabilityRevocationStatus is rebuilt from durable governance records and
// revoke-time session snapshots on every catalog read. It needs no
// process-local cursor or recovery state.
type CapabilityRevocationStatus struct {
	CompanyID               string                        `json:"companyId"`
	RevocationID            string                        `json:"revocationId"`
	Scope                   string                        `json:"scope"`
	CapabilityKind          string                        `json:"capabilityKind"`
	CapabilityID            string                        `json:"capabilityId"`
	VersionDigest           string                        `json:"versionDigest"`
	QualificationID         string                        `json:"qualificationId"`
	EmployeeID              string                        `json:"employeeId,omitempty"`
	Reason                  string                        `json:"reason"`
	Actor                   string                        `json:"actor"`
	AcceptedAt              string                        `json:"acceptedAt"`
	RevocationAccepted      bool                          `json:"revocationAccepted"`
	EffectiveForNewDispatch bool                          `json:"effectiveForNewDispatch"`
	Quiesced                bool                          `json:"quiesced"`
	AffectedSessionCount    int64                         `json:"affectedSessionCount"`
	LiveSessionCount        int64                         `json:"liveSessionCount"`
	Sessions                []CapabilityRevocationSession `json:"sessions"`
	SessionsTruncated       bool                          `json:"sessionsTruncated"`
	MCPCallCount            int64                         `json:"mcpCallCount"`
	DispatchingMCPCallCount int64                         `json:"dispatchingMcpCallCount"`
	MCPCalls                []CapabilityRevocationMCPCall `json:"mcpCalls"`
	MCPCallsTruncated       bool                          `json:"mcpCallsTruncated"`
}

type CapabilityRevocationSession struct {
	SessionID               string `json:"sessionId"`
	EmployeeID              string `json:"employeeId"`
	TaskID                  string `json:"taskId"`
	MissionID               string `json:"missionId"`
	State                   string `json:"state"`
	StateAtRevocation       string `json:"stateAtRevocation,omitempty"`
	SkillLoadCount          int64  `json:"skillLoadCount"`
	MCPCallCount            int64  `json:"mcpCallCount"`
	DispatchingMCPCallCount int64  `json:"dispatchingMcpCallCount"`
}

type CapabilityRevocationMCPCall struct {
	IntentID           string `json:"intentId"`
	SessionID          string `json:"sessionId"`
	EmployeeID         string `json:"employeeId"`
	ToolName           string `json:"toolName"`
	Status             string `json:"status"`
	StatusAtRevocation string `json:"statusAtRevocation,omitempty"`
	ReasonCode         string `json:"reasonCode,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

type capabilityRevocationSource struct {
	revocationID, scope, capabilityKind, capabilityID string
	versionDigest, qualificationID, employeeID        string
	reason, actor, acceptedAt                         string
}

func listCapabilityRevocationStatuses(ctx context.Context, tx pgx.Tx, companyID string) ([]CapabilityRevocationStatus, bool, error) {
	rows, err := tx.Query(ctx, `WITH current_capabilities AS (
 SELECT 'skill'::text AS capability_kind,id AS capability_id,content_digest AS version_digest,status FROM skill_revisions WHERE company_id=$1
 UNION ALL
 SELECT 'mcp'::text,id,descriptor_digest,status FROM mcp_server_definitions WHERE company_id=$1
), latest_decisions AS (
 SELECT DISTINCT ON (d.capability_kind,d.capability_id)
  d.decision_id,d.capability_kind,d.capability_id,d.version_digest,d.qualification_id,d.decision,d.rationale,d.actor,d.created_at
 FROM capability_decisions d WHERE d.company_id=$1
 ORDER BY d.capability_kind,d.capability_id,d.created_at DESC,d.decision_id DESC
), latest_employee_events AS (
 SELECT DISTINCT ON (e.employee_id,e.capability_kind,e.capability_id)
  e.event_id,e.employee_id,e.capability_kind,e.capability_id,e.version_digest,e.qualification_id,e.event,e.reason,e.created_at
 FROM employee_capability_events e WHERE e.company_id=$1
 ORDER BY e.employee_id,e.capability_kind,e.capability_id,e.event_seq DESC
), active_global_revocations AS (
 SELECT d.decision_id AS revocation_id,'capability'::text AS scope,d.capability_kind,d.capability_id,d.version_digest,d.qualification_id,
  ''::text AS employee_id,d.rationale AS reason,d.actor,d.created_at
 FROM latest_decisions d
 JOIN current_capabilities c ON c.capability_kind=d.capability_kind AND c.capability_id=d.capability_id
 WHERE d.decision='revoked' AND c.status='revoked' AND d.version_digest=c.version_digest
), active_employee_revocations AS (
 SELECT e.event_id AS revocation_id,'employee'::text AS scope,e.capability_kind,e.capability_id,e.version_digest,e.qualification_id,
  e.employee_id,e.reason,'local-owner'::text AS actor,e.created_at
 FROM latest_employee_events e
 WHERE e.event='revoked' AND (e.reason<>'capability_revoked' OR NOT EXISTS (
  SELECT 1 FROM active_global_revocations g
  WHERE g.capability_kind=e.capability_kind AND g.capability_id=e.capability_id
 ))
)
SELECT revocation_id,scope,capability_kind,capability_id,version_digest,qualification_id,employee_id,reason,actor,created_at::text
FROM (
 SELECT * FROM active_global_revocations
 UNION ALL
 SELECT * FROM active_employee_revocations
) active
ORDER BY created_at DESC,revocation_id LIMIT $2`, companyID, maxCapabilityRevocationStatuses+1)
	if err != nil {
		return nil, false, err
	}
	sources := make([]capabilityRevocationSource, 0, maxCapabilityRevocationStatuses+1)
	for rows.Next() {
		var source capabilityRevocationSource
		if err = rows.Scan(&source.revocationID, &source.scope, &source.capabilityKind, &source.capabilityID,
			&source.versionDigest, &source.qualificationID, &source.employeeID, &source.reason, &source.actor, &source.acceptedAt); err != nil {
			rows.Close()
			return nil, false, err
		}
		sources = append(sources, source)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(sources) > maxCapabilityRevocationStatuses
	if truncated {
		sources = sources[:maxCapabilityRevocationStatuses]
	}
	statuses := make([]CapabilityRevocationStatus, 0, len(sources))
	for _, source := range sources {
		status := CapabilityRevocationStatus{
			CompanyID: companyID, RevocationID: source.revocationID, Scope: source.scope,
			CapabilityKind: source.capabilityKind, CapabilityID: source.capabilityID,
			VersionDigest: source.versionDigest, QualificationID: source.qualificationID,
			EmployeeID: source.employeeID, Reason: source.reason, Actor: source.actor,
			AcceptedAt: source.acceptedAt, RevocationAccepted: true,
			EffectiveForNewDispatch: true, Sessions: []CapabilityRevocationSession{},
			MCPCalls: []CapabilityRevocationMCPCall{},
		}
		if err = populateCapabilityRevocationInventory(ctx, tx, &status); err != nil {
			return nil, false, err
		}
		statuses = append(statuses, status)
	}
	return statuses, truncated, nil
}

func populateCapabilityRevocationInventory(ctx context.Context, tx pgx.Tx, status *CapabilityRevocationStatus) error {
	sessionUse := capabilityRevocationSessionUseSQL(status.CapabilityKind)
	var snapshotRows int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM capability_revocation_sessions WHERE company_id=$1 AND revocation_id=$2`, status.CompanyID, status.RevocationID).Scan(&snapshotRows); err != nil {
		return err
	}
	useSnapshot := snapshotRows > 0
	var sessionCountQuery, sessionDetailQuery string
	if useSnapshot {
		sessionCountQuery = `SELECT count(*),count(*) FILTER (WHERE w.state<>'stopped')
FROM capability_revocation_sessions r JOIN worker_sessions w ON w.company_id=r.company_id AND w.id=r.session_id
WHERE r.company_id=$1 AND r.revocation_id=$2`
		sessionDetailQuery = `SELECT w.id,w.employee_id,w.task_id,t.mission_id,w.state,r.session_state_at_revocation,r.skill_load_count,
count(c.intent_id),count(c.intent_id) FILTER (WHERE current.status='dispatching')
FROM capability_revocation_sessions r
JOIN worker_sessions w ON w.company_id=r.company_id AND w.id=r.session_id
JOIN tasks t ON t.company_id=w.company_id AND t.id=w.task_id
LEFT JOIN capability_revocation_mcp_calls c ON c.company_id=r.company_id AND c.revocation_id=r.revocation_id AND c.session_id=r.session_id
LEFT JOIN LATERAL (
 SELECT e.status FROM mcp_tool_call_events e WHERE e.company_id=c.company_id AND e.intent_id=c.intent_id ORDER BY e.event_seq DESC LIMIT 1
) current ON true
WHERE r.company_id=$1 AND r.revocation_id=$2
GROUP BY w.id,w.employee_id,w.task_id,t.mission_id,w.state,r.skill_load_count
ORDER BY w.id LIMIT $3`
	} else {
		sessionCountQuery = `WITH uses AS (` + sessionUse + `)
SELECT count(*),count(*) FILTER (WHERE s.state<>'stopped')
			FROM uses u JOIN worker_sessions s ON s.company_id=$1 AND s.id=u.session_id`
		sessionDetailQuery = `WITH uses AS (` + sessionUse + `)
SELECT s.id,s.employee_id,s.task_id,t.mission_id,s.state,''::text AS state_at_revocation,u.skill_load_count,u.mcp_call_count,u.dispatching_mcp_call_count
FROM uses u JOIN worker_sessions s ON s.company_id=$1 AND s.id=u.session_id
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
ORDER BY s.id LIMIT $4`
	}
	sessionCountArgs := []any{status.CompanyID, status.RevocationID}
	if !useSnapshot {
		sessionCountArgs = []any{status.CompanyID, status.CapabilityID, status.EmployeeID}
	}
	if err := tx.QueryRow(ctx, sessionCountQuery, sessionCountArgs...).
		Scan(&status.AffectedSessionCount, &status.LiveSessionCount); err != nil {
		return err
	}
	detailArgs := []any{status.CompanyID, status.RevocationID, maxRevocationInventoryItems + 1}
	if !useSnapshot {
		detailArgs = []any{status.CompanyID, status.CapabilityID, status.EmployeeID, maxRevocationInventoryItems + 1}
	}
	rows, err := tx.Query(ctx, sessionDetailQuery, detailArgs...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item CapabilityRevocationSession
		if err = rows.Scan(&item.SessionID, &item.EmployeeID, &item.TaskID, &item.MissionID, &item.State, &item.StateAtRevocation,
			&item.SkillLoadCount, &item.MCPCallCount, &item.DispatchingMCPCallCount); err != nil {
			rows.Close()
			return err
		}
		status.Sessions = append(status.Sessions, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if len(status.Sessions) > maxRevocationInventoryItems {
		status.Sessions = status.Sessions[:maxRevocationInventoryItems]
		status.SessionsTruncated = true
	}
	if status.CapabilityKind == "mcp" {
		var callsCountQuery, callsDetailQuery string
		if useSnapshot {
			callsCountQuery = `SELECT count(*),count(*) FILTER (WHERE current.status='dispatching')
FROM capability_revocation_mcp_calls c
JOIN LATERAL (SELECT status FROM mcp_tool_call_events e WHERE e.company_id=c.company_id AND e.intent_id=c.intent_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
WHERE c.company_id=$1 AND c.revocation_id=$2`
			callsDetailQuery = `SELECT i.intent_id,i.session_id,i.employee_id,i.tool_name,current.status,c.status_at_revocation,COALESCE(current.reason_code,''),i.created_at::text
FROM capability_revocation_mcp_calls c
JOIN mcp_tool_call_intents i ON i.company_id=c.company_id AND i.intent_id=c.intent_id
JOIN LATERAL (SELECT status,reason_code FROM mcp_tool_call_events e WHERE e.company_id=c.company_id AND e.intent_id=c.intent_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
WHERE c.company_id=$1 AND c.revocation_id=$2
ORDER BY i.created_at DESC,i.intent_id LIMIT $3`
		} else {
			callsCountQuery = `SELECT count(*),count(*) FILTER (WHERE current.status='dispatching')
FROM mcp_tool_call_intents i
JOIN LATERAL (SELECT status FROM mcp_tool_call_events e WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
				WHERE i.company_id=$1 AND i.capability_id=$2 AND ($3='' OR i.employee_id=$3)`
			callsDetailQuery = `SELECT i.intent_id,i.session_id,i.employee_id,i.tool_name,current.status,''::text AS status_at_revocation,COALESCE(current.reason_code,''),i.created_at::text
FROM mcp_tool_call_intents i
JOIN LATERAL (SELECT status,reason_code FROM mcp_tool_call_events e WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
WHERE i.company_id=$1 AND i.capability_id=$2 AND ($3='' OR i.employee_id=$3)
ORDER BY i.created_at DESC,i.intent_id LIMIT $4`
		}
		callArgs := []any{status.CompanyID, status.RevocationID}
		if !useSnapshot {
			callArgs = []any{status.CompanyID, status.CapabilityID, status.EmployeeID}
		}
		if err = tx.QueryRow(ctx, callsCountQuery, callArgs...).Scan(&status.MCPCallCount, &status.DispatchingMCPCallCount); err != nil {
			return err
		}
		callDetailArgs := append(append([]any(nil), callArgs...), maxRevocationInventoryItems+1)
		callRows, queryErr := tx.Query(ctx, callsDetailQuery, callDetailArgs...)
		if queryErr != nil {
			return queryErr
		}
		for callRows.Next() {
			var item CapabilityRevocationMCPCall
			if err = callRows.Scan(&item.IntentID, &item.SessionID, &item.EmployeeID, &item.ToolName, &item.Status, &item.StatusAtRevocation, &item.ReasonCode, &item.CreatedAt); err != nil {
				callRows.Close()
				return err
			}
			status.MCPCalls = append(status.MCPCalls, item)
		}
		callRows.Close()
		if err = callRows.Err(); err != nil {
			return err
		}
		if len(status.MCPCalls) > maxRevocationInventoryItems {
			status.MCPCalls = status.MCPCalls[:maxRevocationInventoryItems]
			status.MCPCallsTruncated = true
		}
	}
	status.Quiesced = status.LiveSessionCount == 0 && status.DispatchingMCPCallCount == 0
	return nil
}

func capabilityRevocationSessionUseSQL(kind string) string {
	if kind == "skill" {
		return `SELECT e.payload->>'sessionId' AS session_id,e.payload->>'employeeId' AS employee_id,
count(DISTINCT e.payload->>'loadReference') AS skill_load_count,0::bigint AS mcp_call_count,0::bigint AS dispatching_mcp_call_count
FROM events e WHERE e.company_id=$1 AND e.kind='capability.skill.loaded' AND e.payload->>'skillId'=$2
AND ($3='' OR e.payload->>'employeeId'=$3)
GROUP BY e.payload->>'sessionId',e.payload->>'employeeId'`
	}
	return `SELECT i.session_id,i.employee_id,0::bigint AS skill_load_count,count(*) AS mcp_call_count,
count(*) FILTER (WHERE current.status='dispatching') AS dispatching_mcp_call_count
FROM mcp_tool_call_intents i
JOIN LATERAL (SELECT status FROM mcp_tool_call_events e WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY e.event_seq DESC LIMIT 1) current ON true
WHERE i.company_id=$1 AND i.capability_id=$2 AND ($3='' OR i.employee_id=$3)
GROUP BY i.session_id,i.employee_id`
}
