// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

func (k *Kernel) TXSend(ctx context.Context, b Binding, w Task, key, body string) (Receipt, error) {
	if len(body) > core.MaxContent {
		return Receipt{}, core.TooLarge
	}
	if body == "" {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, b.scope, &b, key, "collab.send", struct {
		Task       string
		Generation int64
		Body       string
	}{w.ID, w.Generation, body}, func(tx pgx.Tx) (Receipt, error) {
		t, e := checkWork(ctx, tx, b, w)
		if e != nil {
			return Receipt{}, e
		}
		if b.employee != core.EmployeePlanningID || t.Kind != core.TaskKindBootstrapPlan || t.State != "working" {
			return Receipt{}, core.Denied
		}
		work, message := newID(), newID()
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,$4,$5)", b.scope.company, work, t.Mission, core.EmployeeBackendID, core.TaskKindCompute)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body) VALUES($1,$2,$3,$4,$5,'emp-backend','request',$6)", b.scope.company, message, t.Mission, work, b.employee, body)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO obligations(company_id,id,task_id,owner) VALUES($1,$2,$3,'emp-backend')", b.scope.company, message, work)
		if e != nil {
			return Receipt{}, e
		}
		plan := []byte(`{"template":"r0-bootstrap@1","milestones":["compute","verify"],"owner":"emp-backend","checker":"emp-review","dependencies":[],"contract":"r0-arithmetic@1","assumptions":["scripted arithmetic fixture"],"ambiguities":[]}`)
		_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed',plan=$3 WHERE company_id=$1 AND id=$2", b.scope.company, t.ID, plan)
		return Receipt{ID: message, Status: "persisted"}, e
	})
}

func (k *Kernel) TXSubmit(ctx context.Context, b Binding, w Task, key string, content []byte) (Receipt, error) {
	return k.txSubmit(ctx, b, w, key, content, false)
}

func (k *Kernel) TXSubmitProduct(ctx context.Context, b Binding, w Task, key string, content []byte) (Receipt, error) {
	if b.session == "" {
		return Receipt{}, core.Denied
	}
	return k.txSubmit(ctx, b, w, key, content, true)
}

func (k *Kernel) txSubmit(ctx context.Context, b Binding, w Task, key string, content []byte, product bool) (Receipt, error) {
	if len(content) > core.MaxContent {
		return Receipt{}, core.TooLarge
	}
	if len(content) == 0 {
		return Receipt{}, core.Malformed
	}
	if b.session != "" {
		var kind string
		if err := k.pool.QueryRow(ctx, "SELECT kind FROM tasks WHERE company_id=$1 AND id=$2", b.scope.company, w.ID).Scan(&kind); err != nil {
			return Receipt{}, err
		}
		if kind == "peer_backend" || kind == "peer_frontend" {
			return k.TXSubmitQualifiedPeer(ctx, b, w, key, content)
		}
	}
	// Authorize before staging, and revalidate at the database publication boundary.
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Receipt{}, e
	}
	if e = k.guard(ctx, tx, b.scope, &b); e == nil {
		_, e = checkWork(ctx, tx, b, w)
	}
	_ = tx.Rollback(ctx)
	if e != nil {
		return Receipt{}, e
	}
	hash := sha256.Sum256(content)
	expectedDigest := hex.EncodeToString(hash[:])
	stage, e := k.TXWrite(ctx, b.scope, &b, "stage-"+fingerprint(key)[:48], "artifact.stage", []string{w.ID, expectedDigest}, func(tx pgx.Tx) (Receipt, error) {
		t, e := checkWork(ctx, tx, b, w)
		if e != nil {
			return Receipt{}, e
		}
		if t.State != "working" || (t.Kind != core.TaskKindCompute && t.Kind != core.TaskKindCompat && t.Kind != core.TaskKindPeerBackend && t.Kind != core.TaskKindPeerFrontend) {
			return Receipt{}, core.Denied
		}
		if product {
			if t.Kind != core.TaskKindCompat {
				return Receipt{}, core.Denied
			}
			if _, e = k.currentProductQualification(ctx, tx, b, t, expectedDigest); e != nil {
				return Receipt{}, e
			}
		}
		var existing, digest string
		e = tx.QueryRow(ctx, "SELECT id,digest FROM artifact_staging WHERE company_id=$1 AND task_id=$2", b.scope.company, w.ID).Scan(&existing, &digest)
		if e == nil {
			if digest != expectedDigest {
				return Receipt{}, core.Conflict
			}
			return Receipt{ID: existing, Status: "staging"}, nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, e
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO artifact_staging VALUES($1,$2,$3,$4)", b.scope.company, id, w.ID, expectedDigest)
		return Receipt{ID: id, Status: "staging"}, e
	})
	if e != nil {
		return Receipt{}, e
	}
	digest, e := putBlob(k.root, b.scope.company, content)
	if e != nil {
		return Receipt{}, e
	}
	return k.TXWrite(ctx, b.scope, &b, key, "artifact.submit", struct {
		Task       string
		Generation int64
		Digest     string
	}{w.ID, w.Generation, digest}, func(tx pgx.Tx) (Receipt, error) {
		t, e := checkWork(ctx, tx, b, w)
		if e != nil {
			return Receipt{}, e
		}
		if t.State != "working" || (t.Kind != core.TaskKindCompute && t.Kind != core.TaskKindCompat && t.Kind != core.TaskKindPeerBackend && t.Kind != core.TaskKindPeerFrontend) {
			return Receipt{}, core.Denied
		}
		var qualification productQualification
		if product {
			if t.Kind != core.TaskKindCompat {
				return Receipt{}, core.Denied
			}
			qualification, e = k.currentProductQualification(ctx, tx, b, t, digest)
			if e != nil {
				return Receipt{}, e
			}
		}
		id := stage.ID
		_, e = tx.Exec(ctx, "INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,contract) SELECT $1,$2,$3,$4,$5,$6,'ready',contract FROM missions WHERE company_id=$1 AND id=$7", b.scope.company, id, t.ID, b.employee, digest, len(content), t.Mission)
		if e != nil {
			return Receipt{}, e
		}
		if product {
			_, e = tx.Exec(ctx, `INSERT INTO task_validation_artifact_qualifications(company_id,task_id,artifact_id,checkpoint_id,check_id,session_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, b.scope.company, t.ID, id, qualification.CheckpointID, qualification.CheckID, b.session, qualification.BindingDigest, qualification.WorkspaceDigest, qualification.WorkspaceRevision, qualification.RunnerRevision)
			if e != nil {
				return Receipt{}, e
			}
		}
		_, e = tx.Exec(ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2", b.scope.company, t.ID)
		return Receipt{ID: id, Status: "candidate"}, e
	})
}

func (k *Kernel) TXResolve(ctx context.Context, b Binding, obligation, artifact, key string) (Receipt, error) {
	// Evidence content must remain readable; a DB ready flag alone is insufficient.
	var expectedDigest string
	e := k.pool.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready'", b.scope.company, artifact).Scan(&expectedDigest)
	if errors.Is(e, pgx.ErrNoRows) {
		return Receipt{}, core.OutOfScope
	}
	if e != nil {
		return Receipt{}, e
	}
	if _, e = readBlob(k.root, b.scope.company, expectedDigest); e != nil {
		return Receipt{}, core.Integrity
	}
	return k.TXWrite(ctx, b.scope, &b, key, "obligation.resolve", []string{obligation, artifact}, func(tx pgx.Tx) (Receipt, error) {
		var task, owner, state string
		e := tx.QueryRow(ctx, "SELECT task_id,owner,state FROM obligations WHERE company_id=$1 AND id=$2", b.scope.company, obligation).Scan(&task, &owner, &state)
		if errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if e != nil {
			return Receipt{}, e
		}
		if owner != b.employee || state != "pending" {
			return Receipt{}, core.Denied
		}
		var digest string
		e = tx.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND task_id=$3 AND author=$4 AND state='ready'", b.scope.company, artifact, task, b.employee).Scan(&digest)
		if errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if e != nil {
			return Receipt{}, e
		}
		t, e := taskRow(ctx, tx, b.scope, task)
		if e != nil {
			return Receipt{}, e
		}
		ms, e := missionState(ctx, tx, b.scope, t.Mission)
		if e != nil {
			return Receipt{}, e
		}
		if ms != "active" {
			return Receipt{}, core.Denied
		}
		_, e = tx.Exec(ctx, "UPDATE obligations SET state='fulfilled',evidence_id=$3 WHERE company_id=$1 AND id=$2", b.scope.company, obligation, artifact)
		if e != nil {
			return Receipt{}, e
		}
		response := newID()
		_, e = tx.Exec(ctx, "INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body,evidence_id) VALUES($1,$2,$3,$4,$5,'emp-planning','response',$6,$7)", b.scope.company, response, t.Mission, task, b.employee, "candidate supplied; sha256="+digest, artifact)
		return Receipt{ID: response, Status: "fulfilled"}, e
	})
}

func (k *Kernel) TXVerify(ctx context.Context, b Binding, id, key string) (Receipt, error) {
	if b.employee != "emp-review" {
		return Receipt{}, core.Denied
	}
	var digest, author, task, contract string
	e := k.pool.QueryRow(ctx, "SELECT digest,author,task_id,contract FROM artifacts WHERE company_id=$1 AND id=$2", b.scope.company, id).Scan(&digest, &author, &task, &contract)
	if errors.Is(e, pgx.ErrNoRows) {
		return Receipt{}, core.OutOfScope
	}
	if e != nil {
		return Receipt{}, e
	}
	if contract != core.Contract {
		return Receipt{}, core.Denied
	}
	content, e := readBlob(k.root, b.scope.company, digest)
	if e != nil {
		return Receipt{}, core.Integrity
	}
	verdict := "passed"
	if core.CheckCandidate(author, b.employee, content) != nil {
		verdict = "failed"
	}
	return k.TXWrite(ctx, b.scope, &b, key, "review.submit", []string{id, digest, core.Contract}, func(tx pgx.Tx) (Receipt, error) {
		var state, currentDigest string
		e := tx.QueryRow(ctx, "SELECT state,digest FROM artifacts WHERE company_id=$1 AND id=$2", b.scope.company, id).Scan(&state, &currentDigest)
		if e != nil {
			return Receipt{}, e
		}
		if state != "ready" || digest != currentDigest {
			return Receipt{}, core.Integrity
		}
		t, e := taskRow(ctx, tx, b.scope, task)
		if e != nil {
			return Receipt{}, e
		}
		ms, e := missionState(ctx, tx, b.scope, t.Mission)
		if e != nil {
			return Receipt{}, e
		}
		if ms != "active" {
			return Receipt{}, core.Denied
		}
		_, e = tx.Exec(ctx, "UPDATE artifacts SET verdict=$3,verifier=$4 WHERE company_id=$1 AND id=$2", b.scope.company, id, verdict, b.employee)
		if e != nil {
			return Receipt{}, e
		}
		if verdict == "passed" {
			_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", b.scope.company, task)
			if e != nil {
				return Receipt{}, e
			}
		}
		return Receipt{ID: id, Status: verdict}, nil
	})
}
