// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"polis/internal/codex"
	"polis/internal/control"
	"polis/internal/provider"
)

const (
	evidenceRoot = "evidence/development/r0.5b14-product-tool-surface-v4-live-qualification-attempt-3/runtime-preflight"
	runtimeRoot  = ".runtime/windows/r0.5b14-product-surface-v4"
	manifestPath = `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json`
	authPath     = `C:\Users\chyinan\.codex\auth.json`
	sessionID    = "r05b14-product-surface-v4-preflight"
)

type preflightReport struct {
	RecordType             string                                   `json:"record_type"`
	Status                 string                                   `json:"status"`
	ProviderReservations   int                                      `json:"provider_reservations"`
	ProviderEgress         int                                      `json:"provider_egress"`
	TurnStarted            bool                                     `json:"turn_started"`
	ProcessCreated         bool                                     `json:"process_created"`
	PIDPresent             bool                                     `json:"pid_present"`
	Initialize             string                                   `json:"initialize"`
	ThreadStart            string                                   `json:"thread_start"`
	RegisteredToolCount    int                                      `json:"registered_tool_count"`
	RegisteredToolManifest string                                   `json:"registered_tool_manifest"`
	CleanStop              string                                   `json:"clean_stop"`
	RuntimeManifest        string                                   `json:"runtime_manifest"`
	RuntimeVersion         string                                   `json:"runtime_version"`
	BinarySHA256           string                                   `json:"binary_sha256"`
	HelperSHA256           string                                   `json:"helper_sha256"`
	SurfaceID              string                                   `json:"surface_id"`
	ToolCount              int                                      `json:"tool_count"`
	ManifestDigest         string                                   `json:"manifest_digest"`
	SchemaBytes            int                                      `json:"schema_bytes"`
	SchemaDigest           string                                   `json:"schema_digest"`
	LaunchEnvelope         string                                   `json:"launch_envelope_fingerprint"`
	ExecutionFingerprint   string                                   `json:"exact_surface_execution_fingerprint"`
	Identity               provider.ProductSurfaceExecutionIdentity `json:"execution_identity"`
	ProtocolEvidence       string                                   `json:"protocol_evidence"`
	CompletedAt            time.Time                                `json:"completed_at"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reconcile" {
		if err := reconcileLiveEvidence(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "freshness" {
		if err := verifyPostCanaryFreshness(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type postCanaryFreshness struct {
	RecordType               string    `json:"record_type"`
	Status                   string    `json:"status"`
	SurfaceUnchanged         bool      `json:"surface_unchanged"`
	RuntimeBindingUnchanged  bool      `json:"runtime_binding_unchanged"`
	LaunchEnvelopeUnchanged  bool      `json:"launch_envelope_unchanged"`
	ExecutionFingerprintSame bool      `json:"execution_fingerprint_unchanged"`
	SurfaceID                string    `json:"surface_id"`
	ToolCount                int       `json:"tool_count"`
	ManifestDigest           string    `json:"manifest_digest"`
	SchemaBytes              int       `json:"schema_bytes"`
	SchemaDigest             string    `json:"schema_digest"`
	RuntimeVersion           string    `json:"runtime_version"`
	BinarySHA256             string    `json:"binary_sha256"`
	HelperSHA256             string    `json:"helper_sha256"`
	LaunchEnvelope           string    `json:"launch_envelope_fingerprint"`
	ExecutionFingerprint     string    `json:"exact_surface_execution_fingerprint"`
	CheckedAt                time.Time `json:"checked_at"`
}

func verifyPostCanaryFreshness() error {
	qualificationRoot := filepath.Dir(evidenceRoot)
	authorizationRaw, err := os.ReadFile(filepath.Join(qualificationRoot, "diagnostic-authorization.json"))
	if err != nil {
		return err
	}
	var authorization provider.ProductSurfaceDiagnosticAuthorization
	if err = json.Unmarshal(authorizationRaw, &authorization); err != nil {
		return err
	}
	config := provider.CodexRuntimeConfig{
		RuntimeManifestPath: manifestPath, AuthFile: authPath, Root: runtimeRoot, EvidenceRoot: filepath.Join(qualificationRoot, "provider"),
		Model: "gpt-5.6-luna", Effort: "medium", ExpectedVersion: "0.154.0-alpha.6.2", TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: provider.ProductToolSurface(),
		Purpose: provider.ProductSurfaceDiagnosticAttempt3Purpose, ToolSurfaceQualification: provider.ProductToolSurfaceQualification, DiagnosticOnly: true,
		DiagnosticAuthorizationPath: filepath.Join(qualificationRoot, "diagnostic-authorization.json"), DiagnosticReservationPath: filepath.Join(qualificationRoot, "diagnostic-reservation.json"),
	}
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, authorization.ProviderSessionID)
	if err != nil {
		return err
	}
	fingerprint, err := provider.ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		return err
	}
	surface := provider.ProductToolSurface()
	surfaceUnchanged := identity.SurfaceID == authorization.ExecutionIdentity.SurfaceID && identity.ToolCount == authorization.ExecutionIdentity.ToolCount && identity.ManifestDigest == authorization.ExecutionIdentity.ManifestDigest && identity.AggregateSchemaBytes == authorization.ExecutionIdentity.AggregateSchemaBytes && identity.AggregateSchemaDigest == authorization.ExecutionIdentity.AggregateSchemaDigest && surface.ManifestDigest == identity.ManifestDigest && surface.AggregateSchemaBytes == identity.AggregateSchemaBytes && surface.AggregateSchemaDigest == identity.AggregateSchemaDigest
	runtimeUnchanged := reflect.DeepEqual(identity.ProviderRuntime, authorization.ExecutionIdentity.ProviderRuntime)
	launchUnchanged := identity.LaunchEnvelope.Fingerprint == authorization.ExecutionIdentity.LaunchEnvelope.Fingerprint
	executionUnchanged := fingerprint == authorization.ExecutionFingerprint && fingerprint == provider.ProductExactSurfaceExecutionFingerprint
	status := "PASSED"
	if !surfaceUnchanged || !runtimeUnchanged || !launchUnchanged || !executionUnchanged {
		status = "STALE"
	}
	report := postCanaryFreshness{RecordType: "polis-r0.5b14-post-canary-freshness@1", Status: status, SurfaceUnchanged: surfaceUnchanged, RuntimeBindingUnchanged: runtimeUnchanged, LaunchEnvelopeUnchanged: launchUnchanged, ExecutionFingerprintSame: executionUnchanged, SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount, ManifestDigest: identity.ManifestDigest, SchemaBytes: identity.AggregateSchemaBytes, SchemaDigest: identity.AggregateSchemaDigest, RuntimeVersion: identity.ProviderRuntime.NativeVersion, BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256, LaunchEnvelope: identity.LaunchEnvelope.Fingerprint, ExecutionFingerprint: fingerprint, CheckedAt: time.Now().UTC()}
	if status != "PASSED" {
		return fmt.Errorf("post-canary freshness is stale: %+v", report)
	}
	return writeJSON(filepath.Join(qualificationRoot, "post-canary-freshness.json"), report)
}

type terminalEvidence struct {
	AuthorizationID    string    `json:"authorization_id"`
	ProcessStartedAt   time.Time `json:"process_started_at"`
	ProcessIdentity    string    `json:"process_identity"`
	ProcessID          int       `json:"process_id"`
	ProcessCreated     bool      `json:"process_created"`
	PIDPresent         bool      `json:"pid_present"`
	ProviderEgress     int       `json:"provider_egress"`
	TurnTerminal       string    `json:"turn_terminal"`
	CleanStop          bool      `json:"clean_stop"`
	ProtocolError      bool      `json:"protocol_evidence_error"`
	BusinessMutations  int       `json:"business_mutation_count"`
	CompanyCount       int       `json:"company_count"`
	MissionCount       int       `json:"mission_count"`
	TaskCount          int       `json:"task_count"`
	WorkerSessionCount int       `json:"worker_session_count"`
}

type launchEvidenceReconciliation struct {
	RecordType          string    `json:"record_type"`
	Status              string    `json:"status"`
	AuthorizationID     string    `json:"authorization_id"`
	ProcessCreated      bool      `json:"process_created"`
	PIDPresent          bool      `json:"pid_present"`
	ProcessID           int       `json:"process_id"`
	ProcessStartedAt    time.Time `json:"process_started_at"`
	ProcessIdentity     string    `json:"process_identity"`
	EvidenceBasis       string    `json:"evidence_basis"`
	ProviderEgress      int       `json:"provider_egress"`
	TurnTerminal        string    `json:"turn_terminal"`
	CleanStop           bool      `json:"clean_stop"`
	ProtocolEvidenceOK  bool      `json:"protocol_evidence_ok"`
	BusinessSideEffects int       `json:"business_side_effects"`
	ReconciledAt        time.Time `json:"reconciled_at"`
}

func reconcileLiveEvidence() error {
	terminalPath := filepath.Join("evidence/development/r0.5b14-product-tool-surface-v4-live-qualification-attempt-3", "provider-transport-terminal.json")
	raw, err := os.ReadFile(terminalPath)
	if err != nil {
		return err
	}
	var terminal terminalEvidence
	if err = json.Unmarshal(raw, &terminal); err != nil {
		return err
	}
	if terminal.ProcessID <= 0 || terminal.ProcessStartedAt.IsZero() {
		return errors.New("live terminal evidence does not contain a usable process identity")
	}
	if terminal.ProviderEgress != 1 || terminal.TurnTerminal != "completed" || !terminal.CleanStop || terminal.ProtocolError {
		return errors.New("live terminal evidence is not a completed clean diagnostic turn")
	}
	businessSideEffects := terminal.CompanyCount + terminal.MissionCount + terminal.TaskCount + terminal.WorkerSessionCount + terminal.BusinessMutations
	if businessSideEffects != 0 {
		return errors.New("live terminal evidence contains business side effects")
	}
	reconciled := launchEvidenceReconciliation{
		RecordType: "polis-r0.5b14-launch-evidence-reconciliation@1", Status: "PASSED", AuthorizationID: terminal.AuthorizationID,
		ProcessCreated: true, PIDPresent: true, ProcessID: terminal.ProcessID, ProcessStartedAt: terminal.ProcessStartedAt, ProcessIdentity: terminal.ProcessIdentity,
		EvidenceBasis: "process_started_at_and_process_id_present_in_provider-transport-terminal.json", ProviderEgress: terminal.ProviderEgress, TurnTerminal: terminal.TurnTerminal,
		CleanStop: terminal.CleanStop, ProtocolEvidenceOK: !terminal.ProtocolError, BusinessSideEffects: businessSideEffects, ReconciledAt: time.Now().UTC(),
	}
	return writeJSON(filepath.Join("evidence/development/r0.5b14-product-tool-surface-v4-live-qualification-attempt-3", "launch-evidence-reconciliation.json"), reconciled)
}

func run() error {
	if _, err := os.Stat(filepath.Join(runtimeRoot, sessionID)); err == nil {
		return errors.New("B14 local preflight runtime session already exists; refusing reuse")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(filepath.Join(evidenceRoot, sessionID)); err == nil {
		return errors.New("B14 local preflight evidence session already exists; refusing reuse")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	config := provider.CodexRuntimeConfig{
		RuntimeManifestPath:         manifestPath,
		AuthFile:                    authPath,
		Root:                        runtimeRoot,
		EvidenceRoot:                evidenceRoot,
		Model:                       "gpt-5.6-luna",
		Effort:                      "medium",
		TransportPolicy:             codex.DefaultTransportPolicy(),
		ToolSurface:                 provider.ProductToolSurface(),
		Purpose:                     provider.ProductLocalProcessLaunchQualificationPurpose,
		ToolSurfaceQualification:    provider.ProductToolSurfaceQualification,
		DiagnosticOnly:              true,
		DiagnosticAuthorizationPath: filepath.Join(evidenceRoot, "unused-diagnostic-authorization.json"),
		DiagnosticReservationPath:   filepath.Join(evidenceRoot, "unused-diagnostic-reservation.json"),
	}
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(bound, sessionID)
	if err != nil {
		return err
	}
	if identity.LaunchEnvelope.Fingerprint != "dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d" {
		return fmt.Errorf("unexpected B14 launch envelope fingerprint %q", identity.LaunchEnvelope.Fingerprint)
	}
	if got, err := provider.ComputeProductSurfaceExecutionFingerprint(identity); err != nil {
		return err
	} else if got != provider.ProductExactSurfaceExecutionFingerprint {
		return fmt.Errorf("unexpected B14 exact-surface execution fingerprint %q", got)
	}
	bound.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	bound.ExactSurfaceExecutionFingerprint = provider.ProductExactSurfaceExecutionFingerprint
	runtime := provider.NewCodexRuntime(bound)
	adapter, err := control.NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		return err
	}
	if err = adapter.QualifyLocalLaunch(context.Background(), sessionID); err != nil {
		return err
	}

	protocolPath := filepath.Join(evidenceRoot, sessionID, "protocol.jsonl")
	protocol, err := os.ReadFile(protocolPath)
	if err != nil {
		return err
	}
	summary, err := provider.SummarizeProductSurfaceDiagnosticProtocol(protocol)
	if err != nil {
		return err
	}
	if summary.TurnStartCount != 0 || summary.RegisteredToolCount != 7 || summary.RegisteredToolsManifestDigest != identity.ManifestDigest {
		return fmt.Errorf("B14 local preflight protocol does not prove zero-turn @4 registration: %+v", summary)
	}
	report := preflightReport{
		RecordType: "polis-r0.5b14-local-production-preflight@1", Status: "PASSED", ProviderReservations: 0, ProviderEgress: 0, TurnStarted: false,
		ProcessCreated: true, PIDPresent: true, Initialize: "PASS", ThreadStart: "PASS", RegisteredToolCount: summary.RegisteredToolCount, RegisteredToolManifest: summary.RegisteredToolsManifestDigest, CleanStop: "PASS",
		RuntimeManifest: manifestPath, RuntimeVersion: identity.ProviderRuntime.NativeVersion, BinarySHA256: identity.ProviderRuntime.BinarySHA256, HelperSHA256: identity.ProviderRuntime.HelperSHA256,
		SurfaceID: identity.SurfaceID, ToolCount: identity.ToolCount, ManifestDigest: identity.ManifestDigest, SchemaBytes: identity.AggregateSchemaBytes, SchemaDigest: identity.AggregateSchemaDigest,
		LaunchEnvelope: identity.LaunchEnvelope.Fingerprint, ExecutionFingerprint: provider.ProductExactSurfaceExecutionFingerprint, Identity: identity, ProtocolEvidence: protocolPath, CompletedAt: time.Now().UTC(),
	}
	return writeJSON(filepath.Join(evidenceRoot, "local-preflight.json"), report)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
