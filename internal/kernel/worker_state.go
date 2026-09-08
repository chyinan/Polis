// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/runner"
)

func (k *Kernel) TXCreateProbe(ctx context.Context, s Scope, mission string) (Task, error) {
	if !core.ValidID(mission) {
		return Task{}, core.Malformed
	}
	digest, e := putBlob(k.root, s.company, []byte(fixture.Source))
	if e != nil {
		return Task{}, e
	}
	r, e := k.TXWrite(ctx, s, nil, "probe-"+mission, "probe.create", mission, func(tx pgx.Tx) (Receipt, error) {
		var occupied bool
		e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND state IN ('active','paused'))", s.company).Scan(&occupied)
		if e != nil {
			return Receipt{}, e
		}
		if occupied {
			return Receipt{}, core.Conflict
		}
		id, bootstrap, message := newID(), newID(), newID()
		_, e = tx.Exec(ctx, "INSERT INTO missions(company_id,id,state,activation_id,contract) VALUES($1,$2,'active',$3,$4)", s.company, mission, newID(), fixture.Revision)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state,plan) VALUES($1,$2,$3,'emp-planning','bootstrap_plan','completed',$4)", s.company, bootstrap, mission, []byte(`{"template":"signed-zero@1","owner":"emp-backend","checker":"emp-review","milestones":["signed-number","optional-unit"],"dependencies":[],"assumptions":["disposable fixture"],"ambiguities":[]}`))
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,'emp-backend','compat')", s.company, id, mission)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body) VALUES($1,$2,$3,$4,'emp-planning','emp-backend','request','preserve signed-zero compatibility and implement the authorized formatting milestones')", s.company, message, mission, id)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO obligations(company_id,id,task_id,owner) VALUES($1,$2,$3,'emp-backend')", s.company, message, id)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", s.company, id, digest)
		return Receipt{id, "ready"}, e
	})
	if e != nil {
		return Task{}, e
	}
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Task{}, e
	}
	defer tx.Rollback(ctx)
	return taskRow(ctx, tx, s, r.ID)
}

func (k *Kernel) TXNewWorker(ctx context.Context, s Scope, task, profile string) (Binding, error) {
	var b Binding
	r, e := k.TXWrite(ctx, s, nil, newID(), "worker.restore", []string{task, profile}, func(tx pgx.Tx) (Receipt, error) {
		t, e := taskRow(ctx, tx, s, task)
		if e != nil {
			return Receipt{}, e
		}
		if t.Kind != "compat" || t.State == "completed" {
			return Receipt{}, core.Denied
		}
		ms, e := missionState(ctx, tx, s, t.Mission)
		if e != nil {
			return Receipt{}, e
		}
		if ms != "active" {
			return Receipt{}, core.Denied
		}
		var occupied bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND employee_id=$2 AND state!='stopped')", s.company, t.Owner).Scan(&occupied)
		if e != nil {
			return Receipt{}, e
		}
		if occupied {
			return Receipt{}, core.Denied
		}
		b = Binding{scope: s, employee: t.Owner, incarnation: k.incarnation, session: newID()}
		e = tx.QueryRow(ctx, "UPDATE employees SET epoch=epoch+1 WHERE company_id=$1 AND id=$2 RETURNING epoch", s.company, t.Owner).Scan(&b.epoch)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'restoring')", s.company, b.session, b.employee, task, t.Generation+1, b.epoch, b.incarnation, profile)
		return Receipt{b.session, "restoring"}, e
	})
	_ = r
	return b, e
}

func (k *Kernel) checkSession(ctx context.Context, tx pgx.Tx, b Binding, write bool) (string, error) {
	if b.session == "" || b.incarnation != k.incarnation {
		return "", core.StaleEpoch
	}
	var state, employee, incarnation, mission string
	var epoch, current int64
	e := tx.QueryRow(ctx, `SELECT s.state,s.employee_id,s.epoch,s.incarnation,e.epoch,m.state FROM worker_sessions s JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE s.company_id=$1 AND s.id=$2`, b.scope.company, b.session).Scan(&state, &employee, &epoch, &incarnation, &current, &mission)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", core.OutOfScope
	}
	if e != nil {
		return "", e
	}
	if employee != b.employee || epoch != b.epoch || current != b.epoch || incarnation != b.incarnation {
		return state, core.StaleEpoch
	}
	if write && (state != "active" || mission != "active") {
		return state, core.Denied
	}
	if !write && state != "restoring" && state != "validating" && state != "activation_pending_environment" && state != "active" {
		return state, core.Denied
	}
	return state, nil
}

func (k *Kernel) workerTransition(ctx context.Context, b Binding, from, to, capability string) error {
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker."+to, b.session, func(tx pgx.Tx) (Receipt, error) {
		state, e := k.checkSession(ctx, tx, b, false)
		if e != nil {
			return Receipt{}, e
		}
		if state != from {
			return Receipt{}, core.Denied
		}
		if to == "active" || to == "activation_pending_environment" {
			var mission string
			e = tx.QueryRow(ctx, "SELECT m.state FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&mission)
			if e != nil {
				return Receipt{}, e
			}
			if mission != "active" {
				return Receipt{}, core.Denied
			}
		}
		_, e = tx.Exec(ctx, "UPDATE worker_sessions SET state=$3,capability_digest=COALESCE(NULLIF($4,''),capability_digest) WHERE company_id=$1 AND id=$2", b.scope.company, b.session, to, capability)
		if e != nil {
			return Receipt{}, e
		}
		if to == "active" {
			_, e = tx.Exec(ctx, "UPDATE tasks t SET state='working',generation=s.generation FROM worker_sessions s WHERE t.company_id=s.company_id AND t.id=s.task_id AND s.company_id=$1 AND s.id=$2", b.scope.company, b.session)
		}
		return Receipt{b.session, to}, e
	})
	return e
}
func (k *Kernel) TXValidateWorker(ctx context.Context, b Binding) error {
	return k.workerTransition(ctx, b, "restoring", "validating", "")
}
func (k *Kernel) TXActivateWorker(ctx context.Context, b Binding, capability string) error {
	if capability == "" {
		return core.Denied
	}
	var attached bool
	if e := k.pool.QueryRow(ctx, "SELECT process_pid IS NOT NULL FROM worker_sessions WHERE company_id=$1 AND id=$2", b.scope.company, b.session).Scan(&attached); e != nil {
		return e
	}
	if !attached {
		return core.Denied
	}
	if e := k.workerTransition(ctx, b, "validating", "activation_pending_environment", ""); e != nil {
		return e
	}
	// Native fixture is read-only; the only write capability is this mediated binding.
	return k.workerTransition(ctx, b, "activation_pending_environment", "active", capability)
}
func (k *Kernel) TXBeginStop(ctx context.Context, b Binding) error {
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.stopping", b.session, func(tx pgx.Tx) (Receipt, error) {
		_, e := tx.Exec(ctx, "UPDATE worker_sessions SET state='stopping' WHERE company_id=$1 AND id=$2 AND state NOT IN ('stopped','reconcile_required')", b.scope.company, b.session)
		return Receipt{b.session, "stopping"}, e
	})
	return e
}
func (k *Kernel) TXConfirmStopped(ctx context.Context, b Binding, proof runner.StopProof) error {
	if !proof.For(b.session) {
		return core.Denied
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.stopped", b.session, func(tx pgx.Tx) (Receipt, error) {
		tag, e := tx.Exec(ctx, "UPDATE worker_sessions SET state='stopped',stop_receipt=$3 WHERE company_id=$1 AND id=$2 AND state='stopping' AND process_pid=$4", b.scope.company, b.session, proof.Description(), proof.PID())
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Denied
		}
		return Receipt{b.session, "stopped"}, nil
	})
	return e
}

func (k *Kernel) TXAttachWorker(ctx context.Context, b Binding, p *runner.Process) error {
	id, pid := p.Identity()
	if id != b.session {
		return core.Denied
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.process_attached", id, func(tx pgx.Tx) (Receipt, error) {
		if _, e := k.checkSession(ctx, tx, b, false); e != nil {
			return Receipt{}, e
		}
		tag, e := tx.Exec(ctx, "UPDATE worker_sessions SET process_pid=$3 WHERE company_id=$1 AND id=$2 AND state IN ('restoring','validating') AND process_pid IS NULL", b.scope.company, id, pid)
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Denied
		}
		return Receipt{id, "attached"}, nil
	})
	return e
}

func (k *Kernel) Handover(ctx context.Context, b Binding) (HandoverBundle, error) {
	out := HandoverBundle{EmployeeID: b.employee, Contract: fixture.Revision}
	tx, e := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	if _, e = k.checkSession(ctx, tx, b, false); e != nil {
		return out, e
	}
	var task string
	e = tx.QueryRow(ctx, "SELECT task_id FROM worker_sessions WHERE company_id=$1 AND id=$2", b.scope.company, b.session).Scan(&task)
	if e != nil {
		return out, e
	}
	out.Task, e = taskRow(ctx, tx, b.scope, task)
	if e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", b.scope.company, task).Scan(&out.Workspace.Digest, &out.Workspace.Revision)
	if e != nil {
		return out, e
	}
	rows, e := tx.Query(ctx, "SELECT id,state FROM obligations WHERE company_id=$1 AND task_id=$2 ORDER BY id", b.scope.company, task)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var o Obligation
		if e = rows.Scan(&o.ID, &o.State); e != nil {
			rows.Close()
			return out, e
		}
		out.Obligations = append(out.Obligations, o)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows, e = tx.Query(ctx, `SELECT c.data FROM worker_checkpoints c JOIN worker_sessions s ON s.company_id=c.company_id AND s.id=c.session_id WHERE c.company_id=$1 AND s.task_id=$2 ORDER BY s.epoch,c.id`, b.scope.company, task)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return out, e
		}
		var c Checkpoint
		if e = json.Unmarshal(raw, &c); e != nil {
			rows.Close()
			return out, e
		}
		out.Checkpoints = append(out.Checkpoints, c)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", b.scope.company).Scan(&out.CompanySeq)
	if e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	content, e := readBlob(k.root, b.scope.company, out.Workspace.Digest)
	out.Workspace.Content = string(content)
	return out, e
}
func (k *Kernel) Workspace(ctx context.Context, b Binding) (Workspace, error) {
	h, e := k.Handover(ctx, b)
	return h.Workspace, e
}

func (k *Kernel) TXReplace(ctx context.Context, b Binding, key, expected, content string) (Receipt, error) {
	if len(content) == 0 || len(content) > 4096 {
		return Receipt{}, core.TooLarge
	}
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Receipt{}, e
	}
	e = k.guard(ctx, tx, b.scope, &b)
	tx.Rollback(ctx)
	if e != nil {
		return Receipt{}, e
	}
	digest, e := putBlob(k.root, b.scope.company, []byte(content))
	if e != nil {
		return Receipt{}, e
	}
	return k.TXWrite(ctx, b.scope, &b, key, "workspace.replace", []string{expected, digest}, func(tx pgx.Tx) (Receipt, error) {
		tag, e := tx.Exec(ctx, "UPDATE worker_workspaces w SET digest=$3,revision=revision+1 FROM worker_sessions s WHERE w.company_id=s.company_id AND w.task_id=s.task_id AND s.company_id=$1 AND s.id=$2 AND w.digest=$4", b.scope.company, b.session, digest, expected)
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Conflict
		}
		return Receipt{digest, "persisted"}, nil
	})
}
