// pattern: Imperative Shell
package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/codex"
	"polis/internal/runner"
)

func TestBindCodexRuntimeConfigUsesVerifiedManifestIdentity(t *testing.T) {
	root := t.TempDir()
	manifest := stageRuntimeBindingFixture(t, root, "codex-cli 0.154.0-alpha.6.2")
	config := CodexRuntimeConfig{
		RuntimeManifestPath: manifest.ManifestPath,
		ExpectedVersion:     "0.154.0-alpha.6.2",
		TransportPolicy:     codex.DefaultTransportPolicy(),
	}

	bound, err := BindCodexRuntimeConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if bound.Binary != manifest.CodexBinaryStagedPath || bound.HelperBinary != manifest.CodeModeHostStagedPath {
		t.Fatalf("runtime paths were not resolved from the manifest: binary=%q helper=%q", bound.Binary, bound.HelperBinary)
	}
	if bound.ExpectedVersion != "0.154.0-alpha.6.2" || bound.BinarySHA256 != manifest.CodexBinarySHA256 || bound.HelperSHA256 != manifest.CodeModeHostSHA256 {
		t.Fatalf("runtime identity was not bound to verified artifact: version=%q binary=%q helper=%q", bound.ExpectedVersion, bound.BinarySHA256, bound.HelperSHA256)
	}
}

func TestBindCodexRuntimeConfigRejectsStaleOrWrongRuntimeIdentity(t *testing.T) {
	root := t.TempDir()
	manifest := stageRuntimeBindingFixture(t, root, "codex-cli 0.154.0-alpha.6.2")

	tests := map[string]CodexRuntimeConfig{
		"B10 stale expected version": {
			RuntimeManifestPath: manifest.ManifestPath,
			ExpectedVersion:     "0.155.0-alpha.9.2",
		},
		"expected binary hash mismatch": {
			RuntimeManifestPath: manifest.ManifestPath,
			ExpectedVersion:     "0.154.0-alpha.6.2",
			BinarySHA256:        strings.Repeat("a", 64),
		},
		"expected helper hash mismatch": {
			RuntimeManifestPath: manifest.ManifestPath,
			ExpectedVersion:     "0.154.0-alpha.6.2",
			HelperSHA256:        strings.Repeat("b", 64),
		},
		"wrong helper selected": {
			RuntimeManifestPath: manifest.ManifestPath,
			ExpectedVersion:     "0.154.0-alpha.6.2",
			HelperBinary:        filepath.Join(root, "path-helper.exe"),
		},
		"wrong executable selected": {
			RuntimeManifestPath: manifest.ManifestPath,
			Binary:              filepath.Join(root, "path-codex.exe"),
			ExpectedVersion:     "0.154.0-alpha.6.2",
		},
	}
	if err := os.WriteFile(tests["wrong executable selected"].Binary, []byte("unrelated PATH executable"), 0600); err != nil {
		t.Fatal(err)
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := BindCodexRuntimeConfig(config); err == nil || !strings.Contains(err.Error(), "provider_runtime_identity_mismatch") {
				t.Fatalf("BindCodexRuntimeConfig error=%v, want provider_runtime_identity_mismatch", err)
			}
		})
	}
}

func TestBindCodexRuntimeConfigRejectsMissingExecutable(t *testing.T) {
	root := t.TempDir()
	manifest := stageRuntimeBindingFixture(t, root, "codex-cli 0.154.0-alpha.6.2")
	if err := os.Remove(manifest.CodexBinaryStagedPath); err != nil {
		t.Fatal(err)
	}
	_, err := BindCodexRuntimeConfig(CodexRuntimeConfig{RuntimeManifestPath: manifest.ManifestPath})
	if err == nil || !strings.Contains(err.Error(), "provider_runtime_unavailable") {
		t.Fatalf("BindCodexRuntimeConfig error=%v, want provider_runtime_unavailable", err)
	}
}

func TestBindCodexRuntimeConfigRejectsUnavailableRuntime(t *testing.T) {
	_, err := BindCodexRuntimeConfig(CodexRuntimeConfig{RuntimeManifestPath: filepath.Join(t.TempDir(), "runtime-manifest.json")})
	if err == nil || !strings.Contains(err.Error(), "provider_runtime_unavailable") {
		t.Fatalf("BindCodexRuntimeConfig error=%v, want provider_runtime_unavailable", err)
	}
}

func TestBuildProductSurfaceExecutionIdentityRejectsB10StaleVersion(t *testing.T) {
	root := t.TempDir()
	manifest := stageRuntimeBindingFixture(t, root, "codex-cli 0.154.0-alpha.6.2")
	config := CodexRuntimeConfig{
		RuntimeManifestPath: manifest.ManifestPath,
		Binary:              manifest.CodexBinaryStagedPath,
		HelperBinary:        manifest.CodeModeHostStagedPath,
		AuthFile:            filepath.Join(root, "auth.json"),
		Root:                filepath.Join(root, "runtime"),
		EvidenceRoot:        filepath.Join(root, "evidence"),
		Model:               "gpt-5.6-luna",
		Effort:              "medium",
		ExpectedVersion:     "0.155.0-alpha.9.2",
		TransportPolicy:     codex.DefaultTransportPolicy(),
		ToolSurface:         ProductToolSurface(),
	}

	if _, err := BuildProductSurfaceExecutionIdentity(config, "r05b11-preflight"); err == nil || !strings.Contains(err.Error(), "provider_runtime_identity_mismatch") {
		t.Fatalf("BuildProductSurfaceExecutionIdentity error=%v, want B10 stale runtime mismatch", err)
	}
}

func TestBuildProductSurfaceExecutionIdentityBindsRuntimeImplementationAndProtocol(t *testing.T) {
	root := t.TempDir()
	manifest := stageRuntimeBindingFixture(t, root, "codex-cli 0.154.0-alpha.6.2")
	config := CodexRuntimeConfig{
		RuntimeManifestPath: manifest.ManifestPath,
		AuthFile:            filepath.Join(root, "auth.json"),
		Root:                filepath.Join(root, "runtime"),
		EvidenceRoot:        filepath.Join(root, "evidence"),
		Model:               "gpt-5.6-luna",
		Effort:              "medium",
		TransportPolicy:     codex.DefaultTransportPolicy(),
		ToolSurface:         ProductToolSurface(),
	}

	identity, err := BuildProductSurfaceExecutionIdentity(config, "r05b11-preflight")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ProviderRuntime.RuntimeImplementation != ProductProviderRuntimeImplementation || identity.ProviderRuntime.ProtocolCompatibility != ProductProviderProtocolCompatibility {
		t.Fatalf("runtime compatibility identity missing: %+v", identity.ProviderRuntime)
	}
}

func TestCurrentControlledProviderRuntimeIdentity(t *testing.T) {
	manifestPath := os.Getenv("POLIS_R05B11_RUNTIME_MANIFEST")
	if manifestPath == "" {
		t.Skip("set POLIS_R05B11_RUNTIME_MANIFEST for the local controlled-runtime identity probe")
	}
	config := CodexRuntimeConfig{
		RuntimeManifestPath: manifestPath,
		AuthFile:            "local-preflight-auth.json",
		Root:                ".runtime/windows/r0.5b11-provider-runtime-version-binding-hardening",
		EvidenceRoot:        "evidence/development/r0.5b11-provider-runtime-version-binding-hardening/provider",
		Model:               "gpt-5.6-luna",
		Effort:              "medium",
		TransportPolicy:     codex.DefaultTransportPolicy(),
		ToolSurface:         ProductToolSurface(),
	}
	identity, err := BuildProductSurfaceExecutionIdentity(config, "r05b11-provider-preflight")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("current_provider_runtime=%+v", identity.ProviderRuntime)
	t.Logf("current_launch_envelope=%+v", identity.LaunchEnvelope)
	t.Logf("product_v4_exact_surface_execution_fingerprint=%s", fingerprint)
}

func stageRuntimeBindingFixture(t *testing.T, root, version string) runner.WindowsRuntimeArtifactManifest {
	t.Helper()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(source, "codex.exe")
	helper := filepath.Join(source, "codex-code-mode-host.exe")
	if err := os.WriteFile(binary, []byte("qualified codex binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("qualified code mode host"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := runner.StageWindowsRuntimeArtifact(binary, helper, filepath.Join(root, "controlled"), version, "", "", "windows_native_controlled_staged")
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
