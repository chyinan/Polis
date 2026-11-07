// pattern: Imperative Shell
package probe

import (
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

const r03aT14BVersion = "0.153.4"

type R03AT14BConfig struct {
	Config
	Version         string
	ProblemKey      string
	AuthSourceClass string
	BaselineBinary  string
	T13Evidence     string
	T14AEvidence    string
	T10Evidence     string
}

type t14AQualification struct {
	Status           string `json:"status"`
	DerivedArtifacts map[string]struct {
		ConfigDigest       string                           `json:"source_execution_config_digest"`
		CapabilityDigest   string                           `json:"capability_digest"`
		ManifestDigest     string                           `json:"manifest_digest"`
		LaunchConfigDigest string                           `json:"launch_config_digest"`
		Fingerprint        codex.QualificationFingerprintV3 `json:"qualification_fingerprint"`
	} `json:"derived_artifacts"`
}

type t14BPlan struct {
	ControlConfig      T14AEffectiveExecutionConfig
	CandidateConfig    T14AEffectiveExecutionConfig
	ControlArtifacts   T14ADerivedArtifacts
	CandidateArtifacts T14ADerivedArtifacts
	FactorProof        T14FactorProof
	T13V2Fingerprint   string
}

type t14BExecutionManifest struct {
	FingerprintSchemaVersion    string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest     string                    `json:"canonical_manifest_digest"`
	CanonicalManifest           codex.CanonicalManifestV3 `json:"canonical_manifest"`
	SourceExecutionConfigDigest string                    `json:"source_execution_config_digest"`
	DerivationSchemaVersion     string                    `json:"derivation_schema_version"`
	CapabilityDigest            string                    `json:"capability_digest"`
	LaunchConfigDigest          string                    `json:"launch_config_digest"`
	AuthSnapshotUsed            bool                      `json:"auth_snapshot_used"`
	OfflineRegressionPassed     bool                      `json:"offline_regression_passed"`
}

type R03AT14BResult struct {
	Status                   string           `json:"status"`
	ProblemKey               string           `json:"problem_key"`
	Model                    string           `json:"model"`
	Profile                  string           `json:"profile"`
	Started                  time.Time        `json:"started"`
	Finished                 time.Time        `json:"finished"`
	MediumStarted            int              `json:"medium_started"`
	HighStarted              int              `json:"high_started"`
	DynamicToolCount         int              `json:"dynamic_tool_count"`
	Readiness                any              `json:"readiness"`
	AuthSourceClass          string           `json:"auth_source_class"`
	AuthIdentityFingerprint  string           `json:"auth_identity_fingerprint"`
	AuthCredentialRevision   string           `json:"auth_credential_revision_fingerprint"`
	CanonicalFingerprint     string           `json:"canonical_manifest_digest"`
	ConfiguredTransport      string           `json:"configured_transport_policy"`
	ActualTransport          string           `json:"actual_transport"`
	TurnState                string           `json:"turn_state"`
	Transport                string           `json:"transport"`
	SentinelMatch            string           `json:"sentinel_match"`
	FirstValidOutput         bool             `json:"first_valid_output"`
	FirstValidOutputText     string           `json:"first_valid_output_text,omitempty"`
	FirstValidOutputAt       *time.Time       `json:"first_valid_output_at,omitempty"`
	TimeToFirstOutputMS      int64            `json:"time_to_first_valid_output_ms,omitempty"`
	ReconnectCount           int              `json:"reconnect_count"`
	ReconnectPhases          []string         `json:"reconnect_phases"`
	FirstDisconnectDeltaMS   int64            `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS          int64            `json:"recovery_delta_ms,omitempty"`
	NativeUsageUpdates       int              `json:"native_usage_updates"`
	TokenUsage               codex.TokenUsage `json:"token_usage"`
	StopReceipt              string           `json:"stop_receipt,omitempty"`
	StopConfirmed            bool             `json:"stop_confirmed"`
	ToolEvents               []string         `json:"tool_events"`
	UnresolvedTransportState bool             `json:"unresolved_transport_state"`
	Case                     string           `json:"case"`
	Error                    string           `json:"error,omitempty"`
}

func InspectR03AT14B(cfg R03AT14BConfig) error {
	if err := validateT14BConfig(cfg); err != nil {
		return err
	}
	if err := ensureT14BEvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	plan, err := buildT14BPlan(cfg)
	if err != nil {
		return err
	}
	regressionOutput, err := runner.Run([]string{"bash", "scripts/go.sh", "test", "./internal/codex", "./internal/probe", "-run", "R03AT", "-count=1"}, []string{"PATH=/usr/bin:/bin", "GOPROXY=off", "POLIS_CODEX_T10_BINARY="}, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("preflight_failed: offline regression: %w; output=%s", err, string(regressionOutput))
	}
	factorDiff := map[string]any{
		"passed":                                 true,
		"changed_fields":                         plan.FactorProof.ConfirmedDivergence,
		"non_transport_divergence":               plan.FactorProof.NonTransportDivergence,
		"t13_v2_control_fingerprint":             plan.T13V2Fingerprint,
		"t13_control_reexpressed_v3_fingerprint": plan.ControlArtifacts.Fingerprint.CanonicalManifestDigest,
		"t14b_v3_candidate_fingerprint":          plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest,
		"transport_policy": map[string]string{
			"control_provider_transport_policy":   codex.ProviderTransportPolicyExplicitlyDisabled,
			"candidate_provider_transport_policy": codex.ProviderTransportPolicyNativeDefault,
			"control_websocket_policy":            codex.ProviderTransportPolicyExplicitlyDisabled,
			"candidate_websocket_policy":          codex.ProviderTransportPolicyNativeDefault,
		},
		"auth_comparison":                          plan.FactorProof.AuthComparison,
		"derived_for_cross_schema_comparison_only": true,
	}
	manifest := t14BExecutionManifest{FingerprintSchemaVersion: plan.CandidateArtifacts.Fingerprint.FingerprintSchemaVersion, CanonicalManifestDigest: plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest, CanonicalManifest: plan.CandidateArtifacts.Manifest, SourceExecutionConfigDigest: plan.CandidateArtifacts.ConfigDigest, DerivationSchemaVersion: plan.CandidateArtifacts.DerivationSchemaVersion, CapabilityDigest: plan.CandidateArtifacts.CapabilityDigest, LaunchConfigDigest: plan.CandidateArtifacts.LaunchConfigDigest, AuthSnapshotUsed: false, OfflineRegressionPassed: true}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), map[string]any{"passed": true, "control": "T13 current-auth 0.153.4 re-expressed as v3", "candidate": "T14B current-auth 0.153.4 native-default policy", "derived_for_cross_schema_comparison_only": true, "t13_v2_fingerprint": plan.T13V2Fingerprint, "t13_control_reexpressed_v3": plan.ControlArtifacts.Manifest, "t14b_candidate_v3": plan.CandidateArtifacts.Manifest, "factor_diff": factorDiff, "source_execution_config_digest_control": plan.ControlArtifacts.ConfigDigest, "source_execution_config_digest_candidate": plan.CandidateArtifacts.ConfigDigest, "capability_digest_control": plan.ControlArtifacts.CapabilityDigest, "capability_digest_candidate": plan.CandidateArtifacts.CapabilityDigest, "launch_config_digest_control": plan.ControlArtifacts.LaunchConfigDigest, "launch_config_digest_candidate": plan.CandidateArtifacts.LaunchConfigDigest, "historical_evidence_recomputed_or_overwritten": false}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "status": "preflight_passed", "problem_key": cfg.ProblemKey, "model_calls": 0, "medium_calls": 0, "fingerprint_schema_version": manifest.FingerprintSchemaVersion, "canonical_manifest_digest": manifest.CanonicalManifestDigest, "source_execution_config_digest": manifest.SourceExecutionConfigDigest, "derivation_schema_version": manifest.DerivationSchemaVersion, "capability_digest": manifest.CapabilityDigest, "launch_config_digest": manifest.LaunchConfigDigest, "auth_identity_fingerprint": plan.CandidateConfig.Auth.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": plan.CandidateConfig.Auth.AuthCredentialRevisionFingerprint, "auth_snapshot_used": false, "offline_regression_passed": true, "t13_control_reexpressed_v3": true, "derived_for_cross_schema_comparison_only": true, "t13_t14b_factor_diff": factorDiff})
}

func buildT14BPlan(cfg R03AT14BConfig) (t14BPlan, error) {
	var t13 t12ExecutionManifest
	var t10 t11QualificationState
	var t14a t14AQualification
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "execution-manifest.json"), &t13); err != nil {
		return t14BPlan{}, fmt.Errorf("preflight_failed: T13 manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T10Evidence, "qualification-state.json"), &t10); err != nil {
		return t14BPlan{}, fmt.Errorf("preflight_failed: T10 qualification: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T14AEvidence, "qualification.json"), &t14a); err != nil {
		return t14BPlan{}, fmt.Errorf("preflight_failed: T14A qualification: %w", err)
	}
	if t14a.Status != "PASSED" || t14a.DerivedArtifacts[string(runner.NativeTransportPolicyExplicitlyDisabled)].ConfigDigest == "" || t14a.DerivedArtifacts[string(runner.NativeTransportPolicyNativeDefault)].ConfigDigest == "" {
		return t14BPlan{}, errors.New("preflight_failed: T14A consistency qualification is missing")
	}
	if err := t13.CanonicalManifest.Validate(); err != nil {
		return t14BPlan{}, fmt.Errorf("preflight_failed: T13 v2 manifest: %w", err)
	}
	if t10.Combination.CodexVersion != r03aT14BVersion || t10.Combination.BinarySHA256 == "" || t10.Combination.CodeModeHostSHA256 == "" || t10.Combination.NativeProtocolDigest == "" {
		return t14BPlan{}, errors.New("preflight_failed: T10 0.153.4 fields are incomplete")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return t14BPlan{}, fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, cfg.AuthSourceClass)
	if err != nil {
		return t14BPlan{}, err
	}
	if err := CompareT12AuthSnapshot(t13.CanonicalManifest.Auth, authMaterial.Manifest()); err != nil {
		return t14BPlan{}, err
	}
	base := T14AEffectiveExecutionConfig{BinaryPath: cfg.Binary, BinarySHA256: t10.Combination.BinarySHA256, CodeModeHostPath: filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"), CodeModeHostSHA256: t10.Combination.CodeModeHostSHA256, CodexVersion: t10.Combination.CodexVersion, Model: t10.Combination.Model, Effort: t10.Combination.Effort, RuntimeProfile: t10.Combination.RuntimeProfile, SandboxClass: t10.Combination.SandboxClass, AuthFile: cfg.AuthFile, Auth: authMaterial.Manifest(), ProxyURL: "", ProxyEnvironment: map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"}, Home: "/home/codex", HostHomePath: filepath.Join(cfg.Root, "home"), CWD: "/work", Invocation: "app-server --stdio", DynamicToolCount: 0, ToolSchemaBytes: 0, DeveloperInstructionDigest: digest([]byte(r03aT11Instruction)), PromptDigest: digest([]byte(r03aT11Instruction)), FirstOutputDeadlineMS: 90000, StreamingIdleDeadlineMS: 90000, ReconnectGraceMS: 30000, TotalDeadlineMS: 600000, StopSemantics: "process-group-kill-and-waited-proof", NativeProtocolDigest: t10.Combination.NativeProtocolDigest}
	control := base
	control.WebSocketPolicy = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyExplicitlyDisabled, WebSocketPolicy: codex.ProviderTransportPolicyExplicitlyDisabled}
	candidate := base
	candidate.WebSocketPolicy = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyNativeDefault, WebSocketPolicy: codex.ProviderTransportPolicyNativeDefault}
	controlArtifacts, err := DeriveT14AArtifacts(control)
	if err != nil {
		return t14BPlan{}, err
	}
	candidateArtifacts, err := DeriveT14AArtifacts(candidate)
	if err != nil {
		return t14BPlan{}, err
	}
	if err := ValidateT14ABinding(control, controlArtifacts); err != nil {
		return t14BPlan{}, err
	}
	if err := ValidateT14ABinding(candidate, candidateArtifacts); err != nil {
		return t14BPlan{}, err
	}
	if controlArtifacts.ConfigDigest != t14a.DerivedArtifacts[string(runner.NativeTransportPolicyExplicitlyDisabled)].ConfigDigest || candidateArtifacts.ConfigDigest != t14a.DerivedArtifacts[string(runner.NativeTransportPolicyNativeDefault)].ConfigDigest || controlArtifacts.CapabilityDigest != t14a.DerivedArtifacts[string(runner.NativeTransportPolicyExplicitlyDisabled)].CapabilityDigest || candidateArtifacts.CapabilityDigest != t14a.DerivedArtifacts[string(runner.NativeTransportPolicyNativeDefault)].CapabilityDigest {
		return t14BPlan{}, errors.New("preflight_failed: T14A derived artifacts do not match T14B re-derivation")
	}
	oldFactors := T14CanaryFactors{Combination: controlArtifacts.Manifest.Combination, AuthManifest: controlArtifacts.Manifest.Auth, ProxyEnvironment: control.ProxyEnvironment, CWD: control.CWD, Home: control.Home, Invocation: control.Invocation, Sandbox: control.SandboxClass, WebSocketPolicy: "policy-factor", DynamicToolCount: control.DynamicToolCount, ToolSchemaBytes: control.ToolSchemaBytes, DeveloperInstructionDigest: control.DeveloperInstructionDigest, PromptDigest: control.PromptDigest, Transport: control.WebSocketPolicy}
	newFactors := oldFactors
	newFactors.Combination = candidateArtifacts.Manifest.Combination
	newFactors.AuthManifest = candidateArtifacts.Manifest.Auth
	newFactors.Transport = candidate.WebSocketPolicy
	proof, err := CompareT14Factors(oldFactors, newFactors)
	if err != nil || len(proof.ConfirmedDivergence) != 3 || len(proof.NonTransportDivergence) != 0 {
		return t14BPlan{}, fmt.Errorf("preflight_failed: T13-v3/T14B-v3 factor diff invalid: %v", err)
	}
	return t14BPlan{ControlConfig: control, CandidateConfig: candidate, ControlArtifacts: controlArtifacts, CandidateArtifacts: candidateArtifacts, FactorProof: proof, T13V2Fingerprint: t13.CanonicalManifestDigest}, nil
}

func RunR03AT14B(cfg R03AT14BConfig) (result R03AT14BResult, err error) {
	result = R03AT14BResult{Status: "inconclusive", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, DynamicToolCount: 0, Transport: "not_run", ActualTransport: "not_observable", SentinelMatch: "not_run", ToolEvents: []string{}}
	if err = validateT14BConfig(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV3 {
		return result, errors.New("preflight_failed: T14B preflight is missing, failed or stale")
	}
	var manifest t14BExecutionManifest
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.CanonicalManifestDigest || !manifest.OfflineRegressionPassed || manifest.AuthSnapshotUsed {
		return result, errors.New("preflight_failed: T14B execution manifest is missing or stale")
	}
	plan, err := buildT14BPlan(cfg)
	if err != nil {
		return result, err
	}
	if plan.CandidateArtifacts.ConfigDigest != manifest.SourceExecutionConfigDigest || plan.CandidateArtifacts.CapabilityDigest != manifest.CapabilityDigest || plan.CandidateArtifacts.LaunchConfigDigest != manifest.LaunchConfigDigest || plan.CandidateArtifacts.Fingerprint.CanonicalManifestDigest != manifest.CanonicalManifestDigest {
		return result, errors.New("preflight_failed: T14B live derivation is not bound to preflight manifest")
	}
	result.CanonicalFingerprint = manifest.CanonicalManifestDigest
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
	root := filepath.Join(cfg.Root, "canary")
	if err = os.MkdirAll(root, 0700); err != nil {
		return result, err
	}
	launch := runner.NativeLaunch{Args: plan.CandidateArtifacts.LaunchArgs, Environment: plan.CandidateArtifacts.LaunchEnvironment, ConfigBytes: plan.CandidateArtifacts.LaunchConfig, CapabilityDigest: plan.CandidateArtifacts.CapabilityDigest, LaunchConfigDigest: plan.CandidateArtifacts.LaunchConfigDigest}
	if err = runner.WriteNativeLaunch(filepath.Join(root, "home"), launch); err != nil {
		return result, err
	}
	processID := "r03a-t14b-canary"
	var process *runner.Process
	var client *codex.Client
	var stopProof runner.StopProof
	var processStart, processStop time.Time
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
			_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "actual_transport": result.ActualTransport})
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
	processStart = time.Now().UTC()
	client, err = codex.NewWithModelAndVersion(process, filepath.Join(cfg.Evidence, "native"), cfg.Model, cfg.Version)
	if err != nil {
		return result, err
	}
	if err = client.Initialize(ctx); err != nil {
		return result, err
	}
	thread, err := client.StartThreadWithTools(ctx, "medium", []any{}, r03aT11Instruction)
	if err != nil {
		return result, err
	}
	readinessAt := time.Now().UTC()
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": map[string]any{"binary_sha256": cfg.Config.Binary, "native_version": "codex-cli " + cfg.Version, "model": cfg.Model, "effort": "medium", "cwd": "/work", "sandbox": "read-only", "dynamic_tool_count": 0, "provider_transport_policy": codex.ProviderTransportPolicyNativeDefault, "websocket_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "auth_identity_fingerprint": result.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": result.AuthCredentialRevision}, "process_start": processStart, "thread_id": thread, "registration_completed_before_turn_start": true, "readiness_written_at": readinessAt, "auth_snapshot_used": false}); err != nil {
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
	turn, turnErr := client.TurnWithOptions(ctx, thread, "medium", r03aT11Instruction, codex.TurnOptions{Timeouts: codex.DefaultTurnTimeouts(), OuterDeadline: budget.Started.Add(10 * time.Minute)}, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		result.ToolEvents = append(result.ToolEvents, name)
		return nil, false
	})
	result.TurnState = turn.State
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
	} else if trace, summaryErr := SummarizeT11Protocol(protocolRaw, r03aT11Sentinel); summaryErr != nil {
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
		result.SentinelMatch = trace.SentinelMatch
		result.NativeUsageUpdates = trace.NativeUsageUpdates
		result.TokenUsage = trace.TokenUsage
		if trace.TerminalState != "" {
			result.TurnState = trace.TerminalState
		}
		if turnErr != nil && strings.Contains(turnErr.Error(), "first_valid_output_deadline_exceeded") {
			result.TurnState = "first_valid_output_deadline_exceeded"
		}
		terminalState := trace.TerminalState
		if terminalState == "" {
			terminalState = result.TurnState
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), map[string]any{"process_start": processStart, "readiness_manifest": readinessAt, "initialize": map[string]any{"present": trace.InitializeSent && trace.InitializeReceived}, "thread_start": map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted}, "turn_start": map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted}, "user_message_item": trace.UserMessageStarted, "provider_transport_events": "native/protocol.jsonl", "configured_transport_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "first_valid_output": trace.FirstValidOutputAt, "time_to_first_valid_output_ms": trace.TimeToFirstOutputMS, "disconnect_count": trace.ReconnectCount, "reconnect_phases": trace.ReconnectPhases, "recovery_timestamps": trace.RecoveryTimestamps, "turn_completed": trace.TurnCompletedAt, "native_usage_updates": trace.NativeUsageUpdates, "token_counts": trace.TokenUsage, "terminal_state": terminalState, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "business_side_effects": 0})
		_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "readiness_manifest": readinessAt, "initialize": trace.InitializeSent && trace.InitializeReceived, "thread_started": trace.ThreadStarted, "turn_started": trace.TurnStarted, "user_message_item": trace.UserMessageStarted, "provider_transport_events": "native/protocol.jsonl", "actual_transport": "not_observable", "first_valid_output": trace.FirstValidOutputAt, "turn_completed": trace.TurnCompletedAt, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "tool_events": result.ToolEvents})
		sessionWritten = true
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
		_ = writeT14BQualification(cfg, manifest, result)
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
	if err = writeT14BQualification(cfg, manifest, result); err != nil {
		return result, err
	}
	return result, nil
}

func writeT14BQualification(cfg R03AT14BConfig, manifest t14BExecutionManifest, result R03AT14BResult) error {
	status, evidence, scheduling := codex.QualificationUnqualified, codex.EvidenceInconclusive, "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.Case == "A" {
		status, evidence, scheduling = codex.QualificationQualified, codex.EvidencePassed, "qualified"
	}
	record := codex.QualificationRecordV3{Layer: codex.QualificationL1BaseTransport, Status: status, SchedulingDecision: scheduling, EvidenceResult: evidence, FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV3, CanonicalManifestDigest: manifest.CanonicalManifestDigest, Combination: manifest.CanonicalManifest.Combination, Auth: manifest.CanonicalManifest.Auth, Transport: manifest.CanonicalManifest.Transport, Reason: "T14B native-default WebSocket policy case " + result.Case, CreatedAt: result.Finished}
	if !result.Finished.IsZero() {
		record.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), record)
}

func validateT14BConfig(cfg R03AT14BConfig) error {
	if cfg.Version != r03aT14BVersion || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.AuthSourceClass != r03aT12SourceClass || cfg.Binary == "" || cfg.BaselineBinary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T13Evidence == "" || cfg.T14AEvidence == "" || cfg.T10Evidence == "" {
		return errors.New("preflight_failed: T14B fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT14BEvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T14B evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}
