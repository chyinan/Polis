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

const (
	r03aT11Version     = "0.153.4"
	r03aT11Sentinel    = "POLIS_TRANSPORT_CANARY_OK"
	r03aT11Instruction = "Reply with exactly: " + r03aT11Sentinel
	r03aT11Invocation  = "app-server --stdio"
)

type R03AT11Config struct {
	Config
	Version          string
	HistoricalBinary string
	ProblemKey       string
	T9Evidence       string
	T10Evidence      string
}

type R03AT11Result struct {
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
	TurnState                string           `json:"turn_state"`
	Transport                string           `json:"transport"`
	SentinelMatch            string           `json:"sentinel_match"`
	FirstValidOutput         bool             `json:"first_valid_output"`
	FirstValidOutputText     string           `json:"first_valid_output_text,omitempty"`
	FirstValidOutputAt       *time.Time       `json:"first_valid_output_at,omitempty"`
	FirstValidOutputDeltaMS  int64            `json:"first_valid_output_delta_ms,omitempty"`
	ReconnectCount           int              `json:"reconnect_count"`
	ReconnectPhases          []string         `json:"reconnect_phases"`
	FirstDisconnectDeltaMS   int64            `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS          int64            `json:"recovery_delta_ms,omitempty"`
	TotalElapsedMS           int64            `json:"total_elapsed_ms"`
	NativeUsageUpdates       int              `json:"native_usage_updates"`
	TokenUsage               codex.TokenUsage `json:"token_usage"`
	StopReceipt              string           `json:"stop_receipt,omitempty"`
	StopConfirmed            bool             `json:"stop_confirmed"`
	ToolEvents               []string         `json:"tool_events"`
	OldT9Fingerprint         string           `json:"old_t9_fingerprint"`
	NewT11Fingerprint        string           `json:"new_t11_fingerprint"`
	UnresolvedTransportState bool             `json:"unresolved_transport_state"`
	Error                    string           `json:"error,omitempty"`
}

type t11ExecutionManifest struct {
	Combination                codex.ExecutionCombination    `json:"combination"`
	FingerprintSchemaVersion   string                        `json:"fingerprint_schema_version"`
	CanonicalManifestDigest    string                        `json:"canonical_manifest_digest"`
	AuthIdentityFingerprint    string                        `json:"auth_identity_fingerprint,omitempty"`
	Auth                       codex.AuthFingerprintManifest `json:"auth"`
	ProxyEnvironment           map[string]string             `json:"proxy_environment"`
	DynamicToolCount           int                           `json:"dynamic_tool_count"`
	ToolSchemaBytes            int                           `json:"tool_schema_bytes"`
	DeveloperInstructionDigest string                        `json:"developer_instruction_digest"`
	PromptDigest               string                        `json:"prompt_digest"`
}

type t11ReadinessManifest struct {
	Readiness struct {
		BinarySHA256       string `json:"binary_sha256"`
		CodeModeHostSHA256 string `json:"code_mode_host_sha256"`
		CapabilityDigest   string `json:"capability_digest"`
		ProxyConfigDigest  string `json:"proxy_config_digest"`
		NativeVersion      string `json:"native_version"`
		Model              string `json:"model"`
		Effort             string `json:"effort"`
		CWD                string `json:"cwd"`
		Sandbox            string `json:"sandbox"`
		DynamicToolCount   int    `json:"dynamic_tool_count"`
		SchemaBytes        int    `json:"schema_bytes"`
	} `json:"readiness"`
}

type t11QualificationState struct {
	Layer                    string                     `json:"layer"`
	Status                   string                     `json:"status"`
	SchedulingDecision       string                     `json:"scheduling_decision"`
	EvidenceResult           string                     `json:"evidence_result"`
	ExecutionKey             string                     `json:"execution_key"`
	FingerprintSchemaVersion string                     `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                     `json:"canonical_manifest_digest"`
	Combination              codex.ExecutionCombination `json:"combination"`
	AuthIdentityFingerprint  string                     `json:"auth_identity_fingerprint"`
}

func InspectR03AT11(cfg R03AT11Config) error {
	if err := validateT11Config(cfg); err != nil {
		return err
	}
	if err := ensureT11EvidenceFresh(cfg.Evidence); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}

	proof, comparison, manifest, err := buildT11Preflight(cfg)
	if err != nil {
		_ = writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
			"passed":      false,
			"status":      "preflight_failed",
			"model_calls": 0,
			"error":       err.Error(),
		})
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), comparison); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                     true,
		"status":                     "preflight_passed",
		"problem_key":                cfg.ProblemKey,
		"model_calls":                0,
		"dynamic_tool_count":         0,
		"old_t9_fingerprint":         proof.OldT9Fingerprint,
		"new_t11_fingerprint":        proof.NewT11Fingerprint,
		"fingerprint_schema_version": proof.FingerprintSchemaVersion,
		"only_change":                "Codex/app-server 0.151.0 -> 0.153.4 version factor",
		"proof":                      proof,
	})
}

func buildT11Preflight(cfg R03AT11Config) (T11PreflightProof, map[string]any, t11ExecutionManifest, error) {
	var t9Manifest t11ExecutionManifest
	var t9Readiness t11ReadinessManifest
	var t9State t11QualificationState
	var t10State t11QualificationState
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "execution-manifest.json"), &t9Manifest); err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T9 execution manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "readiness-manifest.json"), &t9Readiness); err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T9 readiness manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T9Evidence, "qualification-state.json"), &t9State); err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T9 qualification state: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T10Evidence, "qualification-state.json"), &t10State); err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T10 qualification state: %w", err)
	}

	if t9State.ExecutionKey != t9Manifest.Combination.Fingerprint() || t9State.Status != "unqualified" || t9State.EvidenceResult != "inconclusive" || t9State.SchedulingDecision != "unqualified_for_business_execution" {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 no-proxy history is not the sealed unqualified record")
	}
	if t9Manifest.Combination.CodexVersion != "0.151.0" || t9Readiness.Readiness.NativeVersion != "codex-cli 0.151.0" || t9Readiness.Readiness.Model != "gpt-5.6-luna" || t9Readiness.Readiness.Effort != "medium" || t9Readiness.Readiness.CWD != "/work" || t9Readiness.Readiness.Sandbox != "read-only" || t9Readiness.Readiness.DynamicToolCount != 0 || t9Readiness.Readiness.SchemaBytes != 0 {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 readiness factors are not the registered baseline")
	}
	if t10State.Layer != "L0_native_process_and_protocol_compatibility" || t10State.Status != "qualified" || t10State.EvidenceResult != "passed" || t10State.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersion || t10State.Combination.CodexVersion != r03aT11Version {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T10 0.153.4 qualification is missing or stale")
	}
	if t10State.CanonicalManifestDigest != t10State.Combination.CurrentFingerprint().CanonicalManifestDigest {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T10 canonical manifest digest does not match its combination")
	}

	t9Protocol, err := os.ReadFile(filepath.Join(cfg.T9Evidence, "native", "protocol.jsonl"))
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T9 native protocol: %w", err)
	}
	for _, required := range []string{
		`"codexHome":"/home/codex"`,
		`"cwd":"/work"`,
		`"sandbox":"read-only"`,
		`"dynamicTools":[]`,
		`"model":"gpt-5.6-luna"`,
		`"model_reasoning_effort":"medium"`,
		`"developerInstructions":"Reply with exactly: POLIS_TRANSPORT_CANARY_OK"`,
		`"method":"thread/started"`,
		`"method":"turn/started"`,
	} {
		if !bytes.Contains(t9Protocol, []byte(required)) {
			return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 protocol does not prove the frozen canary factor " + required)
		}
	}
	if bytes.Count(t9Protocol, []byte(r03aT11Sentinel)) < 2 {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 prompt/developer instruction cannot be reproduced")
	}
	if t9Manifest.DynamicToolCount != 0 || t9Manifest.ToolSchemaBytes != 0 || t9Manifest.ProxyEnvironment["HTTP_PROXY"] != "absent" || t9Manifest.ProxyEnvironment["HTTPS_PROXY"] != "absent" || t9Manifest.ProxyEnvironment["ALL_PROXY"] != "absent" || t9Manifest.ProxyEnvironment["NO_PROXY"] != "absent" || t9Manifest.ProxyEnvironment["POLIS_NATIVE_PROXY"] != "absent" {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 proxy or zero-tool factor is not absent/zero")
	}
	if t9Manifest.DeveloperInstructionDigest != digest([]byte(r03aT11Instruction)) || t9Manifest.PromptDigest != digest([]byte(r03aT11Instruction)) {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 prompt digest does not match the frozen canary prompt")
	}
	if t9Manifest.AuthIdentityFingerprint == "" {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: T9 opaque auth identity fingerprint is missing")
	}

	oldBinary, err := os.ReadFile(cfg.HistoricalBinary)
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: historical 0.151.0 binary: %w", err)
	}
	newBinary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T10 0.153.4 binary: %w", err)
	}
	oldHelper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.HistoricalBinary), "codex-code-mode-host"))
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: historical code-mode-host: %w", err)
	}
	newHelper, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T10 code-mode-host: %w", err)
	}
	oldBinaryDigest, newBinaryDigest := digest(oldBinary), digest(newBinary)
	oldHelperDigest, newHelperDigest := digest(oldHelper), digest(newHelper)
	if oldBinaryDigest != t9Manifest.Combination.BinarySHA256 || newBinaryDigest != t10State.Combination.BinarySHA256 || newHelperDigest != t10State.Combination.CodeModeHostSHA256 || oldHelperDigest != t9Manifest.Combination.CodeModeHostSHA256 {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: binary or code-mode-host digest does not match frozen T9/T10 evidence")
	}
	versionOutput, err := runner.Run([]string{cfg.Binary, "--version"}, []string{"PATH=/usr/bin:/bin"}, 10*time.Second)
	if err != nil || strings.TrimSpace(string(versionOutput)) != "codex-cli "+cfg.Version {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: independent binary version output is not 0.153.4")
	}

	if err := os.MkdirAll(cfg.Root, 0700); err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, err
	}
	oldHome := filepath.Join(cfg.Root, "preflight-old-home")
	newHome := filepath.Join(cfg.Root, "preflight-new-home")
	oldArgs, oldCapability, err := runner.NativeArgs(cfg.HistoricalBinary, oldHome, cfg.AuthFile, "")
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T9 native args: %w", err)
	}
	newArgs, newCapability, err := runner.NativeArgs(cfg.Binary, newHome, cfg.AuthFile, "")
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: T11 native args: %w", err)
	}
	oldConfig, err := os.ReadFile(filepath.Join(oldHome, "config.toml"))
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, err
	}
	newConfig, err := os.ReadFile(filepath.Join(newHome, "config.toml"))
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, err
	}
	if oldCapability != t9Manifest.Combination.CapabilityDigest || newCapability != t10State.Combination.CapabilityDigest || !bytes.Equal(oldConfig, newConfig) || !reflect.DeepEqual(normalizeT11NativeArgs(oldArgs, oldHome, cfg.HistoricalBinary, filepath.Join(filepath.Dir(cfg.HistoricalBinary), "codex-code-mode-host")), normalizeT11NativeArgs(newArgs, newHome, cfg.Binary, filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))) {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: bwrap/config/invocation diverged outside version-specific paths")
	}
	if !bytes.Contains(oldConfig, []byte("supports_websockets = false")) || !bytes.Contains(oldConfig, []byte("responses_websockets = false")) || !bytes.Equal(oldConfig, newConfig) {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, errors.New("preflight_failed: WebSocket-disabled config is not identical")
	}

	authBytes, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	currentAuthMaterial, err := codex.ParseAuthMaterial(authBytes, "mounted_codex_auth_file")
	if err != nil {
		return T11PreflightProof{}, nil, t11ExecutionManifest{}, fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	historicalAuthManifest := codex.AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           codex.AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        "unavailable",
		AuthIdentityFingerprintStatus:                  "unavailable",
		AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              t9Manifest.AuthIdentityFingerprint,
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
	oldFactors := T11CanaryFactors{
		Combination:                t9Manifest.Combination,
		AuthManifest:               historicalAuthManifest,
		ProxyEnvironment:           t9Manifest.ProxyEnvironment,
		CWD:                        t9Readiness.Readiness.CWD,
		Home:                       "/home/codex",
		Invocation:                 r03aT11Invocation,
		Sandbox:                    t9Readiness.Readiness.Sandbox,
		WebSocketPolicy:            "disabled",
		DynamicToolCount:           t9Manifest.DynamicToolCount,
		ToolSchemaBytes:            t9Manifest.ToolSchemaBytes,
		DeveloperInstructionDigest: t9Manifest.DeveloperInstructionDigest,
		PromptDigest:               t9Manifest.PromptDigest,
	}
	newFactors := oldFactors
	newFactors.Combination = t10State.Combination
	newFactors.AuthManifest = currentAuthMaterial.Manifest()
	newFactors.ProxyEnvironment = map[string]string{
		"HTTP_PROXY":         "absent",
		"HTTPS_PROXY":        "absent",
		"ALL_PROXY":          "absent",
		"NO_PROXY":           "absent",
		"POLIS_NATIVE_PROXY": "absent",
	}
	proof, err := CompareT11Factors(oldFactors, newFactors)
	if err != nil {
		return proof, nil, t11ExecutionManifest{}, err
	}
	if len(proof.ConfirmedDivergence) != len(t11VersionDerivedFields) || proof.OldT9Fingerprint != t9State.ExecutionKey || proof.NewT11Fingerprint != t10State.CanonicalManifestDigest {
		return proof, nil, t11ExecutionManifest{}, errors.New("preflight_failed: canonical manifest proof does not match T9/T10 evidence")
	}

	manifest := t11ExecutionManifest{
		Combination:                t10State.Combination,
		FingerprintSchemaVersion:   proof.FingerprintSchemaVersion,
		CanonicalManifestDigest:    proof.NewT11Fingerprint,
		Auth:                       currentAuthMaterial.Manifest(),
		ProxyEnvironment:           newFactors.ProxyEnvironment,
		DynamicToolCount:           0,
		ToolSchemaBytes:            0,
		DeveloperInstructionDigest: t9Manifest.DeveloperInstructionDigest,
		PromptDigest:               t9Manifest.PromptDigest,
	}
	comparison := map[string]any{
		"passed":                                           true,
		"old_t9_fingerprint":                               proof.OldT9Fingerprint,
		"new_t11_fingerprint":                              proof.NewT11Fingerprint,
		"fingerprint_schema_version":                       proof.FingerprintSchemaVersion,
		"confirmed_divergence":                             proof.ConfirmedDivergence,
		"version_derived_fields":                           proof.VersionDerivedFields,
		"non_version_divergence":                           []string{},
		"old_combination":                                  oldFactors.Combination,
		"new_combination":                                  newFactors.Combination,
		"auth_identity_fingerprint_equal":                  true,
		"proxy_environment_equal_and_absent":               true,
		"native_config_bytes_equal":                        true,
		"bwrap_invocation_equal_after_version_paths":       true,
		"websocket_policy_equal":                           true,
		"cwd_home_sandbox_prompt_effort_model_tools_equal": true,
		"t9_history_recomputed":                            false,
		"t7_legacy_fingerprint_participation":              false,
		"t10_provider_egress":                              "not_run",
	}
	return proof, comparison, manifest, nil
}

func RunR03AT11(cfg R03AT11Config) (result R03AT11Result, err error) {
	result = R03AT11Result{
		Status:           "inconclusive",
		ProblemKey:       cfg.ProblemKey,
		Model:            cfg.Model,
		Profile:          cfg.Model + "/medium",
		HighStarted:      0,
		DynamicToolCount: 0,
		Transport:        "not_run",
		SentinelMatch:    "not_run",
		ToolEvents:       []string{},
	}
	if err = validateT11Config(cfg); err != nil {
		return result, err
	}
	var preflight struct {
		Passed                   bool   `json:"passed"`
		ProblemKey               string `json:"problem_key"`
		OldT9Fingerprint         string `json:"old_t9_fingerprint"`
		NewT11Fingerprint        string `json:"new_t11_fingerprint"`
		FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	}
	if err = readT11JSON(filepath.Join(cfg.Evidence, "preflight.json"), &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersion {
		return result, errors.New("preflight_failed: T11 preflight is missing, failed or stale")
	}
	var manifest t11ExecutionManifest
	if err = readT11JSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), &manifest); err != nil || manifest.CanonicalManifestDigest != preflight.NewT11Fingerprint {
		return result, errors.New("preflight_failed: T11 execution manifest is missing or stale")
	}
	result.OldT9Fingerprint = preflight.OldT9Fingerprint
	result.NewT11Fingerprint = preflight.NewT11Fingerprint
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
	if capability != manifest.Combination.CapabilityDigest {
		return result, errors.New("preflight_failed: live T11 capability digest differs from preflight")
	}
	binary, err := os.ReadFile(cfg.Binary)
	if err != nil {
		return result, err
	}
	host, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if err != nil {
		return result, err
	}
	result.Readiness = map[string]any{
		"binary_sha256":         digest(binary),
		"code_mode_host_sha256": digest(host),
		"capability_digest":     capability,
		"native_version":        "codex-cli " + cfg.Version,
		"model":                 cfg.Model,
		"effort":                "medium",
		"cwd":                   "/work",
		"sandbox":               "read-only",
		"proxy_config_digest":   manifest.Combination.ProxyConfigDigest,
		"proxy_category":        "not_configured",
		"dynamic_tool_count":    0,
		"schema_bytes":          0,
		"callback_host":         "not_applicable_no_dynamic_tools",
		"employee_binding":      "not_applicable_transport_canary",
		"websocket_policy":      "disabled",
	}
	readinessAt := time.Now().UTC()
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": result.Readiness, "manifest_source": "T11 canonical preflight"}); err != nil {
		return result, err
	}

	processID := "r03a-t11-canary"
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
		if stopErr != nil {
			result.StopConfirmed = false
			if err == nil {
				err = stopErr
			}
		}
		if stopProof.For(processID) {
			result.StopReceipt = stopProof.Description()
			result.StopConfirmed = true
		}
		result.Finished = time.Now().UTC()
		result.TotalElapsedMS = result.Finished.Sub(result.Started).Milliseconds()
		if err != nil {
			result.Error = err.Error()
		}
		if !sessionWritten {
			_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{
				"allowance_start":    result.Started,
				"process_start":      processStart,
				"process_stop":       processStop,
				"stop_receipt":       result.StopReceipt,
				"stop_confirmed":     result.StopConfirmed,
				"dynamic_tool_count": 0,
			})
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
	readinessAt = time.Now().UTC()
	if err = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{
		"readiness":     result.Readiness,
		"process_start": readinessAt,
		"thread_id":     thread,
		"registration_completed_before_turn_start": true,
	}); err != nil {
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
		result.FirstValidOutputDeltaMS = trace.TimeToFirstOutputMS
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
		traceRecord := map[string]any{
			"process_start":                 processStart,
			"readiness_manifest":            readinessAt,
			"initialize":                    map[string]any{"sent_and_received": trace.InitializeSent && trace.InitializeReceived},
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
			"terminal_state":                trace.TerminalState,
			"process_stop":                  processStop,
			"stop_receipt":                  result.StopReceipt,
			"stop_confirmed":                result.StopConfirmed,
			"dynamic_tool_count":            0,
			"business_side_effects":         0,
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "transport-trace.json"), traceRecord)
		_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{
			"allowance_start":    result.Started,
			"process_start":      processStart,
			"readiness_manifest": readinessAt,
			"initialize":         trace.InitializeSent && trace.InitializeReceived,
			"thread_start":       trace.ThreadStarted,
			"turn_start":         trace.TurnStarted,
			"user_message_item":  trace.UserMessageStarted,
			"first_valid_output": trace.FirstValidOutputAt,
			"turn_completed":     trace.TurnCompletedAt,
			"process_stop":       processStop,
			"stop_receipt":       result.StopReceipt,
			"stop_confirmed":     result.StopConfirmed,
			"dynamic_tool_count": 0,
			"tool_events":        result.ToolEvents,
		})
		sessionWritten = true
	}

	if turnErr != nil {
		if strings.Contains(turnErr.Error(), "deadline") || strings.Contains(turnErr.Error(), "outcome_unknown") || strings.Contains(turnErr.Error(), "reconciliation") {
			result.Transport = "inconclusive"
		} else {
			result.Transport = "failed"
		}
		result.Status = "inconclusive"
		_ = writeT11Qualification(cfg, manifest, result, "no completed turn with resolved first output")
		return result, turnErr
	}
	if result.TurnState == "completed" && result.FirstValidOutput && !result.UnresolvedTransportState {
		result.Transport = "passed"
		result.Status = "passed"
	} else {
		result.Transport = "inconclusive"
		result.Status = "inconclusive"
	}
	if err = writeT11Qualification(cfg, manifest, result, "version-only minimal transport result"); err != nil {
		return result, err
	}
	return result, nil
}

func writeT11Qualification(cfg R03AT11Config, manifest t11ExecutionManifest, result R03AT11Result, reason string) error {
	status, evidence, scheduling := "unqualified", "inconclusive", "unqualified_for_business_execution"
	if result.Status == "passed" && result.Transport == "passed" && result.TurnState == "completed" && result.FirstValidOutput && !result.UnresolvedTransportState {
		status, evidence, scheduling = "qualified", "passed", "qualified"
	}
	return writeJSON(filepath.Join(cfg.Evidence, "qualification-state.json"), map[string]any{
		"layer":                      "L1_base_provider_transport",
		"status":                     status,
		"scheduling_decision":        scheduling,
		"evidence_result":            evidence,
		"execution_key":              manifest.CanonicalManifestDigest,
		"fingerprint_schema_version": manifest.FingerprintSchemaVersion,
		"canonical_manifest_digest":  manifest.CanonicalManifestDigest,
		"combination":                manifest.Combination,
		"auth":                       manifest.Auth,
		"evidence_ref":               filepath.Join(cfg.Evidence, "result.json"),
		"reason":                     reason,
		"transport":                  result.Transport,
		"turn_state":                 result.TurnState,
		"sentinel_match":             result.SentinelMatch,
		"created_at":                 result.Finished,
	})
}

func validateT11Config(cfg R03AT11Config) error {
	if cfg.Version != r03aT11Version || cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ProxyURL != "" || cfg.ProblemKey == "" || cfg.Binary == "" || cfg.HistoricalBinary == "" || cfg.AuthFile == "" || cfg.Evidence == "" || cfg.Root == "" || cfg.T9Evidence == "" || cfg.T10Evidence == "" {
		return errors.New("preflight_failed: T11 fixed configuration is incomplete or diverged")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if _, ok := os.LookupEnv(name); ok {
			return errors.New("preflight_failed: " + name + " must be absent")
		}
	}
	return nil
}

func ensureT11EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "preflight-attempt-") {
			return errors.New("T11 evidence directory is not fresh; refusing retry/reset")
		}
	}
	return nil
}

func readT11JSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	return json.Unmarshal(raw, target)
}
