// pattern: Imperative Shell
package workbench

import (
	"context"
	"os"
	"testing"

	"polis/internal/kernel"
)

func TestPostgresWorkspacePreviewReadsAuthorizedCASBlob(t *testing.T) {
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
	defer runtime.Close()
	scope, err := runtime.TXCreateCompany(ctx, "r06-workspace-preview")
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, "mission-1", "start-1"); err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXPrepareProductTask(ctx, scope, "mission-1", "preview this workspace", "prepare-1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStoreWithBlobRoot(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetWorkspace(ctx, "r06-workspace-preview", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Files) != 1 || view.Files[0].Content == "" || view.Files[0].Path != "workspace.txt" {
		t.Fatalf("workspace view = %+v", view)
	}
}
