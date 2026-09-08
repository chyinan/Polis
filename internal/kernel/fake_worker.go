// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// Step executes at most one bounded script boundary, with no model/provider path.
func (k *Kernel) Step(ctx context.Context, s Scope) (bool, error) {
	for _, employee := range []string{"emp-planning", "emp-backend"} {
		b, e := k.BindFake(ctx, s, employee)
		if e != nil {
			return false, e
		}
		if employee == "emp-backend" {
			var obligation, artifact string
			e = k.pool.QueryRow(ctx, `SELECT o.id,a.id FROM obligations o JOIN artifacts a ON a.company_id=o.company_id AND a.task_id=o.task_id JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE o.company_id=$1 AND o.state='pending' AND a.state='ready' AND m.state='active' LIMIT 1`, s.company).Scan(&obligation, &artifact)
			if e == nil {
				_, e = k.TXResolve(ctx, b, obligation, artifact, "resolve-"+obligation)
				return e == nil, e
			}
			if !errors.Is(e, pgx.ErrNoRows) {
				return false, e
			}
		}
		w, e := k.TXClaim(ctx, b)
		if errors.Is(e, core.Denied) {
			continue
		}
		if e != nil {
			return false, e
		}
		if employee == "emp-planning" {
			_, e = k.TXSend(ctx, b, w, "send-"+w.ID, "compute fixed sum")
		} else {
			_, e = k.TXSubmit(ctx, b, w, "submit-"+w.ID, []byte("Polis R0: 2 + 3 = 5\n"))
		}
		return e == nil, e
	}
	var artifact string
	e := k.pool.QueryRow(ctx, `SELECT a.id FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE a.company_id=$1 AND a.state='ready' AND a.verdict='candidate' AND m.state='active' AND NOT EXISTS(SELECT 1 FROM obligations o WHERE o.company_id=a.company_id AND o.task_id=a.task_id AND o.state='pending') LIMIT 1`, s.company).Scan(&artifact)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	b, e := k.BindFake(ctx, s, "emp-review")
	if e != nil {
		return false, e
	}
	_, e = k.TXVerify(ctx, b, artifact, "verify-"+artifact)
	return e == nil, e
}
