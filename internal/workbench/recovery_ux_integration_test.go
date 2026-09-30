// pattern: Imperative Shell
package workbench

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/kernel"
)

func TestRestartRecoveryProjectionShowsReconciliationRequired(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	root := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := runtime.TXCreateCompany(ctx, "r06-recovery-ux")
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	if err = runtime.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, "mission-1", "start-1"); err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	task, err := runtime.MissionBootstrapTask(ctx, scope, "mission-1")
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	binding, err := runtime.TXNewWorker(ctx, scope, task.ID, "deterministic/fake")
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE worker_sessions SET state='active' WHERE company_id=$1 AND id=$2", "r06-recovery-ux", binding.SessionID()); err != nil {
		pool.Close()
		runtime.Close()
		t.Fatal(err)
	}
	pool.Close()
	runtime.Close()

	restarted, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetCompanyOverview(ctx, "r06-recovery-ux")
	if err != nil {
		t.Fatal(err)
	}
	hasReconcile := false
	for _, employee := range view.Employees {
		if employee.SessionState != nil && *employee.SessionState == "reconcile_required" {
			hasReconcile = true
		}
	}
	if view.Meta.RecoveryState != "recovery_required" || !hasReconcile {
		t.Fatalf("recovery projection = %+v", view.Meta)
	}
}
