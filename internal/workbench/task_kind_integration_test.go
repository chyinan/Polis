// pattern: Imperative Shell
package workbench

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/kernel"
)

func TestWorkbenchDisplaysInternalAndProviderTaskKinds(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B5_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B5 PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("r05b5-workbench-%x", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Task kind projection", "show authoritative Mission tasks", "r05b5-workbench-create")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "r05b5-workbench-start"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXPrepareProductTask(ctx, scope, mission.ID, "show authoritative Mission tasks", "r05b5-workbench-prepare-"+mission.ID); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	overview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Tasks) != 2 {
		t.Fatalf("Workbench Task count = %d, want both authoritative Mission Tasks", len(overview.Tasks))
	}
	seen := map[string]TaskSummary{}
	for _, task := range overview.Tasks {
		seen[task.Kind] = task
	}
	bootstrap, hasBootstrap := seen[string(core.TaskKindBootstrapPlan)]
	compat, hasCompat := seen[string(core.TaskKindCompat)]
	if !hasBootstrap || bootstrap.OwnerEmployeeID != "emp-planning" || bootstrap.Title != "bootstrap_plan" {
		t.Fatalf("Workbench lost or misclassified the planning/control Task: %+v", bootstrap)
	}
	if !hasCompat || compat.OwnerEmployeeID != "emp-backend" || compat.Title != "compat" {
		t.Fatalf("Workbench lost or misclassified the provider-executable Task: %+v", compat)
	}
}
