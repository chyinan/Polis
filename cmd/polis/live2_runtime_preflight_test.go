// pattern: Imperative Shell
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

func TestServeRealAdapterRejectsLive2PurposeAndQualificationDrift(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		l2Fingerprint   string
		binarySHA256    string
		expectedVersion string
		wantError       string
	}{
		{name: "purpose must be forwarded", model: "offline-model", l2Fingerprint: provider.ProductProviderL2Fingerprint, wantError: "single-medium execution budget"},
		{name: "provider L2 fingerprint must be forwarded", model: "gpt-5.6-luna", l2Fingerprint: "stale-l2", wantError: "fingerprints are stale"},
		{name: "runtime artifact pin must be forwarded", model: "gpt-5.6-luna", l2Fingerprint: provider.ProductProviderL2Fingerprint, binarySHA256: strings.Repeat("a", 64), wantError: "provider_runtime_identity_mismatch"},
		{name: "runtime version must be forwarded", model: "gpt-5.6-luna", l2Fingerprint: provider.ProductProviderL2Fingerprint, expectedVersion: "stale-version", wantError: "provider_runtime_identity_mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(root, "codex.exe")
			authFile := filepath.Join(root, "auth.json")
			evidence := filepath.Join(root, "provider-evidence")
			if err := os.Mkdir(evidence, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(binary, []byte("offline placeholder"), 0600); err != nil {
				t.Fatal(err)
			}
			helper := filepath.Join(root, "codex-code-mode-host.exe")
			if err := os.WriteFile(helper, []byte("offline helper placeholder"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(authFile, []byte("no credentials"), 0600); err != nil {
				t.Fatal(err)
			}
			manifest, err := runner.StageWindowsRuntimeArtifact(binary, helper, filepath.Join(root, "controlled"), "codex-cli 0.154.0-alpha.6.2", "", "", "test")
			if err != nil {
				t.Fatal(err)
			}
			setLive2PreflightEnvironment(t, root, manifest.CodexBinaryStagedPath, authFile, evidence, test.model, test.l2Fingerprint)
			t.Setenv("POLIS_PROVIDER_HELPER_BINARY", manifest.CodeModeHostStagedPath)
			t.Setenv("POLIS_PROVIDER_RUNTIME_MANIFEST", manifest.ManifestPath)
			t.Setenv("POLIS_PROVIDER_BINARY_SHA256", manifest.CodexBinarySHA256)
			t.Setenv("POLIS_PROVIDER_HELPER_SHA256", manifest.CodeModeHostSHA256)
			if test.binarySHA256 != "" {
				t.Setenv("POLIS_PROVIDER_BINARY_SHA256", test.binarySHA256)
			}
			if test.expectedVersion != "" {
				t.Setenv("POLIS_PROVIDER_EXPECTED_VERSION", test.expectedVersion)
			}

			adapter, err := buildWorkerAdapter(new(kernel.Kernel))
			if err != nil {
				if strings.Contains(test.wantError, "provider_runtime_identity_mismatch") && strings.Contains(err.Error(), test.wantError) {
					return
				}
				t.Fatal(err)
			}
			if err = adapter.Readiness(context.Background()); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("real provider adapter error=%v, want LIVE_2 gate containing %q", err, test.wantError)
			}
		})
	}
}

func setLive2PreflightEnvironment(t *testing.T, root, binary, authFile, evidence, model, l2Fingerprint string) {
	t.Helper()
	for key, value := range map[string]string{
		"POLIS_WORKER_MODE": "real", "POLIS_PROVIDER_TRANSPORT": "codex",
		"POLIS_PROVIDER_BINARY": binary, "POLIS_PROVIDER_HELPER_BINARY": filepath.Join(filepath.Dir(binary), "codex-code-mode-host.exe"),
		"POLIS_PROVIDER_BINARY_SHA256": provider.ProductProviderBinarySHA256V2,
		"POLIS_PROVIDER_HELPER_SHA256": provider.ProductProviderHelperSHA256V2,
		"POLIS_PROVIDER_AUTH_FILE":     authFile,
		"POLIS_PROVIDER_ROOT":          filepath.Join(root, "provider-root"), "POLIS_PROVIDER_EVIDENCE": evidence,
		"POLIS_PROVIDER_MODEL": model, "POLIS_PROVIDER_EFFORT": "medium",
		"POLIS_PROVIDER_EXPECTED_VERSION":                    "0.154.0-alpha.6.2",
		"POLIS_PROVIDER_PURPOSE":                             provider.Live2AuthorizationPurpose,
		"POLIS_PROVIDER_EXACT_SURFACE_EXECUTION_FINGERPRINT": provider.ProductExactSurfaceExecutionFingerprint,
		"POLIS_PROVIDER_L2_FINGERPRINT":                      l2Fingerprint,
		"POLIS_PROVIDER_EXECUTION_ENVELOPE":                  provider.ProductProviderRuntimeEnvelopeFingerprintV2,
		"POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION":          provider.ProductToolSurfaceQualification,
		"POLIS_PROVIDER_ALLOWANCE_PATH":                      filepath.Join(root, "allowance.json"),
		"POLIS_PROVIDER_TOOL_CALL_LIMIT":                     "16",
	} {
		t.Setenv(key, value)
	}
}
