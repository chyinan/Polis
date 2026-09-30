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

const r03aT13Version = "0.153.4"

type R03AT13Config struct {
	Config
	Version         string
	ProblemKey      string
	AuthSourceClass string
	BaselineBinary  string
	T12Evidence     string
	T10Evidence     string
}

type R03AT13Result struct {
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

func InspectR03AT13(cfg R03AT13Config) error {
	if err := validateT13Config(cfg); err != nil {
		return err
	}
	if err := ensureT13EvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	var t12 t12ExecutionManifest
	var t12Preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	var t12State codex.QualificationRecordV2
	var t10State t11QualificationState
	if err := readT11JSON(filepath.Join(cfg.T12Evidence, "execution-manifest.json"), &t12); err != nil {
		return fmt.Errorf("preflight_failed: T12 execution manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T12Evidence, "preflight.json"), &t12Preflight); err != nil {
		return fmt.Errorf("preflight_failed: T12 preflight: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T12Evidence, "qualification-state.json"), &t12State); err != nil {
		return fmt.Errorf("preflight_failed: T12 qualification state: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T10Evidence, "qualification-state.json"), &t10State); err != nil {
		return fmt.Errorf("preflight_failed: T10 qualification state: %w", err)
	}
	if !t12Preflight.Passed || t12Preflight.ProblemKey != cfg.ProblemKey || t12Preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 || !t12.OfflineRegressionPassed || t12.CanonicalManifestDigest != t12Preflight.CanonicalManifestDigest || t12State.Status != codex.QualificationUnqualified || t12State.EvidenceResult != codex.EvidenceInconclusive {
		return errors.New("preflight_failed: T12 current-auth control is missing or not the sealed inconclusive baseline")
	}
	if err := t12.CanonicalManifest.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: T12 canonical manifest invalid: %w", err)
	}
	if t10State.Status != "qualified" || t10State.EvidenceResult != "passed" || t10State.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersion || t10State.Combination.CodexVersion != r03aT13Version {
		return errors.New("preflight_failed: T10 0.153.4 qualification is missing or stale")
	}
	for _, path := range historicalAllowancePaths() {
		if raw, err := os.ReadFile(path); err != nil || len(raw) == 0 {
			return fmt.Errorf("preflight_failed: sealed historical allowance unavailable: %s", path)
		}
	}
	t12Allowance, err := os.ReadFile(filepath.Join(cfg.T12Evidence, "allowance.json"))
	if err != nil || !bytes.Contains(t12Allowance, []byte(`"medium_turns":1`)) || !bytes.Contains(t12Allowance, []byte(`"high_turns":0`)) {
		return errors.New("preflight_failed: T12 allowance is not sealed at one Medium and zero High")
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
	if err := CompareT12AuthSnapshot(t12.CanonicalManifest.Auth, authManifest); err != nil {
		return err
	}
	if err := authManifest.Validate(); err != nil || authManifest.AuthIdentityFingerprintStatus != "available" || authManifest.AuthCredentialRevisionFingerprintStatus != "available" {
		return errors.New("preflight_failed: current auth identity or credential revision is unavailable")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return fmt.Errorf("preflight_failed: T13 0.153.4 binary: %w", err)
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return fmt.Errorf("preflight_failed: T13 code-mode-host: %w", err)
	}
	if digest(binary) != t10State.Combination.BinarySHA256 || digest(helper) != t10State.Combination.CodeModeHostSHA256 {
		return errors.New("preflight_failed: T13 binary/helper does not match T10 qualification")
	}
	versionOutput, err := runner.Run([]string{cfg.Binary, "--version"}, []string{"PATH=/usr/bin:/bin"}, 10*time.Second)
	if err != nil || strings.TrimSpace(string(versionOutput)) != "codex-cli "+r03aT13Version {
		return errors.New("preflight_failed: T13 binary version output mismatch")
	}
	if err := os.MkdirAll(cfg.Root, 0700); err != nil {
		return err
	}
	oldHome := filepath.Join(cfg.Root, "preflight-old-home")
	newHome := filepath.Join(cfg.Root, "preflight-new-home")
	oldArgs, oldCapability, err := runner.NativeArgs(cfg.BaselineBinary, oldHome, cfg.AuthFile, "")
	if err != nil {
		return fmt.Errorf("preflight_failed: T12 native args: %w", err)
	}
	newArgs, newCapability, err := runner.NativeArgs(cfg.Binary, newHome, cfg.AuthFile, "")
	if err != nil {
		return fmt.Errorf("preflight_failed: T13 native args: %w", err)
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
	if oldCapability != t12.CanonicalManifest.Combination.CapabilityDigest || newCapability != t10State.Combination.CapabilityDigest || !bytes.Equal(oldConfig, newConfig) || !reflect.DeepEqual(normalizeT11NativeArgs(oldArgs, oldHome, cfg.BaselineBinary, oldHelper), normalizeT11NativeArgs(newArgs, newHome, cfg.Binary, newHelper)) || !bytes.Contains(newConfig, []byte("supports_websockets = false")) || !bytes.Contains(newConfig, []byte("responses_websockets = false")) {
		return errors.New("preflight_failed: T12/T13 runtime, config or invocation diverged outside version factor")
	}
	regressionOutput, err := runner.Run([]string{"bash", "scripts/go.sh", "test", "./internal/codex", "./internal/probe", "-run", "R03AT", "-count=1"}, []string{"PATH=/usr/bin:/bin", "GOPROXY=off", "POLIS_CODEX_T10_BINARY="}, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("preflight_failed: offline T1/T2.1 regression: %w; output=%s", err, string(regressionOutput))
	}
	newCombination := t10State.Combination
	newCombination.AuthSourceClass = cfg.AuthSourceClass
	promptDigest := digest([]byte(r03aT11Instruction))
	oldFactors := T11CanaryFactors{Combination: t12.CanonicalManifest.Combination, AuthManifest: t12.CanonicalManifest.Auth, ProxyEnvironment: map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"}, CWD: "/work", Home: "/home/codex", Invocation: r03aT11Invocation, Sandbox: "read-only", WebSocketPolicy: "disabled", DynamicToolCount: 0, ToolSchemaBytes: 0, DeveloperInstructionDigest: promptDigest, PromptDigest: promptDigest}
	newFactors := oldFactors
	newFactors.Combination = newCombination
	newFactors.AuthManifest = authManifest
	proof, err := CompareT11Factors(oldFactors, newFactors)
	if err != nil {
		return err
	}
	if len(proof.ConfirmedDivergence) != len(t11VersionDerivedFields) || len(proof.NonVersionDivergence) != 0 {
		return errors.New("preflight_failed: T12/T13 factor diff contains an unexpected divergence")
	}
	fingerprint := newCombination.CurrentFingerprintV2(authManifest)
	canonical := codex.CanonicalManifestV2{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV2, Combination: newCombination, Auth: authManifest}
	if err := canonical.Validate(); err != nil || fingerprint.Validate() != nil {
		return errors.New("preflight_failed: T13 canonical-manifest-v2 is invalid")
	}
	factorDiff := map[string]any{"passed": true, "control": "T12 current-auth 0.151.0", "candidate": "T13 current-auth 0.153.4", "changed_fields": proof.ConfirmedDivergence, "version_derived_fields": proof.VersionDerivedFields, "non_version_divergence": proof.NonVersionDivergence, "auth_comparison": proof.AuthComparison, "old_t12_canonical_manifest_digest": t12.CanonicalManifestDigest, "new_t13_canonical_manifest_digest": fingerprint.CanonicalManifestDigest}
	manifest := map[string]any{"fingerprint_schema_version": fingerprint.FingerprintSchemaVersion, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "canonical_manifest": canonical, "auth_snapshot_used": false, "offline_regression_passed": true}
	comparison := map[string]any{"passed": true, "control": "T12 current-auth 0.151.0", "candidate": "T13 current-auth 0.153.4", "old_t12_fingerprint": t12.CanonicalManifestDigest, "new_t13_fingerprint": fingerprint.CanonicalManifestDigest, "factor_diff": factorDiff, "auth_identity_equal": true, "auth_credential_revision_equal": true, "runtime_config_invocation_equal_after_version_paths": true, "proxy_environment_equal_and_absent": true, "websocket_policy_equal_disabled": true, "offline_regression": "passed", "historical_evidence_recomputed_or_overwritten": false}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), comparison); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "status": "preflight_passed", "problem_key": cfg.ProblemKey, "model_calls": 0, "medium_calls": 0, "fingerprint_schema_version": fingerprint.FingerprintSchemaVersion, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "auth_identity_fingerprint": authManifest.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": authManifest.AuthCredentialRevisionFingerprint, "auth_snapshot_used": false, "offline_regression_passed": true, "t12_t13_factor_diff": factorDiff})
}

func RunR03AT13(cfg R03AT13Config) (result R03AT13Result, err error) {
	result = R03AT13Result{Status: "inconclusive", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, DynamicToolCount: 0, Transport: "not_run", SentinelMatch: "not_run", ToolEvents: []string{}}
	if err = validateT13Config(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 {
		return result, errors.New("preflight_failed: T13 preflight is missing, failed or stale")
	}
	var manifest struct {
		FingerprintSchemaVersion string                    `json:"fingerprint_schema_version"`
		CanonicalManifestDigest  string                    `json:"canonical_manifest_digest"`
		CanonicalManifest        codex.CanonicalManifestV2 `json:"canonical_manifest"`
		AuthSnapshotUsed         bool                      `json:"auth_snapshot_used"`
		OfflineRegressionPassed  bool                      `json:"offline_regression_passed"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.CanonicalManifestDigest || !manifest.OfflineRegressionPassed || manifest.AuthSnapshotUsed {
		return result, errors.New("preflight_failed: T13 execution manifest is missing or stale")
	}
	if err = manifest.CanonicalManifest.Validate(); err != nil {
		return result, err
	}
	result.CanonicalFingerprint = manifest.CanonicalManifestDigest
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
	args, capability, err := runner.NativeArgs(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, "")
	if err != nil {
		return result, err
	}
	if capability != manifest.CanonicalManifest.Combination.CapabilityDigest {
		return result, errors.New("preflight_failed: live capability digest differs from T13 manifest")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return result, err
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return result, err
	}
	result.Readiness = map[string]any{"binary_sha256": digest(binary), "code_mode_host_sha256": digest(helper), "capability_digest": capability, "native_version": "codex-cli " + cfg.Version, "model": cfg.Model, "effort": "medium", "cwd": "/work", "sandbox": "read-only", "proxy_config_digest": manifest.CanonicalManifest.Combination.ProxyConfigDigest, "proxy_category": "not_configured", "dynamic_tool_count": 0, "schema_bytes": 0, "websocket_policy": "disabled", "auth_identity_fingerprint": result.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": result.AuthCredentialRevision}
	readinessAt := time.Now().UTC()
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": result.Readiness, "auth_snapshot_used": false}); err != nil {
		return result, err
	}
	processID := "r03a-t13-canary"
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
			_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0})
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
		_ = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), map[string]any{"process_start": processStart, "readiness_manifest": readinessAt, "initialize": map[string]any{"present": trace.InitializeSent && trace.InitializeReceived}, "thread_start": map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted}, "turn_start": map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted}, "user_message_item": trace.UserMessageStarted, "first_valid_output": trace.FirstValidOutputAt, "time_to_first_valid_output_ms": trace.TimeToFirstOutputMS, "provider_transport_events": "native/protocol.jsonl", "disconnect_count": trace.ReconnectCount, "reconnect_phases": trace.ReconnectPhases, "recovery_timestamps": trace.RecoveryTimestamps, "turn_completed": trace.TurnCompletedAt, "native_usage_updates": trace.NativeUsageUpdates, "token_counts": trace.TokenUsage, "terminal_state": trace.TerminalState, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "business_side_effects": 0})
		_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "readiness_manifest": readinessAt, "initialize": trace.InitializeSent && trace.InitializeReceived, "thread_started": trace.ThreadStarted, "turn_started": trace.TurnStarted, "user_message_item": trace.UserMessageStarted, "first_valid_output": trace.FirstValidOutputAt, "turn_completed": trace.TurnCompletedAt, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "tool_events": result.ToolEvents})
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
		_ = writeT13Qualification(cfg, manifest, result)
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
	if err = writeT13Qualification(cfg, manifest, result); err != nil {
		return result, err
	}
	return result, nil
}

// RepairR03AT13Evidence recomputes only derived summaries from the immutable
// T13 native protocol log. It never starts a process and never touches the
// allowance counters.
func RepairR03AT13Evidence(cfg R03AT13Config) error {
	var result R03AT13Result
	if err := readT11JSON(filepath.Join(cfg.Evidence, "result.json"), &result); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Evidence, "native", "protocol.jsonl"))
	if err != nil {
		return err
	}
	trace, err := SummarizeT11Protocol(raw, r03aT11Sentinel)
	if err != nil {
		return err
	}
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
	} else if strings.Contains(result.Error, "first_valid_output_deadline_exceeded") {
		result.TurnState = "first_valid_output_deadline_exceeded"
	}
	result.Status = "inconclusive"
	result.Transport = "inconclusive"
	result.Case = "B"

	var session map[string]any
	if err := readT11JSON(filepath.Join(cfg.Evidence, "canary-session.json"), &session); err != nil {
		return err
	}
	session["stop_receipt"] = result.StopReceipt
	session["stop_confirmed"] = result.StopConfirmed
	session["initialize"] = trace.InitializeSent && trace.InitializeReceived
	session["thread_started"] = trace.ThreadStarted
	session["turn_started"] = trace.TurnStarted
	session["user_message_item"] = trace.UserMessageStarted
	session["first_valid_output"] = trace.FirstValidOutputAt
	session["turn_completed"] = trace.TurnCompletedAt
	session["derived_repair"] = true
	terminalState := trace.TerminalState
	if terminalState == "" {
		terminalState = result.TurnState
	}
	traceRecord := map[string]any{
		"process_start":                 session["process_start"],
		"readiness_manifest":            session["readiness_manifest"],
		"initialize":                    map[string]any{"present": trace.InitializeSent && trace.InitializeReceived},
		"thread_start":                  map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted},
		"turn_start":                    map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted},
		"user_message_item":             trace.UserMessageStarted,
		"first_valid_output":            trace.FirstValidOutputAt,
		"time_to_first_valid_output_ms": trace.TimeToFirstOutputMS,
		"provider_transport_events":     "native/protocol.jsonl",
		"disconnect_count":              trace.ReconnectCount,
		"reconnect_phases":              trace.ReconnectPhases,
		"recovery_timestamps":           trace.RecoveryTimestamps,
		"turn_completed":                trace.TurnCompletedAt,
		"native_usage_updates":          trace.NativeUsageUpdates,
		"token_counts":                  trace.TokenUsage,
		"terminal_state":                terminalState,
		"process_stop":                  session["process_stop"],
		"stop_receipt":                  result.StopReceipt,
		"stop_confirmed":                result.StopConfirmed,
		"dynamic_tool_count":            0,
		"business_side_effects":         0,
		"derived_repair":                true,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), traceRecord); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), session); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
}

func writeT13Qualification(cfg R03AT13Config, manifest struct {
	FingerprintSchemaVersion string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                    `json:"canonical_manifest_digest"`
	CanonicalManifest        codex.CanonicalManifestV2 `json:"canonical_manifest"`
	AuthSnapshotUsed         bool                      `json:"auth_snapshot_used"`
	OfflineRegressionPassed  bool                      `json:"offline_regression_passed"`
}, result R03AT13Result) error {
	status, evidence, scheduling := codex.QualificationUnqualified, codex.EvidenceInconclusive, "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.Case == "A" {
		status, evidence, scheduling = codex.QualificationQualified, codex.EvidencePassed, "qualified"
	}
	record := codex.QualificationRecordV2{Layer: codex.QualificationL1BaseTransport, Status: status, SchedulingDecision: scheduling, EvidenceResult: evidence, FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV2, CanonicalManifestDigest: manifest.CanonicalManifestDigest, Combination: manifest.CanonicalManifest.Combination, Auth: manifest.CanonicalManifest.Auth, Reason: "T13 current-auth version-only case " + result.Case, CreatedAt: result.Finished}
	if !result.Finished.IsZero() {
		record.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), record)
}

func validateT13Config(cfg R03AT13Config) error {
	if cfg.Version != r03aT13Version || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.AuthSourceClass != r03aT12SourceClass || cfg.Binary == "" || cfg.BaselineBinary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T12Evidence == "" || cfg.T10Evidence == "" {
		return errors.New("preflight_failed: T13 fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT13EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T13 evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}
