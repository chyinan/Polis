// pattern: Imperative Shell
package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/runner"
	"strings"
	"time"
)

type R03AT6Config struct {
	Config
	ProblemKey  string
	T5Evidence  string
	OldEvidence string
}

type R03AT6Result struct {
	Status                  string           `json:"status"`
	ProblemKey              string           `json:"problem_key"`
	Model                   string           `json:"model"`
	Profile                 string           `json:"profile"`
	Started                 time.Time        `json:"started"`
	Finished                time.Time        `json:"finished"`
	MediumStarted           int              `json:"medium_started"`
	HighStarted             int              `json:"high_started"`
	DynamicToolCount        int              `json:"dynamic_tool_count"`
	Readiness               any              `json:"readiness"`
	TurnState               string           `json:"turn_state"`
	Transport               string           `json:"transport"`
	SentinelMatch           string           `json:"sentinel_match"`
	FirstValidOutput        bool             `json:"first_valid_output"`
	FirstValidOutputDeltaMS int64            `json:"first_valid_output_delta_ms,omitempty"`
	ReconnectCount          int              `json:"reconnect_count"`
	ReconnectPhases         []string         `json:"reconnect_phases"`
	FirstDisconnectDeltaMS  int64            `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS         int64            `json:"recovery_delta_ms,omitempty"`
	TotalElapsedMS          int64            `json:"total_elapsed_ms"`
	NativeUsageUpdates      int              `json:"native_usage_updates"`
	TokenUsage              codex.TokenUsage `json:"token_usage"`
	StopReceipt             string           `json:"stop_receipt,omitempty"`
	StopConfirmed           bool             `json:"stop_confirmed"`
	ToolEvents              []string         `json:"tool_events"`
	Error                   string           `json:"error,omitempty"`
}

func InspectR03AT6(cfg R03AT6Config) error {
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 {
		return errors.New("R0.3A-T6 requires one Medium and zero High")
	}
	if cfg.ProblemKey == "" {
		cfg.ProblemKey = R03AProblemKey
	}
	if cfg.T5Evidence == "" {
		cfg.T5Evidence = filepath.Join("evidence", "development", "r0.3a-t5")
	}
	if cfg.OldEvidence == "" {
		cfg.OldEvidence = filepath.Join("evidence", "development", "r0.3a-t2", "luna-1")
	}
	if _, e := os.Stat(filepath.Join(cfg.Evidence, "preflight.json")); e == nil {
		return errors.New("R0.3A-T6 preflight already exists; refusing automatic rerun")
	}
	if e := assertT6Preflight(cfg); e != nil {
		return e
	}
	if _, e := os.Stat(filepath.Join(cfg.Evidence, "allowance.json")); e == nil {
		return errors.New("R0.3A-T6 allowance already exists")
	}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return e
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":             true,
		"problem_key":        cfg.ProblemKey,
		"t5":                 "passed",
		"old_allowance":      "sealed",
		"dynamic_tool_count": 0,
		"model_calls":        0,
	})
}

func RunR03AT6(cfg R03AT6Config) (result R03AT6Result, err error) {
	if cfg.ProblemKey == "" {
		cfg.ProblemKey = R03AProblemKey
	}
	result = R03AT6Result{Status: "inconclusive", ProblemKey: cfg.ProblemKey, Model: cfg.Model, Profile: cfg.Model + "/medium", HighStarted: 0, DynamicToolCount: 0, Transport: "not_run", SentinelMatch: "not_run"}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	defer func() {
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
	}()
	preflightRaw, e := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if e != nil {
		return result, e
	}
	var preflight struct {
		Passed           bool   `json:"passed"`
		ProblemKey       string `json:"problem_key"`
		DynamicToolCount int    `json:"dynamic_tool_count"`
	}
	if e = json.Unmarshal(preflightRaw, &preflight); e != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.DynamicToolCount != 0 {
		return result, errors.New("R0.3A-T6 preflight failed or stale")
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "problem-key.json"), map[string]any{"problem_key": cfg.ProblemKey, "new_allowance": true}); e != nil {
		return result, e
	}
	budget, e := codex.NewBudget(filepath.Join(cfg.Evidence, "allowance.json"), 1, 0)
	if e != nil {
		return result, e
	}
	result.Started = budget.Started
	ctx, cancel := context.WithDeadline(context.Background(), budget.Started.Add(10*time.Minute))
	defer cancel()
	root := filepath.Join(cfg.Root, "canary")
	if e = os.MkdirAll(root, 0700); e != nil {
		return result, e
	}
	args, capability, e := runner.NativeArgs(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, cfg.ProxyURL)
	if e != nil {
		return result, e
	}
	binary, e := os.ReadFile(cfg.Binary)
	if e != nil {
		return result, e
	}
	host, e := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if e != nil {
		return result, e
	}
	proxyDigest := ""
	if cfg.ProxyURL != "" {
		h := sha256.Sum256([]byte(cfg.ProxyURL))
		proxyDigest = hex.EncodeToString(h[:])
	} else {
		h := sha256.Sum256([]byte("proxy:none"))
		proxyDigest = hex.EncodeToString(h[:])
	}
	result.Readiness = map[string]any{"binary_sha256": digest(binary), "code_mode_host_sha256": digest(host), "capability_digest": capability, "native_version": runner.NativeVersion, "model": cfg.Model, "effort": "medium", "cwd": "/work", "sandbox": "read-only", "proxy_config_digest": proxyDigest, "proxy_category": map[bool]string{true: "configured", false: "not_configured"}[cfg.ProxyURL != ""], "dynamic_tool_count": 0, "schema_bytes": 0, "callback_host": "not_applicable_no_dynamic_tools", "employee_binding": "not_applicable_transport_canary"}
	_ = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), result.Readiness)
	p, e := runner.Start("r03a-t6-canary", args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		return result, e
	}
	stopped := false
	defer func() {
		if !stopped {
			if proof, stopErr := p.Stop(); stopErr == nil {
				result.StopReceipt, result.StopConfirmed = proof.Description(), proof.For("r03a-t6-canary")
			}
		}
		result.TotalElapsedMS = time.Since(result.Started).Milliseconds()
		_ = writeJSON(filepath.Join(cfg.Evidence, "canary-session.json"), map[string]any{"process_start": result.Started, "stop_receipt": result.StopReceipt, "stop_confirmed": result.StopConfirmed, "dynamic_tool_count": 0, "tool_events": result.ToolEvents})
	}()
	c, e := codex.NewWithModel(p, filepath.Join(cfg.Evidence, "native"), cfg.Model)
	if e != nil {
		return result, e
	}
	defer c.Close()
	if e = c.Initialize(ctx); e != nil {
		return result, e
	}
	thread, e := c.StartThreadWithTools(ctx, "medium", []any{}, "Reply with exactly: POLIS_TRANSPORT_CANARY_OK")
	if e != nil {
		return result, e
	}
	_ = writeJSON(filepath.Join(cfg.Evidence, "readiness-manifest.json"), map[string]any{"readiness": result.Readiness, "thread_id": thread, "registration_completed_before_turn_start": true})
	if e = budget.Reserve("medium"); e != nil {
		return result, e
	}
	result.MediumStarted = budget.Medium
	turn, turnErr := c.TurnWithOptions(ctx, thread, "medium", "Reply with exactly: POLIS_TRANSPORT_CANARY_OK", codex.TurnOptions{Timeouts: codex.DefaultTurnTimeouts(), OuterDeadline: budget.Started.Add(10 * time.Minute)}, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		result.ToolEvents = append(result.ToolEvents, name)
		return nil, false
	})
	result.TurnState = turn.State
	result.TokenUsage = turn.Usage
	result.NativeUsageUpdates = turn.UsageUpdates
	protocolPath := filepath.Join(cfg.Evidence, "native", "protocol.jsonl")
	if raw, readErr := os.ReadFile(protocolPath); readErr == nil {
		if summary, summaryErr := analyzeCanaryProtocol(raw, "POLIS_TRANSPORT_CANARY_OK"); summaryErr == nil {
			result.FirstValidOutput = summary.FirstValidOutput
			result.FirstValidOutputDeltaMS = summary.FirstValidOutputDeltaMS
			result.FirstDisconnectDeltaMS = summary.FirstDisconnectDeltaMS
			result.RecoveryDeltaMS = summary.RecoveryDeltaMS
			result.ReconnectCount = summary.ReconnectCount
			result.ReconnectPhases = summary.ReconnectPhases
			result.SentinelMatch = map[bool]string{true: "passed", false: "failed"}[summary.SentinelMatch]
			if summary.TerminalState != "" {
				result.TurnState = summary.TerminalState
			}
		}
	}
	if turnErr != nil {
		if strings.Contains(turnErr.Error(), "deadline") || strings.Contains(turnErr.Error(), "outcome_unknown") || strings.Contains(turnErr.Error(), "reconciliation") {
			result.Transport = "inconclusive"
		} else {
			result.Transport = "failed"
		}
		if strings.Contains(turnErr.Error(), "first_valid_output_deadline_exceeded") {
			result.TurnState = "first_valid_output_deadline_exceeded"
		} else if strings.Contains(turnErr.Error(), "reconnect_deadline_exceeded") {
			result.TurnState = "reconnect_deadline_exceeded"
		}
		return result, turnErr
	}
	if result.ToolEvents == nil {
		result.ToolEvents = []string{}
	}
	if result.TurnState == "completed" && result.FirstValidOutput {
		result.Transport = "passed"
		result.Status = "passed"
		return result, nil
	}
	result.Transport = "inconclusive"
	result.Status = "inconclusive"
	return result, nil
}

func assertT6Preflight(cfg R03AT6Config) error {
	t5ManifestPath := filepath.Join(cfg.T5Evidence, "tool-manifest.json")
	manifestRaw, e := os.ReadFile(t5ManifestPath)
	if e != nil {
		return errors.New("T5 tool manifest missing")
	}
	manifestRaw = bytes.TrimPrefix(manifestRaw, []byte{0xef, 0xbb, 0xbf})
	var manifest struct {
		All16Subsets []struct {
			UniqueNames      bool `json:"UniqueNames"`
			AllSchemasValid  bool `json:"AllSchemasValid"`
			AllHandlersBound bool `json:"AllHandlersBound"`
		} `json:"All16Subsets"`
	}
	if e = json.Unmarshal(manifestRaw, &manifest); e != nil || len(manifest.All16Subsets) != 48 {
		return errors.New("T5 manifest subset qualification missing")
	}
	for _, row := range manifest.All16Subsets {
		if !row.UniqueNames || !row.AllSchemasValid || !row.AllHandlersBound {
			return errors.New("T5 manifest contains a failed subset")
		}
	}
	old, e := os.ReadFile(filepath.Join(cfg.OldEvidence, "allowance.json"))
	if e != nil || !strings.Contains(string(old), `"medium_turns":1`) || !strings.Contains(string(old), `"high_turns":0`) {
		return errors.New("sealed T2 allowance missing or changed")
	}
	key, e := os.ReadFile(filepath.Join(cfg.OldEvidence, "problem-key.json"))
	if e != nil || !strings.Contains(string(key), R03AProblemKey) {
		return errors.New("parent ProblemKey mismatch")
	}
	raw, e := runner.Run([]string{"git", "rev-parse", "HEAD"}, []string{"PATH=/usr/bin:/bin"}, 5*time.Second)
	if e != nil || strings.TrimSpace(string(raw)) != R03ASubjectRevision {
		return errors.New("subject revision drift")
	}
	return nil
}
