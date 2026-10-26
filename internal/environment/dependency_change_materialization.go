// pattern: Functional Core
package environment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"polis/internal/mcptransport"
)

var errDependencyMaterialization = errors.New("invalid dependency change materialization")

// BuildWindowsNodeNPMDependencyResolutionArgs uses npm only to produce a
// lockfile. Exactly one approved registry is supported; arbitrary scripts,
// save operations, workspace expansion, shell commands, and host paths are not
// accepted as inputs.
func BuildWindowsNodeNPMDependencyResolutionArgs(plan NodeNPMProjectPlan, nodeExecutable, npmCLIScript, registryHost string) ([]string, error) {
	if plan.ProfileID != WindowsNodeNPMProfile || plan.SourceKind != NodeSnapshotFilesSource || plan.InstallPolicy != WindowsNodeInstallPolicy ||
		!isAbsoluteWindowsPath(nodeExecutable) || !isAbsoluteWindowsPath(npmCLIScript) {
		return nil, errDependencyMaterialization
	}
	canonicalHosts, err := normalizeRegistryHosts([]string{registryHost})
	if err != nil || len(plan.RegistryHosts) == 0 || canonicalHosts[0] != registryHost {
		return nil, errDependencyMaterialization
	}
	allowed := false
	for _, host := range plan.RegistryHosts {
		if host == registryHost {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errDependencyMaterialization
	}
	return []string{nodeExecutable, npmCLIScript, "install", "--package-lock-only", "--ignore-scripts", "--no-audit", "--no-fund", "--no-save", "--workspaces=false", "--registry=https://" + registryHost + "/"}, nil
}

func DependencyChangeHostsWithinPolicy(policyHosts, proposalHosts []string) bool {
	canonicalPolicy, policyErr := normalizeRegistryHosts(policyHosts)
	canonicalProposal, proposalErr := normalizeRegistryHosts(proposalHosts)
	if policyErr != nil || proposalErr != nil {
		return false
	}
	return dependencyChangeHostsWithinPolicy(canonicalProposal, canonicalPolicy)
}

// ApplyDependencyChangeToPackageJSON produces the only permitted source edit:
// add or replace exact, semver-pinned entries in the root dependencies object.
// It preserves all other package.json values and rejects ambiguous JSON,
// non-string dependency entries, and declarations in another dependency class.
func ApplyDependencyChangeToPackageJSON(packageJSON []byte, proposal DependencyChangeProposal) ([]byte, error) {
	canonical, _, _, err := CanonicalizeDependencyChangeProposal(proposal)
	if err != nil {
		return nil, errDependencyMaterialization
	}
	if len(packageJSON) == 0 || len(packageJSON) > maxNodeManifestBytes {
		return nil, errDependencyMaterialization
	}
	var document map[string]json.RawMessage
	if err := mcptransport.ValidateStrictJSONStructKeys(packageJSON, &nodePackageManifest{}); err != nil ||
		mcptransport.ValidateStrictJSONStructKeys(packageJSON, &document) != nil || json.Unmarshal(packageJSON, &document) != nil || document == nil {
		return nil, errDependencyMaterialization
	}
	for _, category := range []string{"devDependencies", "optionalDependencies", "peerDependencies"} {
		declared, err := packageDependencyObject(document, category)
		if err != nil {
			return nil, errDependencyMaterialization
		}
		for name := range canonical.Dependencies {
			if _, exists := declared[name]; exists {
				return nil, fmt.Errorf("%w: %q is already declared in %s", errDependencyMaterialization, name, category)
			}
		}
	}
	dependencies, err := packageDependencyObject(document, "dependencies")
	if err != nil {
		return nil, errDependencyMaterialization
	}
	changed := false
	for name, version := range canonical.Dependencies {
		var current string
		if raw, exists := dependencies[name]; exists {
			if err := json.Unmarshal(raw, &current); err != nil {
				return nil, errDependencyMaterialization
			}
		}
		if current != version {
			changed = true
		}
		encoded, _ := json.Marshal(version)
		dependencies[name] = encoded
	}
	if !changed {
		return nil, fmt.Errorf("%w: proposal does not change any dependency", errDependencyMaterialization)
	}
	dependencyBytes, err := json.Marshal(dependencies)
	if err != nil {
		return nil, errDependencyMaterialization
	}
	document["dependencies"] = dependencyBytes
	updated, err := json.Marshal(document)
	if err != nil || len(updated) > maxNodeManifestBytes {
		return nil, errDependencyMaterialization
	}
	var metadata nodePackageManifest
	if err := json.Unmarshal(updated, &metadata); err != nil || metadata.Name == "" || metadata.Version == "" || len(metadata.Workspaces) != 0 {
		return nil, errDependencyMaterialization
	}
	return updated, nil
}

// PrepareDependencyChangeProjectFiles creates a defensive copy of the source
// tree with only the package.json overlay applied. It does not run npm, resolve
// packages, mutate a caller slice, or write to a host path.
func PrepareDependencyChangeProjectFiles(files []ProjectSourceFile, plan NodeNPMProjectPlan, proposal DependencyChangeProposal) ([]ProjectSourceFile, []byte, error) {
	if plan.ProfileID != WindowsNodeNPMProfile && plan.ProfileID != LinuxNodeNPMProfile || plan.ProjectRoot == "" || len(files) == 0 {
		return nil, nil, errDependencyMaterialization
	}
	if err := RevalidateNodeNPMProjectFilesForProfile(plan, files, plan.RegistryHosts, plan.ProfileID); err != nil {
		return nil, nil, errDependencyMaterialization
	}
	packagePath := "package.json"
	if plan.ProjectRoot != "." {
		packagePath = path.Join(plan.ProjectRoot, packagePath)
	}
	index := -1
	for candidate := range files {
		if nodeProjectNameEqual(files[candidate].RelativePath, packagePath, plan.ProfileID) {
			if index >= 0 || files[candidate].MediaType != "application/json" {
				return nil, nil, errDependencyMaterialization
			}
			index = candidate
		}
	}
	if index < 0 {
		return nil, nil, errDependencyMaterialization
	}
	updatedManifest, err := ApplyDependencyChangeToPackageJSON(files[index].Content, proposal)
	if err != nil {
		return nil, nil, err
	}
	copyFiles := make([]ProjectSourceFile, len(files))
	for fileIndex, file := range files {
		copyFiles[fileIndex] = ProjectSourceFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: bytes.Clone(file.Content)}
	}
	copyFiles[index].Content = bytes.Clone(updatedManifest)
	return copyFiles, updatedManifest, nil
}

// ValidateDependencyChangeMaterialization accepts a lockfile only when it
// forms a valid project snapshot with the exact deterministic package.json
// overlay. The environment plan validator checks pinned tarballs against the
// full base-policy allowlist; resolver egress itself is separately narrowed to
// the proposal's registry hosts.
func ValidateDependencyChangeMaterialization(baseFiles []ProjectSourceFile, plan NodeNPMProjectPlan, proposal DependencyChangeProposal, updatedPackageJSON, resolvedLockfile []byte, allowedRegistryHosts []string) ([]ProjectSourceFile, NodeNPMProjectPlan, error) {
	canonicalProposal, _, _, proposalErr := CanonicalizeDependencyChangeProposal(proposal)
	policyHosts, policyErr := normalizeRegistryHosts(allowedRegistryHosts)
	if proposalErr != nil || policyErr != nil ||
		!dependencyChangeHostsWithinPolicy(canonicalProposal.RegistryHosts, policyHosts) ||
		!dependencyChangeHostsWithinPolicy(plan.RegistryHosts, policyHosts) {
		return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
	}
	expectedFiles, expectedManifest, err := PrepareDependencyChangeProjectFiles(baseFiles, plan, proposal)
	if err != nil || !bytes.Equal(expectedManifest, updatedPackageJSON) || len(resolvedLockfile) == 0 || len(resolvedLockfile) > maxNodeLockfileBytes {
		return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
	}
	if err := validateStrictNodeLockfileJSON(resolvedLockfile); err != nil {
		return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
	}
	lockfilePath := "package-lock.json"
	if plan.ProjectRoot != "." {
		lockfilePath = path.Join(plan.ProjectRoot, lockfilePath)
	}
	lockIndex := -1
	for index := range expectedFiles {
		if nodeProjectNameEqual(expectedFiles[index].RelativePath, lockfilePath, plan.ProfileID) {
			if lockIndex >= 0 || expectedFiles[index].MediaType != "application/json" {
				return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
			}
			lockIndex = index
		}
	}
	if lockIndex < 0 {
		return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
	}
	expectedFiles[lockIndex].Content = bytes.Clone(resolvedLockfile)
	resolvedPlan, err := InspectNodeNPMProjectFilesForProfile(expectedFiles, policyHosts, plan.ProfileID)
	if err != nil {
		return nil, NodeNPMProjectPlan{}, fmt.Errorf("%w: resolved lockfile failed project validation: %v", errDependencyMaterialization, err)
	}
	if resolvedPlan.ProjectRoot != plan.ProjectRoot || resolvedPlan.PackageName != plan.PackageName || resolvedPlan.LockfileVersion != plan.LockfileVersion {
		return nil, NodeNPMProjectPlan{}, errDependencyMaterialization
	}
	return expectedFiles, resolvedPlan, nil
}

func validateStrictNodeLockfileJSON(raw []byte) error {
	var root struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := mcptransport.ValidateStrictJSONStructKeys(raw, &nodeLockfile{}); err != nil ||
		mcptransport.ValidateStrictJSONStructKeys(raw, &root) != nil || json.Unmarshal(raw, &root) != nil || root.Packages == nil {
		return errDependencyMaterialization
	}
	for _, packageJSON := range root.Packages {
		if err := mcptransport.ValidateStrictJSONStructKeys(packageJSON, &lockedPackage{}); err != nil {
			return errDependencyMaterialization
		}
	}
	return nil
}

func dependencyChangeHostsWithinPolicy(proposalHosts, policyHosts []string) bool {
	allowed := make(map[string]struct{}, len(policyHosts))
	for _, host := range policyHosts {
		allowed[host] = struct{}{}
	}
	for _, host := range proposalHosts {
		if _, exists := allowed[host]; !exists {
			return false
		}
	}
	return true
}

func packageDependencyObject(document map[string]json.RawMessage, field string) (map[string]json.RawMessage, error) {
	raw, exists := document[field]
	if !exists {
		return make(map[string]json.RawMessage), nil
	}
	var dependencies map[string]json.RawMessage
	if json.Unmarshal(raw, &dependencies) != nil || dependencies == nil {
		return nil, errDependencyMaterialization
	}
	for _, dependency := range dependencies {
		var version string
		if json.Unmarshal(dependency, &version) != nil || version == "" {
			return nil, errDependencyMaterialization
		}
	}
	return dependencies, nil
}
