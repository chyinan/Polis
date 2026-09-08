// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"polis/internal/core"
	"polis/internal/dbgen"
	"strings"
)

// ReadSnapshot never takes controller ownership or modifies runtime state.
func ReadSnapshot(ctx context.Context, dsn, company, mission string) (Snapshot, error) {
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return Snapshot{}, e
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "polis_r0_") {
		return Snapshot{}, core.Denied
	}
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return Snapshot{}, e
	}
	defer p.Close()
	k := Kernel{pool: p}
	return k.Snapshot(ctx, Scope{company}, mission)
}

func (k *Kernel) Snapshot(ctx context.Context, s Scope, id string) (Snapshot, error) {
	var out Snapshot
	tx, e := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	out.MissionState, e = missionState(ctx, tx, s, id)
	if e != nil {
		return out, e
	}
	q := dbgen.New(tx)
	tasks, e := q.ListTasks(ctx, dbgen.ListTasksParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return out, e
	}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, Task{t.ID, t.MissionID, t.Owner, t.Kind, t.State, t.Generation})
	}
	out.Employees, e = q.ListEmployees(ctx, s.company)
	if e != nil {
		return out, e
	}
	out.Messages, e = q.ListMessages(ctx, dbgen.ListMessagesParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return out, e
	}
	obs, e := q.ListObligations(ctx, dbgen.ListObligationsParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return out, e
	}
	for _, o := range obs {
		out.Obligations = append(out.Obligations, Obligation{o.ID, o.State})
	}
	arts, e := q.ListArtifacts(ctx, dbgen.ListArtifactsParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return out, e
	}
	for _, a := range arts {
		out.Artifacts = append(out.Artifacts, Artifact{a.ID, a.Digest, a.State, a.Verdict})
	}
	e = tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", s.company).Scan(&out.CompanySeq)
	if e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind='fake.claim'", s.company).Scan(&out.FakeClaims)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}

// This private bootstrap transaction runs on the exact connection holding the
// session advisory lock: losing that connection makes recovery fail, not take over.
func (k *Kernel) txResetFakeState(ctx context.Context) error {
	k.leaseMu.Lock()
	defer k.leaseMu.Unlock()
	if k.lease == nil {
		return core.StaleEpoch
	}
	tx, e := k.lease.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "UPDATE runtime_control SET incarnation=$1 WHERE singleton", k.incarnation)
	if e != nil {
		return e
	}
	rows, e := tx.Query(ctx, "SELECT id FROM companies ORDER BY id FOR UPDATE")
	if e != nil {
		return e
	}
	var companies []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		companies = append(companies, id)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE employees SET epoch=epoch+1")
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE worker_sessions SET state='reconcile_required' WHERE state!='stopped'")
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE tasks t SET state='ready',generation=generation+1 WHERE state='working' AND NOT EXISTS(SELECT 1 FROM worker_sessions s WHERE s.company_id=t.company_id AND s.task_id=t.id AND s.state!='stopped')")
	if e != nil {
		return e
	}
	for _, c := range companies {
		if e = appendEvent(ctx, tx, Scope{c}, "controller.recovered", Receipt{k.incarnation, "fake_only"}); e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	return nil
}

func (k *Kernel) txRecover(ctx context.Context) error {
	if e := k.txResetFakeState(ctx); e != nil {
		return e
	}
	// Files are checked outside the short DB snapshot and before dispatch.
	rows, e := k.pool.Query(ctx, "SELECT company_id,id,digest FROM artifacts ORDER BY company_id,id")
	if e != nil {
		return e
	}
	type ref struct{ c, id, digest string }
	var refs []ref
	for rows.Next() {
		var r ref
		if e = rows.Scan(&r.c, &r.id, &r.digest); e != nil {
			rows.Close()
			return e
		}
		refs = append(refs, r)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	for _, r := range refs {
		_, readErr := readBlob(k.root, r.c, r.digest)
		if readErr == nil {
			continue
		}
		state := "corrupt"
		if errors.Is(readErr, os.ErrNotExist) {
			state = "missing"
		}
		_, e = k.TXWrite(ctx, Scope{r.c}, nil, newID(), "artifact.invalidated", r.id, func(tx pgx.Tx) (Receipt, error) {
			_, e := tx.Exec(ctx, "UPDATE artifacts SET state=$3,verdict='invalidated' WHERE company_id=$1 AND id=$2", r.c, r.id, state)
			return Receipt{r.id, state}, e
		})
		if e != nil {
			return e
		}
	}
	return nil
}
