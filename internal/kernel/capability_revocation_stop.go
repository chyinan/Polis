// pattern: Imperative Shell
package kernel

import (
	"context"

	"polis/internal/core"
)

const capabilityRevocationStopPageSize = 32

// CapabilityRevocationStopCandidate is a WorkerSession captured by an accepted
// capability-revocation snapshot that is not stopped yet or still has a
// dispatching MCP call. The snapshot is the durable retry source; this query
// intentionally does not depend on process-local state.
type CapabilityRevocationStopCandidate struct {
	CompanyID                  string
	SessionID                  string
	State                      string
	HasDispatchingMCPToolCalls bool
}

// CapabilityRevocationStopCandidates returns one bounded keyset page of
// sessions still requiring stop confirmation across all companies.
func (k *Kernel) CapabilityRevocationStopCandidates(ctx context.Context, afterCompanyID, afterSessionID string) ([]CapabilityRevocationStopCandidate, error) {
	if ctx == nil || (afterCompanyID == "") != (afterSessionID == "") ||
		(afterCompanyID != "" && (!core.ValidID(afterCompanyID) || !core.ValidID(afterSessionID))) {
		return nil, core.Malformed
	}
	query := `WITH snapshot_sessions AS (
SELECT DISTINCT r.company_id,r.session_id,w.state,EXISTS (
 SELECT 1 FROM mcp_tool_call_intents i
 JOIN LATERAL (
  SELECT e.status FROM mcp_tool_call_events e
  WHERE e.company_id=i.company_id AND e.intent_id=i.intent_id ORDER BY e.event_seq DESC LIMIT 1
 ) current ON true
 WHERE i.company_id=w.company_id AND i.session_id=w.id AND current.status='dispatching'
) AS has_dispatching_mcp_tool_calls
FROM capability_revocation_sessions r
JOIN worker_sessions w ON w.company_id=r.company_id AND w.id=r.session_id
	WHERE ($1='' OR (r.company_id,r.session_id)>($1,$2))
	)
SELECT company_id,session_id,state,has_dispatching_mcp_tool_calls
FROM snapshot_sessions
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
