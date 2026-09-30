package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectNodeNPMProjectPinsCleanInstallAndAllowedRegistry(t *testing.T) {
	root := writeNodeProject(t, `{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`, `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)

	plan, err := InspectNodeNPMProject(root, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProfileID != WindowsNodeNPMProfile || plan.LockfileVersion != 3 || len(plan.PackageJSONSHA256) != 64 || len(plan.LockfileSHA256) != 64 {
		t.Fatalf("Node environment plan is incomplete: %+v", plan)
	}
	args, err := plan.InstallArgs(`C:\\node\\node.exe`, `C:\\node\\node_modules\\npm\\bin\\npm-cli.js`)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, required := range []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org/"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("install arguments omit %q: %v", required, args)
		}
	}
	if strings.Contains(joined, "cmd.exe") || strings.Contains(joined, "npm run") {
		t.Fatalf("environment plan contains shell/script execution: %v", args)
	}
	policy, _, policyDigest, err := PolicyForNodeNPMPlan(plan, 180_000, 1<<20)
	if err != nil || policyDigest == "" || policy.RegistryHosts[0] != "registry.npmjs.org" || policy.LifecycleScriptsPolicy != "ignore" {
		t.Fatalf("Node policy manifest = %+v digest=%q err=%v", policy, policyDigest, err)
	}
}

func TestInspectNodeNPMProjectRejectsUnapprovedDependencySources(t *testing.T) {
	root := writeNodeProject(t, `{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`, `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://packages.example.invalid/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)
	if _, err := InspectNodeNPMProject(root, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("unapproved dependency registry was accepted")
	}
}

func TestInspectNodeNPMProjectRejectsLocalNPMConfigurationAndManifestDrift(t *testing.T) {
	t.Run("project npmrc", func(t *testing.T) {
		root := writeNodeProject(t, `{"name":"demo","version":"1.0.0"}`, `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)
		if err := os.WriteFile(filepath.Join(root, ".npmrc"), []byte("registry=https://packages.example.invalid"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectNodeNPMProject(root, []string{"registry.npmjs.org"}); err == nil {
			t.Fatal("project npmrc was accepted")
		}
	})
	t.Run("package lock mismatch", func(t *testing.T) {
		root := writeNodeProject(t, `{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`, `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{}}}}`)
		if _, err := InspectNodeNPMProject(root, []string{"registry.npmjs.org"}); err == nil {
			t.Fatal("manifest/lockfile dependency drift was accepted")
		}
	})
}

func TestNodeNPMProjectPlanRevalidatesSourceBeforePreparation(t *testing.T) {
	root := writeNodeProject(t, `{"name":"demo","version":"1.0.0"}`, `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)
	plan, err := InspectNodeNPMProject(root, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Revalidate([]string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("unchanged source plan was rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"demo","version":"1.0.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Revalidate([]string{"registry.npmjs.org"}); err == nil {
		t.Fatal("changed package source was accepted for preparation")
	}
}

func TestInspectNodeNPMProjectFilesBindsManifestToVerifiedDirectorySnapshot(t *testing.T) {
	manifest := []byte(`{"name":"demo","version":"1.0.0"}`)
	lockfile := []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)
	files := []ProjectSourceFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: manifest},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: lockfile},
		{RelativePath: "repo/README.md", MediaType: "text/markdown", Content: []byte("project notes")},
	}
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceKind != NodeSnapshotFilesSource || plan.ProjectRoot != "repo" || len(plan.PackageJSONSHA256) != 64 || len(plan.LockfileSHA256) != 64 {
		t.Fatalf("directory-bound project plan = %+v", plan)
	}
	if err := RevalidateNodeNPMProjectFiles(plan, files, []string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("unchanged snapshot plan was rejected: %v", err)
	}
	changed := append([]ProjectSourceFile(nil), files...)
	changed[1] = ProjectSourceFile{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.1","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.1"}}}`)}
	if err := RevalidateNodeNPMProjectFiles(plan, changed, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("changed directory lockfile was accepted")
	}
	unsafeFiles := append([]ProjectSourceFile(nil), files...)
	unsafeFiles[2] = ProjectSourceFile{RelativePath: "repo/../outside.txt", MediaType: "text/plain", Content: []byte("escape")}
	if _, err := InspectNodeNPMProjectFiles(unsafeFiles, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("path-escaping source file was accepted")
	}
}

func TestInspectNodeNPMProjectFilesUsesDotForRootLevelZIPProjects(t *testing.T) {
	files := []ProjectSourceFile{
		{RelativePath: "package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
	}
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProjectRoot != "." {
		t.Fatalf("root-level ZIP project root = %q, want a safe relative dot path", plan.ProjectRoot)
	}
	if err = RevalidateNodeNPMProjectFiles(plan, files, []string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("unchanged root-level project plan did not revalidate: %v", err)
	}
}

func writeNodeProject(t *testing.T, manifest, lockfile string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(lockfile), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
