// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// CheckpointEvidence binds a receipt to its session and the workspace digest
// current at query time. It is a trusted control assertion, not a worker tool.
func (k *Kernel) CheckpointEvidence(ctx context.Context, b Binding, id string) (bool, error) {
	if id == "" {
		return false, core.Malformed
	}
	var ok bool
	e := k.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_checkpoints c JOIN worker_workspaces w ON w.company_id=c.company_id AND w.task_id=(SELECT task_id FROM worker_sessions WHERE company_id=c.company_id AND id=c.session_id) WHERE c.company_id=$1 AND c.id=$2 AND c.session_id=$3 AND c.digest=w.digest AND c.data->>'kind'='qualified'
 AND c.data->>'finalization_state'='current'
 AND c.data->>'workspace_revision'=w.revision::text
 AND c.data->>'acceptance_checker_revision'=$4
 AND c.data->>'checkpoint_policy_revision'=$5
 AND c.data->>'artifact_eligibility_policy_revision'=$6
 AND c.data->>'contract_supersession_policy_revision'=$7
 AND EXISTS(SELECT 1 FROM worker_checks x WHERE x.company_id=c.company_id AND x.session_id=c.session_id AND x.digest=c.digest AND x.passed))`, b.scope.company, id, b.session, core.PeerAcceptanceCheckerRevision, core.CheckpointPolicyRevision, core.ArtifactEligibilityPolicyRevision, core.PeerContractSupersessionPolicyRevision).Scan(&ok)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, core.OutOfScope
	}
	return ok, e
}
