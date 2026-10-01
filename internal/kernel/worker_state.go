// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
		return Receipt{ID: id, Status: "ready"}, e
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
	return k.TXNewWorkerWithToolBudget(ctx, s, task, profile, 0)
}

func (k *Kernel) TXNewWorkerWithToolBudget(ctx context.Context, s Scope, task, profile string, toolCallLimit int64) (Binding, error) {
	return k.txNewWorkerWithToolBudget(ctx, s, task, profile, toolCallLimit, false)
}

// TXNewProductProviderWorkerWithToolBudget creates a WorkerSession only for
// the single provider-executable product Task in the active Mission. It also
// fences a second provider attempt for a Task that already has a session.
func (k *Kernel) TXNewProductProviderWorkerWithToolBudget(ctx context.Context, s Scope, task, profile string, toolCallLimit int64) (Binding, error) {
	return k.txNewWorkerWithToolBudget(ctx, s, task, profile, toolCallLimit, true)
}

// ValidateProductProviderAuthorizationBinding rereads the persisted product
// Task, immutable validator binding, workspace CAS and restoring session in a
// single read snapshot immediately before provider reservation.
func (k *Kernel) ValidateProductProviderAuthorizationBinding(ctx context.Context, b Binding, profile string, toolCallLimit int64) error {
	if b.session == "" || profile == "" || toolCallLimit <= 0 {
		return core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var incarnation string
	if err = tx.QueryRow(ctx, "SELECT incarnation FROM runtime_control WHERE singleton").Scan(&incarnation); err != nil {
		return err
	}
	if incarnation != k.incarnation || b.incarnation != incarnation {
		return core.StaleEpoch
	}
	var state, employee, taskID, sessionIncarnation, sessionProfile string
	var sessionEpoch, employeeEpoch, sessionToolCallLimit int64
	err = tx.QueryRow(ctx, `SELECT s.state,s.employee_id,s.task_id,s.epoch,s.incarnation,s.profile,s.tool_call_limit,e.epoch
FROM worker_sessions s JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id
WHERE s.company_id=$1 AND s.id=$2`, b.scope.company, b.session).Scan(
		&state, &employee, &taskID, &sessionEpoch, &sessionIncarnation, &sessionProfile, &sessionToolCallLimit, &employeeEpoch,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if employee != b.employee || taskID != b.task || sessionEpoch != b.epoch || employeeEpoch != b.epoch || sessionIncarnation != b.incarnation {
		return core.StaleEpoch
	}
	if state != "restoring" || sessionProfile != profile || sessionToolCallLimit != toolCallLimit {
		return core.Conflict
	}
	task, err := taskRow(ctx, tx, b.scope, b.task)
	if err != nil {
		return err
	}
	if task.Mission == "" || task.State != "ready" || task.Owner != b.employee || !task.IsProductProviderExecutable() ||
		!task.HasValidProductValidationBinding() || task.ValidationBinding.ConfigurationDigest != b.taskValidationBindingDigest {
		return core.Conflict
	}
	mission, err := missionState(ctx, tx, b.scope, task.Mission)
	if err != nil {
		return err
	}
	if mission != "active" {
		return core.Conflict
	}
	providerTasks, err := productProviderTasksTx(ctx, tx, b.scope, task.Mission)
	if err != nil {
		return err
	}
	selected, err := SelectSingleProductProviderTask(providerTasks)
	if err != nil || selected.ID != task.ID {
		return core.Conflict
	}
	var providerSessions, currentSession int64
	err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE s.id=$3)
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND t.mission_id=$2 AND t.kind='compat' AND t.owner='emp-backend'`, b.scope.company, task.Mission, b.session).Scan(&providerSessions, &currentSession)
	if err != nil {
		return err
	}
	if providerSessions != 1 || currentSession != 1 {
		return core.Conflict
	}
	var workspaceDigest string
	var workspaceRevision int64
	err = tx.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", b.scope.company, task.ID).Scan(&workspaceDigest, &workspaceRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Conflict
	}
	if err != nil {
		return err
	}
	if !IsValidProductWorkspaceCAS(workspaceDigest, workspaceRevision) || workspaceDigest != b.workspaceDigest || workspaceRevision != b.workspaceRevision {
		return core.Conflict
	}
	return tx.Commit(ctx)
}

func (k *Kernel) txNewWorkerWithToolBudget(ctx context.Context, s Scope, task, profile string, toolCallLimit int64, productProvider bool) (Binding, error) {
	var b Binding
	if toolCallLimit < 0 {
		return b, core.Malformed
	}
	r, e := k.TXWrite(ctx, s, nil, newID(), "worker.restore", []string{task, profile}, func(tx pgx.Tx) (Receipt, error) {
		var productWorkspaceDigest, productValidationBindingDigest string
		var productWorkspaceRevision int64
		t, e := taskRow(ctx, tx, s, task)
		if e != nil {
			return Receipt{}, e
		}
		if e = requireMemoryTaskWritableTX(ctx, tx, s.company, t.ID); e != nil {
			return Receipt{}, e
		}
		if productProvider {
			if !t.IsProductProviderExecutable() || t.State != "ready" {
				return Receipt{}, core.Denied
			}
			providerTasks, err := productProviderTasksTx(ctx, tx, s, t.Mission)
			if err != nil {
				return Receipt{}, err
			}
			selectedTask, err := SelectSingleProductProviderTask(providerTasks)
			if err != nil {
				return Receipt{}, err
			}
			if selectedTask.ID != t.ID {
				return Receipt{}, core.Conflict
			}
			var previousAttempt bool
			if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND task_id=$2)", s.company, t.ID).Scan(&previousAttempt); err != nil {
				return Receipt{}, err
			}
			if previousAttempt {
				return Receipt{}, core.Denied
			}
			if !t.HasValidProductValidationBinding() {
				return Receipt{}, core.Denied
			}
			if e = tx.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", s.company, t.ID).Scan(&productWorkspaceDigest, &productWorkspaceRevision); e != nil {
				return Receipt{}, e
			}
			if !IsValidProductWorkspaceCAS(productWorkspaceDigest, productWorkspaceRevision) {
				return Receipt{}, core.Integrity
			}
			productValidationBindingDigest = t.ValidationBinding.ConfigurationDigest
		} else if (t.Kind != core.TaskKindBootstrapPlan && t.Kind != core.TaskKindCompute && t.Kind != core.TaskKindCompat && t.Kind != core.TaskKindReview && t.Kind != core.TaskKindPeerBackend && t.Kind != core.TaskKindPeerFrontend && t.Kind != core.TaskKindPeerReview) || t.State == "completed" {
			return Receipt{}, core.Denied
		}
		ms, e := missionState(ctx, tx, s, t.Mission)
		if e != nil {
			return Receipt{}, e
		}
		if ms != "active" {
			return Receipt{}, core.Denied
		}
		schedule, pauseReason, e := lockEmployeeScheduleTX(ctx, tx, s, t.Owner)
		if e != nil {
			return Receipt{}, e
		}
		if schedule.State == core.EmployeeSchedulePaused || schedule.State == core.EmployeeScheduleWaitingQuota {
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
		b = Binding{scope: s, task: t.ID, employee: t.Owner, incarnation: k.incarnation, session: newID(), workspaceDigest: productWorkspaceDigest, workspaceRevision: productWorkspaceRevision, taskValidationBindingDigest: productValidationBindingDigest}
		e = tx.QueryRow(ctx, "UPDATE employees SET epoch=epoch+1 WHERE company_id=$1 AND id=$2 RETURNING epoch", s.company, t.Owner).Scan(&b.epoch)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,tool_call_limit) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'restoring',$9)", s.company, b.session, b.employee, task, t.Generation+1, b.epoch, b.incarnation, profile, toolCallLimit)
		if e != nil {
			return Receipt{}, e
		}
		if e = txEnsureWorkerProcessContainment(ctx, tx, s, b.session, runner.CurrentProcessContainmentMetadata()); e != nil {
			return Receipt{}, e
		}
		schedule.State = core.EmployeeScheduleAdmitted
		if e = persistEmployeeScheduleTX(ctx, tx, s, t.Owner, schedule, pauseReason); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: b.session, Status: "restoring"}, e
	})
	_ = r
	return b, e
}

func (k *Kernel) WorkerToolCallBudget(ctx context.Context, b Binding) (ToolCallBudget, error) {
	var budget ToolCallBudget
	e := k.pool.QueryRow(ctx, "SELECT tool_call_limit,tool_calls_used FROM worker_sessions WHERE company_id=$1 AND id=$2", b.scope.company, b.session).Scan(&budget.Limit, &budget.Used)
	if e != nil {
		return budget, e
	}
	budget.Remaining = -1
	if budget.Limit > 0 {
		budget.Remaining = budget.Limit - budget.Used
	}
	return budget, nil
}

func (k *Kernel) TXConsumeToolCall(ctx context.Context, b Binding, key, name string) (ToolCallBudget, error) {
	var budget ToolCallBudget
	sessionWrite := name != "work_current" && name != "context_read" && name != "workspace_read"
	_, e := k.txWrite(ctx, b.scope, &b, key, "worker.tool_call", name, sessionWrite, func(tx pgx.Tx) (Receipt, error) {
		if !sessionWrite {
			var sessionState, missionState string
			if err := tx.QueryRow(ctx, `SELECT s.state,m.state FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE s.company_id=$1 AND s.id=$2`, b.scope.company, b.session).Scan(&sessionState, &missionState); err != nil {
				return Receipt{}, err
			}
			if sessionState != "active" || missionState != "active" {
				return Receipt{}, core.Denied
			}
		}
		if e := tx.QueryRow(ctx, "SELECT tool_call_limit,tool_calls_used FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE", b.scope.company, b.session).Scan(&budget.Limit, &budget.Used); e != nil {
			return Receipt{}, e
		}
		budget.Remaining = -1
		if budget.Limit > 0 {
			budget.Remaining = budget.Limit - budget.Used
			if budget.Remaining <= 0 {
				return Receipt{}, core.ToolCallBudgetExceeded
			}
		}
		budget.Used++
		if budget.Limit > 0 {
			budget.Remaining = budget.Limit - budget.Used
		}
		if _, e := tx.Exec(ctx, "UPDATE worker_sessions SET tool_calls_used=$3 WHERE company_id=$1 AND id=$2", b.scope.company, b.session, budget.Used); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: b.session, Status: "tool_call"}, nil
	})
	if e == nil && budget.Limit == 0 && budget.Used == 0 {
		budget, e = k.WorkerToolCallBudget(ctx, b)
	}
	return budget, e
}

func (k *Kernel) checkSession(ctx context.Context, tx pgx.Tx, b Binding, write bool) (string, error) {
	if b.session == "" || b.incarnation != k.incarnation {
		return "", core.StaleEpoch
	}
	var state, employee, incarnation, mission, taskID, executionMode string
	var epoch, current int64
	e := tx.QueryRow(ctx, `SELECT s.state,s.employee_id,s.task_id,s.epoch,s.incarnation,e.epoch,m.state,s.execution_mode FROM worker_sessions s JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE s.company_id=$1 AND s.id=$2`, b.scope.company, b.session).Scan(&state, &employee, &taskID, &epoch, &incarnation, &current, &mission, &executionMode)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", core.OutOfScope
	}
	if e != nil {
		return "", e
	}
	if employee != b.employee || (b.task != "" && taskID != b.task) || epoch != b.epoch || current != b.epoch || incarnation != b.incarnation {
		return state, core.StaleEpoch
	}
	if executionMode != "provider" {
		return state, core.Denied
	}
	if write && (state != "active" || mission != "active") {
		return state, core.Denied
	}
	if write {
		if err := requireMemoryTaskWritableTX(ctx, tx, b.scope.company, taskID); err != nil {
			return state, err
		}
	}
	if !write && state != "restoring" && state != "validating" && state != "activation_pending_environment" && state != "active" && state != "stopping" {
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
			if e = requireMemoryTaskWritableTX(ctx, tx, b.scope.company, b.task); e != nil {
				return Receipt{}, e
			}
		}
		_, e = tx.Exec(ctx, "UPDATE worker_sessions SET state=$3,capability_digest=COALESCE(NULLIF($4,''),capability_digest) WHERE company_id=$1 AND id=$2", b.scope.company, b.session, to, capability)
		if e != nil {
			return Receipt{}, e
		}
		if to == "active" {
			_, e = tx.Exec(ctx, "UPDATE tasks t SET state='working',generation=s.generation FROM worker_sessions s WHERE t.company_id=s.company_id AND t.id=s.task_id AND s.company_id=$1 AND s.id=$2", b.scope.company, b.session)
			if e != nil {
				return Receipt{}, e
			}
			if e = ensureEmployeeScheduleTX(ctx, tx, b.scope, b.employee); e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, `UPDATE employee_schedules SET state='working',pause_reason=NULL,updated_at=now()
WHERE company_id=$1 AND employee_id=$2`, b.scope.company, b.employee)
		}
		return Receipt{ID: b.session, Status: to}, e
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
	requestID := "worker-stopping-" + b.session
	_, e := k.TXWrite(ctx, b.scope, nil, requestID, "worker.stopping", b.session, func(tx pgx.Tx) (Receipt, error) {
		if _, e := k.checkSession(ctx, tx, b, false); e != nil {
			return Receipt{}, e
		}
		_, e := tx.Exec(ctx, "UPDATE worker_sessions SET state='stopping' WHERE company_id=$1 AND id=$2 AND state NOT IN ('stopped','reconcile_required')", b.scope.company, b.session)
		return Receipt{ID: b.session, Status: "stopping"}, e
	})
	return e
}

// TXFinalizeWorkerBeforeProcess records a terminal worker state when a
// provider process could not be started. No process proof exists in this
// branch, so it is deliberately separate from TXConfirmStopped.
func (k *Kernel) TXFinalizeWorkerBeforeProcess(ctx context.Context, b Binding, reason, key string) error {
	if reason == "" {
		return core.Malformed
	}
	_, e := k.TXWrite(ctx, b.scope, nil, key, "worker.failed", struct {
		Session string
		Reason  string
	}{b.session, reason}, func(tx pgx.Tx) (Receipt, error) {
		if _, e := k.checkSession(ctx, tx, b, false); e != nil {
			return Receipt{}, e
		}
		tag, e := tx.Exec(ctx, "UPDATE worker_sessions SET state='stopped',stop_receipt=$3 WHERE company_id=$1 AND id=$2 AND process_pid IS NULL AND state NOT IN ('stopped','reconcile_required')", b.scope.company, b.session, "provider-process-not-started:"+reason)
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Conflict
		}
		return Receipt{ID: b.session, Status: "stopped"}, nil
	})
	return e
}
func (k *Kernel) TXConfirmStopped(ctx context.Context, b Binding, proof runner.StopProof) error {
	if !proof.For(b.session) {
		return core.Denied
	}
	if !core.ValidID(b.scope.company) {
		return core.Malformed
	}
	var state, taskID, employeeID, incarnation, stopReceipt string
	var epoch int64
	var processPID int
	if err := k.pool.QueryRow(ctx, `SELECT state,task_id,employee_id,epoch,incarnation,COALESCE(process_pid,0),COALESCE(stop_receipt,'')
FROM worker_sessions WHERE company_id=$1 AND id=$2`, b.scope.company, b.session).Scan(&state, &taskID, &employeeID, &epoch, &incarnation, &processPID, &stopReceipt); err != nil {
		return err
	}
	if state == "stopped" {
		if taskID == b.task && employeeID == b.employee && epoch == b.epoch && incarnation == b.incarnation && processPID == proof.PID() && stopReceipt == proof.Description() {
			return nil
		}
		return core.Denied
	}
	input := struct {
		SessionID string
		PID       int
		Receipt   string
	}{b.session, proof.PID(), proof.Description()}
	requestID := "worker-stopped-" + b.session
	_, e := k.TXWrite(ctx, b.scope, nil, requestID, "worker.stopped", input, func(tx pgx.Tx) (Receipt, error) {
		if _, e := k.checkSession(ctx, tx, b, false); e != nil {
			return Receipt{}, e
		}
		tag, e := tx.Exec(ctx, "UPDATE worker_sessions SET state='stopped',stop_receipt=$3 WHERE company_id=$1 AND id=$2 AND state='stopping' AND process_pid=$4", b.scope.company, b.session, proof.Description(), proof.PID())
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Denied
		}
		if _, e = reconcileEmployeeScheduleTX(ctx, tx, b.scope, b.employee); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: b.session, Status: "stopped"}, nil
	})
	return e
}

func (k *Kernel) TXRecordProviderTerminal(ctx context.Context, b Binding, state, outcome string, usage any, key string) (Receipt, error) {
	if state == "" || outcome == "" {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, b.scope, &b, key, "provider.turn."+state, struct {
		State   string
		Outcome string
		Usage   any
	}{state, outcome, usage}, func(tx pgx.Tx) (Receipt, error) {
		data, err := json.Marshal(struct {
			State   string `json:"state"`
			Outcome string `json:"outcome"`
			Usage   any    `json:"usage"`
		}{state, outcome, usage})
		if err != nil {
			return Receipt{}, err
		}
		id := newID()
		if _, err = tx.Exec(ctx, "INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, "provider_terminal", data); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: state}, nil
	})
}

// TXRecordProviderInitializationFailure records a provider-session lifecycle
// failure before a real turn exists. It is intentionally separate from
// provider.turn.* so Workbench consumers cannot mistake initialization
// failure for an inconclusive model turn.
func (k *Kernel) TXRecordProviderInitializationFailure(ctx context.Context, b Binding, evidence any, key string) (Receipt, error) {
	return k.TXRecordProviderRuntimeFailure(ctx, b, "provider.runtime.initialization_failed", "provider_initialization", evidence, key)
}

func (k *Kernel) TXRecordProviderThreadStartFailure(ctx context.Context, b Binding, evidence any, key string) (Receipt, error) {
	return k.TXRecordProviderRuntimeFailure(ctx, b, "provider.runtime.thread_start_failed", "provider_thread_start", evidence, key)
}

func (k *Kernel) TXRecordProviderRuntimeFailure(ctx context.Context, b Binding, eventKind, observationReason string, evidence any, key string) (Receipt, error) {
	if eventKind == "" || observationReason == "" || evidence == nil || key == "" {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, b.scope, &b, key, eventKind, evidence, func(tx pgx.Tx) (Receipt, error) {
		data, err := json.Marshal(evidence)
		if err != nil {
			return Receipt{}, err
		}
		id := newID()
		if _, err = tx.Exec(ctx, "INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, observationReason, data); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: "initialization_failed"}, nil
	})
}

func (k *Kernel) TXAttachWorker(ctx context.Context, b Binding, p *runner.Process) error {
	if p == nil {
		return core.Malformed
	}
	id, pid := p.Identity()
	metadata := p.ContainmentMetadata()
	if id != b.session || !metadata.Valid() {
		return core.Denied
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.process_attached", struct {
		SessionID   string
		ProcessPID  int
		Containment runner.ProcessContainmentMetadata
	}{id, pid, metadata}, func(tx pgx.Tx) (Receipt, error) {
		state, e := k.checkSession(ctx, tx, b, false)
		if e != nil {
			return Receipt{}, e
		}
		if err := txEnsureWorkerProcessContainment(ctx, tx, b.scope, id, metadata); err != nil {
			return Receipt{}, err
		}
		var currentPID pgtype.Int8
		if e = tx.QueryRow(ctx, "SELECT process_pid FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE", b.scope.company, id).Scan(&currentPID); e != nil {
			return Receipt{}, e
		}
		if currentPID.Valid {
			if currentPID.Int64 != int64(pid) {
				return Receipt{}, core.Conflict
			}
			return Receipt{ID: id, Status: "attached"}, nil
		}
		if state != "restoring" && state != "validating" {
			return Receipt{}, core.Denied
		}
		tag, e := tx.Exec(ctx, "UPDATE worker_sessions SET process_pid=$3 WHERE company_id=$1 AND id=$2 AND state IN ('restoring','validating','stopping') AND process_pid IS NULL", b.scope.company, id, pid)
		if e != nil {
			return Receipt{}, e
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Denied
		}
		return Receipt{ID: id, Status: "attached"}, nil
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
	e = tx.QueryRow(ctx, "SELECT task_id,tool_call_limit,tool_calls_used FROM worker_sessions WHERE company_id=$1 AND id=$2", b.scope.company, b.session).Scan(&task, &out.ToolBudget.Limit, &out.ToolBudget.Used)
	if e != nil {
		return out, e
	}
	out.ToolBudget.Remaining = -1
	if out.ToolBudget.Limit > 0 {
		out.ToolBudget.Remaining = out.ToolBudget.Limit - out.ToolBudget.Used
	}
	out.Task, e = taskRow(ctx, tx, b.scope, task)
	if e != nil {
		return out, e
	}
	out.MemoryStatus, e = memoryTaskStatusTX(ctx, tx, b.scope.company, task)
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
	out.SkillCatalog, out.SkillCatalogTruncated, e = listBoundReadOnlySkills(ctx, tx, b.scope.company, b.employee)
	if e != nil {
		return out, e
	}
	out.SkillLoads, out.SkillLoadsTruncated, e = listTaskSkillLoads(ctx, tx, b.scope.company, task)
	if e != nil {
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
	receipt, _, err := k.txReplace(ctx, b, key, expected, 0, content, false)
	return receipt, err
}

// TXReplaceAtRevision applies product workspace CAS against both the content
// digest and monotonically increasing revision, preventing digest ABA writes.
func (k *Kernel) TXReplaceAtRevision(ctx context.Context, b Binding, key, expected string, expectedRevision int64, content string) (Receipt, error) {
	if expectedRevision < 1 {
		return Receipt{}, core.Malformed
	}
	receipt, _, err := k.txReplace(ctx, b, key, expected, expectedRevision, content, true)
	return receipt, err
}

func (k *Kernel) txReplace(ctx context.Context, b Binding, key, expected string, expectedRevision int64, content string, requireRevision bool) (Receipt, int64, error) {
	if len(content) == 0 || len(content) > 4096 {
		return Receipt{}, 0, core.TooLarge
	}
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Receipt{}, 0, e
	}
	e = k.guard(ctx, tx, b.scope, &b)
	tx.Rollback(ctx)
	if e != nil {
		return Receipt{}, 0, e
	}
	digest, e := putBlob(k.root, b.scope.company, []byte(content))
	if e != nil {
		return Receipt{}, 0, e
	}
	receipt, err := k.TXWrite(ctx, b.scope, &b, key, "workspace.replace", []any{expected, expectedRevision, digest}, func(tx pgx.Tx) (Receipt, error) {
		if requireRevision {
			if _, stateErr := k.requireProductTaskWorking(ctx, tx, b); stateErr != nil {
				return Receipt{}, stateErr
			}
		}
		var nextRevision int64
		var tag pgx.Row
		if requireRevision {
			tag = tx.QueryRow(ctx, `UPDATE worker_workspaces w SET digest=$3,revision=revision+1
FROM worker_sessions s WHERE w.company_id=s.company_id AND w.task_id=s.task_id
AND s.company_id=$1 AND s.id=$2 AND w.digest=$4 AND w.revision=$5 RETURNING w.revision`, b.scope.company, b.session, digest, expected, expectedRevision)
		} else {
			tag = tx.QueryRow(ctx, `UPDATE worker_workspaces w SET digest=$3,revision=revision+1
FROM worker_sessions s WHERE w.company_id=s.company_id AND w.task_id=s.task_id
AND s.company_id=$1 AND s.id=$2 AND w.digest=$4 RETURNING w.revision`, b.scope.company, b.session, digest, expected)
		}
		e = tag.Scan(&nextRevision)
		if e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, e
		}
		receipt := Receipt{ID: digest, Status: "persisted"}
		if requireRevision {
			receipt.Revision = nextRevision
		}
		return receipt, nil
	})
	if err != nil {
		return Receipt{}, 0, err
	}
	return receipt, receipt.Revision, nil
}
