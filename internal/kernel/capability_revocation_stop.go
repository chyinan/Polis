// pattern: Imperative Shell
package kernel

import (
	"context"

	"polis/internal/core"
)

const capabilityRevocationStopPageSize = 32

// CapabilityRevocationStopCandidate is a WorkerSession covered by an accepted
// revocation snapshot or legacy active-revocation use ledger that is not
// stopped yet or still has a dispatching MCP call.
type CapabilityRevocationStopCandidate struct {
	CompanyID                  string
	SessionID                  string
	State                      string
	HasDispatchingMCPToolCalls bool
}

// CapabilityRevocationStopCandidates returns one bounded keyset page of
// sessions still requiring stop confirmation across all companies. Existing
// pre-Schema-73 revocations are recovered from their current durable use rows.
func (k *Kernel) CapabilityRevocationStopCandidates(ctx context.Context, afterCompanyID, afterSessionID string) ([]CapabilityRevocationStopCandidate, error) {
	if ctx == nil || (afterCompanyID == "") != (afterSessionID == "") ||
		(afterCompanyID != "" && (!core.ValidID(afterCompanyID) || !core.ValidID(afterSessionID))) {
		return nil, core.Malformed
	}
	query := `WITH current_capabilities AS (
 SELECT company_id,'skill'::text AS capability_kind,id AS capability_id,content_digest AS version_digest,status FROM skill_revisions
 UNION ALL
 SELECT company_id,'mcp'::text,id,descriptor_digest,status FROM mcp_server_definitions
), latest_decisions AS (
 SELECT DISTINCT ON (d.company_id,d.capability_kind,d.capability_id)
  d.company_id,d.decision_id,d.capability_kind,d.capability_id,d.version_digest,d.decision,d.created_at
 FROM capability_decisions d
 ORDER BY d.company_id,d.capability_kind,d.capability_id,d.created_at DESC,d.decision_id DESC
), latest_employee_events AS (
 SELECT DISTINCT ON (e.company_id,e.employee_id,e.capability_kind,e.capability_id)
  e.company_id,e.event_id,e.employee_id,e.capability_kind,e.capability_id,e.event,e.reason
 FROM employee_capability_events e
 ORDER BY e.company_id,e.employee_id,e.capability_kind,e.capability_id,e.event_seq DESC
), active_global_revocations AS (
 SELECT d.company_id,d.capability_kind,d.capability_id,''::text AS employee_id
 FROM latest_decisions d
 JOIN current_capabilities c ON c.company_id=d.company_id AND c.capability_kind=d.capability_kind AND c.capability_id=d.capability_id
 WHERE d.decision='revoked' AND c.status='revoked' AND d.version_digest=c.version_digest
), active_employee_revocations AS (
 SELECT e.company_id,e.capability_kind,e.capability_id,e.employee_id
 FROM latest_employee_events e
 WHERE e.event='revoked' AND (e.reason<>'capability_revoked' OR NOT EXISTS (
  SELECT 1 FROM active_global_revocations g
  WHERE g.company_id=e.company_id AND g.capability_kind=e.capability_kind AND g.capability_id=e.capability_id
 ))
), active_revocations AS (
 SELECT company_id,capability_kind,capability_id,employee_id FROM active_global_revocations
 UNION ALL
 SELECT company_id,capability_kind,capability_id,employee_id FROM active_employee_revocations
), legacy_usage_sessions AS (
 SELECT r.company_id,e.payload->>'sessionId' AS session_id
 FROM active_revocations r
 JOIN events e ON e.company_id=r.company_id AND e.kind='capability.skill.loaded'
 WHERE r.capability_kind='skill' AND e.payload->>'skillId'=r.capability_id
  AND (r.employee_id='' OR e.payload->>'employeeId'=r.employee_id)
 UNION
 SELECT r.company_id,i.session_id
 FROM active_revocations r
 JOIN mcp_tool_call_intents i ON i.company_id=r.company_id AND i.capability_id=r.capability_id
 WHERE r.capability_kind='mcp' AND (r.employee_id='' OR i.employee_id=r.employee_id)
), revoke_sessions AS (
 SELECT r.company_id,r.session_id FROM capability_revocation_sessions r
 UNION
 SELECT u.company_id,u.session_id FROM legacy_usage_sessions u WHERE u.session_id IS NOT NULL
), pending_sessions AS (
SELECT DISTINCT r.company_id,r.session_id,w.state,EXISTS (
 SELECT 1 FROM mcp_tool_call_intents i
 JOIN LATERAL (
  SELECT e.status FROM mcp_tool_call_events e
  WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY e.event_seq DESC LIMIT 1
 ) current ON true
 WHERE i.company_id=w.company_id AND i.session_id=w.id AND current.status='dispatching'
) AS has_dispatching_mcp_tool_calls
FROM revoke_sessions r
JOIN worker_sessions w ON w.company_id=r.company_id AND w.id=r.session_id
WHERE ($1='' OR (r.company_id,r.session_id)>($1,$2))
	)
SELECT company_id,session_id,state,has_dispatching_mcp_tool_calls
FROM pending_sessions
WHERE (state<>'stopped' OR has_dispatching_mcp_tool_calls)
ORDER BY company_id,session_id
LIMIT $3`
	rows, err := k.pool.Query(ctx, query, afterCompanyID, afterSessionID, capabilityRevocationStopPageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]CapabilityRevocationStopCandidate, 0, capabilityRevocationStopPageSize)
	for rows.Next() {
		var candidate CapabilityRevocationStopCandidate
		if err = rows.Scan(&candidate.CompanyID, &candidate.SessionID, &candidate.State, &candidate.HasDispatchingMCPToolCalls); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}
