// pattern: Imperative Shell
package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/kernel"
	"polis/internal/runner"
	"strings"
	"time"
)

type FrontendRuntimeActivationPreflightReport struct {
	Status                       string                        `json:"status"`
	FailureClass                 string                        `json:"failure_class,omitempty"`
	Error                        string                        `json:"error,omitempty"`
	LaunchEnvelope               runner.NativeLaunchEnvelope   `json:"launch_envelope"`
	ExecutionEnvelopeFingerprint string                        `json:"execution_envelope_fingerprint"`
	TransportPolicy              codex.TransportPolicySnapshot `json:"transport_policy"`
	PaginationRuntimePreflight   string                        `json:"pagination_runtime_preflight"`
	CapabilityDigest             string                        `json:"capability_digest,omitempty"`
	ProcessStarted               bool                          `json:"process_started"`
	InitializeStarted            bool                          `json:"initialize_started"`
	InitializeCompleted          bool                          `json:"initialize_completed"`
	LocalProtocolReady           bool                          `json:"local_protocol_ready"`
	TurnStarted                  bool                          `json:"turn_started"`
	ProviderEgress               bool                          `json:"provider_egress"`
	RegisteredToolCount          int                           `json:"registered_tool_count"`
	InitializeLatencyMS          int64                         `json:"initialize_latency_ms"`
	ThreadStartLatencyMS         int64                         `json:"thread_start_latency_ms"`
	ProtocolEvidencePath         string                        `json:"protocol_evidence_path,omitempty"`
	StderrClassification         string                        `json:"stderr_classification,omitempty"`
	ProcessExit                  string                        `json:"process_exit,omitempty"`
	StopConfirmed                bool                          `json:"stop_confirmed"`
	BusinessMutation             bool                          `json:"business_mutation"`
	AllowanceCreated             bool                          `json:"allowance_created"`
}

func ValidateExecutionEnvelopeFingerprint(expected, actual string) error {
	if expected == "" {
		return errors.New("execution envelope fingerprint is missing")
	}
	if actual == "" || expected != actual {
		return fmt.Errorf("execution envelope fingerprint mismatch: expected %s got %s", expected, actual)
	}
	return nil
}

func FrontendRuntimeActivationPreflight(ctx context.Context, cfg R03APaginationV3Config) (FrontendRuntimeActivationPreflightReport, error) {
	policy := cfg.TransportPolicy
	if err := validateR03ATransportPolicy(policy); err != nil {
		return FrontendRuntimeActivationPreflightReport{Status: "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_FAILED", RegisteredToolCount: len(codex.PeerFrontendToolsV1()), ProviderEgress: false, BusinessMutation: false, AllowanceCreated: false}, err
	}
	report := FrontendRuntimeActivationPreflightReport{
		Status:              "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_FAILED",
		RegisteredToolCount: len(codex.PeerFrontendToolsV1()),
		TurnStarted:         false,
		ProviderEgress:      false,
		BusinessMutation:    false,
		AllowanceCreated:    false,
		TransportPolicy:     policy.Snapshot(),
	}
	if cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.Model == "" {
		return report, errors.New("Frontend runtime launch configuration is incomplete")
	}
	if _, err := os.Stat(cfg.Binary); err != nil {
		return report, fmt.Errorf("controlled Codex binary unavailable: %w", err)
	}
	if _, err := os.Stat(cfg.CodeModeHost); err != nil {
		return report, fmt.Errorf("controlled code-mode host unavailable: %w", err)
	}
	if len(cfg.FrontendRequiredCAS) > 0 {
		if _, err := kernel.ValidatePaginationRuntimeAccess(cfg.RuntimeCASBinding.CanonicalRoot, cfg.FrontendRequiredCAS); err != nil {
			report.PaginationRuntimePreflight = "FAILED"
			return report, err
		}
		report.PaginationRuntimePreflight = "PASSED"
	} else {
		report.PaginationRuntimePreflight = "NOT_PROVIDED"
	}
	home := filepath.Join(cfg.Root, "frontend-runtime-preflight-home")
	args, capability, err := runner.NativeArgsWithTransportPolicy(cfg.Binary, home, cfg.AuthFile, cfg.ProxyURL, runner.NativeTransportPolicyNativeDefault)
	if err != nil {
		return report, err
	}
	envelope, err := runner.DescribeNativeLaunch(cfg.Binary, cfg.CodeModeHost, home, args, runner.NativeEnvironment(home), cfg.Root)
	if err != nil {
		return report, err
	}
	report.LaunchEnvelope, report.CapabilityDigest = envelope, capability
	envelope = runner.BindTransportPolicy(envelope, runner.NativeTransportPolicyBinding{
		Revision:               policy.Revision,
		InitializeTimeoutMS:    policy.InitializeTimeout.Milliseconds(),
		StartAcknowledgementMS: policy.StartAcknowledgement.Milliseconds(),
		FirstOutputDeadlineMS:  policy.FirstOutputDeadline.Milliseconds(),
		ReconnectGraceMS:       policy.ReconnectGrace.Milliseconds(),
		StreamingIdleMS:        policy.StreamingIdle.Milliseconds(),
		TotalTurnDeadlineMS:    policy.TotalTurnDeadline.Milliseconds(),
		ReconciliationMS:       policy.StopReconciliation.Milliseconds(),
	})
	report.LaunchEnvelope = envelope
	report.ExecutionEnvelopeFingerprint = envelope.Fingerprint
	protocolPath := filepath.Join(cfg.Evidence, "frontend-runtime-preflight-native", "protocol.jsonl")
	report.ProtocolEvidencePath = protocolPath
	if err := os.MkdirAll(filepath.Dir(protocolPath), 0700); err != nil {
		return report, err
	}
	process, err := runner.StartWithWorkingDir("r03a-frontend-runtime-preflight", args, runner.NativeEnvironment(home), cfg.Root)
	if err != nil {
		return report, err
	}
	report.ProcessStarted = true
	client, err := codex.NewWithModelAndVersion(process, filepath.Dir(protocolPath), cfg.Model, cfg.ExpectedNativeVersion)
	if err != nil {
		_, _ = process.Stop()
		report.ProcessExit = fmt.Sprint(process.WaitError())
		return finalizeRuntimePreflight(report, err, protocolPath)
	}
	initializeStarted := time.Now()
	report.InitializeStarted = true
	if err = client.InitializeWithPolicy(ctx, policy); err != nil {
		client.Close()
		report.InitializeLatencyMS = time.Since(initializeStarted).Milliseconds()
		report.ProcessExit = fmt.Sprint(process.WaitError())
		return finalizeRuntimePreflight(report, err, protocolPath)
	}
	report.InitializeCompleted = true
	report.InitializeLatencyMS = time.Since(initializeStarted).Milliseconds()
	threadStarted := time.Now()
	if _, err = client.StartThreadWithTools(ctx, "medium", codex.PeerFrontendToolsV1(), "Local runtime activation preflight only. Do not call turn/start or any business tool."); err != nil {
		client.Close()
		report.ThreadStartLatencyMS = time.Since(threadStarted).Milliseconds()
		report.ProcessExit = fmt.Sprint(process.WaitError())
		return finalizeRuntimePreflight(report, err, protocolPath)
	}
	report.ThreadStartLatencyMS = time.Since(threadStarted).Milliseconds()
	report.LocalProtocolReady = true
	client.Close()
	proof, stopErr := process.Stop()
	report.StopConfirmed = stopErr == nil && proof.For("r03a-frontend-runtime-preflight")
	if stopErr == nil {
		report.ProcessExit = "intentional_stop"
	} else {
		report.ProcessExit = fmt.Sprint(process.WaitError())
	}
	if stopErr != nil {
		return finalizeRuntimePreflight(report, stopErr, protocolPath)
	}
	report.Status = "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED"
	return report, nil
}

func finalizeRuntimePreflight(report FrontendRuntimeActivationPreflightReport, err error, protocolPath string) (FrontendRuntimeActivationPreflightReport, error) {
	report.Error = err.Error()
	if strings.Contains(err.Error(), "outcome_unknown: transport: EOF") {
		report.FailureClass = "STDOUT_PIPE_CLOSED_BEFORE_INITIALIZE"
	} else if strings.Contains(err.Error(), "UtilGetPpid") {
		report.FailureClass = "WSL_BWRAP_PPId_PARSE_FAILURE"
	} else {
		report.FailureClass = "RUNTIME_INITIALIZE_OR_LOCAL_PROTOCOL_FAILURE"
	}
	if raw, readErr := os.ReadFile(protocolPath); readErr == nil {
		report.StderrClassification = classifyRuntimeStderr(raw)
	}
	report.Status = "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_FAILED"
	return report, err
}

func classifyRuntimeStderr(raw []byte) string {
	text := string(raw)
	switch {
	case strings.Contains(text, "bwrap: execvp"):
		return "WSL_BWRAP_EXECVP_FAILURE"
	case strings.Contains(text, "UtilGetPpid"):
		return "WSL_UTIL_GET_PPID_PROC1_PARSE_FAILURE"
	case strings.Contains(text, "Failed to parse: /proc/1/stat"):
		return "WSL_PROC1_STAT_PARSE_FAILURE"
	case strings.Contains(text, "stderr"):
		return "BOUNDED_STDERR_PRESENT"
	default:
		return "NO_CLASSIFIED_STDERR"
	}
}
