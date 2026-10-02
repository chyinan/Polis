// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/organization"
	"polis/internal/taskvalidation"
	"strings"
)

func (k *Kernel) TXCreateCompany(ctx context.Context, id string) (Scope, error) {
	if !core.ValidID(id) {
		return Scope{}, core.Malformed
	}
	// Historical CLI/probe callers retain idempotent company bootstrap
	// semantics; product creation uses the explicit organization transaction.
	scope := Scope{company: id}
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return scope, e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "INSERT INTO companies(id,name,workspace_root) VALUES($1,$1,'.') ON CONFLICT DO NOTHING", id)
	if e != nil {
		return scope, e
	}
	if e = k.guard(ctx, tx, scope, nil); e != nil {
		return scope, e
	}
	for _, emp := range organization.DefaultRoster() {
		_, e = tx.Exec(ctx, `INSERT INTO employees(company_id,id,display_name,role_name,model_profile,enabled)
VALUES($1,$2,$3,$4,$5,true) ON CONFLICT DO NOTHING`, id, emp.ID, emp.DisplayName, emp.Role, emp.ModelProfile)
		if e != nil {
			return scope, e
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO employee_schedules(company_id,employee_id,state)
SELECT company_id,id,'sleeping' FROM employees WHERE company_id=$1
ON CONFLICT(company_id,employee_id) DO NOTHING`, id); e != nil {
		return scope, e
	}
	return scope, tx.Commit(ctx)
}
func (k *Kernel) TXCreateMission(ctx context.Context, s Scope, id string) error {
	if !core.ValidID(id) || len(id) > 70 {
		return core.Malformed
	}
	_, e := k.TXWrite(ctx, s, nil, "draft-"+id, "mission.draft", id, func(tx pgx.Tx) (Receipt, error) {
		_, e := tx.Exec(ctx, "INSERT INTO missions(company_id,id,title,goal,contract) VALUES($1,$2,$3,$4,$5)", s.company, id, id, "Mission "+id, core.Contract)
		return Receipt{ID: id, Status: "draft"}, e
	})
	return e
}

func (k *Kernel) TXCreateMissionGoal(ctx context.Context, s Scope, title, goal, key string) (Receipt, error) {
	return k.TXCreateMissionGoalWithAcceptance(ctx, s, title, goal, nil, key)
}

func (k *Kernel) TXCreateMissionGoalWithAcceptance(ctx context.Context, s Scope, title, goal string, acceptance *taskvalidation.AcceptanceContract, key string) (Receipt, error) {
	if strings.TrimSpace(title) == "" || len(title) > 200 || strings.TrimSpace(goal) == "" || len(goal) > core.MaxContent {
		return Receipt{}, core.Malformed
	}
	if taskvalidation.ValidateContract(acceptance) != nil {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, s, nil, key, "mission.create", struct {
		Title              string
		Goal               string
		AcceptanceContract *taskvalidation.AcceptanceContract
	}{title, goal, acceptance}, func(tx pgx.Tx) (Receipt, error) {
		id := newID()
		var acceptanceJSON []byte
		var err error
		if acceptance != nil {
			acceptanceJSON, err = json.Marshal(acceptance)
			if err != nil {
				return Receipt{}, err
			}
		}
		_, err = tx.Exec(ctx, "INSERT INTO missions(company_id,id,title,goal,contract,acceptance_contract) VALUES($1,$2,$3,$4,$5,$6)", s.company, id, title, goal, core.Contract, acceptanceJSON)
		return Receipt{ID: id, Status: "draft"}, err
	})
}

type MissionDetails struct {
	ID, Title, Goal, State string
	AcceptanceContract     *taskvalidation.AcceptanceContract
}

func (k *Kernel) MissionDetails(ctx context.Context, s Scope, id string) (MissionDetails, error) {
	var details MissionDetails
	var acceptanceJSON []byte
	err := k.pool.QueryRow(ctx, "SELECT id,title,goal,state,acceptance_contract FROM missions WHERE company_id=$1 AND id=$2", s.company, id).Scan(&details.ID, &details.Title, &details.Goal, &details.State, &acceptanceJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionDetails{}, core.OutOfScope
	}
	if err == nil && len(acceptanceJSON) > 0 {
		var acceptance taskvalidation.AcceptanceContract
		if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
			return MissionDetails{}, core.Integrity
		}
		details.AcceptanceContract = &acceptance
	}
	return details, err
}

func (k *Kernel) TXPrepareProductTask(ctx context.Context, s Scope, missionID, goal, key string) (Task, error) {
	if !core.ValidID(missionID) || strings.TrimSpace(goal) == "" || len(goal) > core.MaxContent {
		return Task{}, core.Malformed
	}
	content := "# Mission " + missionID + "\n\nGoal: " + goal + "\n"
	digest, err := k.putBlobWithClaim(ctx, s.company, []byte(content))
	if err != nil {
		return Task{}, err
	}
	receipt, err := k.TXWrite(ctx, s, nil, key, "mission.task.prepare", struct {
		Mission string
		Digest  string
	}{missionID, digest}, func(tx pgx.Tx) (Receipt, error) {
		var state string
		var acceptanceJSON []byte
		if err := tx.QueryRow(ctx, "SELECT state,acceptance_contract FROM missions WHERE company_id=$1 AND id=$2", s.company, missionID).Scan(&state, &acceptanceJSON); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		} else if state != "active" {
			return Receipt{}, core.ConflictError{Reason: "mission is not active", CurrentState: state}
		}
		var acceptance *taskvalidation.AcceptanceContract
		if len(acceptanceJSON) > 0 {
			acceptance = new(taskvalidation.AcceptanceContract)
			if json.Unmarshal(acceptanceJSON, acceptance) != nil || taskvalidation.ValidateContract(acceptance) != nil {
				return Receipt{}, core.Integrity
			}
		}
		var bootstrapID string
		if err := tx.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind=$3 AND state='ready' LIMIT 1", s.company, missionID, core.TaskKindBootstrapPlan).Scan(&bootstrapID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.ConflictError{Reason: "mission bootstrap task is not ready", CurrentState: "unavailable"}
			}
			return Receipt{}, err
		}
		if err := k.requireMemoryTaskCleanTX(ctx, tx, s.company, bootstrapID); err != nil {
			return Receipt{}, err
		}
		plan := []byte(`{"template":"product-task@2","owner":"emp-backend","dependencies":[],"ambiguities":[]}`)
		if _, err := tx.Exec(ctx, "UPDATE tasks SET state='completed',plan=$3 WHERE company_id=$1 AND id=$2", s.company, bootstrapID, plan); err != nil {
			return Receipt{}, err
		}
		taskID := newID()
		if _, err := tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,parent_task_id) VALUES($1,$2,$3,$4,$5,$6)", s.company, taskID, missionID, core.EmployeeBackendID, core.TaskKindCompat, bootstrapID); err != nil {
			return Receipt{}, err
		}
		validationBinding, err := taskvalidation.Bind(taskID, missionID, acceptance)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		if validationBinding != nil {
			contractJSON, marshalErr := json.Marshal(validationBinding.Contract)
			if marshalErr != nil {
				return Receipt{}, marshalErr
			}
			if _, err = tx.Exec(ctx, `INSERT INTO task_validation_bindings(company_id,task_id,mission_id,acceptance_revision,runner_kind,runner_revision,configuration_digest,contract)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, s.company, taskID, missionID, validationBinding.AcceptanceRevision, validationBinding.RunnerKind, validationBinding.RunnerRevision, validationBinding.ConfigurationDigest, contractJSON); err != nil {
				return Receipt{}, err
			}
		}
		if err = bindMissionInputManifest(ctx, tx, s, missionID, taskID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", s.company, taskID, digest); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: taskID, Status: "ready"}, nil
	})
	if err != nil {
		return Task{}, err
	}
	return k.taskByID(ctx, s, receipt.ID)
}

func productProviderTasksTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID string) ([]Task, error) {
	rows, err := tx.Query(ctx, "SELECT id,mission_id,owner,kind,state,generation FROM tasks WHERE company_id=$1 AND mission_id=$2 ORDER BY id", scope.company, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providerTasks := make([]Task, 0, 1)
	for rows.Next() {
		var task Task
		if err = rows.Scan(&task.ID, &task.Mission, &task.Owner, &task.Kind, &task.State, &task.Generation); err != nil {
			return nil, err
		}
		if task.IsProductProviderExecutable() {
			providerTasks = append(providerTasks, task)
		}
	}
	return providerTasks, rows.Err()
}

func (k *Kernel) taskByID(ctx context.Context, s Scope, id string) (Task, error) {
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)
	task, err := taskRow(ctx, tx, s, id)
	if err != nil {
		return Task{}, err
	}
	return task, tx.Commit(ctx)
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
			return Receipt{ID: activation, Status: state}, e
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
		_, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,$4,$5)", s.company, newID(), id, core.EmployeePlanningID, core.TaskKindBootstrapPlan)
		return Receipt{ID: activation, Status: "active"}, e
	})
}

func (k *Kernel) TXStartMissionCommand(ctx context.Context, s Scope, id, key string) (Receipt, error) {
	return k.TXWrite(ctx, s, nil, key, "mission.start", id, func(tx pgx.Tx) (Receipt, error) {
		state, err := missionState(ctx, tx, s, id)
		if err != nil {
			return Receipt{}, err
		}
		if state != "draft" {
			return Receipt{}, core.ConflictError{Reason: "mission is already " + state, CurrentState: state}
		}
		var occupied bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND state IN ('active','paused'))", s.company).Scan(&occupied); err != nil {
			return Receipt{}, err
		}
		if occupied {
			return Receipt{}, core.ConflictError{Reason: "company already has an active mission", CurrentState: "occupied"}
		}
		activation := newID()
		if _, err = tx.Exec(ctx, "UPDATE missions SET state='active',activation_id=$3 WHERE company_id=$1 AND id=$2", s.company, id, activation); err != nil {
			return Receipt{}, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,$4,$5)", s.company, newID(), id, core.EmployeePlanningID, core.TaskKindBootstrapPlan)
		return Receipt{ID: id, Status: "active"}, err
	})
}

func (k *Kernel) TXCancelMission(ctx context.Context, s Scope, id, key string) (Receipt, error) {
	return k.TXWrite(ctx, s, nil, key, "mission.cancel", id, func(tx pgx.Tx) (Receipt, error) {
		state, err := missionState(ctx, tx, s, id)
		if err != nil {
			return Receipt{}, err
		}
		if state != "active" && state != "paused" {
			return Receipt{}, core.ConflictError{Reason: "mission cannot be cancelled from " + state, CurrentState: state}
		}
		var live bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE s.company_id=$1 AND t.mission_id=$2 AND s.state!='stopped')", s.company, id).Scan(&live); err != nil {
			return Receipt{}, err
		}
		if live {
			return Receipt{}, core.ConflictError{Reason: "mission still has a live worker session", CurrentState: state}
		}
		if _, err = tx.Exec(ctx, "UPDATE missions SET state='cancelled' WHERE company_id=$1 AND id=$2 AND state IN ('active','paused')", s.company, id); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, "UPDATE tasks SET state='cancelled' WHERE company_id=$1 AND mission_id=$2 AND state NOT IN ('completed','cancelled')", s.company, id); err != nil {
			return Receipt{}, err
		}
		if err = setMissionEmployeeSchedulesTX(ctx, tx, s, id, false); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: "cancelled"}, nil
	})
}

func (k *Kernel) MissionBootstrapTask(ctx context.Context, s Scope, id string) (Task, error) {
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)
	var task Task
	err = tx.QueryRow(ctx, "SELECT t.id,t.mission_id,t.owner,t.kind,t.state,t.generation FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2 AND t.kind=$3 ORDER BY t.id LIMIT 1", s.company, id, core.TaskKindBootstrapPlan).Scan(&task.ID, &task.Mission, &task.Owner, &task.Kind, &task.State, &task.Generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, core.OutOfScope
	}
	if err != nil {
		return Task{}, err
	}
	return task, tx.Commit(ctx)
}
func (k *Kernel) TXSetPaused(ctx context.Context, s Scope, id string, paused bool) error {
	_, e := k.TXSetMissionPausedCommand(ctx, s, id, paused, newID())
	return e
}

func (k *Kernel) TXSetMissionPausedCommand(ctx context.Context, s Scope, id string, paused bool, key string) (Receipt, error) {
	operation, resultingState := "mission.resume", "active"
	allowedState := "paused"
	if paused {
		operation, resultingState, allowedState = "mission.pause", "paused", "active"
	}
	return k.TXWrite(ctx, s, nil, key, operation, struct {
		ID     string
		Paused bool
	}{id, paused}, func(tx pgx.Tx) (Receipt, error) {
		state, e := missionState(ctx, tx, s, id)
		if e != nil {
			return Receipt{}, e
		}
		if state != allowedState {
			return Receipt{}, core.ConflictError{Reason: "mission lifecycle transition is not valid from " + state, CurrentState: state}
		}
		_, e = tx.Exec(ctx, "UPDATE missions SET state=$3 WHERE company_id=$1 AND id=$2", s.company, id, resultingState)
		if e != nil {
			return Receipt{}, e
		}
		if e = setMissionEmployeeSchedulesTX(ctx, tx, s, id, paused); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: id, Status: resultingState}, e
	})
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
	var plan []byte
	e := tx.QueryRow(ctx, `SELECT id,mission_id,owner,kind,state,generation,plan,COALESCE(parent_task_id,''),problem_key
FROM tasks WHERE company_id=$1 AND id=$2`, s.company, id).Scan(&t.ID, &t.Mission, &t.Owner, &t.Kind, &t.State, &t.Generation, &plan, &t.ParentTaskID, &t.ProblemKey)
	if errors.Is(e, pgx.ErrNoRows) {
		e = core.OutOfScope
	}
	if e != nil {
		return t, e
	}
	if len(plan) > 0 {
		t.Plan = json.RawMessage(plan)
	}
	var contractJSON []byte
	var binding taskvalidation.Binding
	e = tx.QueryRow(ctx, `SELECT task_id,mission_id,acceptance_revision,runner_kind,runner_revision,configuration_digest,contract
FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2`, s.company, id).Scan(&binding.TaskID, &binding.MissionID, &binding.AcceptanceRevision, &binding.RunnerKind, &binding.RunnerRevision, &binding.ConfigurationDigest, &contractJSON)
	if errors.Is(e, pgx.ErrNoRows) {
		return t, nil
	}
	if e != nil {
		return t, e
	}
	if json.Unmarshal(contractJSON, &binding.Contract) != nil {
		return t, core.Integrity
	}
	t.ValidationBinding = &binding
	return t, e
}
func (k *Kernel) checkWork(ctx context.Context, tx pgx.Tx, b Binding, w Task) (Task, error) {
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
	if e = k.requireMemoryTaskWritableTX(ctx, tx, b.scope.company, t.ID); e != nil {
		return t, e
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
	e = tx.QueryRow(ctx, `SELECT t.id,t.mission_id,t.owner,t.kind,t.state,t.generation FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE t.company_id=$1 AND t.owner=$2 AND t.kind IN ('bootstrap_plan','compute','peer_backend','peer_frontend') AND t.state='ready' AND m.state='active' AND NOT EXISTS(SELECT 1 FROM tasks x WHERE x.company_id=t.company_id AND x.owner=t.owner AND x.state='working') ORDER BY t.id LIMIT 1`, b.scope.company, b.employee).Scan(&t.ID, &t.Mission, &t.Owner, &t.Kind, &t.State, &t.Generation)
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
