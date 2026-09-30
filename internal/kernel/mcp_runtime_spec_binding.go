// pattern: Functional Core
package kernel

import (
	"path"
	"path/filepath"
	"strings"

	"polis/internal/capabilitysource"
	"polis/internal/mcpowner"
)

func stdioMCPLaunchMatchesDescriptor(hostOS string, spec mcpowner.ProcessSpec, descriptorCommand string, descriptorArgs []string) bool {
	if !filepath.IsAbs(spec.PinnedPackageRoot) || strings.TrimSpace(descriptorCommand) != descriptorCommand || descriptorCommand == "" {
		return false
	}
	executable, ok := resolveStdioMCPDescriptorCommand(spec.PinnedPackageRoot, descriptorCommand)
	if !ok || !sameStdioMCPCommandPath(hostOS, spec.Launch.Executable, executable) || len(spec.Launch.Argv) != len(descriptorArgs)+1 {
		return false
	}
	if !sameStdioMCPCommandPath(hostOS, spec.Launch.Argv[0], executable) {
		return false
	}
	for index, argument := range descriptorArgs {
		if spec.Launch.Argv[index+1] != argument {
			return false
		}
	}
	return true
}

func resolveStdioMCPDescriptorCommand(packageRoot, command string) (string, bool) {
	if filepath.IsAbs(command) {
		return filepath.Clean(command), true
	}
	if strings.ContainsAny(command, `\\:`) || path.IsAbs(command) || path.Clean(command) != command || command == "." || command == ".." || strings.HasPrefix(command, "../") {
		return "", false
	}
	resolved := filepath.Clean(filepath.Join(packageRoot, filepath.FromSlash(command)))
	relative, err := filepath.Rel(filepath.Clean(packageRoot), resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return resolved, true
}

func sameStdioMCPCommandPath(hostOS, left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	return hostOS == "windows" && strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func stdioMCPProcessSpecMatchesSourceManifest(hostOS string, spec mcpowner.ProcessSpec, manifest capabilitysource.StdioMCPBundleManifestData, manifestDigest string) bool {
	if spec.SourcePackageRevisionID == "" || spec.SourcePackageManifestSHA256 == "" || spec.SourcePackageManifestSHA256 != manifestDigest ||
		manifest.ServerName != spec.ExpectedServer.Name || manifest.ServerVersion != spec.ExpectedServer.Version ||
		!stdioMCPLaunchMatchesDescriptor(hostOS, spec, manifest.Command, manifest.Args) {
		return false
	}
	entryPoint, ok := resolveStdioMCPDescriptorCommand(spec.PinnedPackageRoot, manifest.EntryPoint)
	if !ok || !sameStdioMCPCommandPath(hostOS, spec.EntryPoint, entryPoint) || len(spec.PinnedFiles) != len(manifest.Files) {
		return false
	}
	pinned := make(map[string]string, len(spec.PinnedFiles))
	for _, file := range spec.PinnedFiles {
		relative, err := filepath.Rel(filepath.Clean(spec.PinnedPackageRoot), filepath.Clean(file.Path))
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return false
		}
		key := filepath.ToSlash(relative)
		if _, exists := pinned[key]; exists {
			return false
		}
		pinned[key] = file.SHA256
	}
	for _, entry := range manifest.Files {
		if pinned[entry.RelativePath] != entry.ContentSHA256 {
			return false
		}
		delete(pinned, entry.RelativePath)
	}
	return len(pinned) == 0
}
