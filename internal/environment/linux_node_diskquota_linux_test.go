//go:build linux

// pattern: Imperative Shell
package environment

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxNodeWorkspaceFilesystemRejectsOrdinaryDirectoryOnRuntimeDevice(t *testing.T) {
	root := t.TempDir()
	paths := LinuxNodeSandboxPaths{
		RuntimeRoot: root, SystemImageRoot: filepath.Join(root, "image"), WorkspaceRoot: filepath.Join(root, "workspace"),
		NPMCacheRoot: filepath.Join(root, "npm-cache"), CgroupRoot: filepath.Join(root, "cgroup"),
		NodeExecutable: "/usr/bin/node", NPMCLIScript: "/usr/lib/node_modules/npm/bin/npm-cli.js",
	}
	for _, path := range []string{paths.SystemImageRoot, paths.WorkspaceRoot, paths.NPMCacheRoot, paths.CgroupRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := inspectLinuxNodeWorkspaceFilesystem(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateLinuxNodeWorkspaceFilesystemSnapshot(snapshot, DefaultLinuxNodeWorkspaceDiskLimitBytes); !errors.Is(err, ErrLinuxNodeWorkspaceDiskLimitUnavailable) {
		t.Fatalf("ordinary directory filesystem validation error=%v, want dedicated-volume rejection", err)
	}
}
