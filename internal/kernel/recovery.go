// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"polis/internal/core"
	"polis/internal/dbgen"
	"strings"
	"unicode/utf8"
)

// VerifyHistoricalWorkerFence attempts no business mutation. It constructs a
// binding with the old runtime incarnation and confirms the current kernel
// rejects it before TXWrite can create a receipt or event.
func (k *Kernel) VerifyHistoricalWorkerFence(ctx context.Context, company, session, employee, incarnation string, epoch int64) error {
	b := Binding{scope: Scope{company: company}, session: session, employee: employee, incarnation: incarnation, epoch: epoch}
	_, err := k.TXWrite(ctx, b.scope, &b, "recovery-fence-"+session, "recovery.fence", session, func(pgx.Tx) (Receipt, error) {
		return Receipt{}, nil
	})
	if errors.Is(err, core.StaleEpoch) {
		return nil
	}
	if err == nil {
		return errors.New("historical worker fence unexpectedly allowed a write")
	}
	return fmt.Errorf("historical worker fence returned unexpected error: %w", err)
}

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
	tx, e := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return Snapshot{}, e
	}
	defer tx.Rollback(ctx)
	out, e := snapshotMissionTX(ctx, tx, s, id)
	if e != nil {
		return Snapshot{}, e
	}
	return out, tx.Commit(ctx)
}

func snapshotMissionTX(ctx context.Context, tx pgx.Tx, s Scope, id string) (Snapshot, error) {
	var out Snapshot
	var e error
	out.MissionState, e = missionState(ctx, tx, s, id)
	if e != nil {
		return Snapshot{}, e
	}
	q := dbgen.New(tx)
	tasks, e := q.ListTasks(ctx, dbgen.ListTasksParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return Snapshot{}, e
	}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, Task{ID: t.ID, Mission: t.MissionID, Owner: t.Owner, Kind: core.TaskKind(t.Kind), State: t.State, Generation: t.Generation})
	}
	out.Employees, e = q.ListEmployees(ctx, s.company)
	if e != nil {
		return Snapshot{}, e
	}
	out.Messages, e = q.ListMessages(ctx, dbgen.ListMessagesParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return Snapshot{}, e
	}
	obs, e := q.ListObligations(ctx, dbgen.ListObligationsParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return Snapshot{}, e
	}
	for _, o := range obs {
		out.Obligations = append(out.Obligations, Obligation{o.ID, o.State})
	}
	arts, e := q.ListArtifacts(ctx, dbgen.ListArtifactsParams{CompanyID: s.company, MissionID: id})
	if e != nil {
		return Snapshot{}, e
	}
	for _, a := range arts {
		out.Artifacts = append(out.Artifacts, Artifact{a.ID, a.Digest, a.State, a.Verdict})
	}
	e = tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", s.company).Scan(&out.CompanySeq)
	if e != nil {
		return Snapshot{}, e
	}
	e = tx.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind='fake.claim'", s.company).Scan(&out.FakeClaims)
	if e != nil {
		return Snapshot{}, e
	}
	return out, nil
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
		if e = appendEvent(ctx, tx, Scope{c}, "controller.recovered", Receipt{ID: k.incarnation, Status: "fake_only"}); e != nil {
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
	rows, e := k.pool.Query(ctx, "SELECT company_id,id,digest,artifact_kind,bytes FROM artifacts WHERE state!='revoked' ORDER BY company_id,id")
	if e != nil {
		return e
	}
	type ref struct {
		c, id, digest, kind string
		bytes               int64
	}
	var refs []ref
	for rows.Next() {
		var r ref
		if e = rows.Scan(&r.c, &r.id, &r.digest, &r.kind, &r.bytes); e != nil {
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
		limit := int64(core.MaxContent)
		if r.kind == "workspace_snapshot" {
			limit = 8 << 20
		}
		content, readErr := readBlobBounded(k.root, r.c, r.digest, limit)
		if readErr == nil && int64(len(content)) == r.bytes {
			continue
		}
		state := "corrupt"
		if errors.Is(readErr, os.ErrNotExist) {
			state = "missing"
		}
		_, e = k.TXWrite(ctx, Scope{r.c}, nil, newID(), "artifact.invalidated", r.id, func(tx pgx.Tx) (Receipt, error) {
			_, e := tx.Exec(ctx, `UPDATE artifacts SET state=$3,verdict=CASE WHEN artifact_kind='workspace_snapshot' THEN verdict ELSE 'invalidated' END WHERE company_id=$1 AND id=$2`, r.c, r.id, state)
			return Receipt{ID: r.id, Status: state}, e
		})
		if e != nil {
			return e
		}
	}
	snapshotRows, e := k.pool.Query(ctx, `SELECT DISTINCT a.company_id,a.id,f.digest,f.bytes
FROM artifacts a JOIN workspace_snapshot_files f ON f.company_id=a.company_id AND f.artifact_id=a.id
WHERE a.artifact_kind='workspace_snapshot' AND a.state='ready' ORDER BY a.company_id,a.id,f.digest`)
	if e != nil {
		return e
	}
	type snapshotRef struct {
		company, artifact, digest string
		bytes                     int64
	}
	var snapshotRefs []snapshotRef
	for snapshotRows.Next() {
		var item snapshotRef
		if e = snapshotRows.Scan(&item.company, &item.artifact, &item.digest, &item.bytes); e != nil {
			snapshotRows.Close()
			return e
		}
		snapshotRefs = append(snapshotRefs, item)
	}
	if e = snapshotRows.Err(); e != nil {
		snapshotRows.Close()
		return e
	}
	snapshotRows.Close()
	invalidSnapshots := make(map[string]string)
	for _, item := range snapshotRefs {
		content, readErr := readBlobBounded(k.root, item.company, item.digest, workspaceTreeMaxFileBytes)
		if readErr != nil || int64(len(content)) != item.bytes || !utf8.Valid(content) {
			invalidSnapshots[item.company+"/"+item.artifact] = item.company
		}
	}
	for key, company := range invalidSnapshots {
		artifact := strings.TrimPrefix(key, company+"/")
		_, e = k.TXWrite(ctx, Scope{company}, nil, newID(), "artifact.workspace_snapshot.invalidated", artifact, func(tx pgx.Tx) (Receipt, error) {
			_, updateErr := tx.Exec(ctx, `UPDATE artifacts SET state='corrupt' WHERE company_id=$1 AND id=$2 AND artifact_kind='workspace_snapshot' AND state='ready'`, company, artifact)
			return Receipt{ID: artifact, Status: "corrupt"}, updateErr
		})
		if e != nil {
			return e
		}
	}
	return nil
}

// txRecoverWithRuntimeCASBinding is the activation-safe recovery boundary.
// All artifact payloads are checked before any epoch, incarnation, event, or
// artifact mutation is committed.
func (k *Kernel) txRecoverWithRuntimeCASBinding(ctx context.Context) error {
	rows, err := k.pool.Query(ctx, "SELECT company_id,id,digest,artifact_kind,bytes FROM artifacts WHERE state!='revoked' ORDER BY company_id,id")
	if err != nil {
		return err
	}
	type ref struct {
		company, artifact, digest, kind string
		bytes                           int64
	}
	refs := make([]ref, 0)
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.company, &r.artifact, &r.digest, &r.kind, &r.bytes); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, r := range refs {
		limit := int64(core.MaxContent)
		if r.kind == "workspace_snapshot" {
			limit = 8 << 20
		}
		content, err := readBlobBounded(k.root, r.company, r.digest, limit)
		if err != nil || int64(len(content)) != r.bytes {
			if err == nil {
				err = core.Integrity
			}
			return fmt.Errorf("recovery CAS prerequisite failed for artifact %s: %w", r.artifact, err)
		}
	}
	workspaceRows, err := k.pool.Query(ctx, `SELECT company_id,digest,bytes FROM worker_workspace_files
UNION ALL SELECT f.company_id,f.digest,f.bytes FROM workspace_snapshot_files f
JOIN artifacts a ON a.company_id=f.company_id AND a.id=f.artifact_id
WHERE a.artifact_kind='workspace_snapshot' AND a.state='ready' ORDER BY company_id,digest`)
	if err != nil {
		return err
	}
	type workspaceRef struct {
		company, digest string
		bytes           int64
	}
	var workspaceRefs []workspaceRef
	for workspaceRows.Next() {
		var item workspaceRef
		if err = workspaceRows.Scan(&item.company, &item.digest, &item.bytes); err != nil {
			workspaceRows.Close()
			return err
		}
		workspaceRefs = append(workspaceRefs, item)
	}
	if err = workspaceRows.Err(); err != nil {
		workspaceRows.Close()
		return err
	}
	workspaceRows.Close()
	for _, item := range workspaceRefs {
		content, readErr := readBlobBounded(k.root, item.company, item.digest, workspaceTreeMaxFileBytes)
		if readErr != nil || int64(len(content)) != item.bytes || !utf8.Valid(content) {
			return fmt.Errorf("recovery CAS prerequisite failed for workspace file digest %s", item.digest)
		}
	}
	return k.txResetFakeState(ctx)
}
