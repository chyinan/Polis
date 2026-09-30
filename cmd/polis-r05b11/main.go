// pattern: Imperative Shell
package main

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
	"polis/internal/provider"
	"polis/internal/runner"
)

const (
	preflightEvidenceRoot  = "evidence/development/r0.5b11-provider-runtime-version-binding-hardening"
	preflightSessionID     = "r05b11-provider-preflight"
	defaultRuntimeManifest = `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json`
	defaultAuthFile        = `C:\Users\chyinan\.codex\auth.json`
)

type preflightReport struct {
	RecordType            string                                   `json:"record_type"`
	Status                string                                   `json:"status"`
	RuntimeManifest       string                                   `json:"runtime_manifest"`
	RuntimeImplementation string                                   `json:"runtime_implementation"`
	SemanticVersion       string                                   `json:"semantic_version"`
	BinaryPath            string                                   `json:"binary_path"`
	HelperPath            string                                   `json:"helper_path"`
	BinarySHA256          string                                   `json:"binary_sha256"`
	HelperSHA256          string                                   `json:"helper_sha256"`
	LaunchMode            string                                   `json:"launch_mode"`
	ProtocolCompatibility string                                   `json:"protocol_compatibility"`
	ExecutionFingerprint  string                                   `json:"product_v4_exact_surface_execution_fingerprint"`
	SurfaceID             string                                   `json:"surface_id"`
	ToolCount             int                                      `json:"tool_count"`
	ManifestDigest        string                                   `json:"manifest_digest"`
	AggregateSchemaBytes  int                                      `json:"aggregate_schema_bytes"`
	AggregateSchemaDigest string                                   `json:"aggregate_schema_digest"`
	ProcessStart          string                                   `json:"process_start"`
	Initialize            string                                   `json:"initialize"`
	ThreadStart           string                                   `json:"thread_start"`
	TurnStarted           bool                                     `json:"turn_started"`
	ProviderEgress        int                                      `json:"provider_egress"`
	RegisteredToolCount   int                                      `json:"registered_tool_count"`
	CleanStop             string                                   `json:"clean_stop"`
	StopProof             string                                   `json:"stop_proof"`
	Identity              provider.ProductSurfaceExecutionIdentity `json:"identity"`
	CreatedAt             time.Time                                `json:"created_at"`
}

func main() {
	if err := runPreflight(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runPreflight() error {
	evidenceRoot := envOrDefault("POLIS_R05B11_EVIDENCE_ROOT", preflightEvidenceRoot)
	manifestPath := envOrDefault("POLIS_PROVIDER_RUNTIME_MANIFEST", defaultRuntimeManifest)
	authFile := envOrDefault("POLIS_PROVIDER_AUTH_FILE", defaultAuthFile)
	runtimeRoot := envOrDefault("POLIS_PROVIDER_ROOT", filepath.Join(".runtime", "windows", "r0.5b11-provider-runtime-version-binding-hardening"))
	evidenceProviderRoot := filepath.Join(evidenceRoot, "provider")
	policy := codex.DefaultTransportPolicy()
	config := provider.CodexRuntimeConfig{
		RuntimeManifestPath:         manifestPath,
		AuthFile:                    authFile,
		Root:                        runtimeRoot,
		EvidenceRoot:                evidenceProviderRoot,
		Model:                       "gpt-5.6-luna",
		Effort:                      "medium",
		TransportPolicy:             policy,
		ToolSurface:                 provider.ProductToolSurface(),
		Purpose:                     "R0.5B11_PROVIDER_RUNTIME_PREFLIGHT",
		DiagnosticOnly:              true,
		DiagnosticAuthorizationPath: filepath.Join(evidenceRoot, "unused-diagnostic-authorization.json"),
		DiagnosticReservationPath:   filepath.Join(evidenceRoot, "unused-diagnostic-reservation.json"),
		ToolSurfaceQualification:    provider.ProductToolSurfaceQualification,
	}
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, preflightSessionID)
	if err != nil {
		return fmt.Errorf("build preflight identity: %w", err)
	}
	if fingerprint := executionFingerprint(identity); fingerprint != provider.ProductExactSurfaceExecutionFingerprint {
		return fmt.Errorf("current product-v4 execution fingerprint is stale: computed=%s configured=%s", fingerprint, provider.ProductExactSurfaceExecutionFingerprint)
	}
	bound.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	productRuntime := provider.NewCodexRuntime(bound)
	if err = productRuntime.Readiness(context.Background()); err != nil {
		return fmt.Errorf("provider runtime readiness failed: %w", err)
	}

	if err = os.MkdirAll(evidenceProviderRoot, 0700); err != nil {
		return err
	}
	home := filepath.Join(bound.Root, preflightSessionID, "home")
	args, _, err := runner.NativeArgsWithTransportPolicy(bound.Binary, home, bound.AuthFile, "", runner.NativeTransportPolicyNativeDefault)
	if err != nil {
		return fmt.Errorf("build provider launch: %w", err)
	}
	spec, err := runner.NewProcessLaunchSpec(args, runner.NativeEnvironment(home), "", runner.ProcessLaunchDirectories{
		RuntimeRoot: bound.Root, RuntimeHome: home, TempDirectory: filepath.Join(home, "tmp"), EvidenceRoot: evidenceProviderRoot,
	})
	if err != nil {
		return fmt.Errorf("build provider launch spec: %w", err)
	}
	spec.Launcher = "cmd/polis-r05b11 local preflight"
	spec.PreLaunchLifecycle = []string{"manifest_resolve", "readiness_rebind", "native_args", "canonical_process_launch", "protocol_client"}
	process, err := runner.StartWithLaunchSpec(preflightSessionID, spec)
	if err != nil {
		return fmt.Errorf("start provider runtime: %w", err)
	}
	client, err := codex.NewWithModelAndVersion(process, evidenceProviderRoot, bound.Model, bound.ExpectedVersion)
	if err != nil {
		_, _ = process.Stop()
		return fmt.Errorf("create local protocol client: %w", err)
	}

	processStart := "PASS"
	initialize := "FAIL"
	threadStart := "NOT_STARTED"
	stopStatus := "FAIL"
	stopProofDescription := ""
	ctx, cancel := context.WithTimeout(context.Background(), policy.InitializeTimeout+policy.StartAcknowledgement)
	defer cancel()
	if err = client.InitializeWithPolicy(ctx, policy); err != nil {
		stopProof, stopErr := process.Stop()
		client.Close()
		if stopErr == nil {
			stopStatus = "PASS"
			stopProofDescription = stopProof.Description()
		}
		return writePreflightReport(evidenceRoot, preflightReport{
			RecordType: "polis-r0.5b11-provider-runtime-preflight@1", Status: "FAILED", RuntimeManifest: manifestPath,
			RuntimeImplementation: identity.ProviderRuntime.RuntimeImplementation, SemanticVersion: identity.ProviderRuntime.NativeVersion,
			BinaryPath: identity.LaunchEnvelope.BinaryPath, HelperPath: identity.LaunchEnvelope.HelperPath,
			BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256,
			LaunchMode: identity.LaunchEnvelope.LaunchMode, ProtocolCompatibility: identity.ProviderRuntime.ProtocolCompatibility,
			ExecutionFingerprint: executionFingerprint(identity), SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount,
			ManifestDigest: identity.ManifestDigest, AggregateSchemaBytes: identity.AggregateSchemaBytes, AggregateSchemaDigest: identity.AggregateSchemaDigest,
			ProcessStart: processStart, Initialize: initialize, ThreadStart: threadStart, TurnStarted: false, ProviderEgress: 0,
			CleanStop: stopStatus, StopProof: stopProofDescription, Identity: identity, CreatedAt: time.Now().UTC(),
		}, fmt.Errorf("local initialize failed: %w", err))
	}
	initialize = "PASS"
	thread, err := client.StartThreadWithTools(ctx, bound.Effort, bound.ToolSurface.Tools, "R0.5B11 local no-provider preflight; no turn is permitted.")
	if err != nil {
		stopProof, stopErr := process.Stop()
		client.Close()
		if stopErr == nil {
			stopStatus = "PASS"
			stopProofDescription = stopProof.Description()
		}
		return writePreflightReport(evidenceRoot, preflightReport{
			RecordType: "polis-r0.5b11-provider-runtime-preflight@1", Status: "FAILED", RuntimeManifest: manifestPath,
			RuntimeImplementation: identity.ProviderRuntime.RuntimeImplementation, SemanticVersion: identity.ProviderRuntime.NativeVersion,
			BinaryPath: identity.LaunchEnvelope.BinaryPath, HelperPath: identity.LaunchEnvelope.HelperPath,
			BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256,
			LaunchMode: identity.LaunchEnvelope.LaunchMode, ProtocolCompatibility: identity.ProviderRuntime.ProtocolCompatibility,
			ExecutionFingerprint: executionFingerprint(identity), SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount,
			ManifestDigest: identity.ManifestDigest, AggregateSchemaBytes: identity.AggregateSchemaBytes, AggregateSchemaDigest: identity.AggregateSchemaDigest,
			ProcessStart: processStart, Initialize: initialize, ThreadStart: threadStart, TurnStarted: false, ProviderEgress: 0,
			CleanStop: stopStatus, StopProof: stopProofDescription, Identity: identity, CreatedAt: time.Now().UTC(),
		}, fmt.Errorf("local thread/start failed: %w", err))
	}
	if thread == "" {
		return errors.New("local thread/start returned an empty thread ID")
	}
	threadStart = "PASS"
	client.Close()
	stopProof, stopErr := process.Stop()
	if stopErr != nil || !stopProof.For(preflightSessionID) {
		return fmt.Errorf("local provider clean stop failed: %v", stopErr)
	}
	stopStatus = "PASS"
	stopProofDescription = stopProof.Description()
	protocol, err := os.ReadFile(filepath.Join(evidenceProviderRoot, "protocol.jsonl"))
	if err != nil {
		return err
	}
	if err = validateNoProviderProtocol(protocol); err != nil {
		return err
	}
	return writePreflightReport(evidenceRoot, preflightReport{
		RecordType: "polis-r0.5b11-provider-runtime-preflight@1", Status: "PASSED", RuntimeManifest: manifestPath,
		RuntimeImplementation: identity.ProviderRuntime.RuntimeImplementation, SemanticVersion: identity.ProviderRuntime.NativeVersion,
		BinaryPath: identity.LaunchEnvelope.BinaryPath, HelperPath: identity.LaunchEnvelope.HelperPath,
		BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256,
		LaunchMode: identity.LaunchEnvelope.LaunchMode, ProtocolCompatibility: identity.ProviderRuntime.ProtocolCompatibility,
		ExecutionFingerprint: executionFingerprint(identity), SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount,
		ManifestDigest: identity.ManifestDigest, AggregateSchemaBytes: identity.AggregateSchemaBytes, AggregateSchemaDigest: identity.AggregateSchemaDigest,
		ProcessStart: processStart, Initialize: initialize, ThreadStart: threadStart, TurnStarted: false, ProviderEgress: 0,
		RegisteredToolCount: identity.ToolCount, CleanStop: stopStatus, StopProof: stopProofDescription, Identity: identity, CreatedAt: time.Now().UTC(),
	}, nil)
}

func validateNoProviderProtocol(protocol []byte) error {
	text := string(protocol)
	if strings.Contains(text, `"method":"turn/start"`) || strings.Contains(text, `"method":"turn/started"`) {
		return errors.New("no-provider preflight protocol contains a turn request")
	}
	if !strings.Contains(text, `"method":"initialize"`) || !strings.Contains(text, `"method":"thread/start"`) {
		return errors.New("no-provider preflight protocol is missing initialize or thread/start")
	}
	return nil
}

func executionFingerprint(identity provider.ProductSurfaceExecutionIdentity) string {
	fingerprint, err := provider.ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		return ""
	}
	return fingerprint
}

func writePreflightReport(root string, report preflightReport, runErr error) error {
	if runErr != nil {
		report.Status = "FAILED"
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "local-preflight.json"), append(raw, '\n'), 0600); err != nil {
		return err
	}
	if runErr != nil {
		return runErr
	}
	fmt.Printf("r0_5b11_provider_runtime_preflight=%s execution_fingerprint=%s provider_egress=0\n", report.Status, report.ExecutionFingerprint)
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
