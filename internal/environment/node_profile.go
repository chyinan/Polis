// pattern: Imperative Shell
package environment

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// InspectNodeNPMProject reads only bounded package metadata. It does not run
// project code, follow links, or modify the source tree.
func InspectNodeNPMProject(projectRoot string, allowedRegistryHosts []string) (NodeNPMProjectPlan, error) {
	return InspectNodeNPMProjectForProfile(projectRoot, allowedRegistryHosts, WindowsNodeNPMProfile)
}

func InspectLinuxNodeNPMProject(projectRoot string, allowedRegistryHosts []string) (NodeNPMProjectPlan, error) {
	return InspectNodeNPMProjectForProfile(projectRoot, allowedRegistryHosts, LinuxNodeNPMProfile)
}

func InspectNodeNPMProjectForProfile(projectRoot string, allowedRegistryHosts []string, profileID string) (NodeNPMProjectPlan, error) {
	root, err := inspectRegularDirectory(projectRoot)
	if err != nil {
		return NodeNPMProjectPlan{}, err
	}
	for _, name := range []string{".npmrc", "npm-shrinkwrap.json"} {
		if _, err := os.Lstat(filepath.Join(root, name)); err == nil {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: project-local npm configuration or shrinkwrap is not admitted", ErrEnvironmentPlan)
		} else if !errors.Is(err, os.ErrNotExist) {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: project metadata could not be inspected", ErrEnvironmentPlan)
		}
	}
	packageJSON, err := readBoundedRegularFile(filepath.Join(root, "package.json"), maxNodeManifestBytes)
	if err != nil {
		return NodeNPMProjectPlan{}, err
	}
	lockfileJSON, err := readBoundedRegularFile(filepath.Join(root, "package-lock.json"), maxNodeLockfileBytes)
	if err != nil {
		return NodeNPMProjectPlan{}, err
	}
	return inspectNodeNPMMetadata(profileID, "filesystem", root, packageJSON, lockfileJSON, allowedRegistryHosts)
}

// Revalidate rejects a filesystem plan if its package metadata changed after
// inspection. Directory snapshot plans use RevalidateNodeNPMProjectFiles.
func (p NodeNPMProjectPlan) Revalidate(allowedRegistryHosts []string) error {
	if p.SourceKind != "filesystem" {
		return fmt.Errorf("%w: source plan is not a filesystem project", ErrEnvironmentPlan)
	}
	current, err := InspectNodeNPMProjectForProfile(p.ProjectRoot, allowedRegistryHosts, p.ProfileID)
	if err != nil {
		return err
	}
	if !sameProjectMetadata(current, p) {
		return fmt.Errorf("%w: project metadata changed after environment planning", ErrEnvironmentPlan)
	}
	return nil
}

func inspectRegularDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: project root must be absolute", ErrEnvironmentPlan)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: project root must be an existing non-link directory", ErrEnvironmentPlan)
	}
	return filepath.Abs(path)
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > limit {
		return nil, fmt.Errorf("%w: required project metadata is missing, linked, or outside its size bound", ErrEnvironmentPlan)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: project metadata could not be opened", ErrEnvironmentPlan)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("%w: project metadata changed while being opened", ErrEnvironmentPlan)
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: project metadata could not be read", ErrEnvironmentPlan)
	}
	if int64(len(content)) > limit || int64(len(content)) != info.Size() {
		return nil, fmt.Errorf("%w: project metadata changed or exceeded its size bound", ErrEnvironmentPlan)
	}
	return content, nil
}
