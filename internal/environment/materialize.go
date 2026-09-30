// pattern: Imperative Shell
package environment

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var ErrEnvironmentMaterialization = errors.New("verified project source could not be safely materialized")

type MaterializedProject struct {
	WorkspaceRoot string
	ProjectRoot   string
	FileCount     int
	SourceBytes   int64
}

// MaterializeNodeNPMProjectFiles copies a previously verified bounded snapshot
// into a new workspace beneath an application-managed root. It never executes
// files and refuses existing workspaces, unsafe Windows paths and stale plans.
func MaterializeNodeNPMProjectFiles(destinationRoot, workspaceID string, files []ProjectSourceFile, plan NodeNPMProjectPlan, registryHosts []string) (MaterializedProject, error) {
	if !filepath.IsAbs(destinationRoot) || !validMaterializationComponent(workspaceID) {
		return MaterializedProject{}, fmt.Errorf("%w: destination root or workspace identity is invalid", ErrEnvironmentMaterialization)
	}
	root, err := filepath.Abs(filepath.Clean(destinationRoot))
	if err != nil {
		return MaterializedProject{}, fmt.Errorf("%w: destination root could not be resolved", ErrEnvironmentMaterialization)
	}
	if err = validateMaterializationRoot(root); err != nil {
		return MaterializedProject{}, err
	}
	if plan.SourceKind != NodeSnapshotFilesSource {
		return MaterializedProject{}, fmt.Errorf("%w: only verified immutable file snapshots can be materialized", ErrEnvironmentMaterialization)
	}
	files = cloneProjectSourceFiles(files)
	current, err := InspectNodeNPMProjectFilesForProfile(files, registryHosts, plan.ProfileID)
	if err != nil || !sameProjectMetadata(current, plan) {
		return MaterializedProject{}, fmt.Errorf("%w: source snapshot does not match its approved plan", ErrEnvironmentMaterialization)
	}
	for _, file := range files {
		if !safeProfileMaterializationPath(file.RelativePath, plan.ProfileID) {
			return MaterializedProject{}, fmt.Errorf("%w: source contains a path forbidden by the selected profile", ErrEnvironmentMaterialization)
		}
	}
	workspaceRoot := filepath.Join(root, workspaceID)
	if relative, relErr := filepath.Rel(root, workspaceRoot); relErr != nil || relative != workspaceID {
		return MaterializedProject{}, fmt.Errorf("%w: workspace escaped its managed root", ErrEnvironmentMaterialization)
	}
	if err = os.Mkdir(workspaceRoot, 0700); err != nil {
		return MaterializedProject{}, fmt.Errorf("%w: workspace must not exist before materialization", ErrEnvironmentMaterialization)
	}
	cleanup := func() {
		if contained, relErr := filepath.Rel(root, workspaceRoot); relErr == nil && contained == workspaceID {
			_ = os.RemoveAll(workspaceRoot)
		}
	}
	if err = os.Chmod(workspaceRoot, 0700); err != nil {
		cleanup()
		return MaterializedProject{}, fmt.Errorf("%w: workspace permissions could not be restricted", ErrEnvironmentMaterialization)
	}
	materialized := MaterializedProject{WorkspaceRoot: workspaceRoot, FileCount: len(files)}
	for _, file := range files {
		target := filepath.Join(workspaceRoot, filepath.FromSlash(file.RelativePath))
		if relative, relErr := filepath.Rel(workspaceRoot, target); relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: source path escaped its workspace", ErrEnvironmentMaterialization)
		}
		if err = makeMaterializationParents(workspaceRoot, filepath.Dir(target)); err != nil {
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: workspace directory could not be created", ErrEnvironmentMaterialization)
		}
		created, createErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if createErr != nil {
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: workspace file could not be created exclusively", ErrEnvironmentMaterialization)
		}
		if _, writeErr := created.Write(file.Content); writeErr != nil {
			_ = created.Close()
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: workspace file write failed", ErrEnvironmentMaterialization)
		}
		if syncErr := created.Sync(); syncErr != nil {
			_ = created.Close()
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: workspace file flush failed", ErrEnvironmentMaterialization)
		}
		if closeErr := created.Close(); closeErr != nil {
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: workspace file close failed", ErrEnvironmentMaterialization)
		}
		written, verifyErr := readBoundedRegularFile(target, int64(len(file.Content)))
		expectedHash := sha256.Sum256(file.Content)
		actualHash := sha256.Sum256(written)
		if verifyErr != nil || expectedHash != actualHash {
			cleanup()
			return MaterializedProject{}, fmt.Errorf("%w: materialized file failed digest verification", ErrEnvironmentMaterialization)
		}
		materialized.SourceBytes += int64(len(file.Content))
	}
	materialized.ProjectRoot = workspaceRoot
	if plan.ProjectRoot != "." {
		materialized.ProjectRoot = filepath.Join(workspaceRoot, filepath.FromSlash(plan.ProjectRoot))
	}
	materializedPlan, err := InspectNodeNPMProjectForProfile(materialized.ProjectRoot, registryHosts, plan.ProfileID)
	if err != nil || materializedPlan.PackageJSONSHA256 != plan.PackageJSONSHA256 || materializedPlan.LockfileSHA256 != plan.LockfileSHA256 || materializedPlan.PackageName != plan.PackageName {
		cleanup()
		return MaterializedProject{}, fmt.Errorf("%w: materialized package metadata failed hash revalidation", ErrEnvironmentMaterialization)
	}
	return materialized, nil
}

func safeProfileMaterializationPath(value, profileID string) bool {
	if profileID == WindowsNodeNPMProfile {
		return safeWindowsMaterializationPath(value)
	}
	if profileID != LinuxNodeNPMProfile || value == "" || len(value) > 1024 || !utf8.ValidString(value) || strings.ContainsAny(value, `\:`) || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
		for _, character := range component {
			if character < 32 || character == 127 {
				return false
			}
		}
	}
	return true
}

func cloneProjectSourceFiles(files []ProjectSourceFile) []ProjectSourceFile {
	cloned := make([]ProjectSourceFile, len(files))
	for index, file := range files {
		cloned[index] = ProjectSourceFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: append([]byte(nil), file.Content...)}
	}
	return cloned
}

func makeMaterializationParents(root, targetDirectory string) error {
	relative, err := filepath.Rel(root, targetDirectory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrEnvironmentMaterialization
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0700); mkdirErr != nil {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrEnvironmentMaterialization
		}
		if chmodErr := os.Chmod(current, 0700); chmodErr != nil {
			return chmodErr
		}
	}
	return nil
}

func validateMaterializationRoot(root string) error {
	for current := root; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: destination root path must contain only existing non-link directories", ErrEnvironmentMaterialization)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return nil
}

func safeWindowsMaterializationPath(value string) bool {
	if value == "" || len(value) > 1024 || !utf8.ValidString(value) || strings.ContainsAny(value, `\:`) || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if !validMaterializationComponent(component) {
			return false
		}
	}
	return true
}

func validMaterializationComponent(component string) bool {
	if component == "" || component == "." || component == ".." || component != strings.TrimRight(component, " .") || strings.ContainsAny(component, `<>:"/\|?*`) {
		return false
	}
	utf16Units := 0
	for _, character := range component {
		if character < 32 || character == 127 {
			return false
		}
		if character > 0xffff {
			utf16Units += 2
		} else {
			utf16Units++
		}
	}
	if utf16Units == 0 || utf16Units > 255 {
		return false
	}
	base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return false
	}
	return true
}
