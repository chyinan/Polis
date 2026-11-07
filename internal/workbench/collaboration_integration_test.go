// pattern: Imperative Shell
package workbench

import (
	"context"
	"os"
	"testing"

	"polis/internal/kernel"
)

func TestPostgresCollaborationProjectionPreservesMessageAndObligation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	scope, err := runtime.TXCreateCompany(ctx, "r06-collaboration")
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, "mission-1", "start-1"); err != nil {
		t.Fatal(err)
	}
	planner, err := runtime.BindFake(ctx, scope, "emp-planning")
	if err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXClaim(ctx, planner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXSend(ctx, planner, task, "message-1", "Please implement the accepted contract."); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.ListCollaboration(ctx, "r06-collaboration")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Content == "" || items[0].ObligationState == nil || *items[0].ObligationState != "pending" {
		t.Fatalf("collaboration projection = %+v", items)
	}
}
