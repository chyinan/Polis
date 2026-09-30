// pattern: Functional Core
package environment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLinuxNodeProfileUsesCaseSensitiveSourceAndOfflinePolicy(t *testing.T) {
	files := []ProjectSourceFile{
		{RelativePath: "Repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "Repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
		{RelativePath: "Repo/README.md", MediaType: "text/markdown", Content: []byte("project notes")},
		{RelativePath: "repo/notes.md", MediaType: "text/markdown", Content: []byte("case-distinct path")},
		{RelativePath: "Repo/Notes.md", MediaType: "text/markdown", Content: []byte("case-distinct sibling")},
	}
	plan, err := InspectLinuxNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProfileID != LinuxNodeNPMProfile || plan.ProjectRoot != "Repo" || plan.InstallPolicy != LinuxNodeNPMInstallPolicy {
		t.Fatalf("Linux Node plan=%+v", plan)
	}
	if err = RevalidateLinuxNodeNPMProjectFiles(plan, files, []string{"registry.npmjs.org"}); err != nil {
		t.Fatalf("unchanged Linux snapshot did not revalidate: %v", err)
	}
	if _, err = InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"}); err == nil {
		t.Fatal("Windows profile accepted Linux case-colliding paths")
	}
	policy, encoded, digest, err := BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	parsed, canonical, parsedDigest, err := ParseProjectEnvironmentPolicy(encoded)
	if err != nil || parsedDigest != digest || !reflect.DeepEqual(parsed, policy) || !json.Valid(canonical) {
		t.Fatalf("Linux policy roundtrip=%+v digest=%q error=%v", parsed, parsedDigest, err)
	}
	if policy.NetworkPolicy != "deny_all" || policy.InstallPolicy != LinuxNodeNPMInstallPolicy {
		t.Fatalf("Linux offline policy enabled network or scripts: %+v", policy)
	}
	unsafe := policy
	unsafe.NetworkPolicy = "registry_allowlist"
	if _, _, _, err = CanonicalizeLinuxNodeEnvironmentPolicy(unsafe); err == nil {
		t.Fatal("Linux profile admitted live registry network access")
	}
}

func TestLinuxNodeBubblewrapPlansUseDenyNetworkAndPinnedReadOnlyToolchain(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux ownership metadata fixture requires Linux")
	}
	root, err := os.MkdirTemp("/tmp", "polis-linux-node-toolchain-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	systemImage := filepath.Join(root, "image")
	workspace := filepath.Join(root, "workspace")
	cgroupRoot := filepath.Join(root, "delegated-cgroups")
	if err := os.MkdirAll(filepath.Join(systemImage, "usr", "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(systemImage, "usr", "lib", "node_modules", "npm", "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, mountpoint := range []string{"proc", "dev", "tmp", "workspace"} {
		if err := os.Mkdir(filepath.Join(systemImage, mountpoint), 0700); err != nil {
			t.Fatal(err)
		}
	}
	npmCache := filepath.Join(root, "npm-cache")
	if err := os.Mkdir(npmCache, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(npmCache, "_cacache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cgroupRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npmCache, "_cacache", "index-v5"), []byte("offline package index"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "Repo", "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(workspace, ".npm-cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "Repo", "scripts", "build.mjs"), []byte("console.log('fixture')"), 0600); err != nil {
		t.Fatal(err)
	}
	bwrapPath := filepath.Join(root, "bwrap")
	nodePath := filepath.Join(systemImage, "usr", "bin", "node")
	npmPath := filepath.Join(systemImage, "usr", "lib", "node_modules", "npm", "bin", "npm-cli.js")
	for _, path := range []string{bwrapPath, nodePath, npmPath} {
		mode := os.FileMode(0600)
		if path == bwrapPath || path == nodePath {
			mode = 0700
		}
		if err := os.WriteFile(path, []byte("offline fixture"), mode); err != nil {
			t.Fatal(err)
		}
	}
	plan := NodeNPMProjectPlan{
		ProfileID: LinuxNodeNPMProfile, SourceKind: NodeSnapshotFilesSource, ProjectRoot: "Repo", PackageName: "demo",
		PackageJSONSHA256: repeatNodeDigest('a'), LockfileSHA256: repeatNodeDigest('b'), LockfileVersion: 3,
		RegistryHosts: []string{"registry.npmjs.org"}, InstallPolicy: LinuxNodeNPMInstallPolicy,
	}
	config := LinuxNodeSandboxPaths{BubblewrapPath: bwrapPath, RuntimeRoot: root, SystemImageRoot: systemImage, WorkspaceRoot: workspace, NPMCacheRoot: npmCache, CgroupRoot: cgroupRoot, NodeExecutable: "/usr/bin/node", NPMCLIScript: "/usr/lib/node_modules/npm/bin/npm-cli.js"}
	toolchainDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || validateLinuxNodeToolchainSHA256(toolchainDigest) != nil {
		t.Fatalf("Linux toolchain fingerprint=%q error=%v", toolchainDigest, err)
	}
	changedWorkspaceLimit := config
	changedWorkspaceLimit.WorkspaceDiskLimitBytes = DefaultLinuxNodeWorkspaceDiskLimitBytes / 2
	workspaceLimitDigest, err := LinuxNodeToolchainSHA256(changedWorkspaceLimit)
	if err != nil || workspaceLimitDigest == toolchainDigest {
		t.Fatalf("workspace disk limit drift kept its old toolchain digest %q, error=%v", toolchainDigest, err)
	}
	if err = os.Chmod(bwrapPath, 0500); err != nil {
		t.Fatal(err)
	}
	bwrapPermissionDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || bwrapPermissionDigest == toolchainDigest {
		t.Fatalf("bwrap permission drift kept its old toolchain digest %q, error=%v", toolchainDigest, err)
	}
	if err = os.Chmod(bwrapPath, 0700); err != nil {
		t.Fatal(err)
	}
	bwrapPermissionRestored, err := LinuxNodeToolchainSHA256(config)
	if err != nil || bwrapPermissionRestored != toolchainDigest {
		t.Fatalf("restored bwrap permissions did not restore digest: got=%q want=%q err=%v", bwrapPermissionRestored, toolchainDigest, err)
	}
	install, err := BuildLinuxNodeNPMInstallArgv(config, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !containsLinuxArg(install, "--unshare-all") || !containsLinuxArg(install, "--new-session") || !containsLinuxArg(install, "--die-with-parent") || containsLinuxArg(install, "--share-net") {
		t.Fatalf("Linux install argv does not fail closed: %v", install)
	}
	if !containsLinuxSequence(install, "--ro-bind", systemImage, "/") || !containsLinuxSequence(install, "--bind", workspace, "/workspace") || !containsLinuxArg(install, "--offline") || !containsLinuxSequence(install, "--setenv", "NPM_CONFIG_CACHE", "/workspace/.npm-cache") {
		t.Fatalf("Linux install argv lacks pinned read-only rootfs or offline cache: %v", install)
	}
	project, err := BuildLinuxNodeProjectScriptArgv(config, plan, "scripts/build.mjs", []string{"--verify"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsLinuxSequence(project, "--chdir", "/workspace/Repo") || !containsLinuxSequence(project, nodePathInSandbox(config), "/workspace/Repo/scripts/build.mjs", "--verify") {
		t.Fatalf("Linux project command escaped its exact workspace/argv: %v", project)
	}
	if _, err = BuildLinuxNodeProjectScriptArgv(config, plan, "../outside.js", nil); err == nil {
		t.Fatal("Linux project command accepted path traversal")
	}
	badRoot := config
	badRoot.RuntimeRoot = t.TempDir()
	if _, err = BuildLinuxNodeNPMInstallArgv(badRoot, plan); err == nil {
		t.Fatal("Linux sandbox accepted system-image/workspace roots outside the managed runtime root")
	}
	if err = os.WriteFile(nodePath, []byte("changed fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	changedDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || changedDigest == toolchainDigest {
		t.Fatalf("system image drift kept its old toolchain digest %q, error=%v", changedDigest, err)
	}
	if err = os.WriteFile(nodePath, []byte("offline fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	stableDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || stableDigest != toolchainDigest {
		t.Fatalf("restored system image did not restore toolchain digest: got=%q want=%q err=%v", stableDigest, toolchainDigest, err)
	}
	if err = os.Chmod(nodePath, 0500); err != nil {
		t.Fatal(err)
	}
	permissionChangedDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || permissionChangedDigest == toolchainDigest {
		t.Fatalf("system-image permission drift kept its old toolchain digest %q, error=%v", toolchainDigest, err)
	}
	if err = os.Chmod(nodePath, 0700); err != nil {
		t.Fatal(err)
	}
	permissionRestoredDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || permissionRestoredDigest != toolchainDigest {
		t.Fatalf("restored system-image permissions did not restore toolchain digest: got=%q want=%q err=%v", permissionRestoredDigest, toolchainDigest, err)
	}
	if err = os.WriteFile(filepath.Join(npmCache, "_cacache", "index-v5"), []byte("changed package index"), 0600); err != nil {
		t.Fatal(err)
	}
	cacheChangedDigest, err := LinuxNodeToolchainSHA256(config)
	if err != nil || cacheChangedDigest == toolchainDigest {
		t.Fatalf("offline npm cache drift kept its old toolchain digest %q, error=%v", toolchainDigest, err)
	}
}

func TestLinuxNodeOwnershipFingerprintIncludesUIDAndGID(t *testing.T) {
	first, ok := linuxNodeOwnershipFingerprint(linuxNodeOwnerFixture{Uid: 1001, Gid: 1002})
	if !ok || first == "" {
		t.Fatalf("Linux owner metadata was not recognized: %q %t", first, ok)
	}
	changedUID, _ := linuxNodeOwnershipFingerprint(linuxNodeOwnerFixture{Uid: 1003, Gid: 1002})
	changedGID, _ := linuxNodeOwnershipFingerprint(linuxNodeOwnerFixture{Uid: 1001, Gid: 1004})
	if changedUID == first || changedGID == first {
		t.Fatalf("owner changes must change metadata fingerprints: first=%q uid=%q gid=%q", first, changedUID, changedGID)
	}
}

func TestLinuxNodeSystemImageFingerprintRejectsDirectoryMutationDuringScan(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux rootfs metadata fixture requires Linux")
	}
	root := linuxNodeSystemImageFixture(t)
	directory := filepath.Join(root, "bin")
	mutated := false
	_, err := linuxNodeSystemImageSHA256WithObserver(root, func(current string) error {
		if current == directory {
			mutated = true
			return os.Chmod(directory, 0500)
		}
		return nil
	})
	if err == nil || !mutated {
		t.Fatalf("fingerprinting accepted directory mode mutation: mutated=%t err=%v", mutated, err)
	}
}

func TestLinuxNodeSystemImageFingerprintRejectsSymlinkMutationDuringScan(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux rootfs symlink fixture requires Linux")
	}
	root := linuxNodeSystemImageFixture(t)
	link := filepath.Join(root, "node-link")
	if err := os.Symlink("target-one", link); err != nil {
		t.Fatal(err)
	}
	mutated := false
	_, err := linuxNodeSystemImageSHA256WithObserver(root, func(current string) error {
		if current == link {
			mutated = true
			if removeErr := os.Remove(link); removeErr != nil {
				return removeErr
			}
			return os.Symlink("target-two", link)
		}
		return nil
	})
	if err == nil || !mutated {
		t.Fatalf("fingerprinting accepted symlink mutation: mutated=%t err=%v", mutated, err)
	}
}

func linuxNodeSystemImageFixture(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "polis-linux-node-image-scan-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err = os.Mkdir(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "bin", "node"), []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

type linuxNodeOwnerFixture struct {
	Uid uint32
	Gid uint32
}

func TestLinuxNodeOfflineCacheCopiesBoundedRegularFilesAndRejectsLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-cache")
	destination := filepath.Join(root, "workspace-cache")
	if err := os.MkdirAll(filepath.Join(source, "_cacache", "content"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("offline package tarball")
	if err := os.WriteFile(filepath.Join(source, "_cacache", "content", "package.tgz"), content, 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := LinuxNodeNPMCacheSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = MaterializeLinuxNodeNPMCache(source, destination); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(destination, "_cacache", "content", "package.tgz"))
	if err != nil || string(copied) != string(content) {
		t.Fatalf("offline cache copy=%q err=%v", copied, err)
	}
	copiedDigest, err := LinuxNodeNPMCacheSHA256(destination)
	if err != nil || copiedDigest != digest {
		t.Fatalf("offline cache digest=%q want=%q err=%v", copiedDigest, digest, err)
	}
	if err = MaterializeLinuxNodeNPMCache(source, destination); err == nil {
		t.Fatal("offline cache copy accepted a non-empty workspace destination")
	}
	if runtime.GOOS != "windows" {
		linked := filepath.Join(root, "linked-cache")
		if err = os.Mkdir(linked, 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink(filepath.Join(source, "_cacache"), filepath.Join(linked, "external")); err != nil {
			t.Fatal(err)
		}
		if _, err = LinuxNodeNPMCacheSHA256(linked); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cache fingerprint accepted a link: %v", err)
		}
	}
}

func repeatNodeDigest(character byte) string {
	return strings.Repeat(string(character), 64)
}

func containsLinuxArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func containsLinuxSequence(args []string, sequence ...string) bool {
	for index := 0; index+len(sequence) <= len(args); index++ {
		matched := true
		for offset, target := range sequence {
			if args[index+offset] != target {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func nodePathInSandbox(config LinuxNodeSandboxPaths) string { return config.NodeExecutable }
