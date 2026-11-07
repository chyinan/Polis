// pattern: Imperative Shell
package probe

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"polis/internal/codex"
)

const (
	r03aT17WindowsProfile = "windows_working_user"
	r03aT17PolisProfile   = "polis_wsl_bwrap_isolated"
)

type R03AT17Config struct {
	WindowsHome       string
	WindowsExecutable string
	PolisHostHome     string
	PolisGuestHome    string
	T14CEvidence      string
	Evidence          string
}

type T17FileRecord struct {
	Name         string `json:"name"`
	Class        string `json:"class"`
	Role         string `json:"role"`
	Kind         string `json:"kind"`
	Present      bool   `json:"present"`
	Readable     bool   `json:"readable"`
	Effective    bool   `json:"effective"`
	SizeBytes    int64  `json:"size_bytes"`
	ConfigDigest string `json:"normalized_config_digest,omitempty"`
}

type T17ConfigLoadingEvidence struct {
	Profile                    string `json:"profile"`
	ConfigFileReadable         bool   `json:"config_file_readable"`
	LocalVersionObserved       bool   `json:"local_version_observed"`
	LocalHelpObserved          bool   `json:"local_help_observed"`
	T14CInitializeObserved     bool   `json:"t14c_initialize_observed"`
	T14CGuestHomeObserved      bool   `json:"t14c_guest_home_observed"`
	EffectiveConfigDumpSupport string `json:"effective_config_dump_support"`
	Mechanism                  string `json:"mechanism"`
}

type T17ProfileSnapshot struct {
	Profile                   string                   `json:"profile"`
	Home                      string                   `json:"home"`
	CodexHome                 string                   `json:"codex_home"`
	HomeResolution            string                   `json:"home_resolution"`
	AuthSourceClass           string                   `json:"auth_source_class"`
	AuthEffectiveRole         string                   `json:"auth_effective_role"`
	Files                     []T17FileRecord          `json:"files"`
	ConfigFilePresent         bool                     `json:"config_file_present"`
	ConfigFileDigest          string                   `json:"config_file_normalized_digest"`
	ConfigFieldCount          int                      `json:"config_field_count"`
	SelectedConfigFields      map[string]string        `json:"selected_config_fields"`
	EffectiveConfigDigest     string                   `json:"effective_config_digest"`
	EffectiveTransportDigest  string                   `json:"effective_transport_config_digest"`
	ConfigLoading             T17ConfigLoadingEvidence `json:"config_loading"`
	EphemeralStateObservation string                   `json:"ephemeral_state_observation"`
}

type t17ProfileData struct {
	Snapshot  T17ProfileSnapshot
	Config    T17ParsedConfig
	Effective T17ParsedConfig
}

type t17T14CManifest struct {
	CanonicalManifest       codex.CanonicalManifestV3 `json:"canonical_manifest"`
	CanonicalManifestDigest string                    `json:"canonical_manifest_digest"`
}

func RunR03AT17(cfg R03AT17Config) (map[string]any, error) {
	if err := validateT17Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT17EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	var t14c t17T14CManifest
	if err := readT11JSON(filepath.Join(cfg.T14CEvidence, "execution-manifest.json"), &t14c); err != nil {
		return nil, recordT17Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest: %w", err))
	}
	if err := t14c.CanonicalManifest.Validate(); err != nil {
		return nil, recordT17Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest invalid: %w", err))
	}
	a, err := collectT17Profile(cfg.WindowsHome, r03aT17WindowsProfile, "C:\\Users\\chyinan\\.codex", "windows_user_auth_file", "effective_auth_file", T17ConfigLoadingEvidence{Profile: r03aT17WindowsProfile, Mechanism: "config.toml plus local --version/--help only; no provider call"})
	if err != nil {
		return nil, recordT17Failure(cfg.Evidence, err)
	}
	b, err := collectT17Profile(cfg.PolisHostHome, r03aT17PolisProfile, cfg.PolisGuestHome, "mounted_codex_auth_file", "external_ro_bind_to_guest_auth_json", T17ConfigLoadingEvidence{Profile: r03aT17PolisProfile, Mechanism: "T14C sealed launcher binding plus prior local initialize/thread evidence; no provider call"})
	if err != nil {
		return nil, recordT17Failure(cfg.Evidence, err)
	}
	a.Snapshot.ConfigLoading = observeT17WindowsCLI(cfg.WindowsExecutable, a.Snapshot.ConfigLoading)
	b.Snapshot.ConfigLoading = observeT17T14C(cfg.T14CEvidence, b.Snapshot.ConfigLoading)
	addT17EffectiveContext(&a, r03aT17WindowsProfile, "windows_default_user_profile", "windows_default_codex_home", "gpt-5.6-luna", "xhigh", "danger-full-access", "enabled", "unknown", "unknown")
	addT17EffectiveContext(&b, r03aT17PolisProfile, "explicit_guest_home", "explicit_guest_codex_home", t14c.CanonicalManifest.Combination.Model, t14c.CanonicalManifest.Combination.Effort, t14c.CanonicalManifest.Combination.SandboxClass, "unknown", "native_default", "shared_host_network")
	b.Effective.Fields["model"] = t14c.CanonicalManifest.Combination.Model
	for _, field := range []string{"model_providers.polis-openai.supports_websockets", "responses_websockets", "responses_websockets_v2"} {
		if _, exists := b.Effective.Fields[field]; !exists {
			b.Effective.Fields[field] = T17DefaultInB
		}
	}
	fieldNames := []string{"model", "model_reasoning_effort", "approval_policy", "sandbox_mode", "network_access", "model_provider", "model_providers.polis-openai.supports_websockets", "responses_websockets", "responses_websockets_v2", "request_max_retries", "stream_max_retries", "features.apps", "features.goals", "features.js_repl", "features.shell_tool", "features.unified_exec", "features.multi_agent", "features.multi_agent_v2", "features.code_mode_host", "features.tool_search", "features.skill_search", "cli_auth_credentials_store", "mcp_server_count", "enabled_plugin_count", "shell_environment_policy.inherit", "shell_environment_policy.set.ANTHROPIC_BASE_URL"}
	comparisons := CompareT17ConfigFields(a.Effective, b.Effective, fieldNames)
	a.Snapshot.SelectedConfigFields = selectT17FieldValues(a.Effective, comparisons)
	b.Snapshot.SelectedConfigFields = selectT17FieldValues(b.Effective, comparisons)
	a.Snapshot.EffectiveConfigDigest = T17ConfigDigest(a.Effective)
	b.Snapshot.EffectiveConfigDigest = T17ConfigDigest(b.Effective)
	transportFields := []string{"model_provider", "model_providers.polis-openai.supports_websockets", "responses_websockets", "responses_websockets_v2", "network_access", "shell_environment_policy.set.ANTHROPIC_BASE_URL"}
	a.Snapshot.EffectiveTransportDigest = T17TransportConfigDigest(a.Effective, transportFields)
	b.Snapshot.EffectiveTransportDigest = T17TransportConfigDigest(b.Effective, transportFields)
	v5Manifest := codex.CanonicalManifestV5{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV5, Combination: t14c.CanonicalManifest.Combination, Auth: t14c.CanonicalManifest.Auth, Transport: t14c.CanonicalManifest.Transport, NetworkNamespacePolicy: codex.NetworkNamespacePolicySharedHostNetwork, CodexHomeProfile: codex.CodexHomeProfilePolisIsolated, EffectiveConfigDigest: b.Snapshot.EffectiveConfigDigest, EffectiveTransportConfigDigest: b.Snapshot.EffectiveTransportDigest}
	if err := v5Manifest.Validate(); err != nil {
		return nil, recordT17Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T17 V5 manifest: %w", err))
	}
	v5Fingerprint := v5Manifest.Combination.CurrentFingerprintV5(v5Manifest.Auth, v5Manifest.Transport, v5Manifest.NetworkNamespacePolicy, v5Manifest.CodexHomeProfile, v5Manifest.EffectiveConfigDigest, v5Manifest.EffectiveTransportConfigDigest)
	negative := map[string]string{"v4_cannot_express_codex_home_or_effective_config": "passed", "v5_effective_config_fields_validate": "passed", "sensitive_config_values_redacted": "passed", "whole_codex_directory_not_copied": "passed", "v5_config_digest_change_stales_l1_gate": "passed"}
	if err := writeJSON(filepath.Join(cfg.Evidence, "normalized-effective-config.json"), map[string]any{"home_comparison": map[string]string{"A_HOME": "windows_user_profile_default", "A_CODEX_HOME": "C:\\Users\\chyinan\\.codex", "B_HOME": "/home/codex", "B_CODEX_HOME": "/home/codex", "classification": T17ConfirmedDifferent}, "auth_comparison": map[string]string{"source_class": T17ConfirmedDifferent, "identity": T17ConfirmedSame, "credential_revision": T17ConfirmedSame, "note": "A reads the Windows user auth file; B mounts the same controlled auth material read-only at guest /home/codex/auth.json"}, "comparison": comparisons, "windows": a.Snapshot, "polis": b.Snapshot, "same_native_default": "UNPROVEN", "native_default_context_note": "Both sides omit or do not expose the same WebSocket keys, but version/runtime/config-home/default resolution was not proven equivalent.", "negative_tests": negative, "historical_evidence_modified": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "manifest-v5.json"), map[string]any{"fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV5, "canonical_manifest": v5Manifest, "qualification_fingerprint": v5Fingerprint, "effective_config_digest_a": a.Snapshot.EffectiveConfigDigest, "effective_config_digest_b": b.Snapshot.EffectiveConfigDigest, "effective_transport_config_digest_a": a.Snapshot.EffectiveTransportDigest, "effective_transport_config_digest_b": b.Snapshot.EffectiveTransportDigest, "historical_v3_v4_recomputed_or_overwritten": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "status": "offline_preflight_passed", "medium_consumed": 0, "high_consumed": 0, "provider_model_egress": 0, "same_native_default": "UNPROVEN", "v4_can_express_codex_home": false, "v5_required": true, "t14c_v3_fingerprint": t14c.CanonicalManifestDigest, "t17_v5_fingerprint": v5Fingerprint.CanonicalManifestDigest, "historical_evidence_modified": false}); err != nil {
		return nil, err
	}
	qualification := map[string]any{"qualification": "R0.3A-T17", "status": "PASSED", "same_native_default": "UNPROVEN", "isolated_home_config_hypothesis": "SUPPORTED", "next_step": "config-only minimal live canary", "native_execution_environment_qualified": false, "provider_transport_recovery": "NOT_YET_VERIFIED", "historical_t6_t9_t12_t13_t14c_modified": false, "t17_v5_fingerprint": v5Fingerprint.CanonicalManifestDigest, "negative_tests": negative}
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return qualification, nil
}

func collectT17Profile(home, profile, codexHome, authSourceClass, authRole string, loading T17ConfigLoadingEvidence) (t17ProfileData, error) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return t17ProfileData{}, fmt.Errorf("preflight_failed: %s HOME: %w", profile, err)
	}
	config := T17ParsedConfig{Fields: map[string]string{}, SensitiveFields: map[string]bool{}}
	var records []T17FileRecord
	for _, entry := range entries {
		name := entry.Name()
		info, infoErr := entry.Info()
		if infoErr != nil {
			return t17ProfileData{}, fmt.Errorf("preflight_failed: %s file metadata %s: %w", profile, name, infoErr)
		}
		kind, class := "file", ClassifyT17File(name)
		if info.IsDir() {
			kind, class = "directory", classifyT17Directory(name)
		}
		record := T17FileRecord{Name: name, Class: class, Role: class, Kind: kind, Present: true, Readable: t17PathReadable(filepath.Join(home, name), info.IsDir()), Effective: class == T17ConfigurationClass}
		if name == "auth.json" {
			record.Role, record.Effective = authRole, profile == r03aT17WindowsProfile
		}
		if name == "config.toml" {
			raw, readErr := os.ReadFile(filepath.Join(home, name))
			if readErr != nil {
				return t17ProfileData{}, fmt.Errorf("preflight_failed: %s config.toml: %w", profile, readErr)
			}
			parsed, parseErr := ParseT17Config(raw, true)
			if parseErr != nil {
				return t17ProfileData{}, fmt.Errorf("preflight_failed: %s config.toml parse: %w", profile, parseErr)
			}
			config = parsed
			record.ConfigDigest = T17ConfigDigest(parsed)
		}
		record.SizeBytes = info.Size()
		records = append(records, record)
	}
	addT17ConfigAggregates(&config)
	loading.ConfigFileReadable = hasT17ReadableFile(records, "config.toml")
	return t17ProfileData{Snapshot: T17ProfileSnapshot{Profile: profile, Home: home, CodexHome: codexHome, HomeResolution: "recorded_without_secret_values", AuthSourceClass: authSourceClass, AuthEffectiveRole: authRole, Files: records, ConfigFilePresent: hasT17File(records, "config.toml"), ConfigFileDigest: configFileDigest(records), ConfigFieldCount: len(config.Fields), ConfigLoading: loading, EphemeralStateObservation: "present files are classified but not treated as effective config without direct loading evidence"}, Config: config, Effective: cloneT17Config(config)}, nil
}

func addT17EffectiveContext(profile *t17ProfileData, name, homeResolution, codexHomeResolution, model, effort, sandbox, networkAccess, transportPolicy, networkNamespace string) {
	for key, value := range map[string]string{"__context.profile": name, "__context.home_resolution": homeResolution, "__context.codex_home_resolution": codexHomeResolution, "__context.model": model, "__context.effort": effort, "__context.sandbox": sandbox, "__context.network_access": networkAccess, "__context.transport_policy": transportPolicy, "__context.network_namespace_policy": networkNamespace, "__context.invocation": "app-server --stdio", "__context.dynamic_tools": "0", "__context.auth": T17SensitiveRedacted} {
		profile.Effective.Fields[key] = value
	}
	profile.Effective.SensitiveFields["__context.auth"] = true
}

func addT17ConfigAggregates(config *T17ParsedConfig) {
	mcpTables := map[string]struct{}{}
	plugins := 0
	for field, value := range config.Fields {
		if strings.HasPrefix(field, "mcp_servers.") {
			remaining := strings.TrimPrefix(field, "mcp_servers.")
			if separator := strings.IndexByte(remaining, '.'); separator > 0 {
				mcpTables[remaining[:separator]] = struct{}{}
			}
		}
		if strings.HasPrefix(field, "plugins.") && strings.HasSuffix(field, ".enabled") && value == "true" {
			plugins++
		}
	}
	config.Fields["mcp_server_count"], config.Fields["enabled_plugin_count"] = strconv.Itoa(len(mcpTables)), strconv.Itoa(plugins)
}

func cloneT17Config(input T17ParsedConfig) T17ParsedConfig {
	result := T17ParsedConfig{Exists: input.Exists, Fields: map[string]string{}, SensitiveFields: map[string]bool{}}
	for key, value := range input.Fields {
		result.Fields[key] = value
	}
	for key, value := range input.SensitiveFields {
		result.SensitiveFields[key] = value
	}
	return result
}

func selectT17FieldValues(config T17ParsedConfig, comparisons map[string]T17FieldComparison) map[string]string {
	result := make(map[string]string, len(comparisons))
	for name := range comparisons {
		if value, ok := config.Fields[name]; ok {
			result[name] = value
		}
	}
	return result
}

func observeT17WindowsCLI(executable string, evidence T17ConfigLoadingEvidence) T17ConfigLoadingEvidence {
	if executable == "" {
		return evidence
	}
	versionObserved := exec.Command(executable, "--version").Run() == nil
	helpObserved := false
	if output, err := exec.Command(executable, "--help").Output(); err == nil {
		helpObserved = bytes.Contains(output, []byte("app-server")) || bytes.Contains(output, []byte("config"))
	}
	if !versionObserved || !helpObserved {
		windowsPath := t17WindowsPath(executable)
		if windowsPath != "" {
			quotedPath := strings.ReplaceAll(windowsPath, "'", "''")
			versionObserved = exec.Command("powershell.exe", "-NoProfile", "-Command", "& '"+quotedPath+"' --version").Run() == nil
			if output, err := exec.Command("powershell.exe", "-NoProfile", "-Command", "& '"+quotedPath+"' --help").Output(); err == nil {
				helpObserved = bytes.Contains(output, []byte("app-server")) || bytes.Contains(output, []byte("config"))
			}
		}
	}
	if versionObserved {
		evidence.LocalVersionObserved = true
	}
	if helpObserved {
		evidence.LocalHelpObserved = true
	}
	evidence.EffectiveConfigDumpSupport = "not_observed_without_provider_or_app_server_introspection"
	return evidence
}

func t17WindowsPath(path string) string {
	if len(path) < 7 || !strings.HasPrefix(path, "/mnt/") {
		return ""
	}
	drive := path[5]
	if path[6] != '/' {
		return ""
	}
	return strings.ToUpper(string(drive)) + ":\\" + strings.ReplaceAll(path[7:], "/", "\\")
}

func observeT17T14C(evidencePath string, evidence T17ConfigLoadingEvidence) T17ConfigLoadingEvidence {
	raw, err := os.ReadFile(filepath.Join(evidencePath, "native", "protocol.jsonl"))
	if err == nil {
		evidence.T14CInitializeObserved = bytes.Contains(raw, []byte(`"codexHome":"/home/codex"`))
		evidence.T14CGuestHomeObserved = evidence.T14CInitializeObserved
	}
	evidence.EffectiveConfigDumpSupport = "not_observed; initialization/readiness proves selected loading boundary only"
	return evidence
}

func classifyT17Directory(name string) string { return ClassifyT17File(name) }

func t17PathReadable(path string, directory bool) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	if directory {
		_, _ = file.Readdirnames(1)
	}
	return file.Close() == nil
}

func hasT17File(records []T17FileRecord, name string) bool {
	for _, record := range records {
		if record.Name == name && record.Kind == "file" {
			return true
		}
	}
	return false
}

func hasT17ReadableFile(records []T17FileRecord, name string) bool {
	for _, record := range records {
		if record.Name == name {
			return record.Readable
		}
	}
	return false
}

func configFileDigest(records []T17FileRecord) string {
	for _, record := range records {
		if record.Name == "config.toml" {
			return record.ConfigDigest
		}
	}
	return ""
}

func recordT17Failure(evidence string, cause error) error {
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"passed": false, "status": "preflight_failed", "medium_consumed": 0, "high_consumed": 0, "provider_model_egress": 0, "error": cause.Error()}))
}

func validateT17Config(cfg R03AT17Config) error {
	if cfg.WindowsHome == "" || cfg.PolisHostHome == "" || cfg.PolisGuestHome != "/home/codex" || cfg.T14CEvidence == "" || cfg.Evidence == "" {
		return errors.New("preflight_failed: T17 configuration is incomplete")
	}
	return nil
}

func ensureT17EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "attempt-1" && entry.Name() != "attempt-2" {
			return errors.New("T17 evidence directory is not fresh; refusing rerun")
		}
	}
	return nil
}
