// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"testing"

	"polis/internal/core"
)

func TestMissionCommandGoalAndTerminalTransitions(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r05a-command-company")
	must(t, err)

	created, err := k.TXCreateMissionGoal(ctx, scope, "R0.5A goal", "persist and run a deterministic mission", "mission-create-request")
	must(t, err)
	if created.Status != "draft" || created.ID == "" {
		t.Fatalf("unexpected create receipt: %+v", created)
	}

	started, err := k.TXStartMissionCommand(ctx, scope, created.ID, "mission-start-request")
	must(t, err)
	if started.ID != created.ID || started.Status != "active" {
		t.Fatalf("unexpected start receipt: %+v", started)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, created.ID, "mission-start-retry"); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate start error = %v, want %s", err, core.Conflict)
	}

	cancelled, err := k.TXCancelMission(ctx, scope, created.ID, "mission-cancel-request")
	must(t, err)
	if cancelled.ID != created.ID || cancelled.Status != "cancelled" {
		t.Fatalf("unexpected cancel receipt: %+v", cancelled)
	}
	if _, err = k.TXCancelMission(ctx, scope, created.ID, "mission-cancel-retry"); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate cancel error = %v, want %s", err, core.Conflict)
	}

	var title, goal, state string
	must(t, k.pool.QueryRow(ctx, "SELECT title,goal,state FROM missions WHERE company_id=$1 AND id=$2", scope.company, created.ID).Scan(&title, &goal, &state))
	if title != "R0.5A goal" || goal != "persist and run a deterministic mission" || state != "cancelled" {
		t.Fatalf("authoritative mission = %q/%q/%q", title, goal, state)
	}
}

func TestMissionCommandCreateIsIdempotentByRequestKey(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r05a-command-idempotency-company")
	must(t, err)

	first, err := k.TXCreateMissionGoal(ctx, scope, "same request", "same goal", "same-request-key")
	must(t, err)
	second, err := k.TXCreateMissionGoal(ctx, scope, "same request", "same goal", "same-request-key")
	must(t, err)
	if first.ID != second.ID {
		t.Fatalf("same request created two missions: %q and %q", first.ID, second.ID)
	}
	if _, err = k.TXCreateMissionGoal(ctx, scope, "same request", "changed goal", "same-request-key"); !errors.Is(err, core.Conflict) {
		t.Fatalf("changed retry error = %v, want %s", err, core.Conflict)
	}

	var count int
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM missions WHERE company_id=$1", scope.company).Scan(&count))
	if count != 1 {
		t.Fatalf("mission count = %d, want 1", count)
	}
}

func TestProductWorkerFailureAndStopFenceStaleWriter(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r05b1-stale-writer-company")
	must(t, err)
	created, err := k.TXCreateMissionGoal(ctx, scope, "stale writer bridge", "verify old worker writers are fenced", "stale-create")
	must(t, err)
	_, err = k.TXStartMissionCommand(ctx, scope, created.ID, "stale-start")
	must(t, err)
	task, err := k.TXPrepareProductTask(ctx, scope, created.ID, "verify old worker writers are fenced", "stale-prepare")
	must(t, err)
	binding, err := k.TXNewWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	must(t, err)
	stale := binding
	stale.epoch--
	if err = k.TXFinalizeWorkerBeforeProcess(ctx, stale, "stale writer", "stale-failure"); !errors.Is(err, core.StaleEpoch) {
		t.Fatalf("stale worker failure writer error=%v, want %s", err, core.StaleEpoch)
	}
	must(t, k.TXFinalizeWorkerBeforeProcess(ctx, binding, "offline process did not start", "valid-failure"))
	var state string
	must(t, k.pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2", scope.company, binding.session).Scan(&state))
	if state != "stopped" {
		t.Fatalf("worker state=%s, want stopped", state)
	}
}
