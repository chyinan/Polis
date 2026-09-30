// pattern: Functional Core
package kernel

import (
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/capabilitysource"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
	"polis/internal/runner"
)

func TestStdioMCPLaunchMatchesPortableDescriptorCommand(t *testing.T) {
	packageRoot := filepath.Join(t.TempDir(), "package")
	executable := filepath.Join(packageRoot, "runtime", "node.exe")
	entryPoint := "server/main.mjs"
	spec := mcpowner.ProcessSpec{
		Launch: runner.AppContainerLaunchSpec{
			WorkspaceRoot: packageRoot, Executable: executable,
			Argv: []string{executable, entryPoint, "--stdio"}, WorkingDirectory: packageRoot,
		},
		PinnedPackageRoot: packageRoot,
	}
	if !stdioMCPLaunchMatchesDescriptor("windows", spec, "runtime/node.exe", []string{entryPoint, "--stdio"}) {
		t.Fatal("materialized absolute executable did not match the portable package command")
	}
	if !stdioMCPLaunchMatchesDescriptor("windows", spec, executable, []string{entryPoint, "--stdio"}) {
		t.Fatal("absolute legacy descriptor command stopped matching")
	}
	for _, invalid := range []string{"../node.exe", "runtime/../node.exe", "runtime\\node.exe", "C:node.exe", "/node.exe"} {
		if stdioMCPLaunchMatchesDescriptor("windows", spec, invalid, []string{entryPoint, "--stdio"}) {
			t.Errorf("unsafe portable descriptor command %q matched", invalid)
		}
	}
	if stdioMCPLaunchMatchesDescriptor("windows", spec, "runtime/node.exe", []string{"server/other.mjs", "--stdio"}) {
		t.Fatal("launch argv differing from the approved descriptor matched")
	}
	caseVariant := spec
	caseVariant.Launch.Executable = strings.ToUpper(executable)
	caseVariant.Launch.Argv = []string{caseVariant.Launch.Executable, entryPoint, "--stdio"}
	if !stdioMCPLaunchMatchesDescriptor("windows", caseVariant, "runtime/node.exe", []string{entryPoint, "--stdio"}) {
		t.Fatal("Windows path comparison rejected an equivalent case-insensitive executable path")
	}
	if stdioMCPLaunchMatchesDescriptor("linux", caseVariant, "runtime/node.exe", []string{entryPoint, "--stdio"}) {
		t.Fatal("case-insensitive path comparison leaked into the Linux profile")
	}
}

func TestStdioMCPRuntimeSpecBindsImportedPackageRevisionAndFiles(t *testing.T) {
	packageRoot := filepath.Join(t.TempDir(), "package")
	executable := filepath.Join(packageRoot, "runtime", "node.exe")
	entryPoint := filepath.Join(packageRoot, "server", "main.mjs")
	manifest := capabilitysource.StdioMCPBundleManifestData{
		SchemaVersion: "polis-controlled-stdio-mcp@1", Name: "fixture", ServerName: "fixture-server", ServerVersion: "1.0.0",
		Command: "runtime/node.exe", EntryPoint: "server/main.mjs", Args: []string{"server/main.mjs", "--stdio"},
		Files: []capabilitysource.StdioMCPBundleFileManifest{
			{RelativePath: "runtime/node.exe", ContentSHA256: strings.Repeat("a", 64)},
			{RelativePath: "server/main.mjs", ContentSHA256: strings.Repeat("b", 64)},
		},
	}
	manifestDigest := strings.Repeat("c", 64)
	spec := mcpowner.ProcessSpec{
		Launch: runner.AppContainerLaunchSpec{
			WorkspaceRoot: packageRoot, Executable: executable,
			Argv: []string{executable, "server/main.mjs", "--stdio"}, WorkingDirectory: packageRoot,
		},
		PinnedPackageRoot: packageRoot, EntryPoint: entryPoint,
		PinnedFiles: []mcpowner.PinnedFile{
			{Path: executable, SHA256: strings.Repeat("a", 64)},
			{Path: entryPoint, SHA256: strings.Repeat("b", 64)},
		},
		SourcePackageRevisionID: "mcp-package-revision-1", SourcePackageManifestSHA256: manifestDigest,
		ExpectedServer: mcptransport.StdioServerIdentity{Name: manifest.ServerName, Version: manifest.ServerVersion},
	}
	if !stdioMCPProcessSpecMatchesSourceManifest("windows", spec, manifest, manifestDigest) {
		t.Fatal("runtime process spec did not bind to its imported package revision")
	}
	wrongDigest := manifestDigest + "a"
	if stdioMCPProcessSpecMatchesSourceManifest("windows", spec, manifest, wrongDigest) {
		t.Fatal("runtime process spec accepted a different package revision manifest digest")
	}
	wrongServer := manifest
	wrongServer.ServerVersion = "2.0.0"
	if stdioMCPProcessSpecMatchesSourceManifest("windows", spec, wrongServer, manifestDigest) {
		t.Fatal("runtime process spec accepted a package with a different server identity")
	}
	wrongFiles := manifest
	wrongFiles.Files = append(append([]capabilitysource.StdioMCPBundleFileManifest(nil), manifest.Files...), capabilitysource.StdioMCPBundleFileManifest{RelativePath: "extra.txt", ContentSHA256: strings.Repeat("d", 64)})
	if stdioMCPProcessSpecMatchesSourceManifest("windows", spec, wrongFiles, manifestDigest) {
		t.Fatal("runtime process spec accepted a package file outside its pinned inventory")
	}
}
