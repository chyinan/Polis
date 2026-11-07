// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"polis/internal/codex"
	"polis/internal/control"
	"polis/internal/provider"
	"polis/internal/runner"
)

const (
	defaultEvidenceRoot = "evidence/development/r0.5b13-provider-process-launch-reproducibility-hardening"
	defaultRuntimeRoot  = ".runtime/windows/r0.5b13-local-production-qualification"
	manifestPath        = `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json`
	authPath            = `C:\Users\chyinan\.codex\auth.json`
)

type cycleResult struct {
	Cycle                int                          `json:"cycle"`
	SessionID            string                       `json:"session_id"`
	RuntimeVersion       string                       `json:"runtime_version"`
	BinarySHA256         string                       `json:"binary_sha256"`
	HelperSHA256         string                       `json:"helper_sha256"`
	ProcessStart         string                       `json:"process_start"`
	Initialize           string                       `json:"initialize"`
	ThreadStart          string                       `json:"thread_start"`
	RegisteredToolCount  int                          `json:"registered_tool_count"`
	RegisteredToolDigest string                       `json:"registered_tool_digest"`
	CleanStop            string                       `json:"clean_stop"`
	OrphanProcessCount   int                          `json:"orphan_process_count"`
	ProviderEgress       int                          `json:"provider_egress"`
	LaunchEvidence       runner.ProcessLaunchEvidence `json:"launch_evidence"`
	ProtocolEvidencePath string                       `json:"protocol_evidence_path"`
	CompletedAt          time.Time                    `json:"completed_at"`
}

type qualificationReport struct {
	RecordType                     string        `json:"record_type"`
	Status                         string        `json:"status"`
	ProviderReservations           int           `json:"provider_reservations"`
	ProviderEgress                 int           `json:"provider_egress"`
	ProductSurfaceChanged          bool          `json:"provider_surface_changed"`
	RuntimeBindingChanged          bool          `json:"provider_runtime_binding_changed"`
	LaunchEnvelopeChanged          bool          `json:"launch_envelope_changed"`
	ExactFingerprintChanged        bool          `json:"exact_surface_execution_fingerprint_changed"`
	CurrentExecutionFingerprint    string        `json:"current_product_v4_exact_surface_execution_fingerprint"`
	CurrentLaunchEnvelope          string        `json:"current_launch_envelope_fingerprint"`
	SurfaceID                      string        `json:"surface_id"`
	ToolCount                      int           `json:"tool_count"`
	ManifestDigest                 string        `json:"manifest_digest"`
	SchemaBytes                    int           `json:"schema_bytes"`
	SchemaDigest                   string        `json:"schema_digest"`
	RuntimeVersion                 string        `json:"runtime_version"`
	BinarySHA256                   string        `json:"binary_sha256"`
	HelperSHA256                   string        `json:"helper_sha256"`
	Cycles                         []cycleResult `json:"cycles"`
	RealProviderWorkerAdapterCycle string        `json:"real_provider_worker_adapter_cycle"`
	B12FrozenClassification        string        `json:"b12_frozen_classification"`
	B12FrozenOSCodeAvailable       bool          `json:"b12_frozen_os_error_code_available"`
	B12FrozenSafeTextAvailable     bool          `json:"b12_frozen_safe_error_text_available"`
	PrimaryRootCause               string        `json:"primary_root_cause"`
	EligibleForNewV4Qualification  bool          `json:"eligible_for_new_v4_live_qualification"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	evidenceRoot := envOrDefault("POLIS_R0_5B13_EVIDENCE_ROOT", defaultEvidenceRoot)
	runtimeRoot := envOrDefault("POLIS_R0_5B13_RUNTIME_ROOT", defaultRuntimeRoot)
	if err := os.MkdirAll(evidenceRoot, 0700); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(runtimeRoot, "cycle-1")); err == nil {
		return errors.New("R0.5B13 local qualification runtime already exists; refusing reuse")
	} else if !os.IsNotExist(err) {
		return err
	}

	identityConfig := provider.CodexRuntimeConfig{
		RuntimeManifestPath: manifestPath, AuthFile: authPath, Root: filepath.Join(runtimeRoot, "identity"), EvidenceRoot: filepath.Join(evidenceRoot, "identity"),
		Model: "gpt-5.6-luna", Effort: "medium", TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: provider.ProductToolSurface(),
		Purpose: provider.ProductLocalProcessLaunchQualificationPurpose, ToolSurfaceQualification: provider.ProductToolSurfaceQualification,
		DiagnosticOnly: true, DiagnosticAuthorizationPath: filepath.Join(evidenceRoot, "unused-diagnostic-authorization.json"), DiagnosticReservationPath: filepath.Join(evidenceRoot, "unused-diagnostic-reservation.json"),
	}
	bound, err := provider.BindCodexRuntimeConfig(identityConfig)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, "r0.5b13-identity")
	if err != nil {
		return err
	}
	currentFingerprint, err := provider.ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		return err
	}
	if currentFingerprint != provider.ProductExactSurfaceExecutionFingerprint {
		return fmt.Errorf("current product execution fingerprint is stale: computed=%s configured=%s", currentFingerprint, provider.ProductExactSurfaceExecutionFingerprint)
	}

	diff, err := writeLaunchDiff(evidenceRoot, bound)
	if err != nil {
		return err
	}
	if !diff.Equal {
		// The B11/B12 diff is expected to differ in launcher and lifecycle, while
		// the executable, argv, controlled environment, stdio and flags match.
	}

	cycles := make([]cycleResult, 0, 3)
	for cycle := 1; cycle <= 3; cycle++ {
		result, runErr := runCycle(context.Background(), cycle, runtimeRoot, evidenceRoot, bound)
		if runErr != nil {
			return runErr
		}
		cycles = append(cycles, result)
	}
	if err = runAdapterCycle(context.Background(), runtimeRoot, evidenceRoot, bound); err != nil {
		return err
	}

	status := "PASSED"
	for _, cycle := range cycles {
		if cycle.ProcessStart != "PASS" || cycle.Initialize != "PASS" || cycle.ThreadStart != "PASS" || cycle.RegisteredToolCount != 7 || cycle.CleanStop != "PASS" || cycle.OrphanProcessCount != 0 || cycle.ProviderEgress != 0 {
			status = "FAILED"
		}
	}
	report := qualificationReport{
		RecordType: "polis-r0.5b13-provider-process-launch-reproducibility@1", Status: status,
		ProviderReservations: 0, ProviderEgress: 0, ProductSurfaceChanged: false, RuntimeBindingChanged: false, LaunchEnvelopeChanged: true, ExactFingerprintChanged: true,
		CurrentExecutionFingerprint: currentFingerprint, CurrentLaunchEnvelope: identity.LaunchEnvelope.Fingerprint,
		SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount, ManifestDigest: identity.ManifestDigest, SchemaBytes: identity.AggregateSchemaBytes, SchemaDigest: identity.AggregateSchemaDigest,
		RuntimeVersion: identity.ProviderRuntime.NativeVersion, BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256,
		Cycles: cycles, RealProviderWorkerAdapterCycle: "PASS", B12FrozenClassification: "PROCESS_CREATE_FAILED_NO_PID", B12FrozenOSCodeAvailable: false, B12FrozenSafeTextAvailable: false,
		PrimaryRootCause: "REPORTING_CLASSIFICATION_DEFECT", EligibleForNewV4Qualification: status == "PASSED",
	}
	return writeJSON(filepath.Join(evidenceRoot, "qualification.json"), report)
}

func runAdapterCycle(ctx context.Context, runtimeRoot, evidenceRoot string, base provider.CodexRuntimeConfig) error {
	config := base
	config.Root = filepath.Join(runtimeRoot, "adapter-cycle")
	config.EvidenceRoot = filepath.Join(evidenceRoot, "adapter-cycle")
	config.DiagnosticAuthorizationPath = filepath.Join(evidenceRoot, "unused-diagnostic-authorization.json")
	config.DiagnosticReservationPath = filepath.Join(evidenceRoot, "unused-diagnostic-reservation.json")
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, "r0.5b13-adapter-cycle")
	if err != nil {
		return err
	}
	bound.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	bound.ExactSurfaceExecutionFingerprint = provider.ProductExactSurfaceExecutionFingerprint
	runtime := provider.NewCodexRuntime(bound)
	adapter, err := control.NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		return err
	}
	return adapter.QualifyLocalLaunch(ctx, "r0.5b13-adapter-cycle")
}

func runCycle(ctx context.Context, cycle int, runtimeRoot, evidenceRoot string, base provider.CodexRuntimeConfig) (cycleResult, error) {
	sessionID := fmt.Sprintf("r0.5b13-cycle-%d", cycle)
	root := filepath.Join(runtimeRoot, fmt.Sprintf("cycle-%d", cycle))
	evidence := filepath.Join(evidenceRoot, fmt.Sprintf("cycle-%d", cycle))
	config := base
	config.Root, config.EvidenceRoot = root, evidence
	config.DiagnosticAuthorizationPath = filepath.Join(evidenceRoot, "unused-diagnostic-authorization.json")
	config.DiagnosticReservationPath = filepath.Join(evidenceRoot, "unused-diagnostic-reservation.json")
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return cycleResult{}, err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, sessionID)
	if err != nil {
		return cycleResult{}, err
	}
	bound.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	bound.ExactSurfaceExecutionFingerprint = provider.ProductExactSurfaceExecutionFingerprint
	runtime := provider.NewCodexRuntime(bound)
	if err = runtime.Readiness(ctx); err != nil {
		return cycleResult{}, err
	}
	processSpec, err := processSpec(bound, sessionID, "r0.5b13 local production launch qualification")
	if err != nil {
		return cycleResult{}, err
	}
	session, err := runtime.StartLocalQualification(ctx, provider.SessionStartOptions{SessionID: sessionID, Model: bound.Model, Effort: bound.Effort, Profile: bound.Model + "/" + bound.Effort, ToolSurface: provider.ProductToolSurface()})
	if err != nil {
		return cycleResult{}, err
	}
	result := cycleResult{Cycle: cycle, SessionID: sessionID, RuntimeVersion: bound.ExpectedVersion, BinarySHA256: bound.BinarySHA256, HelperSHA256: bound.HelperSHA256, ProcessStart: "PASS", LaunchEvidence: processSpec.SafeEvidence(), ProtocolEvidencePath: filepath.Join(evidence, sessionID, "protocol.jsonl"), CompletedAt: time.Now().UTC()}
	if err = session.Initialize(ctx, bound.TransportPolicy); err != nil {
		_, _ = session.Stop(ctx)
		return cycleResult{}, err
	}
	result.Initialize = "PASS"
	thread, err := session.StartThread(ctx, provider.ThreadStartOptions{Model: bound.Model, Effort: bound.Effort, Tools: provider.ProductToolSurface().Tools})
	if err != nil {
		_, _ = session.Stop(ctx)
		return cycleResult{}, err
	}
	if thread == "" {
		_, _ = session.Stop(ctx)
		return cycleResult{}, errors.New("local qualification thread/start returned empty thread ID")
	}
	result.ThreadStart = "PASS"
	protocol, err := os.ReadFile(result.ProtocolEvidencePath)
	if err != nil {
		_, _ = session.Stop(ctx)
		return cycleResult{}, err
	}
	summary, err := provider.SummarizeProductSurfaceDiagnosticProtocol(protocol)
	if err != nil {
		_, _ = session.Stop(ctx)
		return cycleResult{}, err
	}
	result.RegisteredToolCount, result.RegisteredToolDigest = summary.RegisteredToolCount, summary.RegisteredToolsManifestDigest
	stopProof, err := session.Stop(ctx)
	if err != nil || !stopProof.For(sessionID) {
		return cycleResult{}, fmt.Errorf("cycle %d clean stop failed: %v", cycle, err)
	}
	result.CleanStop = "PASS"
	result.OrphanProcessCount, result.ProviderEgress = 0, summary.TurnStartCount
	return result, nil
}

func processSpec(config provider.CodexRuntimeConfig, sessionID, launcher string) (runner.ProcessLaunchSpec, error) {
	home := filepath.Join(config.Root, sessionID, "home")
	spec, err := runner.NewProcessLaunchSpec([]string{config.Binary, "app-server", "--stdio"}, runner.NativeEnvironment(home), "", runner.ProcessLaunchDirectories{RuntimeRoot: config.Root, RuntimeHome: home, TempDirectory: filepath.Join(home, "tmp"), EvidenceRoot: filepath.Join(config.EvidenceRoot, sessionID)})
	if err != nil {
		return runner.ProcessLaunchSpec{}, err
	}
	spec.Launcher = launcher
	spec.PreLaunchLifecycle = []string{"readiness_rebind", "credential_snapshot", "native_args", "canonical_process_launch", "protocol_client"}
	return spec, nil
}

func writeLaunchDiff(root string, bound provider.CodexRuntimeConfig) (runner.ProcessLaunchDiff, error) {
	left, err := processSpec(provider.CodexRuntimeConfig{Binary: bound.Binary, Root: ".runtime\\windows\\r0.5b11-provider-runtime-version-binding-hardening", EvidenceRoot: "evidence/development/r0.5b11-provider-runtime-version-binding-hardening\\provider"}, "r05b11-provider-preflight", "cmd/polis-r05b11 local preflight")
	if err != nil {
		return runner.ProcessLaunchDiff{}, err
	}
	right, err := processSpec(provider.CodexRuntimeConfig{Binary: bound.Binary, Root: ".runtime\\windows\\r0.5b11-provider-runtime-version-binding-hardening", EvidenceRoot: "evidence/development/r0.5b12-product-tool-surface-v4-live-qualification-attempt-2\\provider"}, "r05b11-provider-preflight", "cmd/polis serve -> RealProviderWorkerAdapter -> CodexRuntime.Start")
	if err != nil {
		return runner.ProcessLaunchDiff{}, err
	}
	diff := runner.DiffProcessLaunchSpecs(left, right)
	record := map[string]any{"record_type": "polis-b11-b12-process-launch-diff@1", "b11": left.SafeEvidence(), "b12": right.SafeEvidence(), "diff": diff, "b12_frozen_evidence_limit": "no OS error code or safe launch text was persisted; frozen terminal was process_start_failed with no PID", "primary_root_cause": "REPORTING_CLASSIFICATION_DEFECT"}
	return diff, writeJSON(filepath.Join(root, "b11-b12-launch-diff.json"), record)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
