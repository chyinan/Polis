// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// TXCreateReviewProbe creates a fixed emp-review task over the already-fixed
// candidate. It shares the immutable content digest, never the author's write
// session or authority. The review task has no external obligation.
func (k *Kernel) TXCreateReviewProbe(ctx context.Context, s Scope, mission, artifact string) (Task, error) {
	var task Task
	_, e := k.TXWrite(ctx, s, nil, "review-"+artifact, "review.task.create", []string{mission, artifact}, func(tx pgx.Tx) (Receipt, error) {
		var digest, author, sourceTask string
		e := tx.QueryRow(ctx, "SELECT digest,author,task_id FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND verdict='candidate'", s.company, artifact).Scan(&digest, &author, &sourceTask)
		if errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if e != nil {
			return Receipt{}, e
		}
		if author == "emp-review" {
			return Receipt{}, core.Denied
		}
		var missionID string
		e = tx.QueryRow(ctx, "SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2", s.company, sourceTask).Scan(&missionID)
		if e != nil {
			return Receipt{}, e
		}
		if missionID != mission {
			return Receipt{}, core.Conflict
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state,plan) VALUES($1,$2,$3,'emp-review','review','ready',$4)", s.company, id, mission, []byte(`{"review_of":"candidate","independence":"emp-review","contract":"signed-zero@1"}`))
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", s.company, id, digest)
		if e != nil {
			return Receipt{}, e
		}
		task = Task{ID: id, Mission: mission, Owner: "emp-review", Kind: "review", State: "ready", Generation: 0}
		return Receipt{id, "ready"}, nil
	})
	return task, e
}
