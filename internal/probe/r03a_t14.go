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
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

const r03aT14Version = "0.153.4"

type R03AT14Config struct {
	Config
	Version         string
	ProblemKey      string
	AuthSourceClass string
	BaselineBinary  string
	T13Evidence     string
	T10Evidence     string
}

type t14ExecutionManifest struct {
	FingerprintSchemaVersion string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                    `json:"canonical_manifest_digest"`
	CanonicalManifest        codex.CanonicalManifestV3 `json:"canonical_manifest"`
	AuthSnapshotUsed         bool                      `json:"auth_snapshot_used"`
	OfflineRegressionPassed  bool                      `json:"offline_regression_passed"`
}

type R03AT14Result struct {
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

func InspectR03AT14(cfg R03AT14Config) error {
	if err := validateT14Config(cfg); err != nil {
		return err
	}
	if err := ensureT14EvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	var t13 t12ExecutionManifest
	var t13Preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	var t13State codex.QualificationRecordV2
	var t13Readiness struct {
		Readiness map[string]any `json:"readiness"`
	}
	var t10State t11QualificationState
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "execution-manifest.json"), &t13); err != nil {
		return fmt.Errorf("preflight_failed: T13 execution manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "preflight.json"), &t13Preflight); err != nil {
		return fmt.Errorf("preflight_failed: T13 preflight: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "qualification-state.json"), &t13State); err != nil {
		return fmt.Errorf("preflight_failed: T13 qualification state: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "readiness-manifest.json"), &t13Readiness); err != nil {
		return fmt.Errorf("preflight_failed: T13 readiness manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T10Evidence, "qualification-state.json"), &t10State); err != nil {
		return fmt.Errorf("preflight_failed: T10 qualification state: %w", err)
	}
	if !t13Preflight.Passed || t13Preflight.ProblemKey != cfg.ProblemKey || t13Preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 || t13.CanonicalManifestDigest != t13Preflight.CanonicalManifestDigest || !t13.OfflineRegressionPassed || t13State.Status != codex.QualificationUnqualified || t13State.EvidenceResult != codex.EvidenceInconclusive || t13Readiness.Readiness["websocket_policy"] != "disabled" {
		return errors.New("preflight_failed: T13 disabled-WebSocket control is missing or not sealed")
	}
	if err := t13.CanonicalManifest.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: T13 canonical manifest invalid: %w", err)
	}
	t13Allowance, err := os.ReadFile(filepath.Join(cfg.T13Evidence, "allowance.json"))
	if err != nil || !bytes.Contains(t13Allowance, []byte(`"medium_turns":1`)) || !bytes.Contains(t13Allowance, []byte(`"high_turns":0`)) {
		return errors.New("preflight_failed: T13 allowance is not sealed at one Medium and zero High")
	}
	for _, path := range historicalAllowancePaths() {
		if raw, err := os.ReadFile(path); err != nil || len(raw) == 0 {
			return fmt.Errorf("preflight_failed: sealed historical allowance unavailable: %s", path)
		}
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, cfg.AuthSourceClass)
	if err != nil {
		return fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	authManifest := authMaterial.Manifest()
	if err := CompareT12AuthSnapshot(t13.CanonicalManifest.Auth, authManifest); err != nil {
		return err
	}
	if err := authManifest.Validate(); err != nil || authManifest.AuthIdentityFingerprintStatus != "available" || authManifest.AuthCredentialRevisionFingerprintStatus != "available" {
		return errors.New("preflight_failed: current auth identity or credential revision is unavailable")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return fmt.Errorf("preflight_failed: T14 binary: %w", err)
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return fmt.Errorf("preflight_failed: T14 code-mode-host: %w", err)
	}
	if digest(binary) != t10State.Combination.BinarySHA256 || digest(helper) != t10State.Combination.CodeModeHostSHA256 || t10State.Combination.CodexVersion != r03aT14Version {
		return errors.New("preflight_failed: T14 binary/helper does not match T10 qualification")
	}
	versionOutput, err := runner.Run([]string{cfg.Binary, "--version"}, []string{"PATH=/usr/bin:/bin"}, 10*time.Second)
	if err != nil || strings.TrimSpace(string(versionOutput)) != "codex-cli "+r03aT14Version {
		return errors.New("preflight_failed: T14 version output mismatch")
	}
	if err := os.MkdirAll(cfg.Root, 0700); err != nil {
		return err
	}
	oldHome := filepath.Join(cfg.Root, "preflight-disabled-home")
	newHome := filepath.Join(cfg.Root, "preflight-default-home")
	oldArgs, oldCapability, err := runner.NativeArgsWithTransportPolicy(cfg.BaselineBinary, oldHome, cfg.AuthFile, "", runner.NativeTransportPolicyExplicitlyDisabled)
	if err != nil {
		return fmt.Errorf("preflight_failed: T13 explicit-disabled args: %w", err)
	}
	newArgs, newCapability, err := runner.NativeArgsWithTransportPolicy(cfg.Binary, newHome, cfg.AuthFile, "", runner.NativeTransportPolicyNativeDefault)
	if err != nil {
		return fmt.Errorf("preflight_failed: T14 native-default args: %w", err)
	}
	oldConfig, err := os.ReadFile(filepath.Join(oldHome, "config.toml"))
	if err != nil {
		return err
	}
	newConfig, err := os.ReadFile(filepath.Join(newHome, "config.toml"))
	if err != nil {
		return err
	}
	oldHelper := filepath.Join(filepath.Dir(cfg.BaselineBinary), "codex-code-mode-host")
	newHelper := filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host")
	if oldCapability != t13.CanonicalManifest.Combination.CapabilityDigest || newCapability == oldCapability || !bytes.Equal(oldConfig, []byte(runner.NativeConfig)) || !bytes.Equal(newConfig, []byte(runner.NativeDefaultConfig)) || !reflect.DeepEqual(normalizeT11NativeArgs(oldArgs, oldHome, cfg.BaselineBinary, oldHelper), normalizeT11NativeArgs(newArgs, newHome, cfg.Binary, newHelper)) || strings.Contains(strings.Join(newArgs, " "), "HTTP_PROXY") || strings.Contains(strings.Join(newArgs, " "), "HTTPS_PROXY") {
		return errors.New("preflight_failed: T13/T14 args/config diverged outside transport policy")
	}
	regressionOutput, err := runner.Run([]string{"bash", "scripts/go.sh", "test", "./internal/codex", "./internal/probe", "-run", "R03AT", "-count=1"}, []string{"PATH=/usr/bin:/bin", "GOPROXY=off", "POLIS_CODEX_T10_BINARY="}, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("preflight_failed: offline transport regression: %w; output=%s", err, string(regressionOutput))
	}
	newCombination := t10State.Combination
	newCombination.AuthSourceClass = cfg.AuthSourceClass
	newCombination.CapabilityDigest = newCapability
	oldFactors := T14CanaryFactors{Combination: t13.CanonicalManifest.Combination, AuthManifest: t13.CanonicalManifest.Auth, ProxyEnvironment: map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"}, CWD: "/work", Home: "/home/codex", Invocation: r03aT11Invocation, Sandbox: "read-only", WebSocketPolicy: "policy-factor", DynamicToolCount: 0, ToolSchemaBytes: 0, DeveloperInstructionDigest: digest([]byte(r03aT11Instruction)), PromptDigest: digest([]byte(r03aT11Instruction)), Transport: codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyExplicitlyDisabled, WebSocketPolicy: codex.ProviderTransportPolicyExplicitlyDisabled}}
	newFactors := oldFactors
	newFactors.Combination = newCombination
	newFactors.AuthManifest = authManifest
	newFactors.Transport = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyNativeDefault, WebSocketPolicy: codex.ProviderTransportPolicyNativeDefault}
	newFactors.Combination.CapabilityDigest = newCapability
	proof, err := CompareT14Factors(oldFactors, newFactors)
	if err != nil {
		return err
	}
	if len(proof.NonTransportDivergence) != 0 || len(proof.ConfirmedDivergence) != 3 {
		return errors.New("preflight_failed: T13/T14 factor diff contains an unexpected divergence")
	}
	fingerprint := newCombination.CurrentFingerprintV3(authManifest, newFactors.Transport)
	canonical := codex.CanonicalManifestV3{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV3, Combination: newCombination, Auth: authManifest, Transport: newFactors.Transport}
	if err := canonical.Validate(); err != nil || fingerprint.Validate() != nil {
		return fmt.Errorf("preflight_failed: T14 canonical-manifest-v3 invalid: %w", err)
	}
	factorDiff := map[string]any{"passed": true, "old_t13_canonical_manifest_v2_digest": t13.CanonicalManifestDigest, "new_t14_canonical_manifest_v3_digest": fingerprint.CanonicalManifestDigest, "changed_fields": proof.ConfirmedDivergence, "transport_policy": map[string]string{"old_provider_transport_policy": codex.ProviderTransportPolicyExplicitlyDisabled, "new_provider_transport_policy": codex.ProviderTransportPolicyNativeDefault, "old_websocket_policy": codex.ProviderTransportPolicyExplicitlyDisabled, "new_websocket_policy": codex.ProviderTransportPolicyNativeDefault}, "non_transport_divergence": proof.NonTransportDivergence, "auth_comparison": proof.AuthComparison, "actual_transport": "not_observable_until_live_protocol"}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), map[string]any{"passed": true, "control": "T13 current-auth 0.153.4", "candidate": "T14 current-auth 0.153.4 native-default-policy", "old_t13_fingerprint": t13.CanonicalManifestDigest, "new_t14_fingerprint": fingerprint.CanonicalManifestDigest, "factor_diff": factorDiff, "auth_identity_equal": true, "auth_credential_revision_equal": true, "runtime_config_invocation_equal_after_policy_paths": true, "proxy_environment_equal_and_absent": true, "websocket_policy_only_change": true, "historical_evidence_recomputed_or_overwritten": false}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), map[string]any{"fingerprint_schema_version": fingerprint.FingerprintSchemaVersion, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "canonical_manifest": canonical, "auth_snapshot_used": false, "offline_regression_passed": true}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "status": "preflight_passed", "problem_key": cfg.ProblemKey, "model_calls": 0, "medium_calls": 0, "fingerprint_schema_version": fingerprint.FingerprintSchemaVersion, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "auth_identity_fingerprint": authManifest.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": authManifest.AuthCredentialRevisionFingerprint, "auth_snapshot_used": false, "offline_regression_passed": true, "t13_t14_factor_diff": factorDiff})
}

func RunR03AT14(cfg R03AT14Config) (result R03AT14Result, err error) {
	result = R03AT14Result{Status: "inconclusive", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, DynamicToolCount: 0, Transport: "not_run", ActualTransport: "not_observable", SentinelMatch: "not_run", ToolEvents: []string{}}
	if err = validateT14Config(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV3 {
		return result, errors.New("preflight_failed: T14 preflight is missing, failed or stale")
	}
	var manifest t14ExecutionManifest
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.CanonicalManifestDigest || !manifest.OfflineRegressionPassed || manifest.AuthSnapshotUsed {
		return result, errors.New("preflight_failed: T14 execution manifest is missing or stale")
	}
	if err = manifest.CanonicalManifest.Validate(); err != nil {
		return result, err
	}
	result.CanonicalFingerprint = manifest.CanonicalManifestDigest
	result.ConfiguredTransport = manifest.CanonicalManifest.Transport.WebSocketPolicy
	result.AuthSourceClass = manifest.CanonicalManifest.Auth.AuthSourceClass
	result.AuthIdentityFingerprint = manifest.CanonicalManifest.Auth.AuthIdentityFingerprint
	result.AuthCredentialRevision = manifest.CanonicalManifest.Auth.AuthCredentialRevisionFingerprint
	currentRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return result, fmt.Errorf("preflight_failed: current auth recheck: %w", err)
	}
	currentMaterial, err := codex.ParseAuthMaterial(currentRaw, cfg.AuthSourceClass)
	if err != nil {
		return result, err
	}
	if err = CompareT12AuthSnapshot(manifest.CanonicalManifest.Auth, currentMaterial.Manifest()); err != nil {
		return result, err
	}
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
	args, capability, err := runner.NativeArgsWithTransportPolicy(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, "", runner.NativeTransportPolicyNativeDefault)
	if err != nil {
		return result, err
	}
	if capability != manifest.CanonicalManifest.Combination.CapabilityDigest {
		return result, errors.New("preflight_failed: live T14 capability digest differs from manifest")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return result, err
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return result, err
	}
	result.Readiness = map[string]any{"binary_sha256": digest(binary), "code_mode_host_sha256": digest(helper), "capability_digest": capability, "native_version": "codex-cli " + cfg.Version, "model": cfg.Model, "effort": "medium", "cwd": "/work", "sandbox": "read-only", "proxy_config_digest": manifest.CanonicalManifest.Combination.ProxyConfigDigest, "proxy_category": "not_configured", "dynamic_tool_count": 0, "schema_bytes": 0, "provider_transport_policy": codex.ProviderTransportPolicyNativeDefault, "websocket_policy": codex.ProviderTransportPolicyNativeDefault, "actual_transport": "not_observable", "auth_identity_fingerprint": result.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": result.AuthCredentialRevision}
	readinessAt := time.Now().UTC()
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": result.Readiness, "auth_snapshot_used": false}); err != nil {
		return result, err
	}
	processID := "r03a-t14-canary"
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
	process, err = runner.Start(processID, args, []string{"PATH=/usr/bin:/bin"})
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
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": result.Readiness, "process_start": processStart, "thread_id": thread, "registration_completed_before_turn_start": true, "auth_snapshot_used": false}); err != nil {
		return result, err
	}
	currentRaw, err = os.ReadFile(cfg.AuthFile)
	if err != nil {
		return result, fmt.Errorf("preflight_failed: current auth recheck before medium: %w", err)
	}
	currentMaterial, err = codex.ParseAuthMaterial(currentRaw, cfg.AuthSourceClass)
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
		_ = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), map[string]any{"process_start": processStart, "readiness_manifest": readinessAt, "initialize": map[string]any{"present": trace.InitializeSent && trace.InitializeReceived}, "thread_start": map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted}, "turn_start": map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted}, "user_message_item": trace.UserMessageStarted, "provider_transport_events": "native/protocol.jsonl", "actual_transport": "not_observable", "configured_transport_policy": codex.ProviderTransportPolicyNativeDefault, "first_valid_output": trace.FirstValidOutputAt, "time_to_first_valid_output_ms": trace.TimeToFirstOutputMS, "disconnect_count": trace.ReconnectCount, "reconnect_phases": trace.ReconnectPhases, "recovery_timestamps": trace.RecoveryTimestamps, "turn_completed": trace.TurnCompletedAt, "native_usage_updates": trace.NativeUsageUpdates, "token_counts": trace.TokenUsage, "terminal_state": terminalState, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "business_side_effects": 0})
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
		_ = writeT14Qualification(cfg, manifest, result)
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
	if err = writeT14Qualification(cfg, manifest, result); err != nil {
		return result, err
	}
	return result, nil
}

func writeT14Qualification(cfg R03AT14Config, manifest t14ExecutionManifest, result R03AT14Result) error {
	status, evidence, scheduling := codex.QualificationUnqualified, codex.EvidenceInconclusive, "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.Case == "A" {
		status, evidence, scheduling = codex.QualificationQualified, codex.EvidencePassed, "qualified"
	}
	record := codex.QualificationRecordV3{Layer: codex.QualificationL1BaseTransport, Status: status, SchedulingDecision: scheduling, EvidenceResult: evidence, FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV3, CanonicalManifestDigest: manifest.CanonicalManifestDigest, Combination: manifest.CanonicalManifest.Combination, Auth: manifest.CanonicalManifest.Auth, Transport: manifest.CanonicalManifest.Transport, Reason: "T14 current-auth native-default WebSocket policy case " + result.Case, CreatedAt: result.Finished}
	if !result.Finished.IsZero() {
		record.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), record)
}

func validateT14Config(cfg R03AT14Config) error {
	if cfg.Version != r03aT14Version || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.AuthSourceClass != r03aT12SourceClass || cfg.Binary == "" || cfg.BaselineBinary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T13Evidence == "" || cfg.T10Evidence == "" {
		return errors.New("preflight_failed: T14 fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT14EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T14 evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}
