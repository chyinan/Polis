// pattern: Imperative Shell
package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeNodeNPMProjectFilesCreatesVerifiedSnapshot(t *testing.T) {
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	materialized, err := MaterializeNodeNPMProjectFiles(root, "env-revision-1", files, plan, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if materialized.WorkspaceRoot != filepath.Join(root, "env-revision-1") || materialized.ProjectRoot != filepath.Join(materialized.WorkspaceRoot, "repo") || materialized.FileCount != len(files) || materialized.SourceBytes == 0 {
		t.Fatalf("materialized project summary = %+v", materialized)
	}
	for _, file := range files {
		path := filepath.Join(materialized.WorkspaceRoot, filepath.FromSlash(file.RelativePath))
		content, readErr := os.ReadFile(path)
		if readErr != nil || string(content) != string(file.Content) {
			t.Fatalf("materialized file %q differs: err=%v", file.RelativePath, readErr)
		}
	}
	if err = RevalidateNodeNPMProjectFiles(plan, files, []string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("source plan changed after materialization: %v", err)
	}
	materializedPlan, err := InspectNodeNPMProject(materialized.ProjectRoot, []string{"registry.npmjs.org"})
	if err != nil || materializedPlan.PackageJSONSHA256 != plan.PackageJSONSHA256 || materializedPlan.LockfileSHA256 != plan.LockfileSHA256 {
		t.Fatalf("materialized package metadata failed revalidation: plan=%+v err=%v", materializedPlan, err)
	}
}

func TestMaterializeNodeNPMProjectFilesRejectsUnsafeNamesAndPlanDriftBeforeWriting(t *testing.T) {
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		files []ProjectSourceFile
		plan  NodeNPMProjectPlan
	}{
		{name: "traversal", files: replaceMaterializeFixture(files, 2, "../escape.txt"), plan: plan},
		{name: "windows device name", files: replaceMaterializeFixture(files, 2, "repo/CON.txt"), plan: plan},
		{name: "metadata drift", files: replaceMaterializeFixture(files, 0, "repo/package.json"), plan: plan},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.name == "metadata drift" {
				test.files[0].Content = []byte(`{"name":"demo","version":"9.9.9"}`)
			}
			_, materializeErr := MaterializeNodeNPMProjectFiles(root, "env-revision-1", test.files, test.plan, []string{"registry.npmjs.org"})
			if materializeErr == nil {
				t.Fatal("unsafe source or stale plan was materialized")
			}
			if _, statErr := os.Lstat(filepath.Join(root, "env-revision-1")); !os.IsNotExist(statErr) {
				t.Fatalf("failed materialization left a workspace behind: %v", statErr)
			}
		})
	}
}

func TestMaterializeNodeNPMProjectFilesRejectsExistingWorkspaceAndSymlinkRoot(t *testing.T) {
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	existing := filepath.Join(root, "existing")
	if err = os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = MaterializeNodeNPMProjectFiles(root, "existing", files, plan, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("existing workspace was overwritten")
	}
	outside := t.TempDir()
	link := filepath.Join(t.TempDir(), "root-link")
	if err = os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable on this host: %v", err)
	}
	if _, err = MaterializeNodeNPMProjectFiles(link, "env-revision-2", files, plan, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("symlink destination root was accepted")
	}
	entries, readErr := os.ReadDir(outside)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("symlink root wrote outside the managed root: entries=%d err=%v", len(entries), readErr)
	}
}

func materializeFixtureFiles() []ProjectSourceFile {
	return []ProjectSourceFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
		{RelativePath: "repo/src/index.js", MediaType: "text/javascript", Content: []byte("export default 'ok';\n")},
	}
}

func replaceMaterializeFixture(files []ProjectSourceFile, index int, relativePath string) []ProjectSourceFile {
	result := append([]ProjectSourceFile(nil), files...)
	result[index].RelativePath = relativePath
	result[index].Content = []byte(strings.Repeat("x", len(result[index].Content)))
	return result
}
