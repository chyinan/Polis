// pattern: Imperative Shell
package environment

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LinuxNodeSandboxPaths struct {
	BubblewrapPath          string
	RuntimeRoot             string
	InstanceIdentity        string
	SystemImageRoot         string
	WorkspaceRoot           string
	WorkspaceDiskLimitBytes uint64
	NPMCacheRoot            string
	CgroupRoot              string
	NodeExecutable          string
	NPMCLIScript            string
}

func BuildLinuxNodeNPMInstallArgv(paths LinuxNodeSandboxPaths, plan NodeNPMProjectPlan) ([]string, error) {
	if plan.ProfileID != LinuxNodeNPMProfile || plan.SourceKind != NodeSnapshotFilesSource || plan.InstallPolicy != LinuxNodeNPMInstallPolicy {
		return nil, fmt.Errorf("%w: unsupported Linux Node/npm install plan", ErrEnvironmentPlan)
	}
	base, _, err := buildLinuxNodeBwrapBase(paths, plan.ProjectRoot)
	if err != nil {
		return nil, err
	}
	argv := append(base, "--", paths.NodeExecutable, paths.NPMCLIScript,
		"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--offline", "--cache=/workspace/.npm-cache")
	return argv, nil
}

func BuildLinuxNodeProjectScriptArgv(paths LinuxNodeSandboxPaths, plan NodeNPMProjectPlan, scriptPath string, args []string) ([]string, error) {
	if plan.ProfileID != LinuxNodeNPMProfile || plan.SourceKind != NodeSnapshotFilesSource || plan.InstallPolicy != LinuxNodeNPMInstallPolicy {
		return nil, fmt.Errorf("%w: unsupported Linux Node/npm project plan", ErrEnvironmentPlan)
	}
	relativeScript, err := NormalizeNodeProjectScriptPath(scriptPath)
	if err != nil || ValidateNodeProjectScriptArgs(args) != nil {
		return nil, fmt.Errorf("%w: script path or arguments are invalid", ErrEnvironmentPlan)
	}
	base, projectRoot, err := buildLinuxNodeBwrapBase(paths, plan.ProjectRoot)
	if err != nil {
		return nil, err
	}
	hostScript := filepath.Join(projectRoot, filepath.FromSlash(relativeScript))
	if !linuxRegularContainedPath(paths.WorkspaceRoot, hostScript) {
		return nil, fmt.Errorf("%w: project script is missing or linked", ErrEnvironmentPlan)
	}
	guestScript := "/workspace/" + relativeScript
	if plan.ProjectRoot != "." {
		guestScript = path.Join("/workspace", plan.ProjectRoot, relativeScript)
	}
	argv := append(base, "--", paths.NodeExecutable, guestScript)
	argv = append(argv, args...)
	return argv, nil
}

func buildLinuxNodeBwrapBase(paths LinuxNodeSandboxPaths, projectRootRelative string) ([]string, string, error) {
	if err := ValidateLinuxNodeSandboxPaths(paths); err != nil {
		return nil, "", err
	}
	if projectRootRelative == "" || (projectRootRelative != "." && (path.IsAbs(projectRootRelative) || path.Clean(projectRootRelative) != projectRootRelative || strings.HasPrefix(projectRootRelative, "../") || projectRootRelative == ".." || !safeProfileMaterializationPath(projectRootRelative, LinuxNodeNPMProfile))) {
		return nil, "", fmt.Errorf("%w: Linux project root is unsafe", ErrEnvironmentPlan)
	}
	projectRoot := paths.WorkspaceRoot
	guestProjectRoot := "/workspace"
	if projectRootRelative != "." {
		projectRoot = filepath.Join(paths.WorkspaceRoot, filepath.FromSlash(projectRootRelative))
		guestProjectRoot = path.Join("/workspace", projectRootRelative)
	}
	if !linuxDirectoryContainedPath(paths.WorkspaceRoot, projectRoot) || !linuxDirectoryContainedPath(paths.WorkspaceRoot, filepath.Join(paths.WorkspaceRoot, ".npm-cache")) {
		return nil, "", fmt.Errorf("%w: project root or offline npm cache is unavailable", ErrEnvironmentPlan)
	}
	argv := []string{
		paths.BubblewrapPath,
		"--unshare-all", "--die-with-parent", "--new-session",
		"--ro-bind", paths.SystemImageRoot, "/",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
		"--bind", paths.WorkspaceRoot, "/workspace",
		"--dir", "/tmp/home", "--chdir", guestProjectRoot,
		"--clearenv", "--setenv", "HOME", "/tmp/home", "--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "LANG", "C.UTF-8", "--setenv", "NPM_CONFIG_CACHE", "/workspace/.npm-cache",
	}
	return argv, projectRoot, nil
}

func ValidateLinuxNodeSandboxPaths(paths LinuxNodeSandboxPaths) error {
	if !linuxRegularExecutable(paths.BubblewrapPath) || !linuxSandboxGuestPath(paths.NodeExecutable) || !linuxSandboxGuestPath(paths.NPMCLIScript) || !linuxSandboxRoot(paths.RuntimeRoot) || !linuxSandboxRoot(paths.SystemImageRoot) || !linuxSandboxRoot(paths.WorkspaceRoot) || !linuxSandboxRoot(paths.NPMCacheRoot) || !linuxSandboxRoot(paths.CgroupRoot) {
		return fmt.Errorf("%w: Linux sandbox paths are not canonical", ErrEnvironmentPlan)
	}
	if !linuxContainedDirectory(paths.RuntimeRoot, paths.SystemImageRoot) || !linuxContainedDirectory(paths.RuntimeRoot, paths.WorkspaceRoot) || !linuxContainedDirectory(paths.RuntimeRoot, paths.NPMCacheRoot) || !linuxContainedDirectory(paths.RuntimeRoot, paths.CgroupRoot) || linuxDirectoriesOverlap(paths.SystemImageRoot, paths.WorkspaceRoot) || linuxDirectoriesOverlap(paths.SystemImageRoot, paths.NPMCacheRoot) || linuxDirectoriesOverlap(paths.SystemImageRoot, paths.CgroupRoot) || linuxDirectoriesOverlap(paths.WorkspaceRoot, paths.NPMCacheRoot) || linuxDirectoriesOverlap(paths.WorkspaceRoot, paths.CgroupRoot) || linuxDirectoriesOverlap(paths.NPMCacheRoot, paths.CgroupRoot) {
		return fmt.Errorf("%w: Linux system image, workspace, npm cache and delegated cgroup root must be separate descendants of the Polis runtime root", ErrEnvironmentPlan)
	}
	nodePath := filepath.Join(paths.SystemImageRoot, filepath.FromSlash(strings.TrimPrefix(paths.NodeExecutable, "/")))
	npmPath := filepath.Join(paths.SystemImageRoot, filepath.FromSlash(strings.TrimPrefix(paths.NPMCLIScript, "/")))
	if !linuxRegularExecutable(nodePath) || !linuxRegularContainedPath(paths.SystemImageRoot, npmPath) {
		return fmt.Errorf("%w: pinned Node/npm files are unavailable", ErrEnvironmentPlan)
	}
	for _, mountpoint := range []string{"proc", "dev", "tmp", "workspace"} {
		if !linuxDirectoryContainedPath(paths.SystemImageRoot, filepath.Join(paths.SystemImageRoot, mountpoint)) {
			return fmt.Errorf("%w: curated Linux system image is missing a required mountpoint", ErrEnvironmentPlan)
		}
	}
	return nil
}

func linuxContainedDirectory(root, candidate string) bool {
	if !linuxSandboxRoot(root) || !linuxSandboxRoot(candidate) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func linuxDirectoriesOverlap(left, right string) bool {
	leftRelative, leftErr := filepath.Rel(filepath.Clean(left), filepath.Clean(right))
	rightRelative, rightErr := filepath.Rel(filepath.Clean(right), filepath.Clean(left))
	return leftErr == nil && (leftRelative == "." || (leftRelative != ".." && !strings.HasPrefix(leftRelative, ".."+string(filepath.Separator)))) || rightErr == nil && (rightRelative == "." || (rightRelative != ".." && !strings.HasPrefix(rightRelative, ".."+string(filepath.Separator))))
}

func linuxSandboxGuestPath(value string) bool {
	if !strings.HasPrefix(value, "/") || value == "/" || len(value) > 4096 || strings.ContainsRune(value, '\x00') || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(strings.TrimPrefix(value, "/"), "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

func linuxSandboxRoot(value string) bool {
	if !filepath.IsAbs(value) || filepath.Clean(value) == string(filepath.Separator) {
		return false
	}
	for current := filepath.Clean(value); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return true
}

func linuxRegularExecutable(value string) bool {
	if !filepath.IsAbs(value) || !linuxNoLinkAncestors(value) {
		return false
	}
	info, err := os.Lstat(value)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0111 != 0
}

func linuxNoLinkAncestors(value string) bool {
	if !filepath.IsAbs(value) {
		return false
	}
	for current := filepath.Clean(value); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return true
}

func linuxRegularContainedPath(root, value string) bool {
	if !linuxSandboxRoot(root) || !filepath.IsAbs(value) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(value))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	current := filepath.Clean(root)
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	info, err := os.Lstat(current)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

func linuxDirectoryContainedPath(root, value string) bool {
	if !linuxSandboxRoot(root) || !filepath.IsAbs(value) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(value))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	current := filepath.Clean(root)
	if relative == "." {
		return true
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}
