// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"strings"
	"testing"
	"time"
)

func TestRequireBusinessQualificationBlocksT6WithoutCreatingAllowance(t *testing.T) {
	dir := t.TempDir()
	c := codex.ExecutionCombination{CodexVersion: "0.151.0", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "auth-file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	record := codex.QualificationRecord{Layer: codex.QualificationL1BaseTransport, Status: codex.QualificationUnqualified, EvidenceResult: codex.EvidenceInconclusive, ExecutionKey: c.Fingerprint(), Combination: c, CreatedAt: time.Now(), Reason: "no_first_valid_output_after_structured_reconnects"}
	writeJSONFile := func(path string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	recordPath := filepath.Join(dir, "record.json")
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSONFile(recordPath, record)
	writeJSONFile(manifestPath, struct {
		Combination codex.ExecutionCombination `json:"combination"`
	}{c})
	if err := RequireBusinessQualification(recordPath, manifestPath); err == nil {
		t.Fatal("T6 unqualified transport record allowed business execution")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("qualification gate created unexpected side effects: %d files", len(entries))
	}
}

func TestRequireR03AT2QualificationRevalidatesV7SurfaceAndInputs(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "codex.exe")
	helper := filepath.Join(dir, "codex-code-mode-host.exe")
	auth := filepath.Join(dir, "auth.json")
	config := filepath.Join(dir, "config.toml")
	for path, content := range map[string]string{binary: "binary", helper: "helper", auth: `{"tokens":{}}`, config: "model_reasoning_effort = \"medium\"\n"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	authMaterial, err := codex.ParseAuthMaterial([]byte(`{"tokens":{}}`), "controlled_diagnostic_auth_material")
	if err != nil {
		t.Fatal(err)
	}
	combination := codex.ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: digest([]byte("binary")), Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "windows-native", SandboxClass: "read-only", ProxyConfigDigest: strings.Repeat("a", 64), AuthSourceClass: "controlled_diagnostic_auth_material", CodeModeHostSHA256: digest([]byte("helper")), CapabilityDigest: strings.Repeat("b", 64), NativeProtocolDigest: strings.Repeat("c", 64)}
	base := codex.CanonicalManifestV6{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV6, Combination: combination, Auth: authMaterial.Manifest(), Transport: codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyNativeDefault, WebSocketPolicy: codex.ProviderTransportPolicyNativeDefault}, Runtime: codex.RuntimeExecutionEnvelope{Platform: "windows", Architecture: "amd64", RuntimeProfile: "windows-native", FilesystemIsolation: "diagnostic_home_only", ProcessIsolation: "windows_process_handle_no_job_object", NetworkPolicy: codex.RuntimeNetworkPolicyNativeOSNetwork, CWDRole: "diagnostic_workspace", LaunchMechanism: "CreateProcess", StdioMode: "redirected_standard_pipes"}, CodexHomeProfile: codex.CodexHomeProfileWindowsDiagnostic, EffectiveConfigDigest: strings.Repeat("d", 64), EffectiveTransportConfigDigest: strings.Repeat("e", 64)}
	surface, _, _, err := buildT21ToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	surface.ThreadStartPayloadDigest = strings.Repeat("f", 64)
	surface.ThreadStartPayloadBytes = 1
	manifest := codex.CanonicalManifestV7{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV7, Base: base, ToolSurface: surface}
	fingerprint := base.Combination.CurrentFingerprintV7(manifest)
	manifestPath := filepath.Join(dir, "execution-manifest.json")
	executionPath := filepath.Join(dir, "execution-config.json")
	qualificationPath := filepath.Join(dir, "qualification.json")
	writeTestJSON(t, manifestPath, map[string]any{"qualification": "R0.3A-T21B", "fingerprint_schema_version": manifest.FingerprintSchemaVersion, "canonical_manifest": manifest, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "qualification_fingerprint": fingerprint})
	writeTestJSON(t, executionPath, map[string]any{"selected_config_raw_sha256": digest([]byte("model_reasoning_effort = \"medium\"\n")), "qualification_fingerprint": fingerprint, "tool_manifest_digest": surface.AggregateManifestDigest, "tool_surface": surface})
	writeTestJSON(t, qualificationPath, map[string]any{"status": "PASSED", "eligible_for_real_backend": true, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest})
	cfg := R03AT2Config{Config: Config{Binary: binary, CodeModeHost: helper, AuthFile: auth, SelectedConfigPath: config, ExecutionConfigPath: executionPath, QualificationPath: qualificationPath, ExecutionManifestPath: manifestPath, Model: "gpt-5.6-luna", MediumLimit: 1, HighLimit: 0}, ExecutionFingerprint: fingerprint.CanonicalManifestDigest}
	if _, err := RequireR03AT2Qualification(cfg); err != nil {
		t.Fatal(err)
	}
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestContinuation5FrozenSurfaceDriftStopsBeforeRuntime(t *testing.T) {
	base := filepath.Join("..", "..", "evidence", "development", "r0.3a-t21b")
	cfg := R03AT2Config{Config: Config{
		ExecutionManifestPath: filepath.Join(base, "execution-manifest.json"),
		ExecutionConfigPath:   filepath.Join(base, "execution-config.json"),
		QualificationPath:     filepath.Join(base, "qualification.json"),
		Binary:                "not-read-before-surface-gate", CodeModeHost: "not-read-before-surface-gate",
		AuthFile: "not-read-before-surface-gate", SelectedConfigPath: "not-read-before-surface-gate",
		ToolCallLimit: 48,
	}, ExecutionFingerprint: "f3f4e0f8dc64ed373bea7716237ad14932cf98c887acead7e6b5d193216b90af"}
	surface, _, _, err := buildT21ToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("current tools=%d manifest=%s schema=%s bytes=%d", surface.ToolCount, surface.AggregateManifestDigest, surface.AggregateSchemaDigest, surface.AggregateSchemaBytes)
	_, err = RequireR03AT2Qualification(cfg)
	if err == nil || err.Error() != "business execution gate blocked: formal PeerBackendTools surface drifted" {
		t.Fatalf("expected exact surface rejection before runtime: %v", err)
	}
}
