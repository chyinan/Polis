// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
)

const (
	r03aT20Version            = "0.153.4"
	r03aT20Model              = "gpt-5.6-luna"
	r03aT20Effort             = "medium"
	r03aT20Sentinel           = "POLIS_TRANSPORT_CANARY_OK"
	r03aT20Prompt             = "Reply with exactly: POLIS_TRANSPORT_CANARY_OK"
	r03aT20DiagnosticProfile  = "windows_native_isolated"
	r03aT20EvidenceSchema     = "t20-windows-live-run-v1"
	r03aT20ExecutionConfigV1  = "t20-effective-execution-config-v1"
	r03aT20LaunchBindingV1    = "t20-launch-binding-v1"
	r03aT20ExpectedT19Status  = "PASSED"
	r03aT20ExpectedT19Outcome = "PASSED"
)

type R03AT20Config struct {
	WindowsBinary       string
	WindowsCodeModeHost string
	T19Evidence         string
	T17Evidence         string
	AuthFile            string
	SelectedLinuxConfig string
	Evidence            string
}

type t20T19Qualification struct {
	Status                     string                           `json:"status"`
	OfflineQualification       string                           `json:"offline_qualification"`
	Eligible                   bool                             `json:"eligible_for_windows_native_minimal_live_canary"`
	MediumAllowanceCreated     bool                             `json:"medium_allowance_created"`
	ProviderEgress             int                              `json:"provider_egress"`
	WindowsManifest            codex.CanonicalManifestV6        `json:"windows_manifest"`
	WindowsFingerprint         codex.QualificationFingerprintV6 `json:"windows_fingerprint"`
	HistoricalEvidenceModified bool                             `json:"historical_t18_t14c_t16_t17_modified"`
}

type t20T19NormalizedManifest struct {
	Windows            codex.CanonicalManifestV6        `json:"windows"`
	WindowsFingerprint codex.QualificationFingerprintV6 `json:"windows_fingerprint"`
}

type t20LaunchBinding struct {
	SchemaVersion      string   `json:"schema_version"`
	ExecutableSHA256   string   `json:"executable_sha256"`
	CodeModeHostSHA256 string   `json:"code_mode_host_sha256"`
	Invocation         []string `json:"invocation"`
	WorkingDirectory   string   `json:"working_directory_role"`
	HomeDirectory      string   `json:"home_directory_role"`
	CodexHomeDirectory string   `json:"codex_home_directory_role"`
	AuthSourceRole     string   `json:"auth_source_role"`
	ConfigSourceRole   string   `json:"config_source_role"`
	ProxyEnvironment   string   `json:"proxy_environment"`
	UnsetProxyNames    []string `json:"unset_proxy_names"`
	TransportPolicy    string   `json:"transport_policy"`
	Model              string   `json:"model"`
	Effort             string   `json:"effort"`
	Sandbox            string   `json:"sandbox"`
	DynamicToolCount   int      `json:"dynamic_tool_count"`
	DiagnosticProfile  string   `json:"diagnostic_profile"`
}

type t20ExecutionConfig struct {
	SchemaVersion                     string                           `json:"schema_version"`
	CanonicalManifest                 codex.CanonicalManifestV6        `json:"canonical_manifest"`
	QualificationFingerprint          codex.QualificationFingerprintV6 `json:"qualification_fingerprint"`
	AuthSnapshotRequired              bool                             `json:"auth_snapshot_required"`
	AuthSourceRole                    string                           `json:"auth_source_role"`
	AuthIdentityFingerprint           string                           `json:"auth_identity_fingerprint"`
	AuthCredentialRevisionFingerprint string                           `json:"auth_credential_revision_fingerprint"`
	SelectedConfigRawSHA256           string                           `json:"selected_config_raw_sha256"`
	PromptDigest                      string                           `json:"prompt_digest"`
	DeveloperInstructionDigest        string                           `json:"developer_instruction_digest"`
	LaunchBinding                     t20LaunchBinding                 `json:"launch_binding"`
	LaunchBindingCanonicalJSON        string                           `json:"launch_binding_canonical_json"`
	LaunchConfigDigest                string                           `json:"launch_config_digest"`
}

type t20LiveRun struct {
	SchemaVersion              string   `json:"schema_version"`
	Status                     string   `json:"status"`
	PreflightPassed            bool     `json:"preflight_passed"`
	ProviderOrInternetAccessed bool     `json:"provider_or_internet_accessed"`
	MediumStarted              int      `json:"medium_started"`
	ProviderEgress             int      `json:"provider_egress"`
	ProcessStarted             bool     `json:"process_started"`
	InitializeCompleted        bool     `json:"initialize_completed"`
	ThreadStarted              bool     `json:"thread_started"`
	TurnStarted                bool     `json:"turn_started"`
	UserMessageItem            bool     `json:"user_message_item"`
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

func RunR03AT20Preflight(cfg R03AT20Config) (map[string]any, error) {
	if err := validateT20Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT20EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}

	var t19Qualification t20T19Qualification
	if err := readT11JSON(filepath.Join(cfg.T19Evidence, "qualification.json"), &t19Qualification); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 qualification: %w", err))
	}
	var t19Manifest t20T19NormalizedManifest
	if err := readT11JSON(filepath.Join(cfg.T19Evidence, "normalized-manifest.json"), &t19Manifest); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 normalized manifest: %w", err))
	}
	var t19Run t19WindowsRun
	if err := readT11JSON(filepath.Join(cfg.T19Evidence, "windows-run.json"), &t19Run); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 Windows runner evidence: %w", err))
	}
	if err := t19Qualification.WindowsManifest.Validate(); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 Windows manifest: %w", err))
	}
	if err := t19Manifest.Windows.Validate(); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 normalized Windows manifest: %w", err))
	}
	if err := t19Qualification.WindowsFingerprint.Validate(); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: T19 Windows fingerprint: %w", err))
	}
	if t19Qualification.Status != r03aT20ExpectedT19Status || t19Qualification.OfflineQualification != r03aT20ExpectedT19Outcome || !t19Qualification.Eligible || t19Qualification.MediumAllowanceCreated || t19Qualification.ProviderEgress != 0 || t19Qualification.HistoricalEvidenceModified {
		return nil, recordT20PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T19 offline eligibility is not sealed"))
	}
	if t19Qualification.WindowsFingerprint != t19Manifest.WindowsFingerprint {
		return nil, recordT20PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T19 Windows fingerprint records disagree"))
	}
	if t19Qualification.WindowsManifest != t19Manifest.Windows {
		return nil, recordT20PreflightFailure(cfg.Evidence, errors.New("preflight_failed: T19 Windows manifest records disagree"))
	}
	if err := validateT20WindowsManifest(t19Qualification.WindowsManifest, t19Qualification.WindowsFingerprint); err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, err)
	}

	binaryRaw, err := os.ReadFile(cfg.WindowsBinary)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows Codex binary: %w", err))
	}
	codeModeHostRaw, err := os.ReadFile(cfg.WindowsCodeModeHost)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: Windows code-mode-host: %w", err))
	}
	selectedConfigRaw, err := os.ReadFile(cfg.SelectedLinuxConfig)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: selected diagnostic config: %w", err))
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: controlled auth material: %w", err))
	}
	binarySHA := digest(binaryRaw)
	codeModeHostSHA := digest(codeModeHostRaw)
	selectedConfigSHA := digest(selectedConfigRaw)
	if binarySHA != t19Qualification.WindowsManifest.Combination.BinarySHA256 || codeModeHostSHA != t19Qualification.WindowsManifest.Combination.CodeModeHostSHA256 || selectedConfigSHA != t19Run.SelectedConfigRawSHA256 {
		return nil, recordT20PreflightFailure(cfg.Evidence, errors.New("preflight_failed: Windows binary, code-mode-host, or selected config drifted from T19"))
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, t19Qualification.WindowsManifest.Auth.AuthSourceClass)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: current auth parse: %w", err))
	}
	if authMaterial.Manifest() != t19Qualification.WindowsManifest.Auth {
		return nil, recordT20PreflightFailure(cfg.Evidence, errors.New("preflight_failed: current auth identity or credential revision differs from T19"))
	}

	promptDigest := digest([]byte(r03aT20Prompt))
	expectedFactors := t20FactorsFromT19(t19Qualification.WindowsManifest, t19Run, promptDigest)
	actualFactors := expectedFactors
	actualFactors.BinarySHA256 = binarySHA
	actualFactors.CodeModeHostSHA256 = codeModeHostSHA
	actualFactors.AuthSourceClass = authMaterial.SourceClass
	actualFactors.AuthIdentityFingerprint = authMaterial.IdentityFingerprint
	actualFactors.AuthCredentialRevisionFingerprint = authMaterial.CredentialRevisionFingerprint
	factorComparison := CompareT20ControlledFactors(expectedFactors, actualFactors)
	if !factorComparison.Passed {
		return nil, recordT20PreflightFailureWithDetails(cfg.Evidence, errors.New("preflight_failed: T20 controlled factor drift"), factorComparison)
	}

	launchBinding := t20LaunchBinding{
		SchemaVersion:      r03aT20LaunchBindingV1,
		ExecutableSHA256:   binarySHA,
		CodeModeHostSHA256: codeModeHostSHA,
		Invocation:         []string{"app-server", "--stdio"},
		WorkingDirectory:   "external_temporary_diagnostic_workspace",
		HomeDirectory:      "external_temporary_diagnostic_home",
		CodexHomeDirectory: "external_temporary_diagnostic_home",
		AuthSourceRole:     "read_only_external_auth_snapshot",
		ConfigSourceRole:   "exact_selected_non_secret_T14C_config_snapshot",
		ProxyEnvironment:   "no_injected_proxy",
		UnsetProxyNames:    []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"},
		TransportPolicy:    "native_default",
		Model:              r03aT20Model,
		Effort:             r03aT20Effort,
		Sandbox:            "read-only",
		DynamicToolCount:   0,
		DiagnosticProfile:  r03aT20DiagnosticProfile,
	}
	launchRaw, err := json.Marshal(launchBinding)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: launch binding serialization: %w", err))
	}
	launchDigest := digest(launchRaw)
	executionConfig := t20ExecutionConfig{
		SchemaVersion:                     r03aT20ExecutionConfigV1,
		CanonicalManifest:                 t19Qualification.WindowsManifest,
		QualificationFingerprint:          t19Qualification.WindowsFingerprint,
		AuthSnapshotRequired:              true,
		AuthSourceRole:                    "read_only_external_auth_snapshot; identity/revision only in evidence",
		AuthIdentityFingerprint:           authMaterial.IdentityFingerprint,
		AuthCredentialRevisionFingerprint: authMaterial.CredentialRevisionFingerprint,
		SelectedConfigRawSHA256:           selectedConfigSHA,
		PromptDigest:                      promptDigest,
		DeveloperInstructionDigest:        promptDigest,
		LaunchBinding:                     launchBinding,
		LaunchBindingCanonicalJSON:        string(launchRaw),
		LaunchConfigDigest:                launchDigest,
	}
	executionRaw, err := json.Marshal(executionConfig)
	if err != nil {
		return nil, recordT20PreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: execution config serialization: %w", err))
	}
	executionDigest := digest(executionRaw)
	manifestDigest := t19Qualification.WindowsFingerprint.CanonicalManifestDigest
	preflight := map[string]any{
		"passed":                         true,
		"status":                         "offline_preflight_passed",
		"qualification":                  "R0.3A-T20",
		"medium_consumed":                0,
		"high_consumed":                  0,
		"provider_egress":                0,
		"t19_control_fingerprint":        manifestDigest,
		"t20_windows_v6_fingerprint":     manifestDigest,
		"execution_config_digest":        executionDigest,
		"launch_config_digest":           launchDigest,
		"launch_binding_digest_verified": true,
		"auth_recheck_passed":            true,
		"auth_snapshot_required":         true,
		"binary_recheck_passed":          true,
		"selected_config_recheck_passed": true,
		"controlled_factor_comparison":   factorComparison,
		"historical_evidence_modified":   false,
	}
	manifestRecord := map[string]any{
		"qualification":                        "R0.3A-T20",
		"fingerprint_schema_version":           codex.CanonicalManifestFingerprintSchemaVersionV6,
		"canonical_manifest":                   t19Qualification.WindowsManifest,
		"canonical_manifest_digest":            manifestDigest,
		"qualification_fingerprint":            t19Qualification.WindowsFingerprint,
		"source_execution_config_digest":       executionDigest,
		"derivation_schema_version":            r03aT20ExecutionConfigV1,
		"launch_config_digest":                 launchDigest,
		"launch_binding_canonical_json":        string(launchRaw),
		"auth_identity_fingerprint":            authMaterial.IdentityFingerprint,
		"auth_credential_revision_fingerprint": authMaterial.CredentialRevisionFingerprint,
		"auth_snapshot_used":                   true,
		"medium_consumed":                      0,
		"provider_egress":                      0,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-config.json"), executionConfig); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifestRecord); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "factor-diff.json"), factorComparison); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), preflight); err != nil {
		return nil, err
	}
	return preflight, nil
}

func FinalizeR03AT20(cfg R03AT20Config) (map[string]any, error) {
	var manifestRecord struct {
		Qualification               string                           `json:"qualification"`
		FingerprintSchemaVersion    string                           `json:"fingerprint_schema_version"`
		CanonicalManifest           codex.CanonicalManifestV6        `json:"canonical_manifest"`
		CanonicalManifestDigest     string                           `json:"canonical_manifest_digest"`
		QualificationFingerprint    codex.QualificationFingerprintV6 `json:"qualification_fingerprint"`
		SourceExecutionConfigDigest string                           `json:"source_execution_config_digest"`
	}
	if err := readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifestRecord); err != nil {
		return nil, err
	}
	var live t20LiveRun
	if err := readT11JSON(filepath.Join(cfg.Evidence, "windows-live-run.json"), &live); err != nil {
		return nil, err
	}
	if manifestRecord.Qualification != "R0.3A-T20" || manifestRecord.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV6 || manifestRecord.CanonicalManifestDigest != manifestRecord.QualificationFingerprint.CanonicalManifestDigest {
		return nil, errors.New("T20 finalization refused: execution manifest is invalid")
	}
	if live.ProviderOrInternetAccessed && live.ProviderEgress == 0 {
		return nil, errors.New("T20 finalization refused: live provider evidence is inconsistent")
	}
	deltaOutput, deltaOutputRecorded, err := deriveT20DeltaOutput(filepath.Join(cfg.Evidence, "provider-events.jsonl"))
	if err != nil {
		return nil, err
	}
	normalizedOutput := live.FirstValidOutputText
	sentinelMatch := live.SentinelMatch
	if deltaOutputRecorded {
		normalizedOutput = deltaOutput
		if strings.TrimSpace(deltaOutput) == r03aT20Sentinel {
			sentinelMatch = "exact"
		} else {
			sentinelMatch = "mismatch"
		}
	}
	input := T20OutcomeInput{TurnStarted: live.TurnStarted, FirstValidOutput: live.FirstValidOutput, TurnCompleted: live.TurnCompleted, ReconnectCount: live.ReconnectCount, UnresolvedTransport: live.UnresolvedTransportState, FailureMode: live.FailureMode}
	outcome := ClassifyT20Outcome(input)
	if live.Status == "preflight_failed" || !live.PreflightPassed {
		outcome = T20Outcome{Result: "NOT_STARTED", L1Qualification: "unqualified_for_business_execution", FailureMode: "preflight_failed", RuntimeFinding: "NOT_ESTABLISHED"}
	}
	result := map[string]any{
		"qualification":                              "R0.3A-T20",
		"result":                                     outcome.Result,
		"windows_native_l1":                          outcome.L1Qualification,
		"failure_mode":                               outcome.FailureMode,
		"runtime_execution_envelope_factor":          outcome.RuntimeFinding,
		"canonical_manifest_digest":                  manifestRecord.CanonicalManifestDigest,
		"execution_config_digest":                    manifestRecord.SourceExecutionConfigDigest,
		"auth_identity_fingerprint":                  manifestRecord.CanonicalManifest.Auth.AuthIdentityFingerprint,
		"auth_credential_revision_fingerprint":       manifestRecord.CanonicalManifest.Auth.AuthCredentialRevisionFingerprint,
		"medium_consumed":                            live.MediumStarted,
		"provider_egress":                            live.ProviderEgress,
		"process_started":                            live.ProcessStarted,
		"initialize_completed":                       live.InitializeCompleted,
		"thread_started":                             live.ThreadStarted,
		"turn_started":                               live.TurnStarted,
		"user_message_item":                          live.UserMessageItem,
		"first_valid_output":                         live.FirstValidOutput,
		"first_valid_output_text":                    normalizedOutput,
		"time_to_first_valid_output_ms":              live.TimeToFirstOutputMS,
		"reconnect_count":                            live.ReconnectCount,
		"reconnect_phases":                           live.ReconnectPhases,
		"first_disconnect_delta_ms":                  live.FirstDisconnectDeltaMS,
		"recovery_delta_ms":                          live.RecoveryDeltaMS,
		"native_usage_updates":                       live.NativeUsageUpdates,
		"token_usage":                                live.TokenUsage,
		"turn_state":                                 live.TurnState,
		"turn_completed":                             live.TurnCompleted,
		"sentinel_match":                             sentinelMatch,
		"runner_recorded_sentinel_match":             live.SentinelMatch,
		"sentinel_summary_derived_from_delta_events": deltaOutputRecorded,
		"unresolved_transport_state":                 live.UnresolvedTransportState,
		"terminal_state":                             live.TerminalState,
		"stop_confirmed":                             live.StopConfirmed,
		"stop_mechanism":                             live.StopMechanism,
		"exit_code":                                  live.ExitCode,
		"process_start":                              live.ProcessStart,
		"process_stop":                               live.ProcessStop,
		"historical_evidence_modified":               false,
		"error":                                      live.Error,
	}
	qualification := map[string]any{
		"qualification":                     "R0.3A-T20",
		"status":                            outcome.Result,
		"windows_native_l1":                 outcome.L1Qualification,
		"evidence_result":                   outcome.Result,
		"canonical_manifest_digest":         manifestRecord.CanonicalManifestDigest,
		"runtime_execution_envelope_factor": outcome.RuntimeFinding,
		"historical_evidence_modified":      false,
		"medium_allowance_created":          live.MediumStarted == 1,
		"medium_consumed":                   live.MediumStarted,
		"provider_egress":                   live.ProviderEgress,
		"failure_mode":                      outcome.FailureMode,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "result.json"), result); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "transport-summary.json"), map[string]any{
		"derived_from":                          "provider-events.jsonl",
		"first_valid_output_text":               normalizedOutput,
		"sentinel_match":                        sentinelMatch,
		"runner_recorded_sentinel_match":        live.SentinelMatch,
		"duplicate_item_completed_text_ignored": deltaOutputRecorded && live.SentinelMatch != sentinelMatch,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return result, nil
}

func deriveT20DeltaOutput(path string) (string, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var output []string
	for _, line := range strings.Split(string(raw), "\n") {
		var event struct {
			Method string `json:"method"`
			Text   string `json:"text"`
		}
		if json.Unmarshal([]byte(line), &event) != nil || event.Method != "item/agentMessage/delta" {
			continue
		}
		output = append(output, event.Text)
	}
	if len(output) == 0 {
		return "", false, nil
	}
	return strings.Join(output, ""), true, nil
}

func t20FactorsFromT19(manifest codex.CanonicalManifestV6, run t19WindowsRun, promptDigest string) T20ControlledFactors {
	return T20ControlledFactors{
		CodexVersion:                      manifest.Combination.CodexVersion,
		BinarySHA256:                      manifest.Combination.BinarySHA256,
		CodeModeHostSHA256:                manifest.Combination.CodeModeHostSHA256,
		Model:                             manifest.Combination.Model,
		Effort:                            manifest.Combination.Effort,
		EffectiveConfigDigest:             manifest.EffectiveConfigDigest,
		EffectiveTransportConfigDigest:    manifest.EffectiveTransportConfigDigest,
		AuthSourceClass:                   manifest.Auth.AuthSourceClass,
		AuthIdentityFingerprint:           manifest.Auth.AuthIdentityFingerprint,
		AuthCredentialRevisionFingerprint: manifest.Auth.AuthCredentialRevisionFingerprint,
		ProxyPolicy:                       run.ProxyPolicy,
		ProviderTransportPolicy:           manifest.Transport.ProviderTransportPolicy,
		DynamicToolCount:                  fmt.Sprint(run.DynamicToolCount),
		SandboxProfile:                    manifest.Combination.SandboxClass,
		RuntimeProfile:                    manifest.Runtime.RuntimeProfile,
		CodexHomeProfile:                  string(manifest.CodexHomeProfile),
		CWDRole:                           manifest.Runtime.CWDRole,
		PromptDigest:                      run.PromptDigest,
		DeveloperInstructionDigest:        run.DeveloperInstructionDigest,
		NativeProtocolSchemaDigest:        manifest.Combination.NativeProtocolDigest,
		CapabilityDigest:                  manifest.Combination.CapabilityDigest,
	}
}

func validateT20WindowsManifest(manifest codex.CanonicalManifestV6, fingerprint codex.QualificationFingerprintV6) error {
	if manifest.Combination.CodexVersion != r03aT20Version || manifest.Combination.Model != r03aT20Model || manifest.Combination.Effort != r03aT20Effort || manifest.Combination.RuntimeProfile != "windows-native" || manifest.Combination.SandboxClass != "read-only" || manifest.Transport.ProviderTransportPolicy != "native_default" || manifest.Transport.WebSocketPolicy != "native_default" || manifest.Runtime.Platform != "windows" || manifest.Runtime.RuntimeProfile != "windows-native" || manifest.Runtime.NetworkPolicy != codex.RuntimeNetworkPolicyNativeOSNetwork || manifest.Runtime.CWDRole != "diagnostic_workspace" || manifest.Runtime.LaunchMechanism != "CreateProcess" || manifest.Runtime.StdioMode != "redirected_standard_pipes" || manifest.CodexHomeProfile != codex.CodexHomeProfileWindowsDiagnostic || manifest.EffectiveConfigDigest == "" || manifest.EffectiveTransportConfigDigest == "" {
		return errors.New("preflight_failed: T19 Windows manifest is not the T20-controlled profile")
	}
	if fingerprint.CanonicalManifestDigest == "" {
		return errors.New("preflight_failed: T19 Windows fingerprint is empty")
	}
	return nil
}

func validateT20Config(cfg R03AT20Config) error {
	if cfg.WindowsBinary == "" || cfg.WindowsCodeModeHost == "" || cfg.T19Evidence == "" || cfg.T17Evidence == "" || cfg.AuthFile == "" || cfg.SelectedLinuxConfig == "" || cfg.Evidence == "" {
		return errors.New("preflight_failed: T20 configuration is incomplete")
	}
	return nil
}

func ensureT20EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T20 evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}

func recordT20PreflightFailure(evidence string, cause error) error {
	return recordT20PreflightFailureWithDetails(evidence, cause, T20FactorComparison{Passed: false})
}

func recordT20PreflightFailureWithDetails(evidence string, cause error, comparison T20FactorComparison) error {
	record := map[string]any{"qualification": "R0.3A-T20", "passed": false, "status": "preflight_failed", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "controlled_factor_comparison": comparison, "historical_evidence_modified": false, "error": cause.Error(), "recorded_at": time.Now().UTC()}
	writeErr := writeJSON(filepath.Join(evidence, "preflight.json"), record)
	return errors.Join(cause, writeErr)
}
