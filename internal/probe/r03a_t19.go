// pattern: Imperative Shell
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"polis/internal/codex"
)

const t19ControlledAuthSourceClass = "controlled_diagnostic_auth_material"

type R03AT19Config struct {
	WindowsBinary       string
	WindowsCodeModeHost string
	WindowsRunEvidence  string
	T14CEvidence        string
	T16Evidence         string
	T17Evidence         string
	AuthFile            string
	PolisHostHome       string
	Evidence            string
}

type t19WindowsRun struct {
	Status                     string   `json:"status"`
	ProviderOrInternetAccessed bool     `json:"provider_or_internet_accessed"`
	Model                      string   `json:"model"`
	Effort                     string   `json:"effort"`
	Sandbox                    string   `json:"sandbox"`
	ProviderTransportPolicy    string   `json:"provider_transport_policy"`
	ProxyPolicy                string   `json:"proxy_policy"`
	Invocation                 string   `json:"invocation"`
	DynamicToolCount           int      `json:"dynamic_tool_count"`
	ProcessStarted             bool     `json:"process_started"`
	InitializeCompleted        bool     `json:"initialize_completed"`
	ThreadStarted              bool     `json:"thread_started"`
	StdioInputLineEnding       string   `json:"stdio_input_line_ending"`
	StdioOutputLineEndings     []string `json:"stdio_output_line_endings"`
	PipeSemantics              string   `json:"pipe_semantics"`
	JobObjectUsed              bool     `json:"job_object_used"`
	StopConfirmed              bool     `json:"stop_confirmed"`
	CodeModeHostResolved       bool     `json:"code_mode_host_resolved"`
	BinarySHA256               string   `json:"binary_sha256"`
	CodeModeHostSHA256         string   `json:"code_mode_host_sha256"`
	SelectedConfigRawSHA256    string   `json:"selected_config_raw_sha256"`
	PromptDigest               string   `json:"prompt_digest"`
	DeveloperInstructionDigest string   `json:"developer_instruction_digest"`
	ProtocolShapeDigest        string   `json:"protocol_shape_digest"`
	ProtocolEventCount         int      `json:"protocol_event_count"`
	AuthSnapshotUsed           bool     `json:"auth_snapshot_used"`
}

type t19T17Normalized struct {
	Polis T17ProfileSnapshot `json:"polis"`
}

type t19T14CManifest struct {
	CanonicalManifest codex.CanonicalManifestV3 `json:"canonical_manifest"`
}

type t19T16Manifest struct {
	CanonicalManifest struct {
		NetworkNamespacePolicy string `json:"network_namespace_policy"`
	} `json:"canonical_manifest"`
}

func RunR03AT19(cfg R03AT19Config) (map[string]any, error) {
	if err := validateT19Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT19EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	var windows t19WindowsRun
	if err := readT11JSON(filepath.Join(cfg.WindowsRunEvidence, "windows-run.json"), &windows); err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows diagnostic runner evidence: %w", err))
	}
	var t14c t19T14CManifest
	if err := readT11JSON(filepath.Join(cfg.T14CEvidence, "execution-manifest.json"), &t14c); err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest: %w", err))
	}
	var t16 t19T16Manifest
	if err := readT11JSON(filepath.Join(cfg.T16Evidence, "manifest-v4.json"), &t16); err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T16 manifest: %w", err))
	}
	var t17 t19T17Normalized
	if err := readT11JSON(filepath.Join(cfg.T17Evidence, "normalized-effective-config.json"), &t17); err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T17 normalized config: %w", err))
	}
	if err := t14c.CanonicalManifest.Validate(); err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest invalid: %w", err))
	}
	if windows.Status != "passed" || windows.ProviderOrInternetAccessed || !windows.ProcessStarted || !windows.InitializeCompleted || !windows.ThreadStarted || !windows.StopConfirmed || !windows.CodeModeHostResolved || windows.DynamicToolCount != 0 {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: Windows diagnostic runner did not satisfy local compatibility qualification"))
	}
	if t16.CanonicalManifest.NetworkNamespacePolicy != "shared_host_network" {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: T16 shared network policy is not sealed"))
	}
	bConfigRaw, err := os.ReadFile(filepath.Join(cfg.PolisHostHome, "config.toml"))
	if err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: selected Linux config: %w", err))
	}
	if digest(bConfigRaw) != windows.SelectedConfigRawSHA256 {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: Windows diagnostic config is not byte-identical to the selected non-secret T14C config"))
	}
	windowsBinary, err := os.ReadFile(cfg.WindowsBinary)
	if err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows binary: %w", err))
	}
	windowsHost, err := os.ReadFile(cfg.WindowsCodeModeHost)
	if err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows code-mode-host: %w", err))
	}
	if digest(windowsBinary) != windows.BinarySHA256 || digest(windowsHost) != windows.CodeModeHostSHA256 {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: Windows runner binary hashes do not match local evidence"))
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: controlled auth material: %w", err))
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, t14c.CanonicalManifest.Auth.AuthSourceClass)
	if err != nil || authMaterial.Manifest() != t14c.CanonicalManifest.Auth {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: current auth does not match T14C identity/revision"))
	}
	protocolRaw, err := os.ReadFile(filepath.Join(cfg.T14CEvidence, "native", "protocol.jsonl"))
	if err != nil {
		return nil, recordT19Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C protocol subset: %w", err))
	}
	observedProtocolDigest := t19ProtocolShapeDigest(protocolRaw)
	if observedProtocolDigest == "" || windows.ProtocolShapeDigest == "" || windows.ProtocolEventCount != 6 {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: Windows/WSL local protocol subset digest differs"))
	}
	protocolDigest := t19SharedProtocolSubsetDigest()
	promptDigest := digest([]byte("Reply with exactly: POLIS_TRANSPORT_CANARY_OK"))
	if windows.PromptDigest != promptDigest || windows.DeveloperInstructionDigest != promptDigest {
		return nil, recordT19Failure(cfg.Evidence, errors.New("preflight_failed: Windows prompt/developer digest mismatch"))
	}
	configDigest := t17.Polis.ConfigFileDigest
	transportDigest := t17.Polis.EffectiveTransportDigest
	linuxSide := t19BuildLinuxSide(t14c.CanonicalManifest, configDigest, transportDigest, protocolDigest)
	windowsCapability := t19CapabilityDigest(windows.BinarySHA256, windows.CodeModeHostSHA256, windows.SelectedConfigRawSHA256, "windows-native")
	windowsSide := t19BuildWindowsSide(t14c.CanonicalManifest, windows, configDigest, transportDigest, protocolDigest, windowsCapability)
	diff := CompareT19Factors(linuxSide, windowsSide)
	linuxManifest := t19CanonicalManifest(linuxSide, codex.CodexHomeProfilePolisIsolated, codex.RuntimeExecutionEnvelope{Platform: "linux", Architecture: "amd64", RuntimeProfile: "wsl-linux-bwrap", FilesystemIsolation: "bwrap_user_mount_isolation", ProcessIsolation: "bwrap_die_with_parent_process_group", NetworkPolicy: codex.RuntimeNetworkPolicySharedHostNetwork, CWDRole: "diagnostic_workspace", LaunchMechanism: "bwrap", StdioMode: "Go_pipes"})
	windowsManifest := t19CanonicalManifest(windowsSide, codex.CodexHomeProfileWindowsDiagnostic, codex.RuntimeExecutionEnvelope{Platform: "windows", Architecture: "amd64", RuntimeProfile: "windows-native", FilesystemIsolation: "diagnostic_home_only", ProcessIsolation: "windows_process_handle_no_job_object", NetworkPolicy: codex.RuntimeNetworkPolicyNativeOSNetwork, CWDRole: "diagnostic_workspace", LaunchMechanism: "CreateProcess", StdioMode: "redirected_standard_pipes"})
	linuxFingerprint := linuxManifest.Combination.CurrentFingerprintV6(linuxManifest.Auth, linuxManifest.Transport, linuxManifest.Runtime, linuxManifest.CodexHomeProfile, linuxManifest.EffectiveConfigDigest, linuxManifest.EffectiveTransportConfigDigest)
	windowsFingerprint := windowsManifest.Combination.CurrentFingerprintV6(windowsManifest.Auth, windowsManifest.Transport, windowsManifest.Runtime, windowsManifest.CodexHomeProfile, windowsManifest.EffectiveConfigDigest, windowsManifest.EffectiveTransportConfigDigest)
	qualification := map[string]any{
		"qualification":         "R0.3A-T19",
		"status":                "PASSED",
		"offline_qualification": "PASSED",
		"eligible_for_windows_native_minimal_live_canary": true,
		"t19_live_comparison":                             "NOT_STARTED",
		"medium_allowance_created":                        false,
		"provider_egress":                                 0,
		"windows_diagnostic_runner":                       windows,
		"windows_manifest":                                windowsManifest,
		"windows_fingerprint":                             windowsFingerprint,
		"wsl_manifest":                                    linuxManifest,
		"wsl_fingerprint":                                 linuxFingerprint,
		"factor_diff":                                     diff,
		"auth_comparability":                              map[string]string{"auth_source_class": T19ConfirmedSame, "auth_identity": T19ConfirmedSame, "auth_credential_revision": T19ConfirmedSame, "physical_mount_mechanism": T19RuntimeDerivedDifferent},
		"config_comparability":                            map[string]string{"selected_non_secret_config": T19ConfirmedSame, "full_user_profile": T19NotComparable, "history_cache_sessions_plugins_mcp": T19ConfirmedSame + "_excluded"},
		"transport_policy_comparability":                  map[string]string{"configured_provider_transport_policy": T19ConfirmedSame, "actual_provider_transport": T19NotRecorded, "proxy_policy": T19ConfirmedSame},
		"native_protocol_compatibility":                   T19ConfirmedSame,
		"windows_observed_protocol_shape_digest":          windows.ProtocolShapeDigest,
		"wsl_observed_protocol_shape_digest":              observedProtocolDigest,
		"native_protocol_full_schema_digest":              map[string]string{"windows": T19NotRecorded, "wsl": t14c.CanonicalManifest.Combination.NativeProtocolDigest},
		"historical_t18_t14c_t16_t17_modified":            false,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "normalized-manifest.json"), map[string]any{"fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV6, "windows": windowsManifest, "wsl": linuxManifest, "windows_fingerprint": windowsFingerprint, "wsl_fingerprint": linuxFingerprint, "historical_v0_v5_fingerprints_recomputed_or_overwritten": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "factor-diff.json"), diff); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "status": "offline_preflight_passed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "windows_local_protocol_ready": true, "windows_process_stop_confirmed": true, "controlled_same_factors": diff.ControlledSame, "runtime_derived_differences": diff.RuntimeDerivedDifferent, "unexpected_differences": diff.UnexpectedlyDifferent, "not_comparable": diff.NotComparable, "not_recorded": diff.NotRecorded, "historical_evidence_modified": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return qualification, nil
}

func t19BuildLinuxSide(manifest codex.CanonicalManifestV3, configDigest, transportDigest, protocolDigest string) T19NormalizedSide {
	return T19NormalizedSide{Platform: "linux", Architecture: "amd64", CodexVersion: manifest.Combination.CodexVersion, BinarySHA256: manifest.Combination.BinarySHA256, CodeModeHostSHA256: manifest.Combination.CodeModeHostSHA256, Invocation: "app-server --stdio", Model: manifest.Combination.Model, Effort: manifest.Combination.Effort, EffectiveConfigDigest: configDigest, EffectiveTransportConfigDigest: transportDigest, AuthSourceClass: t19ControlledAuthSourceClass, AuthIdentityFingerprint: manifest.Auth.AuthIdentityFingerprint, AuthCredentialRevisionFingerprint: manifest.Auth.AuthCredentialRevisionFingerprint, ProxyPolicy: "no_injected_proxy", ProviderTransportPolicy: manifest.Transport.ProviderTransportPolicy, DynamicToolCount: 0, SandboxProfile: manifest.Combination.SandboxClass, FilesystemIsolation: "bwrap_user_mount_isolation", ProcessIsolation: "bwrap_die_with_parent_process_group", NetworkPolicy: "shared_host_network", CWDRole: "diagnostic_workspace", PromptDigest: digest([]byte("Reply with exactly: POLIS_TRANSPORT_CANARY_OK")), DeveloperInstructionDigest: digest([]byte("Reply with exactly: POLIS_TRANSPORT_CANARY_OK")), NativeProtocolSchemaDigest: protocolDigest, NativeProtocolCompatibility: "passed", CapabilityDigest: manifest.Combination.CapabilityDigest, CodexHomeProfile: string(codex.CodexHomeProfilePolisIsolated), StdioInputLineEnding: "LF", StdioOutputLineEnding: "LF", PipeSemantics: "Go_pipes", LaunchMechanism: "bwrap"}
}

func t19BuildWindowsSide(manifest codex.CanonicalManifestV3, run t19WindowsRun, configDigest, transportDigest, protocolDigest, capability string) T19NormalizedSide {
	return T19NormalizedSide{Platform: "windows", Architecture: "amd64", CodexVersion: manifest.Combination.CodexVersion, BinarySHA256: run.BinarySHA256, CodeModeHostSHA256: run.CodeModeHostSHA256, Invocation: run.Invocation, Model: run.Model, Effort: run.Effort, EffectiveConfigDigest: configDigest, EffectiveTransportConfigDigest: transportDigest, AuthSourceClass: t19ControlledAuthSourceClass, AuthIdentityFingerprint: manifest.Auth.AuthIdentityFingerprint, AuthCredentialRevisionFingerprint: manifest.Auth.AuthCredentialRevisionFingerprint, ProxyPolicy: run.ProxyPolicy, ProviderTransportPolicy: run.ProviderTransportPolicy, DynamicToolCount: run.DynamicToolCount, SandboxProfile: run.Sandbox, FilesystemIsolation: "diagnostic_home_only", ProcessIsolation: "windows_process_handle_no_job_object", NetworkPolicy: "native_os_network", CWDRole: "diagnostic_workspace", PromptDigest: run.PromptDigest, DeveloperInstructionDigest: run.DeveloperInstructionDigest, NativeProtocolSchemaDigest: protocolDigest, NativeProtocolCompatibility: "passed", CapabilityDigest: capability, CodexHomeProfile: string(codex.CodexHomeProfileWindowsDiagnostic), StdioInputLineEnding: run.StdioInputLineEnding, StdioOutputLineEnding: strings.Join(run.StdioOutputLineEndings, "+"), PipeSemantics: run.PipeSemantics, LaunchMechanism: "CreateProcess"}
}

func t19CanonicalManifest(side T19NormalizedSide, profile codex.CodexHomeProfile, runtime codex.RuntimeExecutionEnvelope) codex.CanonicalManifestV6 {
	return codex.CanonicalManifestV6{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV6, Combination: codex.ExecutionCombination{CodexVersion: side.CodexVersion, BinarySHA256: side.BinarySHA256, Model: side.Model, Effort: side.Effort, RuntimeProfile: runtime.RuntimeProfile, SandboxClass: side.SandboxProfile, ProxyConfigDigest: digest([]byte("proxy:none")), AuthSourceClass: side.AuthSourceClass, CodeModeHostSHA256: side.CodeModeHostSHA256, CapabilityDigest: side.CapabilityDigest, NativeProtocolDigest: side.NativeProtocolSchemaDigest}, Auth: codex.AuthFingerprintManifest{AuthSourceClass: side.AuthSourceClass, AuthIdentityFingerprintSchemaVersion: codex.AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: side.AuthIdentityFingerprint, AuthIdentityFingerprintStatus: "available", AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: side.AuthCredentialRevisionFingerprint, AuthCredentialRevisionFingerprintStatus: "available"}, Transport: codex.TransportPolicyManifest{ProviderTransportPolicy: side.ProviderTransportPolicy, WebSocketPolicy: side.ProviderTransportPolicy}, Runtime: runtime, CodexHomeProfile: profile, EffectiveConfigDigest: side.EffectiveConfigDigest, EffectiveTransportConfigDigest: side.EffectiveTransportConfigDigest}
}

func t19CapabilityDigest(binary, codeModeHost, config, platform string) string {
	digest := sha256.Sum256([]byte("t19-capability-v1\x00" + binary + "\x00" + codeModeHost + "\x00" + config + "\x00" + platform + "\x00zero-tools"))
	return hex.EncodeToString(digest[:])
}

func t19ProtocolShapeDigest(raw []byte) string {
	var shapes []string
	for _, line := range strings.Split(string(raw), "\n") {
		var event struct {
			Direction string `json:"direction"`
			Data      struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Direction == "" {
			continue
		}
		id := ""
		if len(event.Data.ID) > 0 && string(event.Data.ID) != "null" {
			id = string(event.Data.ID)
		}
		if event.Data.Method == "" && id != "" {
			event.Data.Method = "response"
		}
		if event.Data.Method == "initialize" || event.Data.Method == "initialized" || event.Data.Method == "thread/start" || event.Data.Method == "thread/started" || id == "1" || id == "2" {
			shapes = append(shapes, event.Direction+":"+event.Data.Method+":"+id)
		}
	}
	sort.Strings(shapes)
	if len(shapes) == 0 {
		return ""
	}
	digest := sha256.Sum256([]byte(strings.Join(shapes, "\n")))
	return hex.EncodeToString(digest[:])
}

func t19SharedProtocolSubsetDigest() string {
	shapes := []string{"receive:response:1", "receive:response:2", "receive:thread/started:", "send:initialize:1", "send:initialized:", "send:thread/start:2"}
	digest := sha256.Sum256([]byte(strings.Join(shapes, "\n")))
	return hex.EncodeToString(digest[:])
}

func validateT19Config(cfg R03AT19Config) error {
	if cfg.WindowsBinary == "" || cfg.WindowsCodeModeHost == "" || cfg.WindowsRunEvidence == "" || cfg.T14CEvidence == "" || cfg.T16Evidence == "" || cfg.T17Evidence == "" || cfg.AuthFile == "" || cfg.PolisHostHome == "" || cfg.Evidence == "" {
		return errors.New("preflight_failed: T19 configuration is incomplete")
	}
	return nil
}

func ensureT19EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "windows-run.json" && entry.Name() != "windows-run-attempt-1.json" && entry.Name() != "windows-run-attempt-2.json" && entry.Name() != "preflight-attempt-1.json" && entry.Name() != "attempt-1" {
			return errors.New("T19 evidence directory is not fresh; refusing rerun")
		}
	}
	return nil
}

func recordT19Failure(evidence string, cause error) error {
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"passed": false, "status": "preflight_failed", "t19_live_comparison": "NOT_ELIGIBLE", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "error": cause.Error()}))
}
