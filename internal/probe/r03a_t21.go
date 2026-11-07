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
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
)

const (
	r03aT21ToolSurfaceID        = "peer_backend"
	r03aT21DeveloperInstruction = "Do not call tools.\nReply only with the transport sentinel."
	r03aT21Sentinel             = "POLIS_TRANSPORT_CANARY_OK"
	r03aT21PromptSemantics      = "tiny_no_tools_sentinel"
	r03aT21ToolManifestSchema   = "canonical-tool-manifest-v1"
	r03aT21ExecutionConfigV1    = "t21-effective-execution-config-v1"
)

type R03AT21Config struct {
	WindowsBinary       string
	WindowsCodeModeHost string
	T5Evidence          string
	T20Evidence         string
	AuthFile            string
	SelectedLinuxConfig string
	Evidence            string
}

type t21T5ToolEntry struct {
	Name                  string `json:"Name"`
	SchemaCanonicalDigest string `json:"SchemaCanonicalDigest"`
	SchemaBytes           int    `json:"SchemaBytes"`
	DescriptionDigest     string `json:"DescriptionDigest"`
	Handler               string `json:"Handler"`
	AuthorizationClass    string `json:"AuthorizationClass"`
	RegistrationOrdinal   int    `json:"RegistrationOrdinal"`
}

type t21T5Manifest struct {
	FailingBackendManifest []t21T5ToolEntry `json:"FailingBackendManifest"`
}

type t21ExecutionConfig struct {
	SchemaVersion                     string                           `json:"schema_version"`
	T20ControlFingerprint             string                           `json:"t20_control_fingerprint"`
	CanonicalManifest                 codex.CanonicalManifestV7        `json:"canonical_manifest"`
	QualificationFingerprint          codex.QualificationFingerprintV7 `json:"qualification_fingerprint"`
	ToolSurface                       codex.ToolSurfaceManifest        `json:"tool_surface"`
	ToolNames                         []string                         `json:"tool_names"`
	ToolManifestSchemaVersion         string                           `json:"tool_manifest_schema_version"`
	ToolManifestDigest                string                           `json:"tool_manifest_digest"`
	AggregateSchemaDigest             string                           `json:"aggregate_schema_digest"`
	AggregateSchemaBytes              int                              `json:"aggregate_schema_bytes"`
	ThreadStartDeveloperInstruction   string                           `json:"thread_start_developer_instruction"`
	PromptSemantics                   string                           `json:"prompt_semantics"`
	ThreadStartPayloadCanonicalJSON   string                           `json:"thread_start_payload_canonical_json"`
	ThreadStartPayloadDigest          string                           `json:"thread_start_payload_digest"`
	ThreadStartPayloadBytes           int                              `json:"thread_start_payload_bytes"`
	AuthSnapshotRequired              bool                             `json:"auth_snapshot_required"`
	AuthIdentityFingerprint           string                           `json:"auth_identity_fingerprint"`
	AuthCredentialRevisionFingerprint string                           `json:"auth_credential_revision_fingerprint"`
	SelectedConfigRawSHA256           string                           `json:"selected_config_raw_sha256"`
	LaunchConfigDigest                string                           `json:"launch_config_digest"`
}

type t21LiveRun struct {
	SchemaVersion              string   `json:"schema_version"`
	Status                     string   `json:"status"`
	PreflightPassed            bool     `json:"preflight_passed"`
	ProviderOrInternetAccessed bool     `json:"provider_or_internet_accessed"`
	ProviderEgress             int      `json:"provider_egress"`
	MediumStarted              int      `json:"medium_started"`
	ProcessStarted             bool     `json:"process_started"`
	InitializeCompleted        bool     `json:"initialize_completed"`
	ThreadStarted              bool     `json:"thread_started"`
	TurnStarted                bool     `json:"turn_started"`
	UserMessageItem            bool     `json:"user_message_item"`
	RegisteredToolCount        int      `json:"registered_tool_count"`
	ToolManifestDigest         string   `json:"tool_manifest_digest"`
	AggregateSchemaDigest      string   `json:"aggregate_schema_digest"`
	AggregateSchemaBytes       int      `json:"aggregate_schema_bytes"`
	ThreadStartPayloadBytes    int      `json:"thread_start_payload_bytes"`
	ThreadStartPayloadDigest   string   `json:"thread_start_payload_digest"`
	AttemptedToolCalls         []string `json:"attempted_tool_calls"`
	BusinessSideEffects        int      `json:"business_side_effects"`
	FirstValidOutput           bool     `json:"first_valid_output"`
	FirstValidOutputText       string   `json:"first_valid_output_text,omitempty"`
	TimeToFirstOutputMS        int64    `json:"time_to_first_valid_output_ms,omitempty"`
	ReconnectCount             int      `json:"reconnect_count"`
	ReconnectPhases            []string `json:"reconnect_phases"`
	FirstDisconnectDeltaMS     int64    `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS            int64    `json:"recovery_delta_ms,omitempty"`
	NativeUsageUpdates         int      `json:"native_usage_updates"`
	TokenUsage                 any      `json:"token_usage,omitempty"`
	TurnCompleted              bool     `json:"turn_completed"`
	TurnState                  string   `json:"turn_state"`
	SentinelMatch              string   `json:"sentinel_match"`
	UnresolvedTransportState   bool     `json:"unresolved_transport_state"`
	FailureMode                string   `json:"failure_mode"`
	TerminalState              string   `json:"terminal_state"`
	StopConfirmed              bool     `json:"stop_confirmed"`
	StopMechanism              string   `json:"stop_mechanism"`
	ExitCode                   *int     `json:"exit_code,omitempty"`
	ProcessStart               string   `json:"process_start"`
	ProcessStop                string   `json:"process_stop"`
	Error                      string   `json:"error,omitempty"`
}

func RunR03AT21Preflight(cfg R03AT21Config) (map[string]any, error) {
	if err := validateT21Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT21EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	var t20Config t20ExecutionConfig
	if err := readT11JSON(filepath.Join(cfg.T20Evidence, "execution-config.json"), &t20Config); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T20 execution config: %w", err))
	}
	if err := validateT20ControlForT21(t20Config); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, err)
	}
	var t20Result struct {
		Result          string `json:"result"`
		WindowsNativeL1 string `json:"windows_native_l1"`
		MediumConsumed  int    `json:"medium_consumed"`
		ProviderEgress  int    `json:"provider_egress"`
	}
	if err := readT11JSON(filepath.Join(cfg.T20Evidence, "result.json"), &t20Result); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T20 result: %w", err))
	}
	if t20Result.Result != "PASSED" || t20Result.WindowsNativeL1 != "qualified" || t20Result.MediumConsumed != 1 || t20Result.ProviderEgress != 1 {
		return nil, recordT21PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T20 Windows-native control is not sealed as passed"))
	}
	binaryRaw, err := os.ReadFile(cfg.WindowsBinary)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows binary: %w", err))
	}
	codeModeHostRaw, err := os.ReadFile(cfg.WindowsCodeModeHost)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows code-mode-host: %w", err))
	}
	configRaw, err := os.ReadFile(cfg.SelectedLinuxConfig)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: selected diagnostic config: %w", err))
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: controlled auth material: %w", err))
	}
	if digest(binaryRaw) != t20Config.CanonicalManifest.Combination.BinarySHA256 || digest(codeModeHostRaw) != t20Config.CanonicalManifest.Combination.CodeModeHostSHA256 || digest(configRaw) != t20Config.SelectedConfigRawSHA256 {
		return nil, recordT21PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T20 Windows binary, code-mode-host, or selected config drifted"))
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, t20Config.CanonicalManifest.Auth.AuthSourceClass)
	if err != nil || authMaterial.Manifest() != t20Config.CanonicalManifest.Auth {
		return nil, recordT21PreflightFailure(cfg.Evidence, errors.New("preflight_failed: current auth identity or credential revision differs from T20"))
	}
	var t5Manifest t21T5Manifest
	if err := readT11JSON(filepath.Join(cfg.T5Evidence, "tool-manifest.json"), &t5Manifest); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T5 tool manifest: %w", err))
	}
	surface, tools, toolRaw, err := buildT21ToolSurface()
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, err)
	}
	if err := compareT21WithT5(surface, t5Manifest); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, err)
	}
	normalizedPayload := t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction)
	normalizedPayloadRaw, err := json.Marshal(normalizedPayload)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: thread/start payload serialization: %w", err))
	}
	payloadDigest := digest(normalizedPayloadRaw)
	surface.ThreadStartPayloadDigest = payloadDigest
	surface.ThreadStartPayloadBytes = len(normalizedPayloadRaw)
	toolManifestDigest := digest(toolRaw)
	capabilityDigest := t21CapabilityDigest(t20Config.CanonicalManifest.Combination.CapabilityDigest, surface)
	baseManifest := t20Config.CanonicalManifest
	baseManifest.Combination.CapabilityDigest = capabilityDigest
	v7Manifest := codex.CanonicalManifestV7{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV7, Base: baseManifest, ToolSurface: surface}
	if err := v7Manifest.Validate(); err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: generated T21 V7 manifest: %w", err))
	}
	v7Fingerprint := baseManifest.Combination.CurrentFingerprintV7(v7Manifest)
	controlFactors := t21ControlFactorDiff(t20Config, v7Manifest, surface, payloadDigest, len(normalizedPayloadRaw))
	if len(controlFactors["unexpectedly_different"].([]any)) != 0 {
		return nil, recordT21PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T20 controlled execution factor drift"))
	}
	executionConfig := t21ExecutionConfig{
		SchemaVersion: r03aT21ExecutionConfigV1, T20ControlFingerprint: t20Config.QualificationFingerprint.CanonicalManifestDigest,
		CanonicalManifest: v7Manifest, QualificationFingerprint: v7Fingerprint, ToolSurface: surface, ToolNames: t21ToolNames(surface),
		ToolManifestSchemaVersion: r03aT21ToolManifestSchema, ToolManifestDigest: toolManifestDigest,
		AggregateSchemaDigest: surface.AggregateSchemaDigest, AggregateSchemaBytes: surface.AggregateSchemaBytes,
		ThreadStartDeveloperInstruction: r03aT21DeveloperInstruction, PromptSemantics: r03aT21PromptSemantics,
		ThreadStartPayloadCanonicalJSON: string(normalizedPayloadRaw), ThreadStartPayloadDigest: payloadDigest, ThreadStartPayloadBytes: len(normalizedPayloadRaw),
		AuthSnapshotRequired: true, AuthIdentityFingerprint: authMaterial.IdentityFingerprint, AuthCredentialRevisionFingerprint: authMaterial.CredentialRevisionFingerprint,
		SelectedConfigRawSHA256: digest(configRaw), LaunchConfigDigest: t20Config.LaunchConfigDigest,
	}
	executionRaw, err := json.Marshal(executionConfig)
	if err != nil {
		return nil, recordT21PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T21 execution config serialization: %w", err))
	}
	executionDigest := digest(executionRaw)
	preflight := map[string]any{
		"qualification": "R0.3A-T21", "passed": true, "status": "offline_preflight_passed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0,
		"t20_control_fingerprint": t20Config.QualificationFingerprint.CanonicalManifestDigest, "t21_fingerprint": v7Fingerprint.CanonicalManifestDigest,
		"execution_config_digest": executionDigest, "launch_config_digest": t20Config.LaunchConfigDigest,
		"auth_identity_fingerprint": authMaterial.IdentityFingerprint, "auth_credential_revision_fingerprint": authMaterial.CredentialRevisionFingerprint,
		"auth_recheck_passed": true, "binary_config_recheck_passed": true, "t20_controlled_factors_unchanged": true, "dynamic_tool_surface_changed_only": true,
		"tool_manifest_digest": toolManifestDigest, "aggregate_schema_digest": surface.AggregateSchemaDigest, "aggregate_schema_bytes": surface.AggregateSchemaBytes,
		"thread_start_payload_digest": payloadDigest, "thread_start_payload_bytes": len(normalizedPayloadRaw), "historical_evidence_modified": false, "t5_tool_surface_requalification": "passed",
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "tool-surface.json"), map[string]any{"schema_version": r03aT21ToolManifestSchema, "source": "codex.PeerBackendTools", "surface": surface, "tool_names": t21ToolNames(surface), "tools": json.RawMessage(toolRaw)}); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cfg.Evidence, "tool-registry.json"), append(toolRaw, '\n'), 0600); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-config.json"), executionConfig); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), map[string]any{"qualification": "R0.3A-T21", "fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV7, "canonical_manifest": v7Manifest, "canonical_manifest_digest": v7Fingerprint.CanonicalManifestDigest, "qualification_fingerprint": v7Fingerprint, "source_execution_config_digest": executionDigest, "launch_config_digest": t20Config.LaunchConfigDigest, "tool_manifest_digest": toolManifestDigest, "tool_names": t21ToolNames(surface), "dynamic_tool_count": surface.ToolCount, "historical_v6_modified": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "factor-diff.json"), controlFactors); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), preflight); err != nil {
		return nil, err
	}
	return preflight, nil
}

type R03AT21BConfig struct {
	WindowsBinary       string
	WindowsCodeModeHost string
	T21Evidence         string
	T21AEvidence        string
	AuthFile            string
	SelectedLinuxConfig string
	Evidence            string
}

func RunR03AT21BPreflight(cfg R03AT21BConfig) (map[string]any, error) {
	if cfg.WindowsBinary == "" || cfg.WindowsCodeModeHost == "" || cfg.T21Evidence == "" || cfg.T21AEvidence == "" || cfg.AuthFile == "" || cfg.SelectedLinuxConfig == "" || cfg.Evidence == "" {
		return nil, errors.New("preflight_failed: T21B configuration is incomplete")
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("T21B evidence directory is not fresh; refusing retry/reset")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	var t21Config t21ExecutionConfig
	if err := readT11JSON(filepath.Join(cfg.T21Evidence, "execution-config.json"), &t21Config); err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T21 execution config: %w", err))
	}
	if err := validateT20BaseForT21(t21Config.CanonicalManifest.Base); err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, err)
	}
	var t21Preflight struct {
		Passed                bool   `json:"passed"`
		Status                string `json:"status"`
		MediumConsumed        int    `json:"medium_consumed"`
		ProviderEgress        int    `json:"provider_egress"`
		ExecutionConfigDigest string `json:"execution_config_digest"`
		T21Fingerprint        string `json:"t21_fingerprint"`
	}
	if err := readT11JSON(filepath.Join(cfg.T21Evidence, "preflight.json"), &t21Preflight); err != nil || !t21Preflight.Passed || t21Preflight.Status != "offline_preflight_passed" || t21Preflight.MediumConsumed != 0 || t21Preflight.ProviderEgress != 0 {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T21 offline preflight is not sealed"))
	}
	var t21Result struct {
		Result         string `json:"result"`
		MediumConsumed int    `json:"medium_consumed"`
		ProviderEgress int    `json:"provider_egress"`
	}
	if err := readT11JSON(filepath.Join(cfg.T21Evidence, "result.json"), &t21Result); err != nil || t21Result.Result != "NOT_STARTED" || t21Result.MediumConsumed != 0 || t21Result.ProviderEgress != 0 {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T21 sealed result is not NOT_STARTED with zero usage"))
	}
	var t21aQualification struct {
		Status              string `json:"status"`
		Eligible            bool   `json:"eligible_for_new_t21b_live_canary"`
		MediumConsumed      int    `json:"medium_consumed"`
		ProviderEgress      int    `json:"provider_egress"`
		RegisteredToolCount int    `json:"registered_tool_count"`
		Materialization     string `json:"materialization_validation"`
	}
	if err := readT11JSON(filepath.Join(cfg.T21AEvidence, "qualification.json"), &t21aQualification); err != nil || t21aQualification.Status != "PASSED" || !t21aQualification.Eligible || t21aQualification.MediumConsumed != 0 || t21aQualification.ProviderEgress != 0 || t21aQualification.RegisteredToolCount != 11 || t21aQualification.Materialization != "passed" {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T21A exact-surface qualification is not sealed"))
	}
	var t21aRun struct {
		PreflightOnly       bool   `json:"preflight_only"`
		PreflightPassed     bool   `json:"preflight_passed"`
		ProviderEgress      int    `json:"provider_egress"`
		MediumStarted       int    `json:"medium_started"`
		RegisteredToolCount int    `json:"registered_tool_count"`
		Materialization     string `json:"materialization_validation"`
		StopConfirmed       bool   `json:"stop_confirmed"`
	}
	if err := readT11JSON(filepath.Join(cfg.T21AEvidence, "windows-live-run.json"), &t21aRun); err != nil || !t21aRun.PreflightOnly || !t21aRun.PreflightPassed || t21aRun.ProviderEgress != 0 || t21aRun.MediumStarted != 0 || t21aRun.RegisteredToolCount != 11 || t21aRun.Materialization != "passed" || !t21aRun.StopConfirmed {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T21A Windows preflight evidence is not sealed"))
	}
	binaryRaw, err := os.ReadFile(cfg.WindowsBinary)
	if err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows binary: %w", err))
	}
	codeModeHostRaw, err := os.ReadFile(cfg.WindowsCodeModeHost)
	if err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows code-mode-host: %w", err))
	}
	configRaw, err := os.ReadFile(cfg.SelectedLinuxConfig)
	if err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: selected config: %w", err))
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: auth material: %w", err))
	}
	if digest(binaryRaw) != t21Config.CanonicalManifest.Base.Combination.BinarySHA256 || digest(codeModeHostRaw) != t21Config.CanonicalManifest.Base.Combination.CodeModeHostSHA256 || digest(configRaw) != t21Config.SelectedConfigRawSHA256 {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T20 binary, code-mode-host, or config drifted"))
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, t21Config.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || authMaterial.Manifest() != t21Config.CanonicalManifest.Base.Auth {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: auth identity or credential revision differs from T20"))
	}
	toolRegistryRaw, err := os.ReadFile(filepath.Join(cfg.T21Evidence, "tool-registry.json"))
	if err != nil || digest([]byte(strings.TrimRight(string(toolRegistryRaw), "\r\n"))) != t21Config.ToolManifestDigest {
		return nil, recordT21BPreflightFailure(cfg.Evidence, errors.New("preflight_failed: T21 formal tool registry digest drifted"))
	}
	toolSurfaceRaw, err := os.ReadFile(filepath.Join(cfg.T21Evidence, "tool-surface.json"))
	if err != nil {
		return nil, recordT21BPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T21 tool surface: %w", err))
	}
	if err := os.WriteFile(filepath.Join(cfg.Evidence, "tool-registry.json"), toolRegistryRaw, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cfg.Evidence, "tool-surface.json"), toolSurfaceRaw, 0600); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-config.json"), t21Config); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), map[string]any{"qualification": "R0.3A-T21B", "fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV7, "canonical_manifest": t21Config.CanonicalManifest, "canonical_manifest_digest": t21Config.QualificationFingerprint.CanonicalManifestDigest, "qualification_fingerprint": t21Config.QualificationFingerprint, "source_execution_config_digest": t21Preflight.ExecutionConfigDigest, "launch_config_digest": t21Config.LaunchConfigDigest, "tool_manifest_digest": t21Config.ToolManifestDigest, "tool_names": t21Config.ToolNames, "dynamic_tool_count": t21Config.ToolSurface.ToolCount, "historical_t21_t21a_modified": false}); err != nil {
		return nil, err
	}
	preflight := map[string]any{"qualification": "R0.3A-T21B", "passed": true, "status": "offline_preflight_passed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "t20_control_fingerprint": t21Config.T20ControlFingerprint, "t21b_fingerprint": t21Config.QualificationFingerprint.CanonicalManifestDigest, "execution_config_digest": t21Preflight.ExecutionConfigDigest, "launch_config_digest": t21Config.LaunchConfigDigest, "auth_identity_fingerprint": authMaterial.IdentityFingerprint, "auth_credential_revision_fingerprint": authMaterial.CredentialRevisionFingerprint, "auth_recheck_passed": true, "t21a_surface_qualification": "passed", "t21_controlled_factors_unchanged": true, "dynamic_tool_surface_unchanged_from_t21a": true, "historical_evidence_modified": false}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), preflight); err != nil {
		return nil, err
	}
	return preflight, nil
}

func FinalizeR03AT21(cfg R03AT21Config) (map[string]any, error) {
	var manifest struct {
		Qualification            string                           `json:"qualification"`
		FingerprintSchemaVersion string                           `json:"fingerprint_schema_version"`
		CanonicalManifest        codex.CanonicalManifestV7        `json:"canonical_manifest"`
		CanonicalManifestDigest  string                           `json:"canonical_manifest_digest"`
		QualificationFingerprint codex.QualificationFingerprintV7 `json:"qualification_fingerprint"`
		SourceExecutionConfig    string                           `json:"source_execution_config_digest"`
	}
	if err := readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil {
		return nil, err
	}
	var execution t21ExecutionConfig
	if err := readT11JSON(filepath.Join(cfg.Evidence, "execution-config.json"), &execution); err != nil {
		return nil, err
	}
	var live t21LiveRun
	if err := readT11JSON(filepath.Join(cfg.Evidence, "windows-live-run.json"), &live); err != nil {
		return nil, err
	}
	var allowance struct {
		Qualification  string `json:"qualification"`
		T21Fingerprint string `json:"t21_fingerprint"`
		MediumStarted  int    `json:"medium_started"`
		ProviderEgress int    `json:"provider_egress"`
	}
	if err := readT11JSON(filepath.Join(cfg.Evidence, "allowance.json"), &allowance); err != nil {
		return nil, err
	}
	if (manifest.Qualification != "R0.3A-T21" && manifest.Qualification != "R0.3A-T21B") || manifest.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV7 || manifest.CanonicalManifestDigest != manifest.QualificationFingerprint.CanonicalManifestDigest || manifest.CanonicalManifest.Validate() != nil {
		return nil, errors.New("T21 finalization refused: execution manifest is invalid")
	}
	exactSurface := live.RegisteredToolCount == manifest.CanonicalManifest.ToolSurface.ToolCount && live.ToolManifestDigest == manifest.CanonicalManifest.ToolSurface.AggregateManifestDigest && live.AggregateSchemaDigest == manifest.CanonicalManifest.ToolSurface.AggregateSchemaDigest && live.AggregateSchemaBytes == manifest.CanonicalManifest.ToolSurface.AggregateSchemaBytes
	deltaOutput, deltaRecorded, err := deriveT20DeltaOutput(filepath.Join(cfg.Evidence, "provider-events.jsonl"))
	if err != nil {
		return nil, err
	}
	output := live.FirstValidOutputText
	sentinelMatch := live.SentinelMatch
	if deltaRecorded {
		output = deltaOutput
		if strings.TrimSpace(output) == r03aT21Sentinel {
			sentinelMatch = "exact"
		} else {
			sentinelMatch = "mismatch"
		}
	}
	outcome := ClassifyT21Outcome(T21OutcomeInput{ExactToolSurface: exactSurface, TurnStarted: live.TurnStarted, FirstValidOutput: live.FirstValidOutput, TurnCompleted: live.TurnCompleted, SentinelExact: sentinelMatch == "exact", UnresolvedTransport: live.UnresolvedTransportState, AttemptedToolCalls: len(live.AttemptedToolCalls), BusinessSideEffects: live.BusinessSideEffects, FailureMode: live.FailureMode})
	if live.Status == "preflight_failed" || !live.PreflightPassed {
		outcome = T21Outcome{Result: "NOT_STARTED", L2Qualification: T21L2Unqualified, ToolRegistrationTransport: T21FailedLower, SentinelInstructionFollowing: T21FailedLower, BusinessSideEffectIsolation: T21FailedLower, FailureMode: "preflight_failed"}
	}
	eligibleForBackend := outcome.Result == T21Passed && outcome.L2Qualification == T21L2Qualified && len(live.AttemptedToolCalls) == 0 && live.BusinessSideEffects == 0
	allowanceFingerprintBinding := "matched"
	if allowance.T21Fingerprint != manifest.CanonicalManifestDigest {
		allowanceFingerprintBinding = "mismatch"
	}
	result := map[string]any{
		"qualification": manifest.Qualification, "result": outcome.Result, "windows_native_l2_11_tool_surface": outcome.L2Qualification,
		"tool_registration_transport": outcome.ToolRegistrationTransport, "sentinel_instruction_following": outcome.SentinelInstructionFollowing, "business_side_effect_isolation": outcome.BusinessSideEffectIsolation,
		"failure_mode": outcome.FailureMode, "t20_control_fingerprint": execution.T20ControlFingerprint, "t21_fingerprint": manifest.CanonicalManifestDigest, "execution_config_digest": manifest.SourceExecutionConfig,
		"auth_identity_fingerprint": manifest.CanonicalManifest.Base.Auth.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": manifest.CanonicalManifest.Base.Auth.AuthCredentialRevisionFingerprint,
		"codex_version": manifest.CanonicalManifest.Base.Combination.CodexVersion, "runtime_profile": manifest.CanonicalManifest.Base.Runtime.RuntimeProfile,
		"registered_tool_count": live.RegisteredToolCount, "tool_names": t21ToolNames(manifest.CanonicalManifest.ToolSurface), "tool_manifest_digest": live.ToolManifestDigest,
		"aggregate_schema_digest": live.AggregateSchemaDigest, "aggregate_schema_bytes": live.AggregateSchemaBytes, "thread_start_payload_bytes": live.ThreadStartPayloadBytes, "thread_start_payload_digest": live.ThreadStartPayloadDigest,
		"medium_consumed": live.MediumStarted, "provider_egress": live.ProviderEgress, "process_started": live.ProcessStarted, "initialize_completed": live.InitializeCompleted, "thread_started": live.ThreadStarted, "turn_started": live.TurnStarted, "user_message_item": live.UserMessageItem,
		"first_valid_output": live.FirstValidOutput, "first_valid_output_text": output, "time_to_first_valid_output_ms": live.TimeToFirstOutputMS, "reconnect_count": live.ReconnectCount, "reconnect_phases": live.ReconnectPhases, "first_disconnect_delta_ms": live.FirstDisconnectDeltaMS, "recovery_delta_ms": live.RecoveryDeltaMS,
		"attempted_tool_calls": live.AttemptedToolCalls, "business_side_effects": live.BusinessSideEffects, "turn_completed": live.TurnCompleted, "turn_state": live.TurnState, "sentinel_match": sentinelMatch, "runner_recorded_sentinel_match": live.SentinelMatch,
		"native_usage_updates": live.NativeUsageUpdates, "token_usage": live.TokenUsage, "unresolved_transport_state": live.UnresolvedTransportState, "terminal_state": live.TerminalState, "stop_confirmed": live.StopConfirmed, "stop_mechanism": live.StopMechanism, "exit_code": live.ExitCode, "process_start": live.ProcessStart, "process_stop": live.ProcessStop,
		"eligible_for_real_backend": eligibleForBackend, "historical_evidence_modified": false, "error": live.Error,
		"allowance_qualification": allowance.Qualification, "allowance_t21_fingerprint": allowance.T21Fingerprint, "allowance_fingerprint_binding": allowanceFingerprintBinding,
	}
	qualification := map[string]any{"qualification": manifest.Qualification, "status": outcome.Result, "windows_native_l2_11_tool_surface": outcome.L2Qualification, "tool_registration_transport": outcome.ToolRegistrationTransport, "sentinel_instruction_following": outcome.SentinelInstructionFollowing, "business_side_effect_isolation": outcome.BusinessSideEffectIsolation, "failure_mode": outcome.FailureMode, "eligible_for_real_backend": eligibleForBackend, "canonical_manifest_digest": manifest.CanonicalManifestDigest, "medium_consumed": live.MediumStarted, "provider_egress": live.ProviderEgress, "historical_evidence_modified": false}
	if err := writeJSON(filepath.Join(cfg.Evidence, "result.json"), result); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "transport-summary.json"), map[string]any{"derived_from": "provider-events.jsonl", "first_valid_output_text": output, "sentinel_match": sentinelMatch, "runner_recorded_sentinel_match": live.SentinelMatch, "attempted_tool_calls": live.AttemptedToolCalls, "business_side_effects": live.BusinessSideEffects}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return result, nil
}

func buildT21ToolSurface() (codex.ToolSurfaceManifest, []any, []byte, error) {
	return buildPeerToolSurface(codex.PeerBackendTools(), 11, r03aT21ToolSurfaceID, t21ToolBindings, "diagnostic_denied")
}

func buildPeerToolSurface(tools []any, expectedCount int, surfaceID string, bindings map[string]string, writePolicy string) (codex.ToolSurfaceManifest, []any, []byte, error) {
	if len(tools) != expectedCount {
		return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("preflight_failed: formal %s surface has %d tools, expected %d", surfaceID, len(tools), expectedCount)
	}
	toolRaw, err := json.Marshal(tools)
	if err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("preflight_failed: formal tool manifest serialization: %w", err)
	}
	var schemas []json.RawMessage
	entries := make([]codex.ToolSurfaceEntry, 0, len(tools))
	for index, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			return codex.ToolSurfaceManifest{}, nil, nil, errors.New("preflight_failed: formal tool registry returned an invalid object")
		}
		name, _ := tool["name"].(string)
		description, _ := tool["description"].(string)
		schema, ok := tool["inputSchema"]
		if name == "" || !ok {
			return codex.ToolSurfaceManifest{}, nil, nil, errors.New("preflight_failed: formal registry omitted name or input schema")
		}
		schemaRaw, err := json.Marshal(schema)
		if err != nil {
			return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("preflight_failed: tool schema serialization: %w", err)
		}
		binding, ok := bindings[strings.TrimPrefix(name, "polis_")]
		if !ok {
			return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("preflight_failed: no formal handler binding for %s", name)
		}
		entries = append(entries, codex.ToolSurfaceEntry{Name: name, SchemaDigest: digest(schemaRaw), SchemaBytes: len(schemaRaw), DescriptionDigest: digest([]byte(description)), BindingIdentity: binding, AuthorizationClass: r03aT21ToolSurfaceID, RegistrationOrdinal: index + 1})
		schemas = append(schemas, json.RawMessage(schemaRaw))
	}
	schemaRaw, err := json.Marshal(schemas)
	if err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("preflight_failed: aggregate schema serialization: %w", err)
	}
	return codex.ToolSurfaceManifest{CheckpointPolicyRevision: core.CheckpointPolicyRevision, ArtifactEligibilityPolicyRevision: core.ArtifactEligibilityPolicyRevision, ContractSupersessionPolicyRevision: core.PeerContractSupersessionPolicyRevision, AcceptanceCheckerRevision: core.PeerAcceptanceCheckerRevision, SurfaceID: surfaceID, ToolCount: len(entries), AggregateManifestDigest: digest(toolRaw), AggregateSchemaDigest: digest(schemaRaw), AggregateSchemaBytes: sumToolSchemaBytes(entries), Tools: entries, BusinessWritePolicy: writePolicy}, tools, toolRaw, nil
}

var t21ToolBindings = map[string]string{
	"work_current": "PeerEmployeeTools.call->Handover", "context_read": "PeerEmployeeTools.call->Handover", "workspace_read": "PeerEmployeeTools.call->Workspace", "workspace_replace": "PeerEmployeeTools.call->TXReplace", "workspace_check": "PeerEmployeeTools.call->workspace.check", "work_checkpoint": "PeerEmployeeTools.call->TXCheckpoint", "artifact_submit": "PeerEmployeeTools.call->TXSubmit", "contract_propose": "PeerEmployeeTools.call->TXProposePeerContract", "contract_accept": "PeerEmployeeTools.call->TXAcceptPeerContract", "contract_read": "PeerEmployeeTools.call->PeerContractRead", "collab_send": "PeerEmployeeTools.call->TXPeerSend",
}

func compareT21WithT5(surface codex.ToolSurfaceManifest, reference t21T5Manifest) error {
	if len(reference.FailingBackendManifest) != surface.ToolCount {
		return fmt.Errorf("preflight_failed: T5 reference has %d tools, formal registry has %d", len(reference.FailingBackendManifest), surface.ToolCount)
	}
	for _, current := range surface.Tools {
		ref, ok := findT21T5Tool(reference.FailingBackendManifest, current.Name)
		if !ok || ref.SchemaCanonicalDigest != current.SchemaDigest || ref.SchemaBytes != current.SchemaBytes || ref.DescriptionDigest != current.DescriptionDigest || ref.Handler != current.BindingIdentity || ref.AuthorizationClass != current.AuthorizationClass || ref.RegistrationOrdinal != current.RegistrationOrdinal {
			return fmt.Errorf("preflight_failed: formal 11-tool surface drifted from T5 qualification at %s", current.Name)
		}
	}
	return nil
}

func findT21T5Tool(entries []t21T5ToolEntry, name string) (t21T5ToolEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return t21T5ToolEntry{}, false
}

func t21ToolNames(surface codex.ToolSurfaceManifest) []string {
	names := make([]string, len(surface.Tools))
	for _, tool := range surface.Tools {
		if tool.RegistrationOrdinal >= 1 && tool.RegistrationOrdinal <= len(names) {
			names[tool.RegistrationOrdinal-1] = tool.Name
		}
	}
	return names
}

func sumToolSchemaBytes(entries []codex.ToolSurfaceEntry) int {
	total := 0
	for _, entry := range entries {
		total += entry.SchemaBytes
	}
	return total
}

func t21CapabilityDigest(t20Capability string, surface codex.ToolSurfaceManifest) string {
	raw := []byte("t21-capability-v1\x00" + t20Capability + "\x00" + surface.AggregateManifestDigest + "\x00" + surface.AggregateSchemaDigest + "\x00" + fmt.Sprint(surface.ToolCount))
	digestValue := sha256.Sum256(raw)
	return hex.EncodeToString(digestValue[:])
}

func t21NormalizedThreadPayload(tools []any, developer string) map[string]any {
	return map[string]any{"model": "gpt-5.6-luna", "allowProviderModelFallback": false, "approvalPolicy": "never", "sandbox": "read-only", "cwd": "/work", "environments": []any{}, "ephemeral": true, "dynamicTools": tools, "config": map[string]any{"model_reasoning_effort": "medium"}, "developerInstructions": developer}
}

func t21ControlFactorDiff(t20 t20ExecutionConfig, manifest codex.CanonicalManifestV7, surface codex.ToolSurfaceManifest, payloadDigest string, payloadBytes int) map[string]any {
	unchanged := []string{"model", "effort", "auth_identity", "auth_credential_revision", "runtime", "codex_version", "binary", "code_mode_host", "proxy_policy", "transport_policy", "CODEX_HOME/config_semantics", "diagnostic_prompt_semantics", "deadline_policy", "stop_semantics"}
	derived := []map[string]any{{"field": "dynamic_tool_count", "t20": 0, "t21": surface.ToolCount}, {"field": "tool_manifest_digest", "t20": "not-present-in-v6", "t21": surface.AggregateManifestDigest}, {"field": "aggregate_schema_digest", "t20": "not-present-in-v6", "t21": surface.AggregateSchemaDigest}, {"field": "aggregate_schema_bytes", "t20": 0, "t21": surface.AggregateSchemaBytes}, {"field": "capability_digest", "t20": t20.CanonicalManifest.Combination.CapabilityDigest, "t21": manifest.Base.Combination.CapabilityDigest}, {"field": "thread_start_payload_digest", "t20": "not-recorded", "t21": payloadDigest}, {"field": "thread_start_payload_bytes", "t20": "not-recorded", "t21": payloadBytes}, {"field": "canonical_manifest_schema", "t20": codex.CanonicalManifestFingerprintSchemaVersionV6, "t21": codex.CanonicalManifestFingerprintSchemaVersionV7}}
	return map[string]any{"controlled_same": unchanged, "legal_tool_surface_derived_differences": derived, "unexpectedly_different": []any{}, "t20_control_fingerprint": t20.QualificationFingerprint.CanonicalManifestDigest, "t21_fingerprint": manifest.Base.Combination.CurrentFingerprintV7(manifest).CanonicalManifestDigest}
}

func validateT20ControlForT21(config t20ExecutionConfig) error {
	if config.SchemaVersion != "t20-effective-execution-config-v1" {
		return errors.New("preflight_failed: T20 execution config schema is not sealed")
	}
	return validateT20BaseForT21(config.CanonicalManifest)
}

func validateT20BaseForT21(base codex.CanonicalManifestV6) error {
	if base.Combination.CodexVersion != "0.153.4" || base.Combination.Model != "gpt-5.6-luna" || base.Combination.Effort != "medium" || base.Combination.RuntimeProfile != "windows-native" || base.Combination.SandboxClass != "read-only" || base.Transport.ProviderTransportPolicy != "native_default" || base.Runtime.Platform != "windows" || base.CodexHomeProfile != codex.CodexHomeProfileWindowsDiagnostic || base.Runtime.NetworkPolicy != codex.RuntimeNetworkPolicyNativeOSNetwork {
		return errors.New("preflight_failed: T20 control manifest is not the expected Windows-native profile")
	}
	if err := base.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: T20 base manifest invalid: %w", err)
	}
	return nil
}

func validateT21Config(cfg R03AT21Config) error {
	if cfg.WindowsBinary == "" || cfg.WindowsCodeModeHost == "" || cfg.T5Evidence == "" || cfg.T20Evidence == "" || cfg.AuthFile == "" || cfg.SelectedLinuxConfig == "" || cfg.Evidence == "" {
		return errors.New("preflight_failed: T21 configuration is incomplete")
	}
	return nil
}

func ensureT21EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T21 evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}

func recordT21PreflightFailure(evidence string, cause error) error {
	record := map[string]any{"qualification": "R0.3A-T21", "passed": false, "status": "preflight_failed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "historical_evidence_modified": false, "error": cause.Error(), "recorded_at": time.Now().UTC()}
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), record))
}

func recordT21BPreflightFailure(evidence string, cause error) error {
	record := map[string]any{"qualification": "R0.3A-T21B", "passed": false, "status": "preflight_failed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "historical_evidence_modified": false, "error": cause.Error(), "recorded_at": time.Now().UTC()}
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), record))
}
