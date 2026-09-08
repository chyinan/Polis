// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

func (k *Kernel) TXCreateCompany(ctx context.Context, id string) (Scope, error) {
	s := Scope{id}
	if !core.ValidID(id) {
		return Scope{}, core.Malformed
	}
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return s, e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "INSERT INTO companies(id) VALUES($1) ON CONFLICT DO NOTHING", id)
	if e != nil {
		return s, e
	}
	if e = k.guard(ctx, tx, s, nil); e != nil {
		return s, e
	}
	for _, emp := range []string{"emp-planning", "emp-backend", "emp-frontend", "emp-review"} {
		_, e = tx.Exec(ctx, "INSERT INTO employees(company_id,id) VALUES($1,$2) ON CONFLICT DO NOTHING", id, emp)
		if e != nil {
			return s, e
		}
	}
	return s, tx.Commit(ctx)
}
func (k *Kernel) TXCreateMission(ctx context.Context, s Scope, id string) error {
	if !core.ValidID(id) || len(id) > 70 {
		return core.Malformed
	}
	_, e := k.TXWrite(ctx, s, nil, "draft-"+id, "mission.draft", id, func(tx pgx.Tx) (Receipt, error) {
		_, e := tx.Exec(ctx, "INSERT INTO missions(company_id,id,contract) VALUES($1,$2,$3)", s.company, id, core.Contract)
		return Receipt{id, "draft"}, e
	})
	return e
}
func missionState(ctx context.Context, tx pgx.Tx, s Scope, id string) (string, error) {
	var state string
	e := tx.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2", s.company, id).Scan(&state)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", core.OutOfScope
	}
	return state, e
}
func (k *Kernel) TXStartMission(ctx context.Context, s Scope, id, key string) (Receipt, error) {
	return k.TXWrite(ctx, s, nil, key, "mission.start", id, func(tx pgx.Tx) (Receipt, error) {
		state, e := missionState(ctx, tx, s, id)
		if e != nil {
			return Receipt{}, e
		}
		if state == "succeeded" {
			return Receipt{}, core.Denied
		}
		if state != "draft" {
			var activation string
			e = tx.QueryRow(ctx, "SELECT activation_id FROM missions WHERE company_id=$1 AND id=$2", s.company, id).Scan(&activation)
			return Receipt{activation, state}, e
		}
		var occupied bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND state IN ('active','paused'))", s.company).Scan(&occupied)
		if e != nil {
			return Receipt{}, e
		}
		if occupied {
			return Receipt{}, core.Conflict
		}
		activation := newID()
		_, e = tx.Exec(ctx, "UPDATE missions SET state='active',activation_id=$3 WHERE company_id=$1 AND id=$2", s.company, id, activation)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,'emp-planning','bootstrap_plan')", s.company, newID(), id)
		return Receipt{activation, "active"}, e
	})
}
func (k *Kernel) TXSetPaused(ctx context.Context, s Scope, id string, paused bool) error {
	_, e := k.TXWrite(ctx, s, nil, newID(), "mission.pause", struct {
		ID     string
		Paused bool
	}{id, paused}, func(tx pgx.Tx) (Receipt, error) {
		state, e := missionState(ctx, tx, s, id)
		if e != nil {
			return Receipt{}, e
		}
		if state != "active" && state != "paused" {
			return Receipt{}, core.Denied
		}
		state = "active"
		if paused {
			state = "paused"
		}
		_, e = tx.Exec(ctx, "UPDATE missions SET state=$3 WHERE company_id=$1 AND id=$2", s.company, id, state)
		return Receipt{id, state}, e
	})
	return e
}
func (k *Kernel) BindFake(ctx context.Context, s Scope, employee string) (Binding, error) {
	b := Binding{scope: s, employee: employee, incarnation: k.incarnation}
	e := k.pool.QueryRow(ctx, "SELECT epoch FROM employees WHERE company_id=$1 AND id=$2", s.company, employee).Scan(&b.epoch)
	if errors.Is(e, pgx.ErrNoRows) {
		e = core.OutOfScope
	}
	return b, e
}
func taskRow(ctx context.Context, tx pgx.Tx, s Scope, id string) (Task, error) {
	var t Task
	e := tx.QueryRow(ctx, "SELECT id,mission_id,owner,kind,state,generation FROM tasks WHERE company_id=$1 AND id=$2", s.company, id).Scan(&t.ID, &t.Mission, &t.Owner, &t.Kind, &t.State, &t.Generation)
	if errors.Is(e, pgx.ErrNoRows) {
		e = core.OutOfScope
	}
	return t, e
}
func checkWork(ctx context.Context, tx pgx.Tx, b Binding, w Task) (Task, error) {
	t, e := taskRow(ctx, tx, b.scope, w.ID)
	if e != nil {
		return t, e
	}
	if t.Owner != b.employee {
		return t, core.Denied
	}
	if t.Generation != w.Generation {
		return t, core.Conflict
	}
	state, e := missionState(ctx, tx, b.scope, t.Mission)
	if e != nil {
		return t, e
	}
	if state != "active" {
		return t, core.Denied
	}
	return t, nil
}
func (k *Kernel) TXClaim(ctx context.Context, b Binding) (Task, error) {
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Task{}, e
	}
	defer tx.Rollback(ctx)
	if e = k.guard(ctx, tx, b.scope, &b); e != nil {
		return Task{}, e
	}
	var t Task
	e = tx.QueryRow(ctx, `SELECT t.id,t.mission_id,t.owner,t.kind,t.state,t.generation FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE t.company_id=$1 AND t.owner=$2 AND t.kind IN ('bootstrap_plan','compute') AND t.state='ready' AND m.state='active' AND NOT EXISTS(SELECT 1 FROM tasks x WHERE x.company_id=t.company_id AND x.owner=t.owner AND x.state='working') ORDER BY t.id LIMIT 1`, b.scope.company, b.employee).Scan(&t.ID, &t.Mission, &t.Owner, &t.Kind, &t.State, &t.Generation)
	if errors.Is(e, pgx.ErrNoRows) {
		return Task{}, core.Denied
	}
	if e != nil {
		return t, e
	}
	t.Generation++
	t.State = "working"
	_, e = tx.Exec(ctx, "UPDATE tasks SET state='working',generation=$3 WHERE company_id=$1 AND id=$2", b.scope.company, t.ID, t.Generation)
	if e != nil {
		return t, e
	}
	if e = appendEvent(ctx, tx, b.scope, "fake.claim", t); e != nil {
		return t, e
	}
	return t, tx.Commit(ctx)
}
