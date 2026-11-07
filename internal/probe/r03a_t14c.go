// pattern: Imperative Shell
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

const r03aT14CVersion = "0.153.4"

type R03AT14CConfig struct {
	Config
	Version         string
	ProblemKey      string
	AuthSourceClass string
	T13Evidence     string
	T14AEvidence    string
	T10Evidence     string
}

type t14CPlan struct {
	ControlConfig            T14AEffectiveExecutionConfig
	CandidateConfig          T14AEffectiveExecutionConfig
	ControlArtifacts         T14ADerivedArtifacts
	CandidateArtifacts       T14ADerivedArtifacts
	FactorProof              T14FactorProof
	T13V2Fingerprint         string
	T13ControlV3Fingerprint  string
	T13HostGuestMappingProof map[string]any
	BindReport               t14CBindReport
}

type t14CBindReport struct {
	Passed                  bool           `json:"passed"`
	HostHomePath            string         `json:"host_home_path"`
	GuestHomePath           string         `json:"guest_home_path"`
	HostHomePathIsHost      bool           `json:"host_home_path_is_host"`
	GuestHomePathIsGuest    bool           `json:"guest_home_path_is_guest"`
	Sources                 []T14CBindFact `json:"sources"`
	GuestTargets            []string       `json:"guest_targets"`
	LaunchHomeBinding       bool           `json:"launch_home_binding"`
	NoGuestSourceConflation bool           `json:"no_guest_source_conflation"`
}

type t14CExecutionManifest struct {
	FingerprintSchemaVersion    string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest     string                    `json:"canonical_manifest_digest"`
	CanonicalManifest           codex.CanonicalManifestV3 `json:"canonical_manifest"`
	SourceExecutionConfigDigest string                    `json:"source_execution_config_digest"`
	DerivationSchemaVersion     string                    `json:"derivation_schema_version"`
	CapabilityDigest            string                    `json:"capability_digest"`
	LaunchConfigDigest          string                    `json:"launch_config_digest"`
	AuthSnapshotUsed            bool                      `json:"auth_snapshot_used"`
	OfflineRegressionPassed     bool                      `json:"offline_regression_passed"`
	BindPreflightPassed         bool                      `json:"bind_preflight_passed"`
	HostHomePath                string                    `json:"host_home_path"`
	GuestHomePath               string                    `json:"guest_home_path"`
	T13V2Fingerprint            string                    `json:"t13_v2_fingerprint"`
	T13ControlV3Fingerprint     string                    `json:"t13_control_reexpressed_v3_fingerprint"`
	FactorProof                 T14FactorProof            `json:"factor_proof"`
	BindReport                  t14CBindReport            `json:"bind_report"`
}

type R03AT14CResult struct {
	Status                   string                     `json:"status"`
	ProblemKey               string                     `json:"problem_key"`
	Model                    string                     `json:"model"`
	Profile                  string                     `json:"profile"`
	Started                  time.Time                  `json:"started"`
	Finished                 time.Time                  `json:"finished"`
	MediumStarted            int                        `json:"medium_started"`
	HighStarted              int                        `json:"high_started"`
	ProviderEgress           int                        `json:"provider_egress"`
	DynamicToolCount         int                        `json:"dynamic_tool_count"`
	Readiness                any                        `json:"readiness"`
	ProcessStarted           bool                       `json:"process_started"`
	InitializeCompleted      bool                       `json:"initialize_completed"`
	ThreadStarted            bool                       `json:"thread_started"`
	TurnStarted              bool                       `json:"turn_started"`
	TurnCompleted            bool                       `json:"turn_completed"`
	AuthSourceClass          string                     `json:"auth_source_class"`
	AuthIdentityFingerprint  string                     `json:"auth_identity_fingerprint"`
	AuthCredentialRevision   string                     `json:"auth_credential_revision_fingerprint"`
	CanonicalFingerprint     string                     `json:"canonical_manifest_digest"`
	SourceConfigDigest       string                     `json:"source_execution_config_digest"`
	CapabilityDigest         string                     `json:"capability_digest"`
	LaunchConfigDigest       string                     `json:"launch_config_digest"`
	ConfiguredTransport      string                     `json:"configured_transport_policy"`
	ActualTransport          string                     `json:"actual_transport"`
	TurnState                string                     `json:"turn_state"`
	Transport                string                     `json:"transport"`
	SentinelMatch            string                     `json:"sentinel_match"`
	FirstValidOutput         bool                       `json:"first_valid_output"`
	FirstValidOutputText     string                     `json:"first_valid_output_text,omitempty"`
	FirstValidOutputAt       *time.Time                 `json:"first_valid_output_at,omitempty"`
	TimeToFirstOutputMS      int64                      `json:"time_to_first_valid_output_ms,omitempty"`
	ReconnectCount           int                        `json:"reconnect_count"`
	ReconnectPhases          []string                   `json:"reconnect_phases"`
	FirstDisconnectDeltaMS   int64                      `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS          int64                      `json:"recovery_delta_ms,omitempty"`
	RecoveryTimestamps       []*time.Time               `json:"recovery_timestamps,omitempty"`
	NativeUsageUpdates       int                        `json:"native_usage_updates"`
	TokenUsage               codex.TokenUsage           `json:"token_usage"`
	TurnLifecycle            []codex.TurnLifecycleEvent `json:"turn_lifecycle,omitempty"`
	StopReceipt              string                     `json:"stop_receipt,omitempty"`
	StopConfirmed            bool                       `json:"stop_confirmed"`
	ToolEvents               []string                   `json:"tool_events"`
	UnresolvedTransportState bool                       `json:"unresolved_transport_state"`
	Case                     string                     `json:"case"`
	Error                    string                     `json:"error,omitempty"`
}

func InspectR03AT14C(cfg R03AT14CConfig) error {
	if err := validateT14CConfig(cfg); err != nil {
		return err
	}
	if err := ensureT14CEvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cfg.Root, "home"), 0700); err != nil {
		return recordT14CPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: host home directory: %w", err))
	}
	plan, err := buildT14CPlan(cfg)
	if err != nil {
		return recordT14CPreflightFailure(cfg.Evidence, err)
	}
	regressionOutput, err := runner.Run([]string{"bash", "scripts/go.sh", "test", "./internal/codex", "./internal/probe", "-run", "R03AT", "-count=1"}, []string{"PATH=/usr/bin:/bin", "GOPROXY=off"}, 2*time.Minute)
	if err != nil {
		return recordT14CPreflightFailure(cfg.Evidence, fmt.Errorf("preflight_failed: offline regression: %w; output=%s", err, string(regressionOutput)))
	}
	factorDiff := map[string]any{
		"passed":                                 true,
		"changed_fields":                         plan.FactorProof.ConfirmedDivergence,
		"non_transport_divergence":               plan.FactorProof.NonTransportDivergence,
		"unchanged_fields":                       []string{"auth identity", "credential revision", "runtime", "sandbox", "proxy environment", "HOME/CODEX_HOME semantics", "model", "effort", "dynamic tools", "developer instruction", "prompt", "deadlines", "stop semantics", "host/guest mapping semantics"},
		"t13_v2_control_fingerprint":             plan.T13V2Fingerprint,
		"t13_control_reexpressed_v3_fingerprint": plan.T13ControlV3Fingerprint,
		"t14c_native_default_v3_fingerprint":     plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest,
		"transport_policy": map[string]string{
			"control_provider_transport_policy":   codex.ProviderTransportPolicyExplicitlyDisabled,
			"candidate_provider_transport_policy": codex.ProviderTransportPolicyNativeDefault,
			"control_websocket_policy":            codex.ProviderTransportPolicyExplicitlyDisabled,
			"candidate_websocket_policy":          codex.ProviderTransportPolicyNativeDefault,
		},
		"auth_comparison":                            plan.FactorProof.AuthComparison,
		"t13_host_guest_mapping":                     plan.T13HostGuestMappingProof,
		"corrected_host_guest_mapping_is_equivalent": true,
		"derived_for_cross_schema_comparison_only":   true,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), map[string]any{
		"passed":    true,
		"control":   "T13 current-auth 0.153.4 explicitly-disabled policy re-expressed as v3",
		"candidate": "T14C current-auth 0.153.4 native-default policy",
		"derived_for_cross_schema_comparison_only":      true,
		"t13_v2_fingerprint":                            plan.T13V2Fingerprint,
		"t13_control_reexpressed_v3":                    plan.ControlArtifacts.Manifest,
		"t14c_candidate_v3":                             plan.CandidateArtifacts.Manifest,
		"factor_diff":                                   factorDiff,
		"source_execution_config_digest_control":        plan.ControlArtifacts.ConfigDigest,
		"source_execution_config_digest_candidate":      plan.CandidateArtifacts.ConfigDigest,
		"capability_digest_control":                     plan.ControlArtifacts.CapabilityDigest,
		"capability_digest_candidate":                   plan.CandidateArtifacts.CapabilityDigest,
		"launch_config_digest_control":                  plan.ControlArtifacts.LaunchConfigDigest,
		"launch_config_digest_candidate":                plan.CandidateArtifacts.LaunchConfigDigest,
		"host_guest_bind_report":                        plan.BindReport,
		"historical_evidence_recomputed_or_overwritten": false,
	}); err != nil {
		return err
	}
	manifest := t14CExecutionManifest{
		FingerprintSchemaVersion:    plan.CandidateArtifacts.Fingerprint.FingerprintSchemaVersion,
		CanonicalManifestDigest:     plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest,
		CanonicalManifest:           plan.CandidateArtifacts.Manifest,
		SourceExecutionConfigDigest: plan.CandidateArtifacts.ConfigDigest,
		DerivationSchemaVersion:     plan.CandidateArtifacts.DerivationSchemaVersion,
		CapabilityDigest:            plan.CandidateArtifacts.CapabilityDigest,
		LaunchConfigDigest:          plan.CandidateArtifacts.LaunchConfigDigest,
		AuthSnapshotUsed:            false,
		OfflineRegressionPassed:     true,
		BindPreflightPassed:         plan.BindReport.Passed,
		HostHomePath:                plan.CandidateConfig.HostHomePath,
		GuestHomePath:               plan.CandidateConfig.Home,
		T13V2Fingerprint:            plan.T13V2Fingerprint,
		T13ControlV3Fingerprint:     plan.T13ControlV3Fingerprint,
		FactorProof:                 plan.FactorProof,
		BindReport:                  plan.BindReport,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                                   true,
		"status":                                   "preflight_passed",
		"problem_key":                              cfg.ProblemKey,
		"model_calls":                              0,
		"medium_calls":                             0,
		"provider_egress":                          0,
		"fingerprint_schema_version":               manifest.FingerprintSchemaVersion,
		"canonical_manifest_digest":                manifest.CanonicalManifestDigest,
		"source_execution_config_digest":           manifest.SourceExecutionConfigDigest,
		"derivation_schema_version":                manifest.DerivationSchemaVersion,
		"capability_digest":                        manifest.CapabilityDigest,
		"launch_config_digest":                     manifest.LaunchConfigDigest,
		"auth_identity_fingerprint":                plan.CandidateConfig.Auth.AuthIdentityFingerprint,
		"auth_credential_revision_fingerprint":     plan.CandidateConfig.Auth.AuthCredentialRevisionFingerprint,
		"auth_snapshot_used":                       false,
		"offline_regression_passed":                true,
		"bind_preflight_passed":                    true,
		"host_home_path":                           plan.CandidateConfig.HostHomePath,
		"guest_home_path":                          plan.CandidateConfig.Home,
		"t13_control_reexpressed_v3":               true,
		"derived_for_cross_schema_comparison_only": true,
		"t13_t14c_factor_diff":                     factorDiff,
		"bind_report":                              plan.BindReport,
	})
}

func buildT14CPlan(cfg R03AT14CConfig) (t14CPlan, error) {
	var t13 t12ExecutionManifest
	var t10 t11QualificationState
	var t14a t14AQualification
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "execution-manifest.json"), &t13); err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T13 manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T10Evidence, "qualification-state.json"), &t10); err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T10 qualification: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T14AEvidence, "qualification.json"), &t14a); err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: corrected T14A qualification: %w", err)
	}
	if t14a.Status != "PASSED" || t14a.DerivedArtifacts[string(runner.NativeTransportPolicyExplicitlyDisabled)].ConfigDigest == "" || t14a.DerivedArtifacts[string(runner.NativeTransportPolicyNativeDefault)].ConfigDigest == "" {
		return t14CPlan{}, errors.New("preflight_failed: corrected T14A consistency qualification is missing")
	}
	if err := t13.CanonicalManifest.Validate(); err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T13 v2 manifest: %w", err)
	}
	if t13.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 || t13.CanonicalManifestDigest != "f8db1f97ba8d5fe37c01f0442a50273f742f414e2b718926dba1f401dddd7cd0" {
		return t14CPlan{}, errors.New("preflight_failed: T13 current-auth control is not the sealed v2 record")
	}
	if t10.Status != "qualified" || t10.EvidenceResult != "passed" || t10.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersion || t10.Combination.CodexVersion != r03aT14CVersion || t10.Combination.BinarySHA256 == "" || t10.Combination.CodeModeHostSHA256 == "" || t10.Combination.NativeProtocolDigest == "" {
		return t14CPlan{}, errors.New("preflight_failed: T10 0.153.4 qualification is missing or stale")
	}
	for _, path := range historicalAllowancePaths() {
		if raw, err := os.ReadFile(path); err != nil || len(raw) == 0 {
			return t14CPlan{}, fmt.Errorf("preflight_failed: sealed historical allowance unavailable: %s", path)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(cfg.T13Evidence, "allowance.json")); err != nil || !bytes.Contains(raw, []byte(`"medium_turns":1`)) || !bytes.Contains(raw, []byte(`"high_turns":0`)) {
		return t14CPlan{}, errors.New("preflight_failed: T13 allowance is not sealed at one Medium and zero High")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, cfg.AuthSourceClass)
	if err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	authManifest := authMaterial.Manifest()
	if err := CompareT12AuthSnapshot(t13.CanonicalManifest.Auth, authManifest); err != nil {
		return t14CPlan{}, err
	}
	if authManifest.AuthIdentityFingerprint != "dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58" || authManifest.AuthCredentialRevisionFingerprint != "3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7" {
		return t14CPlan{}, errors.New("preflight_failed: current auth does not match the frozen T13 identity and credential revision")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T14C 0.153.4 binary: %w", err)
	}
	helperPath := filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host")
	helper, err := os.ReadFile(helperPath)
	if err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T14C code-mode-host: %w", err)
	}
	if digest(binary) != t10.Combination.BinarySHA256 || digest(helper) != t10.Combination.CodeModeHostSHA256 {
		return t14CPlan{}, errors.New("preflight_failed: T14C binary/helper does not match T10 qualified SHA")
	}
	base := T14AEffectiveExecutionConfig{
		BinaryPath:                 cfg.Binary,
		BinarySHA256:               t10.Combination.BinarySHA256,
		CodeModeHostPath:           helperPath,
		CodeModeHostSHA256:         t10.Combination.CodeModeHostSHA256,
		CodexVersion:               t10.Combination.CodexVersion,
		Model:                      t10.Combination.Model,
		Effort:                     t10.Combination.Effort,
		RuntimeProfile:             t10.Combination.RuntimeProfile,
		SandboxClass:               t10.Combination.SandboxClass,
		AuthFile:                   cfg.AuthFile,
		Auth:                       authManifest,
		ProxyURL:                   "",
		ProxyEnvironment:           absentProxyEnvironment(),
		Home:                       "/home/codex",
		HostHomePath:               filepath.Join(cfg.Root, "home"),
		CWD:                        "/work",
		Invocation:                 "app-server --stdio",
		DynamicToolCount:           0,
		ToolSchemaBytes:            0,
		DeveloperInstructionDigest: digest([]byte(r03aT11Instruction)),
		PromptDigest:               digest([]byte(r03aT11Instruction)),
		FirstOutputDeadlineMS:      90000,
		StreamingIdleDeadlineMS:    90000,
		ReconnectGraceMS:           30000,
		TotalDeadlineMS:            600000,
		StopSemantics:              "process-group-kill-and-waited-proof",
		NativeProtocolDigest:       t10.Combination.NativeProtocolDigest,
	}
	control := base
	control.WebSocketPolicy = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyExplicitlyDisabled, WebSocketPolicy: codex.ProviderTransportPolicyExplicitlyDisabled}
	candidate := base
	candidate.WebSocketPolicy = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyNativeDefault, WebSocketPolicy: codex.ProviderTransportPolicyNativeDefault}
	controlArtifacts, err := DeriveT14AArtifacts(control)
	if err != nil {
		return t14CPlan{}, err
	}
	candidateArtifacts, err := DeriveT14AArtifacts(candidate)
	if err != nil {
		return t14CPlan{}, err
	}
	if err := ValidateT14ABinding(control, controlArtifacts); err != nil {
		return t14CPlan{}, err
	}
	if err := ValidateT14ABinding(candidate, candidateArtifacts); err != nil {
		return t14CPlan{}, err
	}
	if strings.Join(controlArtifacts.LaunchArgs, "\x00") != strings.Join(candidateArtifacts.LaunchArgs, "\x00") || strings.Join(controlArtifacts.LaunchEnvironment, "\x00") != strings.Join(candidateArtifacts.LaunchEnvironment, "\x00") {
		return t14CPlan{}, errors.New("preflight_failed: WebSocket policy changed non-policy launch arguments or environment")
	}
	// The launch digest must differ because the effective config file changes;
	// argument/environment equality is the non-policy namespace invariant.
	if controlArtifacts.LaunchConfigDigest == candidateArtifacts.LaunchConfigDigest {
		return t14CPlan{}, errors.New("preflight_failed: policy change did not alter launch digest")
	}
	for _, policy := range []string{string(runner.NativeTransportPolicyExplicitlyDisabled), string(runner.NativeTransportPolicyNativeDefault)} {
		branch := t14a.DerivedArtifacts[policy]
		artifacts := controlArtifacts
		if policy == string(runner.NativeTransportPolicyNativeDefault) {
			artifacts = candidateArtifacts
		}
		if branch.CapabilityDigest != artifacts.CapabilityDigest || branch.Fingerprint.CanonicalManifestDigest != artifacts.Fingerprint.CanonicalManifestDigest {
			return t14CPlan{}, fmt.Errorf("preflight_failed: corrected T14A branch %s does not match fresh derivation", policy)
		}
	}
	t13Protocol, err := os.ReadFile(filepath.Join(cfg.T13Evidence, "native", "protocol.jsonl"))
	if err != nil {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T13 protocol mapping evidence: %w", err)
	}
	mapping := map[string]any{
		"historical_t13_guest_codex_home_observed": bytes.Contains(t13Protocol, []byte(`"codexHome":"/home/codex"`)),
		"historical_t13_guest_cwd_observed":        bytes.Contains(t13Protocol, []byte(`"cwd":"/work"`)),
		"current_guest_home":                       candidate.Home,
		"current_host_home_source":                 candidate.HostHomePath,
		"host_guest_namespace_separated":           filepath.Clean(candidate.HostHomePath) != filepath.Clean(candidate.Home),
		"equivalence_basis":                        "T13 native initialize/readiness observed guest /home/codex and /work; T14C uses the same guest targets with a distinct host source path",
	}
	if !mapping["historical_t13_guest_codex_home_observed"].(bool) || !mapping["historical_t13_guest_cwd_observed"].(bool) || !mapping["host_guest_namespace_separated"].(bool) {
		return t14CPlan{}, errors.New("preflight_failed: corrected host/guest mapping is not equivalent to T13 observed guest semantics")
	}
	oldFactors := t14FactorsFromArtifacts(control, controlArtifacts)
	newFactors := t14FactorsFromArtifacts(candidate, candidateArtifacts)
	proof, err := CompareT14Factors(oldFactors, newFactors)
	if err != nil {
		return t14CPlan{}, err
	}
	if len(proof.ConfirmedDivergence) != 3 || len(proof.NonTransportDivergence) != 0 {
		return t14CPlan{}, fmt.Errorf("preflight_failed: T13-v3/T14C-v3 factor diff invalid: changed=%v non_transport=%v", proof.ConfirmedDivergence, proof.NonTransportDivergence)
	}
	bindReport, err := inspectT14CBindPaths(candidate)
	if err != nil {
		return t14CPlan{}, err
	}
	return t14CPlan{ControlConfig: control, CandidateConfig: candidate, ControlArtifacts: controlArtifacts, CandidateArtifacts: candidateArtifacts, FactorProof: proof, T13V2Fingerprint: t13.CanonicalManifestDigest, T13ControlV3Fingerprint: controlArtifacts.Fingerprint.CanonicalManifestDigest, T13HostGuestMappingProof: mapping, BindReport: bindReport}, nil
}

func t14FactorsFromArtifacts(config T14AEffectiveExecutionConfig, artifacts T14ADerivedArtifacts) T14CanaryFactors {
	return T14CanaryFactors{Combination: artifacts.Manifest.Combination, AuthManifest: artifacts.Manifest.Auth, ProxyEnvironment: config.ProxyEnvironment, CWD: config.CWD, Home: config.Home, Invocation: config.Invocation, Sandbox: config.SandboxClass, WebSocketPolicy: "policy-factor", DynamicToolCount: config.DynamicToolCount, ToolSchemaBytes: config.ToolSchemaBytes, DeveloperInstructionDigest: config.DeveloperInstructionDigest, PromptDigest: config.PromptDigest, Transport: config.WebSocketPolicy}
}

func absentProxyEnvironment() map[string]string {
	return map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"}
}

func inspectT14CBindPaths(config T14AEffectiveExecutionConfig) (t14CBindReport, error) {
	if filepath.Clean(config.HostHomePath) == filepath.Clean(config.Home) {
		return t14CBindReport{}, errors.New("preflight_failed: HostHomePath and guest HOME are the same namespace path")
	}
	launch, err := runner.BuildNativeLaunch(config.BinaryPath, config.CodeModeHostPath, config.HostHomePath, config.AuthFile, config.ProxyURL, nativePolicy(config.WebSocketPolicy), config.CodeModeHostSHA256)
	if err != nil {
		return t14CBindReport{}, fmt.Errorf("preflight_failed: derive launch for bind validation: %w", err)
	}
	args := launch.Args
	var facts []T14CBindFact
	var targets []string
	launchHomeBinding := false
	noGuestSourceConflation := true
	for i := 0; i < len(args); i++ {
		if args[i] != "--bind" && args[i] != "--ro-bind" {
			continue
		}
		if i+2 >= len(args) {
			return t14CBindReport{}, errors.New("preflight_failed: incomplete bwrap bind tuple")
		}
		source, target := args[i+1], args[i+2]
		targets = append(targets, target)
		if target == config.Home {
			launchHomeBinding = source == config.HostHomePath
			if source == config.Home {
				noGuestSourceConflation = false
			}
		}
		fact := T14CBindFact{HostPath: source, GuestPath: target, ExpectedType: expectedT14CBindType(target), SourceNamespace: "host"}
		info, statErr := os.Stat(source)
		if statErr != nil {
			return t14CBindReport{}, fmt.Errorf("preflight_failed: bind source unavailable: %s: %w", source, statErr)
		}
		fact.Exists = true
		fact.ActualType = actualT14CBindType(info)
		if fact.ActualType == "" {
			return t14CBindReport{}, fmt.Errorf("preflight_failed: unsupported bind source type: %s", source)
		}
		file, openErr := os.Open(source)
		if openErr == nil {
			fact.Readable = true
			_ = file.Close()
		}
		facts = append(facts, fact)
		i += 2
	}
	if err := t14CGuestTargetsAreAbsolute(targets); err != nil {
		return t14CBindReport{}, err
	}
	if err := ValidateT14CNamespaceBindings(facts, config.HostHomePath, config.Home); err != nil {
		return t14CBindReport{}, err
	}
	if !launchHomeBinding || !noGuestSourceConflation {
		return t14CBindReport{}, errors.New("preflight_failed: bwrap HOME binding does not use a distinct host source")
	}
	return t14CBindReport{Passed: true, HostHomePath: config.HostHomePath, GuestHomePath: config.Home, HostHomePathIsHost: true, GuestHomePathIsGuest: true, Sources: facts, GuestTargets: targets, LaunchHomeBinding: launchHomeBinding, NoGuestSourceConflation: noGuestSourceConflation}, nil
}

func expectedT14CBindType(target string) string {
	switch target {
	case "/usr", "/lib", "/lib64", "/etc/ssl", "/home/codex":
		return T14CDirectory
	default:
		return T14CRegularFile
	}
}

func actualT14CBindType(info os.FileInfo) string {
	if info.Mode().IsRegular() {
		return T14CRegularFile
	}
	if info.IsDir() {
		return T14CDirectory
	}
	return ""
}

func RunR03AT14C(cfg R03AT14CConfig) (result R03AT14CResult, err error) {
	result = R03AT14CResult{Status: "not_started", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, ProviderEgress: 0, DynamicToolCount: 0, ActualTransport: "not_observable", Transport: "not_run", SentinelMatch: "not_run", ToolEvents: []string{}}
	if err = validateT14CConfig(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV3 {
		return result, errors.New("preflight_failed: T14C preflight is missing, failed or stale")
	}
	var manifest t14CExecutionManifest
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.CanonicalManifestDigest || !manifest.OfflineRegressionPassed || !manifest.BindPreflightPassed || manifest.AuthSnapshotUsed {
		return result, errors.New("preflight_failed: T14C execution manifest is missing or stale")
	}
	plan, err := buildT14CPlan(cfg)
	if err != nil {
		return result, err
	}
	if plan.CandidateArtifacts.ConfigDigest != manifest.SourceExecutionConfigDigest || plan.CandidateArtifacts.CapabilityDigest != manifest.CapabilityDigest || plan.CandidateArtifacts.LaunchConfigDigest != manifest.LaunchConfigDigest || plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest != manifest.CanonicalManifestDigest || plan.CandidateConfig.HostHomePath != manifest.HostHomePath {
		return result, errors.New("preflight_failed: T14C live derivation is not bound to the frozen manifest")
	}
	result.CanonicalFingerprint = manifest.CanonicalManifestDigest
	result.SourceConfigDigest = manifest.SourceExecutionConfigDigest
	result.CapabilityDigest = manifest.CapabilityDigest
	result.LaunchConfigDigest = manifest.LaunchConfigDigest
	result.ConfiguredTransport = manifest.CanonicalManifest.Transport.WebSocketPolicy
	result.AuthSourceClass = manifest.CanonicalManifest.Auth.AuthSourceClass
	result.AuthIdentityFingerprint = manifest.CanonicalManifest.Auth.AuthIdentityFingerprint
	result.AuthCredentialRevision = manifest.CanonicalManifest.Auth.AuthCredentialRevisionFingerprint
	if err = os.WriteFile(filepath.Join(cfg.Evidence, "problem-key.json"), []byte(fmt.Sprintf("{\"problem_key\":%q,\"new_allowance\":true}\n", cfg.ProblemKey)), 0600); err != nil {
		return result, err
	}
	budget, err := codex.NewBudget(filepath.Join(cfg.Evidence, "allowance.json"), 1, 0)
	if err != nil {
		return result, err
	}
	result.Started = budget.Started
	ctx, cancel := context.WithDeadline(context.Background(), budget.Started.Add(10*time.Minute))
	defer cancel()
	if err = os.MkdirAll(plan.CandidateConfig.HostHomePath, 0700); err != nil {
		return result, err
	}
	launch := runner.NativeLaunch{Args: plan.CandidateArtifacts.LaunchArgs, Environment: plan.CandidateArtifacts.LaunchEnvironment, ConfigBytes: plan.CandidateArtifacts.LaunchConfig, CapabilityDigest: plan.CandidateArtifacts.CapabilityDigest, LaunchConfigDigest: plan.CandidateArtifacts.LaunchConfigDigest}
	if err = runner.WriteNativeLaunch(plan.CandidateConfig.HostHomePath, launch); err != nil {
		return result, err
	}
	processID := "r03a-t14c-canary"
	var process *runner.Process
	var client *codex.Client
	var stopProof runner.StopProof
	var processStart, processStop time.Time
	var readinessAt time.Time
	var sessionWritten bool
	stopProcess := func() error {
		if client != nil {
			client.Close()
			client = nil
		}
		if process == nil {
			return nil
		}
		proof, stopErr := process.Stop()
		if stopErr == nil {
			stopProof = proof
		}
		process = nil
		processStop = time.Now().UTC()
		return stopErr
	}
	defer func() {
		stopErr := stopProcess()
		if stopErr != nil && err == nil {
			err = stopErr
		}
		if stopProof.For(processID) {
			result.StopReceipt = stopProof.Description()
			result.StopConfirmed = true
		}
		result.Finished = time.Now().UTC()
		if result.Started.IsZero() {
			result.Started = result.Finished
		}
		if !sessionWritten {
			_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "configured_transport_policy": result.ConfiguredTransport, "actual_transport": result.ActualTransport, "process_started": result.ProcessStarted, "initialize_completed": result.InitializeCompleted, "thread_started": result.ThreadStarted, "turn_started": result.TurnStarted, "turn_completed": result.TurnCompleted})
		}
		if err != nil {
			result.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
	}()
	process, err = runner.Start(processID, launch.Args, launch.Environment)
	if err != nil {
		return result, err
	}
	result.ProcessStarted = true
	processStart = time.Now().UTC()
	client, err = codex.NewWithModelAndVersion(process, filepath.Join(cfg.Evidence, "native"), cfg.Model, cfg.Version)
	if err != nil {
		return result, err
	}
	if err = client.Initialize(ctx); err != nil {
		return result, err
	}
	result.InitializeCompleted = true
	thread, err := client.StartThreadWithTools(ctx, "medium", []any{}, r03aT11Instruction)
	if err != nil {
		return result, err
	}
	result.ThreadStarted = true
	readinessAt = time.Now().UTC()
	readiness := map[string]any{"binary_sha256": plan.CandidateArtifacts.Manifest.Combination.BinarySHA256, "code_mode_host_sha256": plan.CandidateArtifacts.Manifest.Combination.CodeModeHostSHA256, "capability_digest": plan.CandidateArtifacts.CapabilityDigest, "native_version": "codex-cli " + cfg.Version, "model": cfg.Model, "effort": "medium", "runtime_profile": plan.CandidateConfig.RuntimeProfile, "cwd": plan.CandidateConfig.CWD, "sandbox": plan.CandidateConfig.SandboxClass, "dynamic_tool_count": 0, "provider_transport_policy": codex.ProviderTransportPolicyNativeDefault, "websocket_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "host_home_path_class": "host_source", "host_home_path": plan.CandidateConfig.HostHomePath, "guest_home": plan.CandidateConfig.Home, "auth_identity_fingerprint": result.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": result.AuthCredentialRevision, "auth_snapshot_used": false}
	result.Readiness = readiness
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": readiness, "process_start": processStart, "thread_id": thread, "readiness_written_at": readinessAt, "registration_completed_before_turn_start": true}); err != nil {
		return result, err
	}
	currentRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return result, fmt.Errorf("preflight_failed: auth recheck before Medium: %w", err)
	}
	currentMaterial, err := codex.ParseAuthMaterial(currentRaw, cfg.AuthSourceClass)
	if err != nil {
		return result, err
	}
	if err = CompareT12AuthSnapshot(manifest.CanonicalManifest.Auth, currentMaterial.Manifest()); err != nil {
		return result, err
	}
	if err = budget.Reserve("medium"); err != nil {
		return result, err
	}
	result.MediumStarted = budget.Medium
	result.ProviderEgress = 1
	turn, turnErr := client.TurnWithOptions(ctx, thread, "medium", r03aT11Instruction, codex.TurnOptions{Timeouts: codex.DefaultTurnTimeouts(), OuterDeadline: budget.Started.Add(10 * time.Minute)}, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		result.ToolEvents = append(result.ToolEvents, name)
		return nil, false
	})
	result.TurnState = turn.State
	result.TurnLifecycle = turn.Lifecycle
	result.TurnStarted = turn.State != "" || len(turn.Lifecycle) > 0
	result.TurnCompleted = hasTurnLifecycle(turn.Lifecycle, codex.TurnTerminationConfirmed)
	result.TurnCompleted = result.TurnCompleted || turn.State == "completed"
	result.TokenUsage = turn.Usage
	result.NativeUsageUpdates = turn.UsageUpdates
	result.UnresolvedTransportState = turn.ReconciliationRequired
	if stopErr := stopProcess(); stopErr != nil && turnErr == nil {
		turnErr = stopErr
	}
	if stopProof.For(processID) {
		result.StopReceipt = stopProof.Description()
		result.StopConfirmed = true
	}
	protocolRaw, readErr := os.ReadFile(filepath.Join(cfg.Evidence, "native", "protocol.jsonl"))
	if readErr != nil {
		if turnErr == nil {
			turnErr = readErr
		}
	} else {
		trace, summaryErr := SummarizeT11Protocol(protocolRaw, r03aT11Sentinel)
		if summaryErr != nil {
			if turnErr == nil {
				turnErr = summaryErr
			}
		} else {
			result.FirstValidOutput = trace.FirstValidOutput
			result.FirstValidOutputText = trace.AssistantOutput
			result.FirstValidOutputAt = trace.FirstValidOutputAt
			result.TimeToFirstOutputMS = trace.TimeToFirstOutputMS
			result.ReconnectCount = trace.ReconnectCount
			result.ReconnectPhases = trace.ReconnectPhases
			result.FirstDisconnectDeltaMS = trace.FirstDisconnectDeltaMS
			result.RecoveryDeltaMS = trace.RecoveryDeltaMS
			result.RecoveryTimestamps = trace.RecoveryTimestamps
			result.SentinelMatch = trace.SentinelMatch
			result.NativeUsageUpdates = trace.NativeUsageUpdates
			result.TokenUsage = trace.TokenUsage
			result.TurnCompleted = result.TurnCompleted || trace.TurnCompletedAt != nil
			if trace.TerminalState != "" {
				result.TurnState = trace.TerminalState
			}
			if turnErr != nil {
				if strings.Contains(turnErr.Error(), "first_valid_output_deadline_exceeded") {
					result.TurnState = "first_valid_output_deadline_exceeded"
				} else if strings.Contains(turnErr.Error(), "reconnect_deadline_exceeded") {
					result.TurnState = "reconnect_deadline_exceeded"
				}
			}
			terminalState := trace.TerminalState
			if terminalState == "" {
				terminalState = result.TurnState
			}
			if err = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), map[string]any{"process_start": processStart, "readiness_manifest": readinessAt, "initialize": map[string]any{"present": trace.InitializeSent && trace.InitializeReceived}, "thread_start": map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted}, "turn_start": map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted}, "user_message_item": trace.UserMessageStarted, "provider_transport_events": "native/protocol.jsonl", "configured_transport_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "reconnect_count": trace.ReconnectCount, "reconnect_phases": trace.ReconnectPhases, "recovery_timestamps": trace.RecoveryTimestamps, "first_valid_output": trace.FirstValidOutputAt, "time_to_first_valid_output_ms": trace.TimeToFirstOutputMS, "turn_completed": trace.TurnCompletedAt, "native_usage_updates": trace.NativeUsageUpdates, "token_counts": trace.TokenUsage, "terminal_state": terminalState, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "business_side_effects": 0}); err != nil {
				return result, err
			}
			if err = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "readiness_manifest": readinessAt, "initialize": trace.InitializeSent && trace.InitializeReceived, "thread_started": trace.ThreadStarted, "turn_started": trace.TurnStarted, "user_message_item": trace.UserMessageStarted, "turn_completed": trace.TurnCompletedAt, "configured_transport_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "tool_events": result.ToolEvents}); err != nil {
				return result, err
			}
			sessionWritten = true
		}
	}
	if turnErr != nil {
		if strings.Contains(turnErr.Error(), "deadline") || strings.Contains(turnErr.Error(), "outcome_unknown") || strings.Contains(turnErr.Error(), "reconciliation") {
			result.Transport = "inconclusive"
			result.Case = "B"
		} else {
			result.Transport = "failed"
			result.Case = "C"
		}
		result.Status = "inconclusive"
		result.Finished = time.Now().UTC()
		if qualificationErr := writeT14CQualification(cfg, manifest, result); qualificationErr != nil {
			return result, errors.Join(turnErr, qualificationErr)
		}
		return result, turnErr
	}
	if result.TurnState == "completed" && result.FirstValidOutput && !result.UnresolvedTransportState {
		result.Status = "passed"
		result.Transport = "passed"
		result.Case = "A"
	} else {
		result.Status = "inconclusive"
		result.Transport = "inconclusive"
		result.Case = "B"
	}
	result.Finished = time.Now().UTC()
	if err = writeT14CQualification(cfg, manifest, result); err != nil {
		return result, err
	}
	return result, nil
}

func hasTurnLifecycle(events []codex.TurnLifecycleEvent, state codex.TurnLifecycleState) bool {
	for _, event := range events {
		if event.State == state {
			return true
		}
	}
	return false
}

func writeT14CQualification(cfg R03AT14CConfig, manifest t14CExecutionManifest, result R03AT14CResult) error {
	status, evidence, scheduling := codex.QualificationUnqualified, codex.EvidenceInconclusive, "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.Case == "A" {
		status, evidence, scheduling = codex.QualificationQualified, codex.EvidencePassed, "qualified"
	}
	record := codex.QualificationRecordV3{Layer: codex.QualificationL1BaseTransport, Status: status, SchedulingDecision: scheduling, EvidenceResult: evidence, FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV3, CanonicalManifestDigest: manifest.CanonicalManifestDigest, Combination: manifest.CanonicalManifest.Combination, Auth: manifest.CanonicalManifest.Auth, Transport: manifest.CanonicalManifest.Transport, Reason: "T14C native-default WebSocket policy case " + result.Case, CreatedAt: result.Finished}
	if !result.Finished.IsZero() {
		record.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), record)
}

func recordT14CPreflightFailure(evidence string, cause error) error {
	recordErr := writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"passed": false, "status": "preflight_failed", "model_calls": 0, "medium_calls": 0, "provider_egress": 0, "error": cause.Error()})
	return errors.Join(cause, recordErr)
}

func validateT14CConfig(cfg R03AT14CConfig) error {
	if cfg.Version != r03aT14CVersion || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.AuthSourceClass != r03aT12SourceClass || cfg.Binary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T13Evidence == "" || cfg.T14AEvidence == "" || cfg.T10Evidence == "" {
		return errors.New("preflight_failed: T14C fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT14CEvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T14C evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}
