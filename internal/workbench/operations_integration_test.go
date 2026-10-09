// pattern: Imperative Shell
package workbench

import (
	"context"

	"testing"

	"polis/internal/kernel"
	"polis/internal/pgfixture"
)

func TestPostgresOperationsProjectionPreservesUnavailableTokenSemantics(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err = createPGTestCompany(runtime, ctx, "r06-operations"); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetOperations(ctx, "r06-operations")
	if err != nil {
		t.Fatal(err)
	}
	if view.ToolBudgetQuality != "unavailable" || view.InputTokens != nil || view.OutputTokens != nil || view.PostgreSQLStatus != "ready" {
		t.Fatalf("operations view = %+v", view)
	}
}
