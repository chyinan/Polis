package environment

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestApplyDependencyChangeToPackageJSONChangesOnlyRootProductionDependencies(t *testing.T) {
	input := []byte(`{"name":"demo","version":"1.0.0","description":"preserve me","scripts":{"test":"node test.js"},"dependencies":{"left-pad":"1.3.0"},"devDependencies":{"test-kit":"2.0.0"}}`)
	proposal := dependencyChangeFixture("1.4.0")
	updated, err := ApplyDependencyChangeToPackageJSON(input, proposal)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(input, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(updated, &after); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "version", "description", "scripts", "devDependencies"} {
		if !bytes.Equal(before[key], after[key]) {
			t.Fatalf("unrelated package.json field %q changed: before=%s after=%s", key, before[key], after[key])
		}
	}
	var dependencies map[string]string
	if err := json.Unmarshal(after["dependencies"], &dependencies); err != nil {
		t.Fatal(err)
	}
	if dependencies["left-pad"] != "1.4.0" || dependencies["new-package"] != "2.3.4" {
		t.Fatalf("root dependencies=%v", dependencies)
	}
	if bytes.Equal(input, updated) {
		t.Fatal("dependency overlay returned the unchanged manifest")
	}
}

func TestBuildWindowsDependencyResolutionArgsIsFixedAndRegistryBound(t *testing.T) {
	plan := NodeNPMProjectPlan{
		ProfileID: WindowsNodeNPMProfile, SourceKind: NodeSnapshotFilesSource,
		InstallPolicy: WindowsNodeInstallPolicy, RegistryHosts: []string{"registry.npmjs.org"},
	}
	args, err := BuildWindowsNodeNPMDependencyResolutionArgs(plan, `C:\node\node.exe`, `C:\node\npm-cli.js`, "registry.npmjs.org")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`C:\node\node.exe`, `C:\node\npm-cli.js`, "install", "--package-lock-only", "--ignore-scripts", "--no-audit", "--no-fund", "--no-save", "--workspaces=false", "--registry=https://registry.npmjs.org/"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("resolver argv=%q, want fixed vector %q", args, want)
	}
	for _, forbidden := range []string{"cmd.exe", "powershell", "npm run", "--script-shell", "host/path"} {
		for _, arg := range args {
			if strings.Contains(strings.ToLower(arg), forbidden) {
				t.Fatalf("resolver argv contains forbidden capability %q: %q", forbidden, arg)
			}
		}
	}
	if _, err = BuildWindowsNodeNPMDependencyResolutionArgs(plan, `C:\node\node.exe`, `C:\node\npm-cli.js`, "registry.example.invalid"); err == nil {
		t.Fatal("resolver accepted a registry outside the plan")
	}
	plan.ProfileID = LinuxNodeNPMProfile
	if _, err = BuildWindowsNodeNPMDependencyResolutionArgs(plan, `C:\node\node.exe`, `C:\node\npm-cli.js`, "registry.npmjs.org"); err == nil {
		t.Fatal("Windows registry resolver accepted the Linux deny-all profile")
	}
}

func TestApplyDependencyChangeToPackageJSONRejectsAmbiguousOrConflictingManifests(t *testing.T) {
	proposal := dependencyChangeFixture("1.4.0")
	for name, input := range map[string][]byte{
		"duplicate root key":     []byte(`{"name":"demo","name":"shadow","version":"1.0.0"}`),
		"duplicate nested key":   []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0","left-pad":"1.4.0"}}`),
		"case variant key":       []byte(`{"name":"demo","version":"1.0.0","Dependencies":{"left-pad":"1.3.0"}}`),
		"non-object dependency":  []byte(`{"name":"demo","version":"1.0.0","dependencies":[]}`),
		"non-string dependency":  []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":1}}`),
		"category conflict":      []byte(`{"name":"demo","version":"1.0.0","devDependencies":{"left-pad":"1.3.0"}}`),
		"unchanged proposal":     []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.4.0","new-package":"2.3.4"}}`),
		"workspaces unsupported": []byte(`{"name":"demo","version":"1.0.0","workspaces":["packages/*"]}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ApplyDependencyChangeToPackageJSON(input, proposal); err == nil {
				t.Fatal("unsafe or conflicting package manifest was accepted")
			}
		})
	}
}

func TestPrepareDependencyChangeProjectFilesIsImmutableAndBoundToProjectRoot(t *testing.T) {
	manifest := []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`)
	lockfile := []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)
	files := []ProjectSourceFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: manifest},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: lockfile},
		{RelativePath: "repo/README.md", MediaType: "text/markdown", Content: []byte("keep")},
	}
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	updatedFiles, updatedManifest, err := PrepareDependencyChangeProjectFiles(files, plan, dependencyChangeFixture("1.4.0"))
	if err != nil {
		t.Fatal(err)
	}
	stalePlan := plan
	stalePlan.PackageJSONSHA256 = strings.Repeat("0", 64)
	if _, _, err := PrepareDependencyChangeProjectFiles(files, stalePlan, dependencyChangeFixture("1.4.0")); err == nil {
		t.Fatal("overlay accepted a base snapshot that did not match the frozen plan")
	}
	if !bytes.Equal(files[0].Content, manifest) || !bytes.Equal(files[1].Content, lockfile) || !bytes.Equal(files[2].Content, []byte("keep")) {
		t.Fatal("input source files were mutated")
	}
	if !bytes.Equal(updatedFiles[0].Content, updatedManifest) || !bytes.Equal(updatedFiles[1].Content, lockfile) || !bytes.Equal(updatedFiles[2].Content, []byte("keep")) {
		t.Fatalf("unexpected bounded overlay: %+v", updatedFiles)
	}
}

func TestValidateDependencyChangeMaterializationRequiresExactManifestAndValidPinnedLock(t *testing.T) {
	proposal := dependencyChangeFixture("1.4.0")
	baseFiles := []ProjectSourceFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}},"node_modules/left-pad":{"version":"1.3.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)},
	}
	plan, err := InspectNodeNPMProjectFiles(baseFiles, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err := PrepareDependencyChangeProjectFiles(baseFiles, plan, proposal)
	if err != nil {
		t.Fatal(err)
	}
	lock := []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0","dependencies":{"left-pad":"1.4.0","new-package":"2.3.4"}},"node_modules/left-pad":{"version":"1.4.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.4.0.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="},"node_modules/new-package":{"version":"2.3.4","resolved":"https://registry.npmjs.org/new-package/-/new-package-2.3.4.tgz","integrity":"sha512-YxQ7G4n4sT4xFq6wKxJ7L4U2wQfV9Q2b0y3vW8s5x8w="}}}`)
	files, resolvedPlan, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, lock, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatalf("valid resolved lockfile rejected: %v", err)
	}
	if resolvedPlan.PackageJSONSHA256 == plan.PackageJSONSHA256 || resolvedPlan.LockfileSHA256 == plan.LockfileSHA256 || len(files) != len(baseFiles) {
		t.Fatalf("resolved materialization did not produce a new exact plan: %+v", resolvedPlan)
	}
	wrongManifest := bytes.Replace(manifest, []byte(`"new-package":"2.3.4"`), []byte(`"new-package":"2.3.5"`), 1)
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, wrongManifest, lock, []string{"registry.npmjs.org"}); !errors.Is(err, errDependencyMaterialization) {
		t.Fatalf("different package manifest returned error %v", err)
	}
	duplicateLock := bytes.Replace(lock, []byte(`"lockfileVersion":3`), []byte(`"lockfileVersion":3,"lockfileVersion":2`), 1)
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, duplicateLock, []string{"registry.npmjs.org"}); !errors.Is(err, errDependencyMaterialization) {
		t.Fatalf("duplicate lockfile key returned error %v", err)
	}
	caseVariantLock := bytes.Replace(lock, []byte(`"lockfileVersion":3`), []byte(`"LockfileVersion":3`), 1)
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, caseVariantLock, []string{"registry.npmjs.org"}); !errors.Is(err, errDependencyMaterialization) {
		t.Fatalf("case-variant lockfile key returned error %v", err)
	}
	unsafeLock := bytes.Replace(lock, []byte("registry.npmjs.org"), []byte("registry.example.invalid"), 1)
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, unsafeLock, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("lockfile containing an unapproved tarball host was accepted")
	}
	approvedMirrorLock := bytes.Replace(lock, []byte("https://registry.npmjs.org/new-package"), []byte("https://mirror.example.invalid/new-package"), 1)
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, approvedMirrorLock, []string{"registry.npmjs.org", "mirror.example.invalid"}); err != nil {
		t.Fatalf("existing base-policy mirror was rejected from resolved lockfile: %v", err)
	}
	if _, _, err := ValidateDependencyChangeMaterialization(baseFiles, plan, proposal, manifest, lock, []string{"packages.example.invalid"}); err == nil {
		t.Fatal("proposal registry host outside the base environment policy was accepted")
	}
}

func dependencyChangeFixture(version string) DependencyChangeProposal {
	return DependencyChangeProposal{
		MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "dependency-change-1",
		RegistryHosts: []string{"registry.npmjs.org"},
		Dependencies:  map[string]string{"left-pad": version, "new-package": "2.3.4"},
		Rationale:     "approved dependency change fixture",
	}
}
