// pattern: Functional Core
package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
	"polis/internal/runner"
)

var errStdioMCPRuntimeObservationDenied = errors.New("stdio MCP runtime observation requires the latest approved imported package")

func selectStdioMCPRuntimeObservation(catalog kernel.CapabilityCatalog, serverID, packageRevisionID, qualificationID string) (kernel.MCPServerDefinition, kernel.StdioMCPPackageRevision, capabilitysource.StdioMCPBundleManifestData, error) {
	var server kernel.MCPServerDefinition
	for _, candidate := range catalog.MCPServers {
		if candidate.ID == serverID {
			server = candidate
			break
		}
	}
	if server.ID == "" || server.Transport != "stdio" || server.Endpoint != nil || server.Command == nil || server.Status != "approved" {
		return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
	}
	var selected, latest kernel.StdioMCPPackageRevision
	for _, candidate := range catalog.MCPPackages {
		if candidate.ServerID != serverID || candidate.CompanyID != server.CompanyID {
			continue
		}
		if latest.ID == "" {
			latest = candidate
		}
		if candidate.ID == packageRevisionID {
			selected = candidate
		}
	}
	if selected.ID == "" || selected.ID != latest.ID || selected.ManifestDigest == "" {
		return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
	}
	var manifest capabilitysource.StdioMCPBundleManifestData
	if json.Unmarshal(selected.Manifest, &manifest) != nil || capabilitysource.ValidateStdioMCPBundleManifest(manifest, selected.ManifestDigest) != nil {
		return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
	}
	var serverArgs []string
	if json.Unmarshal(server.Args, &serverArgs) != nil || manifest.Name != server.Name || manifest.Command != *server.Command || !reflect.DeepEqual(manifest.Args, serverArgs) {
		return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
	}
	qualificationFound := false
	for _, qualification := range catalog.Qualifications {
		if qualification.QualificationID == qualificationID && qualification.CapabilityKind == "mcp" && qualification.CapabilityID == serverID && qualification.VersionDigest == server.DescriptorDigest && qualification.Profile == "stdio_mcp@1" && qualification.Status == "metadata_verified" {
			qualificationFound = true
			break
		}
	}
	if !qualificationFound {
		return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
	}
	for _, decision := range catalog.Decisions {
		if decision.CapabilityKind == "mcp" && decision.CapabilityID == serverID && decision.VersionDigest == server.DescriptorDigest && decision.QualificationID == qualificationID {
			if decision.Decision == "approved" {
				return server, selected, manifest, nil
			}
			return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
		}
	}
	return kernel.MCPServerDefinition{}, kernel.StdioMCPPackageRevision{}, capabilitysource.StdioMCPBundleManifestData{}, errStdioMCPRuntimeObservationDenied
}

func buildStdioMCPRuntimeProcessSpec(sandboxRoot, packageRoot, systemRoot, launchID, packageRevisionID, packageManifestDigest string, manifest capabilitysource.StdioMCPBundleManifestData, files []capabilitysource.StdioMCPBundleFile) (mcpowner.ProcessSpec, error) {
	if !filepath.IsAbs(sandboxRoot) || !filepath.IsAbs(packageRoot) || !filepath.IsAbs(systemRoot) || !validMCPRuntimeLaunchID(launchID) || !core.ValidID(packageRevisionID) || capabilitysource.ValidateStdioMCPBundleManifest(manifest, packageManifestDigest) != nil || !pathWithinMCPRuntimeRoot(sandboxRoot, packageRoot) {
		return mcpowner.ProcessSpec{}, core.Malformed
	}
	if err := capabilitysource.VerifyStdioMCPBundle(manifest, packageManifestDigest, files); err != nil {
		return mcpowner.ProcessSpec{}, core.Integrity
	}
	executable := filepath.Join(packageRoot, filepath.FromSlash(manifest.Command))
	entryPoint := filepath.Join(packageRoot, filepath.FromSlash(manifest.EntryPoint))
	pinnedFiles := make([]mcpowner.PinnedFile, 0, len(files))
	for _, file := range files {
		pinnedFiles = append(pinnedFiles, mcpowner.PinnedFile{Path: filepath.Join(packageRoot, filepath.FromSlash(file.RelativePath)), SHA256: file.ContentSHA256})
	}
	spec := mcpowner.ProcessSpec{
		Launch: runner.AppContainerLaunchSpec{
			ID: launchID, WorkspaceRoot: sandboxRoot, Executable: executable,
			Argv: append([]string{executable}, manifest.Args...), WorkingDirectory: packageRoot,
			NetworkPolicy: runner.AppContainerNetworkDenyAll, Environment: runner.BuildAppContainerEnvironment(sandboxRoot, systemRoot),
		},
		PinnedPackageRoot: packageRoot, EntryPoint: entryPoint, PinnedFiles: pinnedFiles,
		SourcePackageRevisionID: packageRevisionID, SourcePackageManifestSHA256: packageManifestDigest,
		ExpectedServer: mcptransport.StdioServerIdentity{Name: manifest.ServerName, Version: manifest.ServerVersion},
	}
	var err error
	spec.CommandSHA256, err = mcpowner.ComputeCommandSHA256(spec)
	if err != nil {
		return mcpowner.ProcessSpec{}, fmt.Errorf("failed to fingerprint observed MCP command: %w", err)
	}
	spec.PackageManifestSHA256, err = mcpowner.ComputePackageManifestSHA256(packageRoot, pinnedFiles)
	if err != nil {
		return mcpowner.ProcessSpec{}, fmt.Errorf("failed to fingerprint observed MCP package files: %w", err)
	}
	return spec, nil
}

func validMCPRuntimeLaunchID(value string) bool {
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

func pathWithinMCPRuntimeRoot(root, target string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
