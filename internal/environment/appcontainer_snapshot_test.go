// pattern: Imperative Shell
package environment

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"polis/internal/runner"
)

func TestPrepareWindowsNodeAppContainerSnapshotRemainsUnavailableOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows AppContainer integration runs in its platform-specific test")
	}
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareWindowsNodeAppContainerSnapshot("job-1", "workspace-1", files, plan, []string{"registry.npmjs.org"}); !errors.Is(err, runner.ErrAppContainerUnavailable) {
		t.Fatalf("non-Windows AppContainer preparation error=%v", err)
	}
}

func TestPrepareWindowsNodeNPMInstallSnapshotRemainsUnavailableOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("registry-only Windows profile is not exercised by the cross-platform suite")
	}
	files := materializeFixtureFiles()
	plan, err := InspectNodeNPMProjectFiles(files, []string{"registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	toolchainRoot := t.TempDir()
	nodePath := filepath.Join(toolchainRoot, "node.exe")
	npmPath := filepath.Join(toolchainRoot, "npm-cli.js")
	if err = os.WriteFile(nodePath, []byte("trusted node"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(npmPath, []byte("trusted npm"), 0600); err != nil {
		t.Fatal(err)
	}
	toolchainSHA256, err := WindowsNodeToolchainSHA256FromFiles(nodePath, npmPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareWindowsNodeNPMInstallSnapshot("job-1", "workspace-1", files, plan, []string{"registry.npmjs.org"}, toolchainSHA256, nodePath, npmPath, WindowsNodeWorkspaceStorageConfig{}); !errors.Is(err, runner.ErrAppContainerUnavailable) {
		t.Fatalf("non-Windows registry-only AppContainer preparation error=%v", err)
	}
}

func TestRegularContainedToolchainFileRejectsOutsideAndLinkedPaths(t *testing.T) {
	root := t.TempDir()
	toolchain := filepath.Join(root, ".toolchain")
	if err := os.Mkdir(toolchain, 0700); err != nil {
		t.Fatal(err)
	}
	node := filepath.Join(toolchain, "node.exe")
	if err := os.WriteFile(node, []byte("test node"), 0600); err != nil {
		t.Fatal(err)
	}
	if !regularContainedToolchainFile(root, node) {
		t.Fatal("regular toolchain file inside the workspace was rejected")
	}
	outside := filepath.Join(t.TempDir(), "node.exe")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if regularContainedToolchainFile(root, outside) {
		t.Fatal("toolchain file outside the workspace was accepted")
	}
	if regularContainedToolchainFile(root, filepath.Join(toolchain, "missing.exe")) {
		t.Fatal("missing toolchain file was accepted")
	}
	link := filepath.Join(root, "linked-toolchain")
	if err := os.Symlink(toolchain, link); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}
	if regularContainedToolchainFile(root, filepath.Join(link, "node.exe")) {
		t.Fatal("toolchain file reached through a linked parent was accepted")
	}
	rootLink := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}
	if regularContainedToolchainFile(rootLink, node) {
		t.Fatal("symlinked workspace root was accepted")
	}
}

func TestWindowsNodeToolchainSHA256FromFilesBindsStagedBytes(t *testing.T) {
	root := t.TempDir()
	nodePath := filepath.Join(root, "node.exe")
	npmPath := filepath.Join(root, "npm-cli.js")
	nodeBytes := []byte("trusted node executable fixture")
	npmBytes := []byte("trusted npm cli fixture")
	if err := os.WriteFile(nodePath, nodeBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(npmPath, npmBytes, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := WindowsNodeToolchainSHA256FromFiles(nodePath, npmPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := WindowsNodeToolchainSHA256(nodeBytes, npmBytes)
	if err != nil || got != want {
		t.Fatalf("file-bound toolchain digest=%q want=%q err=%v", got, want, err)
	}
	if err = os.WriteFile(npmPath, []byte("changed npm cli fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := WindowsNodeToolchainSHA256FromFiles(nodePath, npmPath)
	if err != nil || changed == got {
		t.Fatalf("changed staged npm CLI retained digest %q, original %q, err=%v", changed, got, err)
	}
	stageWorkspace := t.TempDir()
	paths, err := StageWindowsNodeToolchain(stageWorkspace, nodePath, npmPath, changed)
	if err != nil {
		t.Fatalf("stage separately approved Node/npm files: %v", err)
	}
	if filepath.Dir(paths.NodeExecutable) != filepath.Join(stageWorkspace, ".polis-toolchain") || filepath.Dir(paths.NPMCLIScript) != filepath.Join(stageWorkspace, ".polis-toolchain") {
		t.Fatalf("trusted toolchain was not staged into its separate directory: %+v", paths)
	}
	stagedDigest, err := WindowsNodeToolchainSHA256FromFiles(paths.NodeExecutable, paths.NPMCLIScript)
	if err != nil || stagedDigest != changed {
		t.Fatalf("staged toolchain digest=%q want=%q err=%v", stagedDigest, changed, err)
	}
	if err = verifyWindowsNodeToolchainStaging(stageWorkspace, paths, changed); err != nil {
		t.Fatalf("valid toolchain staging did not verify: %v", err)
	}
	projectPaths := WindowsNodeToolchainPaths{NodeExecutable: nodePath, NPMCLIScript: npmPath}
	if err = verifyWindowsNodeToolchainStaging(stageWorkspace, projectPaths, changed); err == nil {
		t.Fatal("toolchain files from the project tree were accepted as trusted staging")
	}
	if err = os.Chmod(paths.NodeExecutable, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(paths.NodeExecutable, []byte("tampered staged node"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = verifyWindowsNodeToolchainStaging(stageWorkspace, paths, changed); err == nil {
		t.Fatal("modified staged toolchain bytes were accepted")
	}
	if _, err = StageWindowsNodeToolchain(stageWorkspace, nodePath, npmPath, changed); err == nil {
		t.Fatal("existing toolchain staging directory was overwritten")
	}
}

func TestCloseReadOnlyLeasesRetriesOnlyOutstandingHandles(t *testing.T) {
	closed := &testReadOnlyLease{}
	flaky := &testReadOnlyLease{failFirst: true}
	leases := []io.Closer{closed, flaky}
	if err := closeReadOnlyLeases(leases); err == nil {
		t.Fatal("first failed handle close was not reported")
	}
	if leases[0] != nil || leases[1] == nil || closed.calls != 1 || flaky.calls != 1 {
		t.Fatalf("lease state after partial close: leases=%v successfulCalls=%d flakyCalls=%d", leases, closed.calls, flaky.calls)
	}
	if err := closeReadOnlyLeases(leases); err != nil {
		t.Fatalf("retry outstanding handle close: %v", err)
	}
	if !allReadOnlyLeasesClosed(leases) || closed.calls != 1 || flaky.calls != 2 {
		t.Fatalf("lease state after retry: leases=%v successfulCalls=%d flakyCalls=%d", leases, closed.calls, flaky.calls)
	}
}

func TestSnapshotCloseKeepsSandboxUntilReadOnlyLeaseCleanupSucceeds(t *testing.T) {
	sandbox := &runner.AppContainerSandbox{}
	lease := &testReadOnlyLease{failFirst: true}
	snapshot := &WindowsNodeAppContainerSnapshot{
		sandbox:         sandbox,
		toolchainLeases: []io.Closer{lease},
	}

	if err := snapshot.Close(); err == nil {
		t.Fatal("first snapshot close did not report the outstanding lease failure")
	}
	if snapshot.sandbox != sandbox || len(snapshot.toolchainLeases) != 1 {
		t.Fatal("snapshot discarded its sandbox or failed lease before cleanup could be retried")
	}
	if err := snapshot.Close(); err != nil {
		t.Fatalf("retry snapshot cleanup: %v", err)
	}
	if snapshot.sandbox != nil || len(snapshot.toolchainLeases) != 0 || lease.calls != 2 {
		t.Fatalf("snapshot cleanup did not complete after retry: sandbox=%v leases=%v closes=%d", snapshot.sandbox, snapshot.toolchainLeases, lease.calls)
	}
}

type testReadOnlyLease struct {
	calls     int
	failFirst bool
}

func (lease *testReadOnlyLease) Close() error {
	lease.calls++
	if lease.failFirst && lease.calls == 1 {
		return errors.New("injected handle close failure")
	}
	return nil
}
