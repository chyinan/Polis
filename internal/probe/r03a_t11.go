// pattern: Functional Core
package probe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
)

// T11CanaryFactors is the complete execution-factor snapshot used by the
// version-only preflight. Runtime and transport settings are kept here even
// though they are not part of the L1 execution combination hash.
type T11CanaryFactors struct {
	Combination                codex.ExecutionCombination
	AuthManifest               codex.AuthFingerprintManifest
	ProxyEnvironment           map[string]string
	CWD                        string
	Home                       string
	Invocation                 string
	Sandbox                    string
	WebSocketPolicy            string
	DynamicToolCount           int
	ToolSchemaBytes            int
	DeveloperInstructionDigest string
	PromptDigest               string
}

type T11PreflightProof struct {
	Passed                   bool                           `json:"passed"`
	FingerprintSchemaVersion string                         `json:"fingerprint_schema_version"`
	OldT9Fingerprint         string                         `json:"old_t9_fingerprint"`
	NewT11Fingerprint        string                         `json:"new_t11_fingerprint"`
	NewFingerprint           codex.QualificationFingerprint `json:"new_fingerprint"`
	ConfirmedDivergence      []string                       `json:"confirmed_divergence"`
	VersionDerivedFields     []string                       `json:"version_derived_fields"`
	NonVersionDivergence     []string                       `json:"non_version_divergence,omitempty"`
	AuthComparison           codex.AuthManifestComparison   `json:"auth_comparison"`
}

type T11TransportTrace struct {
	InitializeSent         bool             `json:"initialize_sent"`
	InitializeReceived     bool             `json:"initialize_received"`
	ThreadStartSent        bool             `json:"thread_start_sent"`
	ThreadStarted          bool             `json:"thread_started"`
	TurnStartSent          bool             `json:"turn_start_sent"`
	TurnStarted            bool             `json:"turn_started"`
	UserMessageStarted     bool             `json:"user_message_started"`
	FirstValidOutput       bool             `json:"first_valid_output"`
	FirstValidOutputAt     *time.Time       `json:"first_valid_output_at,omitempty"`
	TimeToFirstOutputMS    int64            `json:"time_to_first_output_ms,omitempty"`
	ReconnectCount         int              `json:"reconnect_count"`
	ReconnectPhases        []string         `json:"reconnect_phases"`
	FirstDisconnectDeltaMS int64            `json:"first_disconnect_delta_ms,omitempty"`
	RecoveryDeltaMS        int64            `json:"recovery_delta_ms,omitempty"`
	RecoveryTimestamps     []*time.Time     `json:"recovery_timestamps,omitempty"`
	TurnCompleted          bool             `json:"turn_completed"`
	TurnCompletedAt        *time.Time       `json:"turn_completed_at,omitempty"`
	TerminalState          string           `json:"terminal_state,omitempty"`
	AssistantOutput        string           `json:"assistant_output,omitempty"`
	SentinelMatch          string           `json:"sentinel_match"`
	NativeUsageUpdates     int              `json:"native_usage_updates"`
	TokenUsage             codex.TokenUsage `json:"token_usage"`
}

var t11VersionDerivedFields = []string{
	"combination.codex_version",
	"combination.binary_sha256",
	"combination.code_mode_host_sha256",
	"combination.capability_digest",
	"combination.native_protocol_digest",
}

func CompareT11Factors(old, next T11CanaryFactors) (T11PreflightProof, error) {
	proof := T11PreflightProof{
		FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersion,
		OldT9Fingerprint:         old.Combination.Fingerprint(),
		NewFingerprint:           next.Combination.CurrentFingerprint(),
		VersionDerivedFields:     append([]string(nil), t11VersionDerivedFields...),
	}
	proof.NewT11Fingerprint = proof.NewFingerprint.CanonicalManifestDigest

	if err := old.Combination.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T9 combination: %w", err)
	}
	if err := next.Combination.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T11 combination: %w", err)
	}
	if err := proof.NewFingerprint.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid canonical T11 fingerprint: %w", err)
	}
	proof.AuthComparison = codex.CompareAuthManifests(old.AuthManifest, next.AuthManifest)

	oldFields := combinationFields(old.Combination)
	nextFields := combinationFields(next.Combination)
	for _, field := range t11VersionDerivedFields {
		if oldFields[field] != nextFields[field] {
			proof.ConfirmedDivergence = append(proof.ConfirmedDivergence, field)
		}
	}
	for field, oldValue := range oldFields {
		if isT11VersionDerivedField(field) {
			continue
		}
		if oldValue != nextFields[field] {
			proof.NonVersionDivergence = append(proof.NonVersionDivergence, field)
		}
	}

	if proof.AuthComparison.SourceClass != codex.AuthComparisonSame {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "auth_source_class")
	}
	if proof.AuthComparison.Identity != codex.AuthComparisonSame {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "auth_identity_fingerprint")
	}
	if proof.AuthComparison.CredentialRevision != codex.AuthComparisonSame {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "auth_credential_revision_fingerprint")
	}
	if !reflect.DeepEqual(old.ProxyEnvironment, next.ProxyEnvironment) {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "proxy_environment")
	}
	for field, oldValue := range map[string]string{
		"cwd":                          old.CWD,
		"home":                         old.Home,
		"invocation":                   old.Invocation,
		"sandbox":                      old.Sandbox,
		"websocket_policy":             old.WebSocketPolicy,
		"developer_instruction_digest": old.DeveloperInstructionDigest,
		"prompt_digest":                old.PromptDigest,
	} {
		if oldValue != factorString(next, field) {
			proof.NonVersionDivergence = append(proof.NonVersionDivergence, field)
		}
	}
	if old.DynamicToolCount != next.DynamicToolCount {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "dynamic_tool_count")
	}
	if old.ToolSchemaBytes != next.ToolSchemaBytes {
		proof.NonVersionDivergence = append(proof.NonVersionDivergence, "tool_schema_bytes")
	}

	if len(proof.NonVersionDivergence) != 0 {
		return proof, errors.New("preflight_failed: confirmed divergence outside version factor: " + fmt.Sprint(proof.NonVersionDivergence))
	}
	proof.Passed = true
	return proof, nil
}

func SummarizeT11Protocol(raw []byte, sentinel string) (T11TransportTrace, error) {
	var trace T11TransportTrace
	var turnStartedAt time.Time
	var firstDisconnectAt time.Time
	var awaitingRecovery bool
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var envelope struct {
			Time      time.Time       `json:"time"`
			Direction string          `json:"direction"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(scan.Bytes(), &envelope); err != nil {
			return trace, err
		}
		if envelope.Direction != "send" && envelope.Direction != "receive" {
			continue
		}
		var data struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return trace, err
		}
		if envelope.Direction == "send" {
			switch data.Method {
			case "initialize":
				trace.InitializeSent = true
			case "thread/start":
				trace.ThreadStartSent = true
			case "turn/start":
				trace.TurnStartSent = true
			}
			continue
		}
		if envelope.Direction != "receive" {
			continue
		}
		if data.Method == "" && len(data.Result) > 0 {
			if trace.InitializeSent && !trace.InitializeReceived {
				trace.InitializeReceived = true
			}
			continue
		}
		switch data.Method {
		case "thread/started":
			trace.ThreadStarted = true
		case "turn/started":
			trace.TurnStarted = true
			if turnStartedAt.IsZero() {
				turnStartedAt = envelope.Time
			}
		case "item/started":
			var params struct {
				Item struct {
					Type string `json:"type"`
				} `json:"item"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			if params.Item.Type == "userMessage" {
				trace.UserMessageStarted = true
			}
		case "error":
			var params struct {
				WillRetry bool `json:"willRetry"`
				Error     struct {
					CodexErrorInfo struct {
						ResponseStreamDisconnected json.RawMessage `json:"responseStreamDisconnected"`
					} `json:"codexErrorInfo"`
				} `json:"error"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			if params.WillRetry && len(params.Error.CodexErrorInfo.ResponseStreamDisconnected) > 0 && string(params.Error.CodexErrorInfo.ResponseStreamDisconnected) != "null" {
				trace.ReconnectCount++
				phase := "pre_first_output_reconnecting"
				if trace.FirstValidOutput {
					phase = "post_first_output_reconnecting"
				}
				trace.ReconnectPhases = append(trace.ReconnectPhases, phase)
				if firstDisconnectAt.IsZero() {
					firstDisconnectAt = envelope.Time
					if !turnStartedAt.IsZero() {
						trace.FirstDisconnectDeltaMS = envelope.Time.Sub(turnStartedAt).Milliseconds()
					}
				}
				awaitingRecovery = true
			}
		case "thread/tokenUsage/updated":
			var params struct {
				TokenUsage codex.TokenUsage `json:"tokenUsage"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			trace.TokenUsage = params.TokenUsage
			trace.NativeUsageUpdates++
			markT11FirstOutput(&trace, turnStartedAt, firstDisconnectAt, envelope.Time, &awaitingRecovery)
		case "item/agentMessage/delta":
			var params struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			trace.AssistantOutput += params.Delta
			markT11FirstOutput(&trace, turnStartedAt, firstDisconnectAt, envelope.Time, &awaitingRecovery)
		case "item/agentMessage/completed", "item/completed":
			var params struct {
				Item struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			if params.Item.Type == "agentMessage" {
				trace.AssistantOutput = params.Item.Text
				markT11FirstOutput(&trace, turnStartedAt, firstDisconnectAt, envelope.Time, &awaitingRecovery)
			}
		case "turn/completed":
			var params struct {
				Turn struct {
					Status string `json:"status"`
				} `json:"turn"`
			}
			if err := json.Unmarshal(data.Params, &params); err != nil {
				return trace, err
			}
			trace.TurnCompleted = true
			completedAt := envelope.Time
			trace.TurnCompletedAt = &completedAt
			trace.TerminalState = params.Turn.Status
		}
	}
	if err := scan.Err(); err != nil {
		return trace, err
	}
	if strings.TrimSpace(trace.AssistantOutput) == sentinel {
		trace.SentinelMatch = "passed"
	} else {
		trace.SentinelMatch = "failed"
	}
	return trace, nil
}

func markT11FirstOutput(trace *T11TransportTrace, turnStartedAt, firstDisconnectAt, at time.Time, awaitingRecovery *bool) {
	if *awaitingRecovery {
		recoveredAt := at
		trace.RecoveryTimestamps = append(trace.RecoveryTimestamps, &recoveredAt)
		if !firstDisconnectAt.IsZero() {
			trace.RecoveryDeltaMS = at.Sub(firstDisconnectAt).Milliseconds()
		}
		*awaitingRecovery = false
	}
	if trace.FirstValidOutput {
		return
	}
	trace.FirstValidOutput = true
	firstOutputAt := at
	trace.FirstValidOutputAt = &firstOutputAt
	if !turnStartedAt.IsZero() {
		trace.TimeToFirstOutputMS = at.Sub(turnStartedAt).Milliseconds()
	}
}

func combinationFields(c codex.ExecutionCombination) map[string]string {
	return map[string]string{
		"combination.codex_version":          c.CodexVersion,
		"combination.binary_sha256":          c.BinarySHA256,
		"combination.model":                  c.Model,
		"combination.effort":                 c.Effort,
		"combination.runtime_profile":        c.RuntimeProfile,
		"combination.sandbox_class":          c.SandboxClass,
		"combination.proxy_config_digest":    c.ProxyConfigDigest,
		"combination.auth_source_class":      c.AuthSourceClass,
		"combination.code_mode_host_sha256":  c.CodeModeHostSHA256,
		"combination.capability_digest":      c.CapabilityDigest,
		"combination.native_protocol_digest": c.NativeProtocolDigest,
	}
}

func isT11VersionDerivedField(field string) bool {
	for _, allowed := range t11VersionDerivedFields {
		if field == allowed {
			return true
		}
	}
	return false
}

func factorString(f T11CanaryFactors, field string) string {
	switch field {
	case "cwd":
		return f.CWD
	case "home":
		return f.Home
	case "invocation":
		return f.Invocation
	case "sandbox":
		return f.Sandbox
	case "websocket_policy":
		return f.WebSocketPolicy
	case "developer_instruction_digest":
		return f.DeveloperInstructionDigest
	case "prompt_digest":
		return f.PromptDigest
	default:
		return ""
	}
}

func normalizeT11NativeArgs(args []string, home, binary, helper string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		switch arg {
		case home:
			out[i] = "<home>"
		case binary:
			out[i] = "<binary>"
		case helper:
			out[i] = "<code-mode-host>"
		default:
			out[i] = arg
		}
	}
	return out
}
