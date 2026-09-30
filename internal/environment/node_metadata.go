// pattern: Functional Core
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
)

const (
	WindowsNodeNPMProfile   = "windows-node-npm@1"
	LinuxNodeNPMProfile     = "linux-node-npm@1"
	NodeSnapshotFilesSource = "snapshot_files"
	maxNodeManifestBytes    = 1 << 20
	maxNodeLockfileBytes    = 8 << 20
	maxProjectSourceFiles   = 250
	maxProjectSourceBytes   = 7 << 20
)

var ErrEnvironmentPlan = errors.New("project does not meet the supported Node/npm preparation policy")

type NodeNPMProjectPlan struct {
	ProfileID         string   `json:"profileId"`
	SourceKind        string   `json:"sourceKind"`
	ProjectRoot       string   `json:"projectRoot"`
	PackageName       string   `json:"packageName"`
	PackageJSONSHA256 string   `json:"packageJsonSha256"`
	LockfileSHA256    string   `json:"lockfileSha256"`
	LockfileVersion   int      `json:"lockfileVersion"`
	RegistryHosts     []string `json:"registryHosts"`
	InstallPolicy     string   `json:"installPolicy"`
}

type ProjectSourceFile struct {
	RelativePath string
	MediaType    string
	Content      []byte
}

type nodePackageManifest struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	Workspaces           json.RawMessage   `json:"workspaces"`
}

type nodeLockfile struct {
	Name            string                   `json:"name"`
	Version         string                   `json:"version"`
	LockfileVersion int                      `json:"lockfileVersion"`
	Packages        map[string]lockedPackage `json:"packages"`
}

type lockedPackage struct {
	Resolved             string            `json:"resolved"`
	Integrity            string            `json:"integrity"`
	Link                 bool              `json:"link"`
	InBundle             bool              `json:"inBundle"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

func inspectNodeNPMMetadata(profileID, sourceKind, projectRoot string, packageJSON, lockfileJSON []byte, allowedRegistryHosts []string) (NodeNPMProjectPlan, error) {
	if len(packageJSON) == 0 || len(packageJSON) > maxNodeManifestBytes || len(lockfileJSON) == 0 || len(lockfileJSON) > maxNodeLockfileBytes {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: package metadata is outside its size bound", ErrEnvironmentPlan)
	}
	var manifest nodePackageManifest
	var lockfile nodeLockfile
	if json.Unmarshal(packageJSON, &manifest) != nil || json.Unmarshal(lockfileJSON, &lockfile) != nil {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: package metadata is not valid JSON", ErrEnvironmentPlan)
	}
	if strings.TrimSpace(manifest.Name) == "" || strings.TrimSpace(manifest.Version) == "" || len(manifest.Workspaces) != 0 {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: package name/version are required and workspaces are not supported by this profile", ErrEnvironmentPlan)
	}
	if lockfile.LockfileVersion != 2 && lockfile.LockfileVersion != 3 {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: only npm lockfile versions 2 and 3 are supported", ErrEnvironmentPlan)
	}
	rootLock, ok := lockfile.Packages[""]
	if !ok || (lockfile.Name != "" && lockfile.Name != manifest.Name) || (lockfile.Version != "" && lockfile.Version != manifest.Version) {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: lockfile root does not match package.json", ErrEnvironmentPlan)
	}
	if !sameDependencies(manifest.Dependencies, rootLock.Dependencies) || !sameDependencies(manifest.DevDependencies, rootLock.DevDependencies) || !sameDependencies(manifest.OptionalDependencies, rootLock.OptionalDependencies) || !sameDependencies(manifest.PeerDependencies, rootLock.PeerDependencies) {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: package.json and package-lock.json dependency declarations differ", ErrEnvironmentPlan)
	}
	registryHosts, err := normalizeRegistryHosts(allowedRegistryHosts)
	if err != nil {
		return NodeNPMProjectPlan{}, err
	}
	for packagePath, entry := range lockfile.Packages {
		if packagePath == "" {
			continue
		}
		if !safeLockfilePackagePath(packagePath) || entry.Link {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: local or linked package entries are not admitted", ErrEnvironmentPlan)
		}
		if entry.InBundle {
			continue
		}
		if !allowedRegistryTarball(entry.Resolved, registryHosts) || strings.TrimSpace(entry.Integrity) == "" {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: dependency source is not a locked HTTPS registry tarball", ErrEnvironmentPlan)
		}
	}
	packageHash := sha256.Sum256(packageJSON)
	lockHash := sha256.Sum256(lockfileJSON)
	installPolicy := ""
	switch profileID {
	case WindowsNodeNPMProfile:
		installPolicy = WindowsNodeInstallPolicy
	case LinuxNodeNPMProfile:
		installPolicy = LinuxNodeNPMInstallPolicy
	default:
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: unsupported Node/npm profile", ErrEnvironmentPlan)
	}
	return NodeNPMProjectPlan{
		ProfileID: profileID, SourceKind: sourceKind, ProjectRoot: projectRoot, PackageName: manifest.Name,
		PackageJSONSHA256: hex.EncodeToString(packageHash[:]), LockfileSHA256: hex.EncodeToString(lockHash[:]),
		LockfileVersion: lockfile.LockfileVersion, RegistryHosts: registryHosts,
		InstallPolicy: installPolicy,
	}, nil
}

func sameProjectMetadata(left, right NodeNPMProjectPlan) bool {
	return left.ProfileID == right.ProfileID && left.SourceKind == right.SourceKind && left.ProjectRoot == right.ProjectRoot && left.PackageName == right.PackageName && left.PackageJSONSHA256 == right.PackageJSONSHA256 && left.LockfileSHA256 == right.LockfileSHA256 && left.LockfileVersion == right.LockfileVersion && sameStringList(left.RegistryHosts, right.RegistryHosts)
}

// InspectNodeNPMProjectFiles validates a verified, bounded directory snapshot
// without materializing it to an arbitrary host path.
func InspectNodeNPMProjectFiles(files []ProjectSourceFile, allowedRegistryHosts []string) (NodeNPMProjectPlan, error) {
	return InspectNodeNPMProjectFilesForProfile(files, allowedRegistryHosts, WindowsNodeNPMProfile)
}

func InspectLinuxNodeNPMProjectFiles(files []ProjectSourceFile, allowedRegistryHosts []string) (NodeNPMProjectPlan, error) {
	return InspectNodeNPMProjectFilesForProfile(files, allowedRegistryHosts, LinuxNodeNPMProfile)
}

func InspectNodeNPMProjectFilesForProfile(files []ProjectSourceFile, allowedRegistryHosts []string, profileID string) (NodeNPMProjectPlan, error) {
	if profileID != WindowsNodeNPMProfile && profileID != LinuxNodeNPMProfile {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: unsupported Node/npm profile", ErrEnvironmentPlan)
	}
	if len(files) == 0 || len(files) > maxProjectSourceFiles {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: project source file count is outside its bound", ErrEnvironmentPlan)
	}
	byPath := make(map[string]ProjectSourceFile, len(files))
	totalBytes := 0
	for _, file := range files {
		clean := path.Clean(file.RelativePath)
		if file.RelativePath == "" || clean != file.RelativePath || path.IsAbs(clean) || strings.ContainsAny(clean, `\:`) || len(clean) > 1024 || len(file.Content) == 0 {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: source file path or size is invalid", ErrEnvironmentPlan)
		}
		for _, part := range strings.Split(clean, "/") {
			if part == "" || part == "." || part == ".." {
				return NodeNPMProjectPlan{}, fmt.Errorf("%w: source file path is unsafe", ErrEnvironmentPlan)
			}
		}
		key := nodeProjectPathKey(clean, profileID)
		if _, duplicate := byPath[key]; duplicate {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: source file paths collide in the selected profile", ErrEnvironmentPlan)
		}
		byPath[key] = file
		totalBytes += len(file.Content)
		if totalBytes > maxProjectSourceBytes {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: project snapshot exceeds its expanded size bound", ErrEnvironmentPlan)
		}
	}
	packageRoots := make([]string, 0, 1)
	for _, file := range byPath {
		if nodeProjectNameEqual(path.Base(file.RelativePath), "package.json", profileID) {
			root := path.Dir(file.RelativePath)
			if root == "." {
				root = ""
			}
			packageRoots = append(packageRoots, root)
		}
	}
	if len(packageRoots) != 1 {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: exactly one project package.json is required", ErrEnvironmentPlan)
	}
	projectRoot := packageRoots[0]
	if projectRoot == "" {
		projectRoot = "."
	}
	rootKey := projectRoot
	if profileID == WindowsNodeNPMProfile {
		rootKey = strings.ToLower(projectRoot)
	}
	for _, reserved := range []string{".npmrc", "npm-shrinkwrap.json"} {
		candidate := path.Join(projectRoot, reserved)
		if _, exists := byPath[nodeProjectPathKey(candidate, profileID)]; exists {
			return NodeNPMProjectPlan{}, fmt.Errorf("%w: project-local npm configuration or shrinkwrap is not admitted", ErrEnvironmentPlan)
		}
	}
	packagePath := path.Join(projectRoot, "package.json")
	lockfilePath := path.Join(projectRoot, "package-lock.json")
	packageEntry, packageFound := byPath[nodeProjectPathKey(packagePath, profileID)]
	lockfileEntry, lockfileFound := byPath[nodeProjectPathKey(lockfilePath, profileID)]
	if !packageFound || !lockfileFound || packageEntry.MediaType != "application/json" || lockfileEntry.MediaType != "application/json" {
		return NodeNPMProjectPlan{}, fmt.Errorf("%w: a single JSON package manifest and lockfile are required in the project root", ErrEnvironmentPlan)
	}
	if _, err := normalizeRegistryHosts(allowedRegistryHosts); err != nil {
		return NodeNPMProjectPlan{}, err
	}
	plan, err := inspectNodeNPMMetadata(profileID, NodeSnapshotFilesSource, rootKey, packageEntry.Content, lockfileEntry.Content, allowedRegistryHosts)
	return plan, err
}

func RevalidateNodeNPMProjectFiles(plan NodeNPMProjectPlan, files []ProjectSourceFile, allowedRegistryHosts []string) error {
	return RevalidateNodeNPMProjectFilesForProfile(plan, files, allowedRegistryHosts, WindowsNodeNPMProfile)
}

func RevalidateLinuxNodeNPMProjectFiles(plan NodeNPMProjectPlan, files []ProjectSourceFile, allowedRegistryHosts []string) error {
	return RevalidateNodeNPMProjectFilesForProfile(plan, files, allowedRegistryHosts, LinuxNodeNPMProfile)
}

func RevalidateNodeNPMProjectFilesForProfile(plan NodeNPMProjectPlan, files []ProjectSourceFile, allowedRegistryHosts []string, profileID string) error {
	if plan.SourceKind != NodeSnapshotFilesSource {
		return fmt.Errorf("%w: source plan is not a verified file snapshot", ErrEnvironmentPlan)
	}
	current, err := InspectNodeNPMProjectFilesForProfile(files, allowedRegistryHosts, profileID)
	if err != nil {
		return err
	}
	if current.ProfileID != plan.ProfileID || current.ProjectRoot != plan.ProjectRoot || current.PackageName != plan.PackageName || current.PackageJSONSHA256 != plan.PackageJSONSHA256 || current.LockfileSHA256 != plan.LockfileSHA256 || current.LockfileVersion != plan.LockfileVersion || !sameStringList(current.RegistryHosts, plan.RegistryHosts) {
		return fmt.Errorf("%w: directory snapshot metadata changed after environment planning", ErrEnvironmentPlan)
	}
	return nil
}

func nodeProjectPathKey(value, profileID string) string {
	if profileID == WindowsNodeNPMProfile {
		return strings.ToLower(value)
	}
	return value
}

func nodeProjectNameEqual(left, right, profileID string) bool {
	if profileID == WindowsNodeNPMProfile {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func InstallArgs(plan NodeNPMProjectPlan, nodeExecutable, npmCLIScript string) ([]string, error) {
	if plan.ProfileID != WindowsNodeNPMProfile || !isAbsoluteWindowsPath(nodeExecutable) || !isAbsoluteWindowsPath(npmCLIScript) || len(plan.RegistryHosts) == 0 || plan.InstallPolicy != "npm ci --ignore-scripts --no-audit --no-fund" {
		return nil, fmt.Errorf("%w: trusted Node/npm toolchain or project plan is invalid", ErrEnvironmentPlan)
	}
	registry := plan.RegistryHosts[0]
	return []string{nodeExecutable, npmCLIScript, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://" + registry + "/"}, nil
}

func (plan NodeNPMProjectPlan) InstallArgs(nodeExecutable, npmCLIScript string) ([]string, error) {
	return InstallArgs(plan, nodeExecutable, npmCLIScript)
}

func normalizeRegistryHosts(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 8 {
		return nil, fmt.Errorf("%w: one to eight explicitly allowed registries are required", ErrEnvironmentPlan)
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
		if !validRegistryHostname(host) {
			return nil, fmt.Errorf("%w: registry allowlist must contain DNS hostnames only", ErrEnvironmentPlan)
		}
		if _, duplicate := seen[host]; duplicate {
			continue
		}
		seen[host] = struct{}{}
		result = append(result, host)
	}
	sort.Strings(result)
	return result, nil
}

func validRegistryHostname(host string) bool {
	if host == "" || len(host) > 253 || strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
	}
	return true
}

func allowedRegistryTarball(value string, allowed []string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" || parsed.Path == "" || parsed.Path == "/" {
		return false
	}
	for _, host := range allowed {
		if strings.EqualFold(parsed.Hostname(), host) {
			return true
		}
	}
	return false
}

func safeLockfilePackagePath(value string) bool {
	if strings.ContainsAny(value, `\:`) || strings.HasPrefix(value, "/") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func sameDependencies(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, version := range left {
		if right[name] != version {
			return false
		}
	}
	return true
}

func sameStringList(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isAbsoluteWindowsPath(value string) bool {
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	return strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, `//`)
}
