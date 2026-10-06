// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/spec"
)

type TaskSemanticRevisionDecisionInput struct {
	TaskID        string
	Revision      int64
	BindingSHA256 string
	Decision      string
	Rationale     string
	RequestID     string
}

// TXDecideTaskSemanticRevision records an installation-owner decision over an
// exact semantic TaskRevision. Approval/rejection/revocation changes only the
// durable decision ledger; qualification remains unverified and no dispatch
// or Worker admission path consults this decision as execution authority.
func (k *Kernel) TXDecideTaskSemanticRevision(ctx context.Context, companyID string, input TaskSemanticRevisionDecisionInput) (Receipt, error) {
	if k == nil || !core.ValidID(companyID) || !core.ValidID(input.TaskID) || input.Revision < 1 || !validCapabilityDigest(input.BindingSHA256) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	decision := spec.FixedTeamTaskRevisionDecision(strings.TrimSpace(input.Decision))
	if decision != spec.TaskRevisionDecisionApproved && decision != spec.TaskRevisionDecisionRejected && decision != spec.TaskRevisionDecisionRevoked {
		return Receipt{}, core.Malformed
	}
	eventID := stableCapabilityID("task-semantic-decision", companyID, input.TaskID, input.BindingSHA256, input.RequestID)
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "task.semantic_revision.decide", input, func(tx pgx.Tx) (Receipt, error) {
		var tableAvailable bool
		if err := tx.QueryRow(ctx, "SELECT to_regclass('public.task_semantic_revision_decisions') IS NOT NULL").Scan(&tableAvailable); err != nil {
			return Receipt{}, err
		}
		if !tableAvailable {
			return Receipt{}, core.Denied
		}
		var qualification string
		if err := tx.QueryRow(ctx, `SELECT qualification FROM task_semantic_revisions
WHERE company_id=$1 AND task_id=$2 AND revision=$3 AND binding_sha256=$4`, companyID, input.TaskID, input.Revision, input.BindingSHA256).Scan(&qualification); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if qualification != "unverified" {
			return Receipt{}, core.Denied
		}
		var current string
		err := tx.QueryRow(ctx, `SELECT decision FROM task_semantic_revision_decisions
WHERE company_id=$1 AND task_id=$2 AND revision=$3 AND binding_sha256=$4
ORDER BY event_seq DESC LIMIT 1`, companyID, input.TaskID, input.Revision, input.BindingSHA256).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			current = ""
		} else if err != nil {
			return Receipt{}, err
		}
		next, transitionErr := spec.TaskRevisionDecisionTransition(current, decision)
		if transitionErr != nil {
			return Receipt{}, core.ConflictError{Reason: transitionErr.Error(), CurrentState: current}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO task_semantic_revision_decisions(company_id,event_id,task_id,revision,binding_sha256,decision,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'local-owner',$8)`, companyID, eventID, input.TaskID, input.Revision, input.BindingSHA256, decision, strings.TrimSpace(input.Rationale), input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: next, Revision: input.Revision}, nil
	})
}
