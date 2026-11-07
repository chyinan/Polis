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

const (
	r03aT12Version     = "0.151.0"
	r03aT12SourceClass = "mounted_codex_auth_file"
)

type R03AT12Config struct {
	Config
	Version         string
	ProblemKey      string
	AuthSourceClass string
	T9Evidence      string
}

type t12ExecutionManifest struct {
	FingerprintSchemaVersion string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                    `json:"canonical_manifest_digest"`
	CanonicalManifest        codex.CanonicalManifestV2 `json:"canonical_manifest"`
	AuthSnapshotUsed         bool                      `json:"auth_snapshot_used"`
	OfflineRegressionPassed  bool                      `json:"offline_regression_passed"`
}

type R03AT12Result struct {
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

func InspectR03AT12(cfg R03AT12Config) error {
	if err := validateT12Config(cfg); err != nil {
		return err
	}
	if err := ensureT12EvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	var t9 t11ExecutionManifest
	var t9Readiness t11ReadinessManifest
	var t9State t11QualificationState
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "execution-manifest.json"), &t9); err != nil {
		return fmt.Errorf("preflight_failed: T9 execution manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "readiness-manifest.json"), &t9Readiness); err != nil {
		return fmt.Errorf("preflight_failed: T9 readiness manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "qualification-state.json"), &t9State); err != nil {
		return fmt.Errorf("preflight_failed: T9 qualification state: %w", err)
	}
	if t9State.ExecutionKey != t9.Combination.Fingerprint() || t9State.Status != "unqualified" || t9State.EvidenceResult != "inconclusive" || t9State.SchedulingDecision != "unqualified_for_business_execution" {
		return errors.New("preflight_failed: T9 no-proxy record is not the sealed historical state")
	}
	if t9.Combination.CodexVersion != r03aT12Version || t9.Combination.Model != "gpt-5.6-luna" || t9.Combination.Effort != "medium" || t9.Combination.RuntimeProfile != "wsl-linux-amd64" || t9.Combination.SandboxClass != "read-only" || t9.Combination.ProxyConfigDigest != digest([]byte("proxy:none")) || t9.DynamicToolCount != 0 || t9.ToolSchemaBytes != 0 {
		return errors.New("preflight_failed: T9 baseline factors are not exact")
	}
	if t9.ProxyEnvironment["HTTP_PROXY"] != "absent" || t9.ProxyEnvironment["HTTPS_PROXY"] != "absent" || t9.ProxyEnvironment["ALL_PROXY"] != "absent" || t9.ProxyEnvironment["NO_PROXY"] != "absent" || t9.ProxyEnvironment["POLIS_NATIVE_PROXY"] != "absent" {
		return errors.New("preflight_failed: T9 proxy environment was not absent")
	}
	if t9.DeveloperInstructionDigest != digest([]byte(r03aT11Instruction)) || t9.PromptDigest != digest([]byte(r03aT11Instruction)) {
		return errors.New("preflight_failed: T9 tiny instruction/prompt digest mismatch")
	}
	t9Protocol, err := os.ReadFile(filepath.Join(cfg.T9Evidence, "native", "protocol.jsonl"))
	if err != nil {
		return fmt.Errorf("preflight_failed: T9 native protocol: %w", err)
	}
	for _, required := range []string{`"codexHome":"/home/codex"`, `"cwd":"/work"`, `"sandbox":"read-only"`, `"dynamicTools":[]`, `"model":"gpt-5.6-luna"`, `"model_reasoning_effort":"medium"`, `"developerInstructions":"Reply with exactly: POLIS_TRANSPORT_CANARY_OK"`, `"method":"thread/started"`, `"method":"turn/started"`} {
		if !bytes.Contains(t9Protocol, []byte(required)) {
			return errors.New("preflight_failed: T9 protocol factor missing: " + required)
		}
	}
	for _, path := range historicalAllowancePaths() {
		if raw, readErr := os.ReadFile(path); readErr != nil || len(raw) == 0 {
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
	if err := authManifest.Validate(); err != nil || authManifest.AuthIdentityFingerprintStatus != "available" {
		return errors.New("preflight_failed: current stable auth identity is unavailable")
	}

	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return fmt.Errorf("preflight_failed: 0.151.0 binary: %w", err)
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return fmt.Errorf("preflight_failed: 0.151.0 code-mode-host: %w", err)
	}
	if digest(binary) != t9.Combination.BinarySHA256 || digest(helper) != t9.Combination.CodeModeHostSHA256 {
		return errors.New("preflight_failed: historical 0.151.0 binary or helper digest changed")
	}
	versionOutput, err := runner.Run([]string{cfg.Binary, "--version"}, []string{"PATH=/usr/bin:/bin"}, 10*time.Second)
	if err != nil || strings.TrimSpace(string(versionOutput)) != "codex-cli "+r03aT12Version {
		return errors.New("preflight_failed: historical binary version output mismatch")
	}
	if err := os.MkdirAll(cfg.Root, 0700); err != nil {
		return err
	}
	args, capability, err := runner.NativeArgs(cfg.Binary, filepath.Join(cfg.Root, "preflight-home"), cfg.AuthFile, "")
	if err != nil {
		return fmt.Errorf("preflight_failed: native args: %w", err)
	}
	if capability != t9.Combination.CapabilityDigest || !strings.HasSuffix(strings.Join(args, " "), "/codex app-server --stdio") || strings.Contains(strings.Join(args, " "), "HTTP_PROXY") || strings.Contains(strings.Join(args, " "), "HTTPS_PROXY") {
		return errors.New("preflight_failed: effective no-proxy invocation diverged")
	}
	config, err := os.ReadFile(filepath.Join(cfg.Root, "preflight-home", "config.toml"))
	if err != nil || !bytes.Contains(config, []byte("supports_websockets = false")) || !bytes.Contains(config, []byte("responses_websockets = false")) {
		return errors.New("preflight_failed: WebSocket-disabled config missing")
	}
	regressionOutput, err := runner.Run([]string{"bash", "scripts/go.sh", "test", "./internal/codex", "./internal/probe", "-run", "R03AT", "-count=1"}, []string{"PATH=/usr/bin:/bin", "GOPROXY=off", "POLIS_CODEX_T10_BINARY="}, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("preflight_failed: T1/T2.1 offline regression: %w; output=%s", err, string(regressionOutput))
	}

	combination := t9.Combination
	combination.AuthSourceClass = cfg.AuthSourceClass
	canonical := codex.CanonicalManifestV2{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV2, Combination: combination, Auth: authManifest}
	if err := canonical.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: T12 canonical manifest: %w", err)
	}
	fingerprint := combination.CurrentFingerprintV2(authManifest)
	manifest := t12ExecutionManifest{FingerprintSchemaVersion: fingerprint.FingerprintSchemaVersion, CanonicalManifestDigest: fingerprint.CanonicalManifestDigest, CanonicalManifest: canonical, AuthSnapshotUsed: false, OfflineRegressionPassed: true}
	comparison := map[string]any{
		"passed":                                               true,
		"baseline":                                             "current-auth 0.151.0 minimal transport; not a version comparison",
		"historical_t9_execution_key":                          t9State.ExecutionKey,
		"t12_canonical_manifest_digest":                        fingerprint.CanonicalManifestDigest,
		"fingerprint_schema_version":                           codex.CanonicalManifestFingerprintSchemaVersionV2,
		"auth_source_class":                                    authManifest.AuthSourceClass,
		"auth_identity_fingerprint_schema_version":             authManifest.AuthIdentityFingerprintSchemaVersion,
		"auth_identity_fingerprint":                            authManifest.AuthIdentityFingerprint,
		"auth_credential_revision_fingerprint_schema_version":  authManifest.AuthCredentialRevisionFingerprintSchemaVersion,
		"auth_credential_revision_fingerprint":                 authManifest.AuthCredentialRevisionFingerprint,
		"auth_snapshot_used":                                   false,
		"auth_material_path_recorded":                          false,
		"current_auth_material_recheck_required_before_medium": true,
		"binary_sha256":                                        digest(binary),
		"code_mode_host_sha256":                                digest(helper),
		"capability_digest":                                    capability,
		"proxy_environment":                                    map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"},
		"websocket_policy":                                     "disabled",
		"dynamic_tool_count":                                   0,
		"offline_regression":                                   "passed",
		"historical_allowances_checked_read_only":              true,
		"historical_evidence_recomputed_or_overwritten":        false,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), comparison); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                               true,
		"status":                               "preflight_passed",
		"problem_key":                          cfg.ProblemKey,
		"model_calls":                          0,
		"medium_calls":                         0,
		"canonical_manifest_digest":            fingerprint.CanonicalManifestDigest,
		"fingerprint_schema_version":           fingerprint.FingerprintSchemaVersion,
		"auth_identity_fingerprint":            authManifest.AuthIdentityFingerprint,
		"auth_credential_revision_fingerprint": authManifest.AuthCredentialRevisionFingerprint,
		"auth_snapshot_used":                   false,
		"offline_regression_passed":            true,
		"current_auth_recheck_before_medium":   true,
	})
}

func RunR03AT12(cfg R03AT12Config) (result R03AT12Result, err error) {
	result = R03AT12Result{Status: "inconclusive", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, DynamicToolCount: 0, Transport: "not_run", SentinelMatch: "not_run", ToolEvents: []string{}}
	if err = validateT12Config(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 {
		return result, errors.New("preflight_failed: T12 preflight is missing, failed or stale")
	}
	var manifest t12ExecutionManifest
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.CanonicalManifestDigest || !manifest.OfflineRegressionPassed {
		return result, errors.New("preflight_failed: T12 execution manifest is missing or stale")
	}
	if err = manifest.CanonicalManifest.Validate(); err != nil {
		return result, fmt.Errorf("preflight_failed: T12 canonical manifest invalid: %w", err)
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
		return result, fmt.Errorf("preflight_failed: current auth recheck: %w", err)
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
		return result, errors.New("preflight_failed: live capability digest differs from T12 manifest")
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

	processID := "r03a-t12-canary"
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
		return result, fmt.Errorf("preflight_failed: current auth recheck before medium: %w", err)
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
		_ = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), map[string]any{"process_start": processStart, "readiness_manifest": readinessAt, "initialize": map[string]any{"present": trace.InitializeSent && trace.InitializeReceived}, "thread_start": map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted}, "turn_start": map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted}, "user_message_item": trace.UserMessageStarted, "first_valid_output": trace.FirstValidOutputAt, "time_to_first_valid_output_ms": trace.TimeToFirstOutputMS, "provider_transport_events": "native/protocol.jsonl", "disconnect_count": trace.ReconnectCount, "reconnect_phases": trace.ReconnectPhases, "recovery_timestamps": trace.RecoveryTimestamps, "turn_completed": trace.TurnCompletedAt, "native_usage_updates": trace.NativeUsageUpdates, "token_counts": trace.TokenUsage, "terminal_state": trace.TerminalState, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "business_side_effects": 0})
		_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"allowance_start": result.Started, "process_start": processStart, "readiness_manifest": readinessAt, "initialize": trace.InitializeSent && trace.InitializeReceived, "thread_started": trace.ThreadStarted, "turn_started": trace.TurnStarted, "user_message_item": trace.UserMessageStarted, "first_valid_output": trace.FirstValidOutputAt, "turn_completed": trace.TurnCompletedAt, "process_stop": processStop, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "tool_events": result.ToolEvents})
		sessionWritten = true
	}
	if turnErr != nil {
		if strings.Contains(turnErr.Error(), "first_valid_output_deadline_exceeded") {
			result.TurnState = "first_valid_output_deadline_exceeded"
		} else if strings.Contains(turnErr.Error(), "reconnect_deadline_exceeded") {
			result.TurnState = "reconnect_deadline_exceeded"
		}
		if strings.Contains(turnErr.Error(), "deadline") || strings.Contains(turnErr.Error(), "outcome_unknown") || strings.Contains(turnErr.Error(), "reconciliation") {
			result.Transport = "inconclusive"
			result.Case = "B"
		} else {
			result.Transport = "failed"
			result.Case = "C"
		}
		result.Status = "inconclusive"
		result.Finished = time.Now().UTC()
		_ = writeT12Qualification(cfg, manifest, result)
		return result, turnErr
	}
	if result.TurnState == "completed" && result.FirstValidOutput && !result.UnresolvedTransportState && result.SentinelMatch != "not_run" {
		result.Status = "passed"
		result.Transport = "passed"
		result.Case = "A"
	} else {
		result.Status = "inconclusive"
		result.Transport = "inconclusive"
		result.Case = "B"
	}
	result.Finished = time.Now().UTC()
	if err = writeT12Qualification(cfg, manifest, result); err != nil {
		return result, err
	}
	return result, nil
}

// RepairR03AT12Evidence only recomputes derived summaries from the already
// sealed native protocol log. It never starts a process or touches allowance.
func RepairR03AT12Evidence(cfg R03AT12Config) error {
	var result R03AT12Result
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
	terminalState := trace.TerminalState
	if terminalState == "" {
		terminalState = result.TurnState
	}
	result.Transport = "inconclusive"
	result.Status = "inconclusive"
	result.Case = "B"

	var session map[string]any
	if err := readT11JSON(filepath.Join(cfg.Evidence, "canary-session.json"), &session); err != nil {
		return err
	}
	traceRecord := map[string]any{
		"process_start":                          session["process_start"],
		"readiness_manifest":                     "readiness-manifest.json",
		"initialize":                             map[string]any{"present": trace.InitializeSent && trace.InitializeReceived},
		"thread_start":                           map[string]any{"sent": trace.ThreadStartSent, "started": trace.ThreadStarted},
		"turn_start":                             map[string]any{"sent": trace.TurnStartSent, "started": trace.TurnStarted},
		"user_message_item":                      trace.UserMessageStarted,
		"first_valid_output":                     trace.FirstValidOutputAt,
		"time_to_first_valid_output_ms":          trace.TimeToFirstOutputMS,
		"provider_transport_events":              "native/protocol.jsonl",
		"disconnect_count":                       trace.ReconnectCount,
		"reconnect_phases":                       trace.ReconnectPhases,
		"recovery_timestamps":                    trace.RecoveryTimestamps,
		"turn_completed":                         trace.TurnCompletedAt,
		"native_usage_updates":                   trace.NativeUsageUpdates,
		"token_counts":                           trace.TokenUsage,
		"terminal_state":                         terminalState,
		"process_stop":                           session["process_stop"],
		"stop_receipt":                           session["stop_receipt"],
		"stop_confirmed":                         session["stop_confirmed"],
		"dynamic_tool_count":                     0,
		"business_side_effects":                  0,
		"derived_from_protocol_after_parser_fix": true,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), traceRecord); err != nil {
		return err
	}
	session["initialize"] = trace.InitializeSent && trace.InitializeReceived
	session["thread_started"] = trace.ThreadStarted
	session["turn_started"] = trace.TurnStarted
	session["user_message_item"] = trace.UserMessageStarted
	session["first_valid_output"] = trace.FirstValidOutputAt
	session["turn_completed"] = trace.TurnCompletedAt
	session["derived_from_protocol_after_parser_fix"] = true
	if err := writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), session); err != nil {
		return err
	}
	var qualification codex.QualificationRecordV2
	if err := readT11JSON(filepath.Join(cfg.Evidence, "qualification-state.json"), &qualification); err == nil {
		qualification.CreatedAt = result.Finished
		if !result.Finished.IsZero() {
			qualification.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
		}
		if err := writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), qualification); err != nil {
			return err
		}
	}
	return writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
}

func writeT12Qualification(cfg R03AT12Config, manifest t12ExecutionManifest, result R03AT12Result) error {
	status, evidence, scheduling := codex.QualificationUnqualified, codex.EvidenceInconclusive, "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.Case == "A" {
		status, evidence, scheduling = codex.QualificationQualified, codex.EvidencePassed, "qualified"
	}
	record := codex.QualificationRecordV2{Layer: codex.QualificationL1BaseTransport, Status: status, SchedulingDecision: scheduling, EvidenceResult: evidence, FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV2, CanonicalManifestDigest: manifest.CanonicalManifestDigest, Combination: manifest.CanonicalManifest.Combination, Auth: manifest.CanonicalManifest.Auth, Reason: "T12 current-auth minimal transport case " + result.Case, CreatedAt: result.Finished}
	if !result.Finished.IsZero() {
		record.ExpiresAt = result.Finished.Add(7 * 24 * time.Hour)
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), record)
}

func validateT12Config(cfg R03AT12Config) error {
	if cfg.Version != r03aT12Version || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.AuthSourceClass != r03aT12SourceClass || cfg.Binary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T9Evidence == "" {
		return errors.New("preflight_failed: T12 fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT12EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T12 evidence directory is not fresh; refusing retry/reset")
	}
	return nil
}

func historicalAllowancePaths() []string {
	return []string{
		"evidence/development/r0.1/real/allowance.json",
		"evidence/development/r0.2/luna-1/allowance.json",
		"evidence/development/r0.2h2/allowance.json",
		"evidence/development/r0.2h3/allowance.json",
		"evidence/development/r0.3a-real/luna-1/allowance.json",
		"evidence/development/r0.3a-t2/luna-1/allowance.json",
		"evidence/development/r0.3a-t3/luna-1/allowance.json",
		"evidence/development/r0.3a-t6/luna-1/allowance.json",
		"evidence/development/r0.3a-t9/luna-1/allowance.json",
	}
}
