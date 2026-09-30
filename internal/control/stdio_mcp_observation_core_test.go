// pattern: Functional Core
package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/capabilitysource"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestSelectStdioMCPRuntimeObservationRequiresLatestApprovedPackage(t *testing.T) {
	manifest := capabilitysource.StdioMCPBundleManifestData{
		SchemaVersion: "polis-controlled-stdio-mcp@1", Name: "fixture", ServerName: "fixture-server", ServerVersion: "1.0.0",
		Command: "runtime/node.exe", EntryPoint: "server/main.mjs", Args: []string{"server/main.mjs"},
		Files: []capabilitysource.StdioMCPBundleFileManifest{{RelativePath: "runtime/node.exe", MediaType: "application/octet-stream", ByteSize: 4, ContentSHA256: repeatMCPObservationDigest('a')}, {RelativePath: "server/main.mjs", MediaType: "text/javascript", ByteSize: 5, ContentSHA256: repeatMCPObservationDigest('b')}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigestBytes := sha256.Sum256(manifestBytes)
	manifestDigest := hex.EncodeToString(manifestDigestBytes[:])
	companyID, serverID, qualID := "company-1", "mcp-1", "qual-1"
	versionDigest := repeatMCPObservationDigest('c')
	catalog := kernel.CapabilityCatalog{
		MCPServers:     []kernel.MCPServerDefinition{{CompanyID: companyID, ID: serverID, Name: "fixture", Transport: "stdio", Command: ptrMCPObservation("runtime/node.exe"), Args: []byte(`["server/main.mjs"]`), DescriptorDigest: versionDigest, Status: "approved"}},
		MCPPackages:    []kernel.StdioMCPPackageRevision{{CompanyID: companyID, ID: "package-1", ServerID: serverID, ManifestDigest: manifestDigest, Manifest: manifestBytes}},
		Qualifications: []kernel.CapabilityQualification{{CompanyID: companyID, QualificationID: qualID, CapabilityKind: "mcp", CapabilityID: serverID, VersionDigest: versionDigest, Profile: "stdio_mcp@1", Status: "metadata_verified"}},
		Decisions:      []kernel.CapabilityDecisionRecord{{CompanyID: companyID, DecisionID: "decision-1", CapabilityKind: "mcp", CapabilityID: serverID, VersionDigest: versionDigest, QualificationID: qualID, Decision: "approved"}},
	}
	selectedServer, selectedPackage, selectedManifest, err := selectStdioMCPRuntimeObservation(catalog, serverID, "package-1", qualID)
	if err != nil || selectedServer.ID != serverID || selectedPackage.ID != "package-1" || selectedManifest.Command != manifest.Command {
		t.Fatalf("current approved package selection=(%+v,%+v,%+v,%v)", selectedServer, selectedPackage, selectedManifest, err)
	}
	catalog.MCPPackages = append([]kernel.StdioMCPPackageRevision{{CompanyID: companyID, ID: "package-2", ServerID: serverID, ManifestDigest: repeatMCPObservationDigest('d')}}, catalog.MCPPackages...)
	if _, _, _, err = selectStdioMCPRuntimeObservation(catalog, serverID, "package-1", qualID); err == nil {
		t.Fatal("older package revision remained eligible after a newer revision was imported")
	}
	catalog.MCPPackages = catalog.MCPPackages[1:]
	catalog.Decisions = append([]kernel.CapabilityDecisionRecord{{CompanyID: companyID, DecisionID: "decision-2", CapabilityKind: "mcp", CapabilityID: serverID, VersionDigest: versionDigest, QualificationID: qualID, Decision: "revoked"}}, catalog.Decisions...)
	if _, _, _, err = selectStdioMCPRuntimeObservation(catalog, serverID, "package-1", qualID); err == nil {
		t.Fatal("revoked metadata approval remained eligible for runtime observation")
	}
}

func TestBuildStdioMCPRuntimeProcessSpecPinsPackageAndUsesDenyAllEnvironment(t *testing.T) {
	sandboxRoot := filepath.Join(t.TempDir(), "sandbox")
	packageRoot := filepath.Join(sandboxRoot, "mcp-observation")
	executableContent := []byte("node")
	entryPointContent := []byte("entry")
	executableHash := sha256.Sum256(executableContent)
	entryPointHash := sha256.Sum256(entryPointContent)
	manifest := capabilitysource.StdioMCPBundleManifestData{
		SchemaVersion: "polis-controlled-stdio-mcp@1", Name: "fixture", ServerName: "fixture-server", ServerVersion: "1.0.0",
		Command: "runtime/node.exe", EntryPoint: "server/main.mjs", Args: []string{"server/main.mjs", "--stdio"},
		Files: []capabilitysource.StdioMCPBundleFileManifest{
			{RelativePath: "runtime/node.exe", MediaType: "application/octet-stream", ByteSize: int64(len(executableContent)), ContentSHA256: hex.EncodeToString(executableHash[:])},
			{RelativePath: "server/main.mjs", MediaType: "text/javascript", ByteSize: int64(len(entryPointContent)), ContentSHA256: hex.EncodeToString(entryPointHash[:])},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(manifestBytes)
	manifestDigest := hex.EncodeToString(manifestHash[:])
	files := []capabilitysource.StdioMCPBundleFile{
		{RelativePath: "runtime/node.exe", MediaType: "application/octet-stream", ByteSize: int64(len(executableContent)), ContentSHA256: hex.EncodeToString(executableHash[:]), Content: executableContent},
		{RelativePath: "server/main.mjs", MediaType: "text/javascript", ByteSize: int64(len(entryPointContent)), ContentSHA256: hex.EncodeToString(entryPointHash[:]), Content: entryPointContent},
	}
	systemRoot := filepath.Join(t.TempDir(), "Windows")
	spec, err := buildStdioMCPRuntimeProcessSpec(sandboxRoot, packageRoot, systemRoot, "mcp-observe-fixture", "package-revision-1", manifestDigest, manifest, files)
	if err != nil {
		t.Fatalf("build runtime observation process spec: %v", err)
	}
	if spec.Launch.NetworkPolicy != runner.AppContainerNetworkDenyAll || spec.Launch.RegistryProxyEndpoint != "" || spec.SourcePackageRevisionID != "package-revision-1" || spec.SourcePackageManifestSHA256 != manifestDigest {
		t.Fatalf("runtime process spec lost the deny-all or package binding: %+v", spec)
	}
	if spec.Launch.Executable != filepath.Join(packageRoot, filepath.FromSlash(manifest.Command)) || spec.EntryPoint != filepath.Join(packageRoot, filepath.FromSlash(manifest.EntryPoint)) || len(spec.PinnedFiles) != 2 {
		t.Fatalf("runtime process spec did not resolve and pin the package files: %+v", spec)
	}
}

func repeatMCPObservationDigest(value byte) string {
	return strings.Repeat(string(value), 64)
}

func ptrMCPObservation(value string) *string { return &value }
