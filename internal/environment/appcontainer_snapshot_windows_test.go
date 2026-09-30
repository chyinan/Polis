// pattern: Imperative Shell
//go:build windows

package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsNodeSnapshotMaterializesInsideFreshAppContainerProfile(t *testing.T) {
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := PrepareWindowsNodeAppContainerSnapshot("environment-test", "workspace-1", files, plan, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := snapshot.Close(); closeErr != nil {
			t.Errorf("close AppContainer profile: %v", closeErr)
		}
	}()
	if !strings.HasPrefix(strings.ToLower(snapshot.materialized.WorkspaceRoot), strings.ToLower(snapshot.sandbox.WorkspaceRoot()+string(filepath.Separator))) {
		t.Fatalf("materialized workspace escaped its AppContainer profile: sandbox=%q workspace=%q", snapshot.sandbox.WorkspaceRoot(), snapshot.materialized.WorkspaceRoot)
	}
	manifestPath := filepath.Join(snapshot.materialized.ProjectRoot, "package.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil || string(manifest) != string(files[0].Content) {
		t.Fatalf("materialized AppContainer package manifest differs: err=%v", err)
	}
	if err = RevalidateNodeNPMProjectFiles(plan, files, []string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("source revision changed during staging: %v", err)
	}
}
