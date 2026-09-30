// pattern: Functional Core
package mcpowner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"polis/internal/mcptransport"
	"polis/internal/runner"
)

const maxPinnedFiles = 64

// ValidateProcessSpec applies the same closed command, package and network
// policy used by the concrete process owner before a profile is persisted.
func ValidateProcessSpec(spec ProcessSpec) error {
	return validateProcessSpec(spec)
}

func validateProcessSpec(spec ProcessSpec) error {
	launch := spec.Launch
	if (spec.SourcePackageRevisionID == "") != (spec.SourcePackageManifestSHA256 == "") || (spec.SourcePackageRevisionID != "" && (!validSourcePackageRevisionID(spec.SourcePackageRevisionID) || !validSHA256(spec.SourcePackageManifestSHA256))) {
		return errors.New("stdio MCP source package binding is incomplete or invalid")
	}
	if launch.NetworkPolicy != runner.AppContainerNetworkDenyAll || launch.RegistryProxyEndpoint != "" {
		return errors.New("stdio MCP processes require a deny-all network profile")
	}
	if err := runner.ValidateAppContainerLaunchSpec(launch); err != nil {
		return err
	}
	if len(spec.PinnedFiles) == 0 || len(spec.PinnedFiles) > maxPinnedFiles {
		return errors.New("stdio MCP process must pin a bounded non-empty file set")
	}
	if spec.ExpectedServer.Name == "" || spec.ExpectedServer.Version == "" || len(spec.ExpectedServer.Name) > 128 || len(spec.ExpectedServer.Version) > 128 {
		return errors.New("stdio MCP expected server identity is incomplete")
	}
	if spec.ApprovedToolSchemaSHA256 != "" && !validSHA256(spec.ApprovedToolSchemaSHA256) {
		return errors.New("stdio MCP approved tool schema digest is invalid")
	}
	if !filepath.IsAbs(spec.PinnedPackageRoot) || !pathWithinOrEqual(launch.WorkspaceRoot, spec.PinnedPackageRoot) || filepath.Clean(launch.WorkingDirectory) != filepath.Clean(spec.PinnedPackageRoot) || !filepath.IsAbs(spec.EntryPoint) || !pathWithin(spec.PinnedPackageRoot, spec.EntryPoint) {
		return errors.New("stdio MCP pinned package root or entry point is outside its workspace")
	}
	if spec.GracefulStopTimeout < 0 || spec.GracefulStopTimeout > 30*time.Second {
		return errors.New("stdio MCP graceful stop timeout is outside the accepted bound")
	}
	seen := make(map[string]struct{}, len(spec.PinnedFiles))
	pinnedPaths := make(map[string]struct{}, len(spec.PinnedFiles))
	for _, file := range spec.PinnedFiles {
		if !filepath.IsAbs(file.Path) || !pathWithin(spec.PinnedPackageRoot, file.Path) || !validSHA256(file.SHA256) {
			return errors.New("stdio MCP pinned file is outside its package root or has an invalid digest")
		}
		path := filepath.Clean(file.Path)
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			return errors.New("stdio MCP pinned file list contains duplicate paths")
		}
		seen[key] = struct{}{}
		pinnedPaths[key] = struct{}{}
	}
	if _, exists := pinnedPaths[strings.ToLower(filepath.Clean(launch.Executable))]; !exists {
		return errors.New("stdio MCP executable must be included in its pinned package manifest")
	}
	if _, exists := pinnedPaths[strings.ToLower(filepath.Clean(spec.EntryPoint))]; !exists {
		return errors.New("stdio MCP command entry point must be included in its pinned package manifest")
	}
	entryPointInArgv := false
	for _, argument := range launch.Argv {
		if samePathArgument(argument, spec.EntryPoint, launch.WorkingDirectory) {
			entryPointInArgv = true
		}
		pathArgument, isPath := commandPathArgument(argument)
		if !isPath {
			continue
		}
		if hasWindowsDrivePrefix(pathArgument) && !filepath.IsAbs(pathArgument) {
			return errors.New("stdio MCP command argument uses an unsupported drive-relative path")
		}
		if !filepath.IsAbs(pathArgument) {
			pathArgument = filepath.Join(launch.WorkingDirectory, pathArgument)
		}
		pathArgument = filepath.Clean(pathArgument)
		if !pathWithin(spec.PinnedPackageRoot, pathArgument) {
			return errors.New("stdio MCP command argument references a file outside its pinned package")
		}
		if _, exists := pinnedPaths[strings.ToLower(pathArgument)]; !exists {
			return errors.New("stdio MCP command argument references an unpinned package file")
		}
	}
	if !entryPointInArgv {
		return errors.New("stdio MCP command entry point must be an explicit launch argument")
	}
	commandSHA256, err := ComputeCommandSHA256(spec)
	if err != nil || commandSHA256 != spec.CommandSHA256 {
		return errors.Join(errors.New("stdio MCP command differs from its qualification digest"), err)
	}
	manifestSHA256, err := ComputePackageManifestSHA256(spec.PinnedPackageRoot, spec.PinnedFiles)
	if err != nil || manifestSHA256 != spec.PackageManifestSHA256 {
		return errors.Join(errors.New("stdio MCP package differs from its pinned manifest digest"), err)
	}
	return nil
}

func validSourcePackageRevisionID(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

// ComputeCommandSHA256 produces the immutable command-profile digest persisted
// by qualification. The per-launch ID and approved tool schema are excluded.
func ComputeCommandSHA256(spec ProcessSpec) (string, error) {
	if !filepath.IsAbs(spec.Launch.WorkspaceRoot) || !filepath.IsAbs(spec.Launch.Executable) || !filepath.IsAbs(spec.Launch.WorkingDirectory) || !filepath.IsAbs(spec.PinnedPackageRoot) || !filepath.IsAbs(spec.EntryPoint) {
		return "", errors.New("stdio MCP command digest requires absolute path inputs")
	}
	environment := append([]string(nil), spec.Launch.Environment...)
	sort.Strings(environment)
	contract := struct {
		WorkspaceRoot         string   `json:"workspaceRoot"`
		PinnedPackageRoot     string   `json:"pinnedPackageRoot"`
		Executable            string   `json:"executable"`
		EntryPoint            string   `json:"entryPoint"`
		Argv                  []string `json:"argv"`
		WorkingDirectory      string   `json:"workingDirectory"`
		NetworkPolicy         string   `json:"networkPolicy"`
		RegistryProxyEndpoint string   `json:"registryProxyEndpoint"`
		Environment           []string `json:"environment"`
	}{
		WorkspaceRoot: filepath.Clean(spec.Launch.WorkspaceRoot), PinnedPackageRoot: filepath.Clean(spec.PinnedPackageRoot),
		Executable: filepath.Clean(spec.Launch.Executable), EntryPoint: filepath.Clean(spec.EntryPoint),
		Argv: append([]string(nil), spec.Launch.Argv...), WorkingDirectory: filepath.Clean(spec.Launch.WorkingDirectory),
		NetworkPolicy: spec.Launch.NetworkPolicy, RegistryProxyEndpoint: spec.Launch.RegistryProxyEndpoint,
		Environment: environment,
	}
	encoded, err := json.Marshal(contract)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// ComputePackageManifestSHA256 returns a deterministic digest for a sorted
// file list relative to the immutable package root.
func ComputePackageManifestSHA256(packageRoot string, files []PinnedFile) (string, error) {
	if !filepath.IsAbs(packageRoot) || len(files) == 0 || len(files) > maxPinnedFiles {
		return "", errors.New("stdio MCP package manifest is empty or outside an absolute root")
	}
	type manifestEntry struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	entries := make([]manifestEntry, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if !filepath.IsAbs(file.Path) || !pathWithin(packageRoot, file.Path) || !validSHA256(file.SHA256) {
			return "", errors.New("stdio MCP package manifest contains an invalid path or digest")
		}
		relative, err := filepath.Rel(filepath.Clean(packageRoot), filepath.Clean(file.Path))
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", errors.New("stdio MCP package manifest path escapes its root")
		}
		key := strings.ToLower(filepath.Clean(relative))
		if _, exists := seen[key]; exists {
			return "", errors.New("stdio MCP package manifest contains duplicate paths")
		}
		seen[key] = struct{}{}
		entries = append(entries, manifestEntry{Path: filepath.ToSlash(relative), SHA256: file.SHA256})
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func commandPathArgument(argument string) (string, bool) {
	value := strings.Trim(strings.TrimSpace(argument), `"'`)
	if filepath.IsAbs(value) || hasWindowsDrivePrefix(value) {
		return value, true
	}
	if strings.HasPrefix(value, "-") {
		if equals := strings.IndexByte(value, '='); equals >= 0 {
			value = strings.Trim(strings.TrimSpace(value[equals+1:]), `"'`)
		}
	}
	if value == "" {
		return "", false
	}
	if value == "." || value == ".." || hasWindowsDrivePrefix(value) || strings.Contains(value, ":") {
		return value, true
	}
	extension := strings.ToLower(filepath.Ext(value))
	switch extension {
	case ".cjs", ".dll", ".exe", ".jar", ".js", ".json", ".mjs", ".node", ".py", ".so", ".wasm", ".yaml", ".yml":
		return value, true
	}
	if filepath.IsAbs(value) || strings.ContainsAny(value, `/\\`) {
		return value, true
	}
	return "", false
}

func hasWindowsDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

func samePathArgument(argument, expectedPath, workingDirectory string) bool {
	value, isPath := commandPathArgument(argument)
	if !isPath {
		return false
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(workingDirectory, value)
	}
	return strings.EqualFold(filepath.Clean(value), filepath.Clean(expectedPath))
}

func cloneProcessSpec(spec ProcessSpec) ProcessSpec {
	spec.Launch.Argv = append([]string(nil), spec.Launch.Argv...)
	spec.Launch.Environment = append([]string(nil), spec.Launch.Environment...)
	spec.PinnedFiles = append([]PinnedFile(nil), spec.PinnedFiles...)
	return spec
}

func cloneTools(tools []mcptransport.StdioToolDefinition) []mcptransport.StdioToolDefinition {
	cloned := make([]mcptransport.StdioToolDefinition, len(tools))
	for index, tool := range tools {
		cloned[index] = tool
		cloned[index].InputSchema = append([]byte(nil), tool.InputSchema...)
	}
	return cloned
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func pathWithin(root, target string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return true
}

func pathWithinOrEqual(root, target string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return true
}
