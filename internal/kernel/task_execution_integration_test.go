// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func TestProviderWorkerSessionRejectsMissionBootstrapTask(t *testing.T) {
	k, scope, bootstrap, _ := prepareR05B5ProductTask(t)
	defer k.Close()

	_, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, bootstrap.ID, "offline-model/medium", 16)
	if !errors.Is(err, core.Denied) {
		t.Fatalf("provider WorkerSession for bootstrap Task error = %v, want %s", err, core.Denied)
	}
	assertR05B5WorkerSessionCount(t, k, scope, 0)
}

func TestDatabasePreventsDuplicateProductTaskKindWithinMission(t *testing.T) {
	k, scope, _, task := prepareR05B5ProductTask(t)
	defer k.Close()

	if _, err := k.pool.Exec(context.Background(), `INSERT INTO tasks(company_id,id,mission_id,owner,kind)
	VALUES($1,$2,$3,$4,$5)`, scope.company, newID(), task.Mission, core.EmployeeBackendID, core.TaskKindCompat); err == nil {
		t.Fatal("database accepted a duplicate product Task kind for one Mission")
	} else {
		var postgresError *pgconn.PgError
		if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
			t.Fatalf("duplicate product Task error = %v, want unique-constraint violation", err)
		}
	}
	if !task.IsProductProviderExecutable() || task.Kind != core.TaskKindCompat || task.Owner != core.EmployeeBackendID {
		t.Fatalf("prepared product Task has wrong execution role: %+v", task)
	}
	var providerTaskCount int
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat' AND owner='emp-backend'", scope.company, task.Mission).Scan(&providerTaskCount); err != nil {
		t.Fatal(err)
	}
	if providerTaskCount != 1 {
		t.Fatalf("provider-executable Task count = %d, want 1", providerTaskCount)
	}
	assertR05B5WorkerSessionCount(t, k, scope, 0)
}

func TestProviderWorkerSessionIsOneShotForExecutableTask(t *testing.T) {
	k, scope, _, task := prepareR05B5ProductTask(t)
	defer k.Close()

	first, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	if first.TaskID() != task.ID {
		t.Fatalf("WorkerSession binding TaskID = %q, want %q", first.TaskID(), task.ID)
	}
	if err = k.TXFinalizeWorkerBeforeProcess(context.Background(), first, "offline worker session fence test", "r05b5-first-session-stop"); err != nil {
		t.Fatal(err)
	}
	_, err = k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, "offline-model/medium", 16)
	if !errors.Is(err, core.Denied) {
		t.Fatalf("second provider WorkerSession for the same Task error = %v, want %s", err, core.Denied)
	}
	assertR05B5WorkerSessionCount(t, k, scope, 1)
}

func TestProductProviderWorkerSessionCapturesValidationAndWorkspaceBinding(t *testing.T) {
	k, scope, _, task := prepareR05B5ProductTask(t)
	defer k.Close()
	if task.ValidationBinding == nil {
		t.Fatal("prepared product Task has no TaskValidationBinding")
	}

	var workspaceDigest string
	var workspaceRevision int64
	if err := k.pool.QueryRow(context.Background(), "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&workspaceDigest, &workspaceRevision); err != nil {
		t.Fatal(err)
	}
	worker, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	if worker.TaskValidationBindingDigest() != task.ValidationBinding.ConfigurationDigest || worker.WorkspaceDigest() != workspaceDigest || worker.WorkspaceRevision() != workspaceRevision {
		t.Fatalf("provider WorkerSession binding did not capture the Task contract/CAS snapshot: validation=%s workspace=%s/%d", worker.TaskValidationBindingDigest(), worker.WorkspaceDigest(), worker.WorkspaceRevision())
	}
}

func TestProductProviderAuthorizationBindingRejectsWorkspaceDrift(t *testing.T) {
	k, scope, _, task := prepareR05B5ProductTask(t)
	defer k.Close()
	profile := "gpt-5.6-luna/medium"
	binding, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, profile, 16)
	if err != nil {
		t.Fatal(err)
	}
	if err = k.ValidateProductProviderAuthorizationBinding(context.Background(), binding, profile, 16); err != nil {
		t.Fatalf("fresh provider authorization binding rejected: %v", err)
	}
	if _, err = k.pool.Exec(context.Background(), "UPDATE worker_workspaces SET revision=revision+1 WHERE company_id=$1 AND task_id=$2", scope.company, task.ID); err != nil {
		t.Fatal(err)
	}
	if err = k.ValidateProductProviderAuthorizationBinding(context.Background(), binding, profile, 16); !errors.Is(err, core.Conflict) {
		t.Fatalf("stale provider authorization binding error = %v, want %s", err, core.Conflict)
	}
}

func TestProductProviderWorkerSessionRequiresTaskValidationBinding(t *testing.T) {
	k, scope, task := prepareR05B5ProductTaskWithoutAcceptance(t)
	defer k.Close()
	if task.ValidationBinding != nil {
		t.Fatal("exploratory Task unexpectedly has TaskValidationBinding")
	}

	_, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, "offline-model/medium", 16)
	if !errors.Is(err, core.Denied) {
		t.Fatalf("provider WorkerSession for Task without TaskValidationBinding error = %v, want %s", err, core.Denied)
	}
	assertR05B5WorkerSessionCount(t, k, scope, 0)
}

func TestTaskValidationBindingCannotBeMutatedOrDeleted(t *testing.T) {
	k, scope, _, task := prepareR05B5ProductTask(t)
	defer k.Close()
	if task.ValidationBinding == nil {
		t.Fatal("prepared product Task has no TaskValidationBinding")
	}

	if _, err := k.pool.Exec(context.Background(), "UPDATE task_validation_bindings SET configuration_digest=$3 WHERE company_id=$1 AND task_id=$2", scope.company, task.ID, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("database accepted a TaskValidationBinding mutation")
	}
	if _, err := k.pool.Exec(context.Background(), "DELETE FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2", scope.company, task.ID); err == nil {
		t.Fatal("database accepted TaskValidationBinding deletion")
	}

	var count int
	var digest string
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*),min(configuration_digest) FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&count, &digest); err != nil {
		t.Fatal(err)
	}
	if count != 1 || digest != task.ValidationBinding.ConfigurationDigest {
		t.Fatalf("immutable TaskValidationBinding count/digest=%d/%s, want 1/%s", count, digest, task.ValidationBinding.ConfigurationDigest)
	}
}

func prepareR05B5ProductTaskWithoutAcceptance(t *testing.T) (*Kernel, Scope, Task) {
	t.Helper()
	dsn := os.Getenv("POLIS_R05B5_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B5 PostgreSQL required")
	}
	k, err := Open(context.Background(), dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	companyID := "r05b5-unqualified-" + newID()
	scope, err := k.TXCreateCompany(context.Background(), companyID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoalWithAcceptance(context.Background(), scope, "R0.5B5 missing binding test", "confirm provider execution requires public criteria", nil, "r05b5-mission-create")
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(context.Background(), scope, mission.ID, "r05b5-mission-start"); err != nil {
		k.Close()
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(context.Background(), scope, mission.ID, "confirm provider execution requires public criteria", "r05b5-task-prepare-"+mission.ID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	return k, scope, task
}

func prepareR05B5ProductTask(t *testing.T) (*Kernel, Scope, Task, Task) {
	t.Helper()
	dsn := os.Getenv("POLIS_R05B5_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B5 PostgreSQL required")
	}
	k, err := Open(context.Background(), dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	companyID := "r05b5-" + newID()
	scope, err := k.TXCreateCompany(context.Background(), companyID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	contract := &taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision,
		RequiredText: []string{
			"Mission ID: {{mission_id}}",
			"Task ID: {{task_id}}",
			"Acknowledgement:",
			"Task summary:",
		},
	}
	mission, err := k.TXCreateMissionGoalWithAcceptance(context.Background(), scope, "R0.5B5 provider Task test", "prepare one provider-executable Task", contract, "r05b5-mission-create")
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(context.Background(), scope, mission.ID, "r05b5-mission-start"); err != nil {
		k.Close()
		t.Fatal(err)
	}
	bootstrap, err := k.MissionBootstrapTask(context.Background(), scope, mission.ID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(context.Background(), scope, mission.ID, "prepare one provider-executable Task", "r05b5-task-prepare-"+mission.ID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if task.ValidationBinding == nil || task.ValidationBinding.TaskID != task.ID || task.ValidationBinding.MissionID != task.Mission {
		k.Close()
		t.Fatalf("TaskValidationBinding does not target the prepared product Task: task=%+v binding=%+v", task, task.ValidationBinding)
	}
	return k, scope, bootstrap, task
}

func assertR05B5WorkerSessionCount(t *testing.T, k *Kernel, scope Scope, want int) {
	t.Helper()
	var count int
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*) FROM worker_sessions WHERE company_id=$1", scope.company).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("WorkerSession count = %d, want %d", count, want)
	}
}
