// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// TXRestoreCandidateForReview imports an already-recorded candidate into a new
// disposable database after the original R0.2 database was cleaned up. It is a
// trusted evidence-reconstruction path, never a worker tool and never a way to
// manufacture a passed verdict. The candidate stays pending until a real
// independent reviewer and the deterministic checker run.
func (k *Kernel) TXRestoreCandidateForReview(ctx context.Context, s Scope, mission, artifactID string, content []byte) (Receipt, error) {
	if len(artifactID) == 0 || len(content) == 0 {
		return Receipt{}, core.Malformed
	}
	digest, e := putBlob(k.root, s.company, content)
	if e != nil {
		return Receipt{}, e
	}
	return k.TXWrite(ctx, s, nil, "restore-"+artifactID, "probe.restore_candidate", []string{mission, artifactID, digest}, func(tx pgx.Tx) (Receipt, error) {
		var taskID string
		e := tx.QueryRow(ctx, "SELECT t.id FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE t.company_id=$1 AND t.mission_id=$2 AND t.owner='emp-backend' AND t.kind='compat'", s.company, mission).Scan(&taskID)
		if errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "UPDATE worker_workspaces SET digest=$3,revision=revision+1 WHERE company_id=$1 AND task_id=$2", s.company, taskID, digest)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2", s.company, taskID)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract) SELECT $1,$2,$3,'emp-backend',$4,$5,'ready','candidate',m.contract FROM missions m WHERE m.company_id=$1 AND m.id=$6 ON CONFLICT (company_id,id) DO NOTHING", s.company, artifactID, taskID, digest, len(content), mission)
		if e != nil {
			return Receipt{}, e
		}
		var exists bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM artifacts WHERE company_id=$1 AND id=$2 AND digest=$3 AND verdict='candidate')", s.company, artifactID, digest).Scan(&exists)
		if e != nil {
			return Receipt{}, e
		}
		if !exists {
			return Receipt{}, core.Conflict
		}
		return Receipt{ID: artifactID, Status: "candidate_recovered"}, nil
	})
}

// TXFinishReview marks the review task completed only after its own compiled
// check receipt passed. It does not alter the source candidate verdict.
func (k *Kernel) TXFinishReview(ctx context.Context, b Binding) (Receipt, error) {
	return k.TXWrite(ctx, b.scope, &b, "review-finish-"+b.session, "review.finish", b.session, func(tx pgx.Tx) (Receipt, error) {
		var task string
		e := tx.QueryRow(ctx, "SELECT task_id FROM worker_sessions WHERE company_id=$1 AND id=$2", b.scope.company, b.session).Scan(&task)
		if e != nil {
			return Receipt{}, e
		}
		var passed bool
		e = tx.QueryRow(ctx, "SELECT COALESCE(bool_and(passed),false) FROM worker_checks WHERE company_id=$1 AND session_id=$2", b.scope.company, b.session).Scan(&passed)
		if e != nil {
			return Receipt{}, e
		}
		if !passed {
			return Receipt{}, core.Denied
		}
		_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", b.scope.company, task)
		return Receipt{ID: task, Status: "completed"}, e
	})
}
