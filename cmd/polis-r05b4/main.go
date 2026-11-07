// pattern: Imperative Shell
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/provider"
)

const (
	evidenceRootDefault  = "evidence/development/r0.5b4-product-tool-surface-v2-live-qualification"
	diagnosticID         = "r0.5b4-live-canary-1"
	diagnosticSessionID  = "r05b4-product-surface-v2-canary"
	qualificationPassed  = "PASSED"
	qualificationStale   = "STALE"
	qualificationUnknown = "INCONCLUSIVE"
)

type exactProductSurface struct {
	RecordType                string `json:"record_type"`
	SurfaceID                 string `json:"surface_id"`
	ToolCount                 int    `json:"tool_count"`
	Tools                     []any  `json:"tools"`
	ManifestDigest            string `json:"manifest_digest"`
	AggregateSchemaBytes      int    `json:"aggregate_schema_bytes"`
	AggregateSchemaDigest     string `json:"aggregate_schema_digest"`
	HistoricalB2SurfaceStatus string `json:"historical_b2_surface_status"`
	HistoricalR03ASurface     string `json:"historical_r03a_surfaces_status"`
}

type offlineL2Record struct {
	RecordType                string          `json:"record_type"`
	Status                    string          `json:"status"`
	SurfaceID                 string          `json:"surface_id"`
	ToolCount                 int             `json:"tool_count"`
	ManifestDigest            string          `json:"manifest_digest"`
	AggregateSchemaBytes      int             `json:"aggregate_schema_bytes"`
	AggregateSchemaDigest     string          `json:"aggregate_schema_digest"`
	Gates                     map[string]bool `json:"gates"`
	TestArtifacts             []string        `json:"test_artifacts"`
	ProviderReservations      int             `json:"provider_reservations"`
	ProviderEgress            int             `json:"provider_egress"`
	DisposableDatabaseRemoved bool            `json:"disposable_database_removed"`
	BusinessDatabaseUsed      bool            `json:"business_database_used"`
	HistoricalB2Status        string          `json:"historical_b2_surface_status"`
	HistoricalR03AStatus      string          `json:"historical_r03a_surfaces_status"`
	RecordedAt                time.Time       `json:"recorded_at"`
}

type offlineQualificationDigestBinding struct {
	SchemaVersion            string `json:"schema_version"`
	OfflineL2ResultSHA256    string `json:"offline_l2_result_sha256"`
	FinalProviderTestsSHA256 string `json:"final_provider_tests_sha256"`
}

type executionManifest struct {
	RecordType                  string                                   `json:"record_type"`
	ExecutionFingerprint        string                                   `json:"product_exact_surface_execution_fingerprint"`
	ExecutionIdentity           provider.ProductSurfaceExecutionIdentity `json:"execution_identity"`
	ReusedHistoricalFingerprint bool                                     `json:"reused_historical_fingerprint"`
	CreatedAt                   time.Time                                `json:"created_at"`
}

type providerTransportTerminal struct {
	RecordType                      string                                           `json:"record_type"`
	AuthorizationID                 string                                           `json:"authorization_id"`
	ExecutionFingerprint            string                                           `json:"product_exact_surface_execution_fingerprint"`
	DiagnosticReservationCount      int                                              `json:"diagnostic_reservation_count"`
	ProviderEgress                  int                                              `json:"provider_egress"`
	RegisteredToolCount             int                                              `json:"registered_tool_count"`
	ProcessStartRequestedAt         time.Time                                        `json:"process_start_requested_at,omitempty"`
	ProcessStartedAt                time.Time                                        `json:"process_started_at,omitempty"`
	ProcessIdentity                 string                                           `json:"process_identity,omitempty"`
	ProcessID                       int                                              `json:"process_id,omitempty"`
	InitializeStartedAt             time.Time                                        `json:"initialize_started_at,omitempty"`
	InitializeFinishedAt            time.Time                                        `json:"initialize_finished_at,omitempty"`
	ThreadStartStartedAt            time.Time                                        `json:"thread_start_started_at,omitempty"`
	ThreadStartFinishedAt           time.Time                                        `json:"thread_start_finished_at,omitempty"`
	TurnCallStartedAt               time.Time                                        `json:"turn_call_started_at,omitempty"`
	TurnCallFinishedAt              time.Time                                        `json:"turn_call_finished_at,omitempty"`
	TurnStartAt                     time.Time                                        `json:"turn_start_at,omitempty"`
	FirstOutputAt                   time.Time                                        `json:"first_output_at,omitempty"`
	FirstOutputLatencyMS            int64                                            `json:"first_output_latency_ms"`
	FirstOutput                     string                                           `json:"first_output"`
	TurnCompletedCount              int                                              `json:"turn_completed_count"`
	TurnCompletedAt                 time.Time                                        `json:"turn_completed_at,omitempty"`
	TokenUsage                      codex.TokenUsage                                 `json:"token_usage"`
	ToolCalls                       int                                              `json:"tool_calls"`
	ReconnectCount                  int                                              `json:"reconnect_count"`
	TurnTerminal                    string                                           `json:"turn_terminal"`
	CleanStop                       bool                                             `json:"clean_stop"`
	SecondReservationDenied         bool                                             `json:"second_reservation_denied"`
	StopCompletedAt                 time.Time                                        `json:"stop_completed_at,omitempty"`
	CredentialSnapshotRemoved       bool                                             `json:"credential_snapshot_removed"`
	CredentialSnapshotCleanupFailed bool                                             `json:"credential_snapshot_cleanup_failed"`
	ElapsedMS                       int64                                            `json:"elapsed_ms"`
	CompanyCount                    int                                              `json:"company_count"`
	MissionCount                    int                                              `json:"mission_count"`
	TaskCount                       int                                              `json:"task_count"`
	WorkerSessionCount              int                                              `json:"worker_session_count"`
	SuccessorCount                  int                                              `json:"successor_count"`
	BusinessMutationCount           int                                              `json:"business_mutation_count"`
	InitializationFailed            bool                                             `json:"initialization_failed"`
	ThreadStartFailed               bool                                             `json:"thread_start_failed"`
	TurnFailed                      bool                                             `json:"turn_failed"`
	ProtocolEvidenceError           bool                                             `json:"protocol_evidence_error"`
	StopError                       bool                                             `json:"stop_error"`
	Protocol                        provider.ProductSurfaceDiagnosticProtocolSummary `json:"protocol"`
}

type liveCanaryResult struct {
	RecordType                   string                    `json:"record_type"`
	Qualification                string                    `json:"product_surface_v2_live_l2"`
	ProductProviderSurfaceStatus string                    `json:"product_provider_surface_status"`
	EligibleForRealProviderSmoke string                    `json:"eligible_for_r0_5b_real_provider_smoke"`
	ProductProviderL2Fingerprint string                    `json:"product_provider_L2_fingerprint,omitempty"`
	ExecutionFingerprint         string                    `json:"product_exact_surface_execution_fingerprint"`
	SurfaceBefore                exactProductSurface       `json:"surface_before"`
	SurfaceAfter                 exactProductSurface       `json:"surface_after"`
	SurfaceFresh                 bool                      `json:"surface_fresh"`
	Terminal                     providerTransportTerminal `json:"provider_terminal"`
	HistoricalB2SurfaceStatus    string                    `json:"historical_b2_surface_status"`
	HistoricalR03ASurfaceStatus  string                    `json:"historical_r03a_surfaces_status"`
	RecordedAt                   time.Time                 `json:"recorded_at"`
}

type duplicateReservationGuard struct {
	RecordType              string    `json:"record_type"`
	AuthorizationID         string    `json:"authorization_id"`
	PriorReservationNumber  int       `json:"prior_reservation_number"`
	SecondReservationStatus string    `json:"second_reservation_status"`
	SecondProviderProcess   bool      `json:"second_provider_process_started"`
	SecondProviderEgress    int       `json:"second_provider_egress"`
	CheckedAt               time.Time `json:"checked_at"`
}

func main() {
	if len(os.Args) < 2 {
		fatal(errors.New("usage: polis-r05b4 {surface|offline-result|offline-finalize|authorize|canary}"))
	}
	var err error
	switch os.Args[1] {
	case "surface":
		err = runSurface()
	case "offline-result":
		err = runOfflineResult()
	case "offline-finalize":
		err = runOfflineFinalize()
	case "authorize":
		err = runAuthorize()
	case "canary":
		err = runCanary()
	default:
		err = fmt.Errorf("unsupported command %q", os.Args[1])
	}
	if err != nil {
		fatal(err)
	}
}

func runOfflineFinalize() error {
	root := evidenceRoot()
	raw, err := os.ReadFile(filepath.Join(root, "offline-l2-result.json"))
	if err != nil {
		return err
	}
	var offline offlineL2Record
	if err = json.Unmarshal(raw, &offline); err != nil {
		return err
	}
	if offline.Status != qualificationPassed || !allGatesPass(offline.Gates) {
		return errors.New("base offline product surface L2 is not PASSED")
	}
	postHardeningTests := filepath.Join(root, "post-hardening-provider-tests.txt")
	if err = verifyGoTestArtifact(postHardeningTests); err != nil {
		return err
	}
	offline.RecordType = "polis-product-surface-offline-l2-final@1"
	offline.TestArtifacts = append(offline.TestArtifacts, filepath.Base(postHardeningTests))
	offline.RecordedAt = time.Now().UTC()
	if err = writeJSONExclusive(filepath.Join(root, "offline-l2-final.json"), offline); err != nil {
		return err
	}
	fmt.Printf("product_surface_offline_L2=%s final_artifact=%s provider_reservations=0 provider_egress=0\n", offline.Status, filepath.Join(root, "offline-l2-final.json"))
	return nil
}

func runSurface() error {
	root := evidenceRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	surface := currentSurface()
	if err := validateExactSurface(surface); err != nil {
		return err
	}
	if err := writeJSONExclusive(filepath.Join(root, "exact-product-surface.json"), surface); err != nil {
		return err
	}
	fmt.Printf("surface_id=%s tool_count=%d manifest=%s aggregate_schema_bytes=%d aggregate_schema_digest=%s\n", surface.SurfaceID, surface.ToolCount, surface.ManifestDigest, surface.AggregateSchemaBytes, surface.AggregateSchemaDigest)
	for _, value := range surface.Tools {
		tool := value.(map[string]any)
		raw, err := json.Marshal(tool)
		if err != nil {
			return err
		}
		fmt.Printf("product_tool=%s exact=%s\n", tool["name"], raw)
	}
	return nil
}

func runOfflineResult() error {
	root := evidenceRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	surface, err := readSurface(filepath.Join(root, "exact-product-surface.json"))
	if err != nil {
		return err
	}
	current := currentSurface()
	if err = validateExactSurface(current); err != nil {
		return err
	}
	if surface.SurfaceID != current.SurfaceID || surface.ToolCount != current.ToolCount || surface.ManifestDigest != current.ManifestDigest || surface.AggregateSchemaBytes != current.AggregateSchemaBytes || surface.AggregateSchemaDigest != current.AggregateSchemaDigest {
		return errors.New("offline product surface does not match the exact current provider registry")
	}
	offlineDirectory := filepath.Join(root, "offline")
	artifactPaths := []string{
		filepath.Join(root, "exact-product-surface-test.txt"),
		filepath.Join(offlineDirectory, "provider-codex-tests.txt"),
		filepath.Join(offlineDirectory, "taskvalidation-tests.txt"),
		filepath.Join(offlineDirectory, "control-tests.txt"),
		filepath.Join(offlineDirectory, "kernel-tests.txt"),
		filepath.Join(offlineDirectory, "workbench-tests.txt"),
	}
	for _, path := range artifactPaths {
		if err = verifyGoTestArtifact(path); err != nil {
			return err
		}
	}
	resultText, err := os.ReadFile(filepath.Join(offlineDirectory, "result.txt"))
	if err != nil {
		return err
	}
	for _, marker := range []string{"provider_registry=PASSED", "dispatcher_authorization=PASSED", "taskvalidation_integration=PASSED", "candidate_fencing_integration=PASSED", "real_provider_worker_adapter_compatibility=PASSED", "provider_traffic=0", "live_business_mutations=0"} {
		if !strings.Contains(string(resultText), marker) {
			return fmt.Errorf("offline qualification result missing gate marker %q", marker)
		}
	}
	var tempRoot string
	var testDatabase string
	for _, line := range strings.Split(string(resultText), "\n") {
		if strings.HasPrefix(line, "postgres_temp_root=") {
			tempRoot = strings.TrimPrefix(line, "postgres_temp_root=")
		}
		if strings.HasPrefix(line, "postgres_database=") {
			testDatabase = strings.TrimPrefix(line, "postgres_database=")
		}
	}
	if !strings.HasPrefix(tempRoot, "/tmp/polis-r05b4-pg.") || filepath.Base(testDatabase) != testDatabase || !strings.HasPrefix(testDatabase, "polis_r0_r05b4_") {
		return errors.New("offline evidence is not bound to a dedicated R0.5B4 temporary database")
	}
	if _, statErr := os.Stat(tempRoot); !errors.Is(statErr, os.ErrNotExist) {
		return errors.New("offline PostgreSQL test cluster was not removed after qualification")
	}
	artifactPaths = append(artifactPaths, filepath.Join(offlineDirectory, "result.txt"))
	allGates := map[string]bool{
		"registration_validity": true, "deterministic_manifest": true, "schema_parseability": true,
		"dispatcher_coverage": true, "authorization_coverage": true, "task_validation_binding_integration": true,
		"candidate_state_fencing_integration": true, "product_adapter_compatibility": true,
	}
	files := make([]string, 0, len(artifactPaths))
	for _, path := range artifactPaths {
		files = append(files, filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator))))
	}
	offline := offlineL2Record{
		RecordType: "polis-product-surface-offline-l2@1", Status: "PASSED", SurfaceID: current.SurfaceID,
		ToolCount: current.ToolCount, ManifestDigest: current.ManifestDigest,
		AggregateSchemaBytes: current.AggregateSchemaBytes, AggregateSchemaDigest: current.AggregateSchemaDigest,
		Gates: allGates, TestArtifacts: files, ProviderReservations: 0, ProviderEgress: 0,
		DisposableDatabaseRemoved: true, BusinessDatabaseUsed: false,
		HistoricalB2Status: "STALE", HistoricalR03AStatus: "HISTORICAL / NOT_REUSABLE", RecordedAt: time.Now().UTC(),
	}
	if err = writeJSONExclusive(filepath.Join(root, "offline-l2-result.json"), offline); err != nil {
		return err
	}
	fmt.Printf("product_surface_offline_L2=%s gates=%d provider_reservations=0 provider_egress=0\n", offline.Status, len(offline.Gates))
	return nil
}

func runAuthorize() error {
	root := evidenceRoot()
	offlineRaw, offline, err := readOfflineL2(root)
	if err != nil {
		return err
	}
	if offline.Status != qualificationPassed {
		return errors.New("product surface offline L2 has not passed")
	}
	storedSurface, err := readSurface(filepath.Join(root, "exact-product-surface.json"))
	if err != nil {
		return err
	}
	if !sameSurfaceIdentity(storedSurface, currentSurface()) {
		return errors.New("exact product surface drifted before diagnostic authorization")
	}
	config, err := loadDiagnosticConfig(root)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(config, diagnosticSessionID)
	if err != nil {
		return err
	}
	config.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	if configured := os.Getenv("POLIS_PROVIDER_EXECUTION_ENVELOPE"); configured != "" && configured != config.ExecutionEnvelope {
		return errors.New("configured execution envelope is stale or belongs to a different launch")
	}
	config.ToolSurfaceQualification = provider.ProductToolSurfaceQualification
	if err = provider.NewCodexRuntime(config).Readiness(context.Background()); err != nil {
		return fmt.Errorf("current provider runtime is not ready for diagnostic authorization: %w", err)
	}
	executionFingerprint, err := provider.ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		return err
	}
	offlineDigest := sha256.Sum256(offlineRaw)
	authorization, err := provider.NewProductSurfaceDiagnosticAuthorization(diagnosticID, diagnosticSessionID, identity, hex.EncodeToString(offlineDigest[:]), time.Now().UTC())
	if err != nil {
		return err
	}
	manifest := executionManifest{RecordType: "polis-product-exact-surface-execution@1", ExecutionFingerprint: executionFingerprint, ExecutionIdentity: identity, ReusedHistoricalFingerprint: false, CreatedAt: time.Now().UTC()}
	if err = writeJSONExclusive(filepath.Join(root, "product-execution-manifest.json"), manifest); err != nil {
		return err
	}
	if err = provider.WriteProductSurfaceDiagnosticAuthorization(filepath.Join(root, "diagnostic-authorization.json"), authorization); err != nil {
		return err
	}
	fmt.Printf("diagnostic_authorization=%s purpose=%s execution_fingerprint=%s model=%s effort=%s reservations=1 provider_egress_limit=1 high=0 retry=0 tool_calls_expected=0 business_allowance=0\n", authorization.AuthorizationID, authorization.Purpose, executionFingerprint, identity.Model, identity.Effort)
	return nil
}

func runCanary() error {
	root := evidenceRoot()
	offlineRaw, offline, err := readOfflineL2(root)
	if err != nil {
		return err
	}
	if offline.Status != qualificationPassed {
		return errors.New("product surface offline L2 is not PASSED; provider canary is not started")
	}
	authorizationPath := filepath.Join(root, "diagnostic-authorization.json")
	bound, err := provider.ReadProductSurfaceDiagnosticAuthorization(authorizationPath)
	if err != nil {
		return err
	}
	config, err := loadDiagnosticConfig(root)
	if err != nil {
		return err
	}
	identity, err := provider.BuildProductSurfaceExecutionIdentity(config, bound.ProviderSessionID)
	if err != nil {
		return err
	}
	storedSurface, err := readSurface(filepath.Join(root, "exact-product-surface.json"))
	if err != nil {
		return err
	}
	if !sameSurfaceIdentity(storedSurface, currentSurface()) {
		return errors.New("exact product surface drifted before reservation; live canary is not started")
	}
	config.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	if configured := os.Getenv("POLIS_PROVIDER_EXECUTION_ENVELOPE"); configured != "" && configured != config.ExecutionEnvelope {
		return errors.New("configured execution envelope is stale or belongs to a different launch")
	}
	config.ToolSurfaceQualification = provider.ProductToolSurfaceQualification
	offlineDigest := sha256.Sum256(offlineRaw)
	expected, err := provider.NewProductSurfaceDiagnosticAuthorization(bound.AuthorizationID, bound.ProviderSessionID, identity, hex.EncodeToString(offlineDigest[:]), bound.IssuedAt)
	if err != nil {
		return err
	}
	if err = provider.ValidateProductSurfaceDiagnosticAuthorization(bound, expected); err != nil {
		return err
	}
	if bound.ReservationLimit != 1 || bound.ProviderAttemptLimit != 1 || bound.ProviderEgressLimit != 1 || bound.RetryLimit != 0 || bound.HighLimit != 0 || bound.BusinessAllowance != 0 {
		return errors.New("diagnostic authorization has unsafe attempt limits")
	}
	if _, err = os.Stat(filepath.Join(root, "live-canary-result.json")); err == nil {
		return errors.New("terminal diagnostic result already exists; no retry")
	}
	if _, err = os.Stat(filepath.Join(root, "provider-transport-terminal.json")); err == nil {
		return errors.New("terminal provider evidence already exists; no retry")
	}
	runtime := provider.NewCodexRuntime(config)
	if _, err = runtime.ReserveProductSurfaceDiagnostic(context.Background(), expected); err != nil {
		return err
	}
	reservationAt := time.Now().UTC()
	terminal := runOneDiagnosticTurn(runtime, config, expected, reservationAt)
	duplicateGuard, duplicateDenied := verifyDuplicateReservationDenied(config.DiagnosticAuthorizationPath, config.DiagnosticReservationPath, expected)
	terminal.SecondReservationDenied = duplicateDenied
	if err = writeJSONExclusive(filepath.Join(root, "duplicate-reservation-guard.json"), duplicateGuard); err != nil {
		return fmt.Errorf("write duplicate-reservation guard evidence: %w", err)
	}
	surfaceAfter := currentSurface()
	if err = validateExactSurface(surfaceAfter); err != nil {
		terminal.ProtocolEvidenceError = true
	}
	if err = writeJSONExclusive(filepath.Join(root, "surface-after.json"), surfaceAfter); err != nil {
		return fmt.Errorf("write post-canary surface evidence: %w", err)
	}
	before, err := readSurface(filepath.Join(root, "exact-product-surface.json"))
	if err != nil {
		return err
	}
	surfaceFresh := sameSurfaceIdentity(before, surfaceAfter)
	status := qualificationUnknown
	if !surfaceFresh {
		status = qualificationStale
	} else if diagnosticPassBoundary(terminal, expected, offline) {
		status = qualificationPassed
	}
	result := liveCanaryResult{
		RecordType: "polis-product-surface-live-l2-result@1", Qualification: status,
		ProductProviderSurfaceStatus: "NOT_QUALIFIED", EligibleForRealProviderSmoke: "NO",
		ExecutionFingerprint: expected.ExecutionFingerprint, SurfaceBefore: before, SurfaceAfter: surfaceAfter,
		SurfaceFresh: surfaceFresh, Terminal: terminal, HistoricalB2SurfaceStatus: "STALE",
		HistoricalR03ASurfaceStatus: "HISTORICAL / NOT_REUSABLE", RecordedAt: time.Now().UTC(),
	}
	if status == qualificationPassed {
		result.ProductProviderSurfaceStatus = "QUALIFIED"
		result.EligibleForRealProviderSmoke = "YES"
		evidence := provider.ProductProviderL2Evidence{
			SchemaVersion: provider.ProductProviderL2EvidenceSchema, ExecutionIdentity: expected.ExecutionIdentity,
			ExecutionFingerprint: expected.ExecutionFingerprint, DiagnosticAuthorizationID: expected.AuthorizationID,
			OfflineQualificationDigest: expected.OfflineQualificationDigest, DiagnosticReservationCount: terminal.DiagnosticReservationCount,
			ProviderEgress: terminal.ProviderEgress, RegisteredToolCount: terminal.RegisteredToolCount,
			FirstOutput: terminal.FirstOutput, ToolCalls: terminal.ToolCalls, ReconnectCount: terminal.ReconnectCount,
			TurnTerminal: terminal.TurnTerminal, CleanStop: terminal.CleanStop, SecondReservationDenied: terminal.SecondReservationDenied,
			CompanyCount: terminal.CompanyCount, MissionCount: terminal.MissionCount, TaskCount: terminal.TaskCount,
			WorkerSessionCount: terminal.WorkerSessionCount, BusinessMutationCount: terminal.BusinessMutationCount,
			SuccessorCount:               terminal.SuccessorCount,
			AggregateManifestDigestAfter: surfaceAfter.ManifestDigest, AggregateSchemaDigestAfter: surfaceAfter.AggregateSchemaDigest,
			FirstOutputLatencyMS: terminal.FirstOutputLatencyMS, ElapsedMS: terminal.ElapsedMS, Usage: terminal.TokenUsage,
		}
		result.ProductProviderL2Fingerprint, err = provider.ComputeProductProviderL2Fingerprint(evidence)
		if err != nil {
			result.Qualification = qualificationUnknown
			result.ProductProviderSurfaceStatus = "NOT_QUALIFIED"
			result.EligibleForRealProviderSmoke = "NO"
			result.ProductProviderL2Fingerprint = ""
		}
	}
	if err = writeJSONExclusive(filepath.Join(root, "provider-transport-terminal.json"), terminal); err != nil {
		return fmt.Errorf("write terminal provider evidence: %w", err)
	}
	if err = writeJSONExclusive(filepath.Join(root, "live-canary-result.json"), result); err != nil {
		return fmt.Errorf("write live canary result: %w", err)
	}
	if err = writeJSONExclusive(filepath.Join(root, "product-live-l2-qualification.json"), result); err != nil {
		return fmt.Errorf("write product live L2 result: %w", err)
	}
	fmt.Printf("product_surface_v2_live_L2=%s product_provider_surface_status=%s eligible_for_r0_5b_real_provider_smoke=%s reservation=%d provider_egress=%d first_output=%q tool_calls=%d reconnects=%d turn_terminal=%s clean_stop=%t\n", result.Qualification, result.ProductProviderSurfaceStatus, result.EligibleForRealProviderSmoke, terminal.DiagnosticReservationCount, terminal.ProviderEgress, terminal.FirstOutput, terminal.ToolCalls, terminal.ReconnectCount, terminal.TurnTerminal, terminal.CleanStop)
	if result.ProductProviderL2Fingerprint != "" {
		fmt.Printf("product_provider_L2_fingerprint=%s\n", result.ProductProviderL2Fingerprint)
	} else {
		fmt.Println("product_provider_L2_fingerprint=NOT_CREATED")
	}
	return nil
}

func runOneDiagnosticTurn(runtime *provider.CodexRuntime, config provider.CodexRuntimeConfig, authorization provider.ProductSurfaceDiagnosticAuthorization, reservationAt time.Time) providerTransportTerminal {
	terminal := providerTransportTerminal{
		RecordType: "polis-product-provider-terminal@1", AuthorizationID: authorization.AuthorizationID,
		ExecutionFingerprint: authorization.ExecutionFingerprint, DiagnosticReservationCount: 1,
		CompanyCount: 0, MissionCount: 0, TaskCount: 0, WorkerSessionCount: 0, SuccessorCount: 0, BusinessMutationCount: 0,
	}
	processStartRequested := time.Now().UTC()
	terminal.ProcessStartRequestedAt = processStartRequested
	session, err := runtime.Start(context.Background(), provider.SessionStartOptions{
		SessionID: authorization.ProviderSessionID, Model: authorization.ExecutionIdentity.Model,
		Effort: authorization.ExecutionIdentity.Effort, Profile: authorization.ExecutionIdentity.Profile,
		ToolSurface: runtime.ToolSurface(),
	})
	if err != nil {
		terminal.TurnTerminal = "process_start_failed"
		terminal.ProtocolEvidenceError = true
		terminal.CredentialSnapshotRemoved = removeDiagnosticAuthSnapshot(config, authorization.ProviderSessionID)
		terminal.CredentialSnapshotCleanupFailed = !terminal.CredentialSnapshotRemoved
		terminal.ElapsedMS = time.Since(reservationAt).Milliseconds()
		return terminal
	}
	terminal.ProcessStartedAt = time.Now().UTC()
	terminal.ProcessIdentity, terminal.ProcessID = session.Process().Identity()
	diagnosticSession, ok := session.(provider.ProductSurfaceDiagnosticSession)
	if !ok {
		terminal.TurnTerminal = "diagnostic_session_type_mismatch"
		terminal.ProtocolEvidenceError = true
	} else {
		terminal.InitializeStartedAt = time.Now().UTC()
		initializeContext, cancelInitialize := context.WithTimeout(context.Background(), config.TransportPolicy.InitializeTimeout)
		err = diagnosticSession.Initialize(initializeContext, config.TransportPolicy)
		cancelInitialize()
		terminal.InitializeFinishedAt = time.Now().UTC()
		if err != nil {
			terminal.InitializationFailed = true
			terminal.TurnTerminal = "initialize_failed"
		} else {
			terminal.ThreadStartStartedAt = time.Now().UTC()
			threadContext, cancelThread := context.WithTimeout(context.Background(), config.TransportPolicy.StartAcknowledgement)
			threadID, threadErr := diagnosticSession.StartDiagnosticThread(threadContext, provider.ThreadStartOptions{
				Model: authorization.ExecutionIdentity.Model, Effort: authorization.ExecutionIdentity.Effort,
				Tools: runtime.ToolSurface().Tools,
			})
			cancelThread()
			terminal.ThreadStartFinishedAt = time.Now().UTC()
			if threadErr != nil {
				terminal.ThreadStartFailed = true
				terminal.TurnTerminal = "thread_start_failed"
			} else {
				terminal.TurnCallStartedAt = time.Now().UTC()
				turnContext, cancelTurn := context.WithTimeout(context.Background(), config.TransportPolicy.TotalTurnDeadline)
				turn, turnErr := diagnosticSession.Turn(turnContext, threadID, provider.ProductSurfaceDiagnosticPrompt, codex.TurnOptions{Policy: &config.TransportPolicy, ToolCallLimit: 0}, nil)
				cancelTurn()
				terminal.TurnCallFinishedAt = time.Now().UTC()
				terminal.TurnTerminal = turn.State
				terminal.TokenUsage = turn.Usage
				terminal.ToolCalls = turn.ToolCalls
				terminal.ReconnectCount = turn.ReconnectAttemptCount
				terminal.TurnFailed = turnErr != nil
			}
		}
	}
	stopProof, stopErr := session.Stop(context.Background())
	terminal.StopCompletedAt = time.Now().UTC()
	terminal.CleanStop = stopErr == nil && stopProof.For(authorization.ProviderSessionID)
	terminal.StopError = stopErr != nil
	if terminal.CleanStop {
		terminal.CredentialSnapshotRemoved = removeDiagnosticAuthSnapshot(config, authorization.ProviderSessionID)
		terminal.CredentialSnapshotCleanupFailed = !terminal.CredentialSnapshotRemoved
	}
	protocolPath := filepath.Join(config.EvidenceRoot, authorization.ProviderSessionID, "protocol.jsonl")
	protocolRaw, protocolErr := os.ReadFile(protocolPath)
	if protocolErr == nil {
		terminal.Protocol, protocolErr = provider.SummarizeProductSurfaceDiagnosticProtocol(protocolRaw)
	}
	terminal.ProtocolEvidenceError = terminal.ProtocolEvidenceError || protocolErr != nil
	if protocolErr == nil {
		terminal.ProviderEgress = terminal.Protocol.TurnStartCount
		terminal.RegisteredToolCount = terminal.Protocol.RegisteredToolCount
		terminal.TurnCompletedCount = terminal.Protocol.TurnCompletedCount
		terminal.TurnCompletedAt = terminal.Protocol.TurnCompletedAt
		terminal.TurnStartAt = terminal.Protocol.TurnStartAt
		terminal.FirstOutputAt = terminal.Protocol.FirstOutputAt
		terminal.FirstOutput = terminal.Protocol.FirstOutput
		if !terminal.TurnStartAt.IsZero() && !terminal.FirstOutputAt.IsZero() {
			terminal.FirstOutputLatencyMS = terminal.FirstOutputAt.Sub(terminal.TurnStartAt).Milliseconds()
		}
		if terminal.ToolCalls != terminal.Protocol.ToolCallEventCount {
			terminal.ProtocolEvidenceError = true
		}
	}
	terminal.ElapsedMS = time.Since(reservationAt).Milliseconds()
	return terminal
}

func verifyDuplicateReservationDenied(authorizationPath, reservationPath string, authorization provider.ProductSurfaceDiagnosticAuthorization) (duplicateReservationGuard, bool) {
	guard := duplicateReservationGuard{
		RecordType: "polis-product-diagnostic-duplicate-guard@1", AuthorizationID: authorization.AuthorizationID,
		SecondProviderProcess: false, SecondProviderEgress: 0, CheckedAt: time.Now().UTC(),
	}
	rawMarker, err := os.ReadFile(reservationPath)
	if err != nil {
		guard.SecondReservationStatus = "PRIOR_MARKER_MISSING"
		return guard, false
	}
	var prior provider.ProductSurfaceDiagnosticReservation
	if err = json.Unmarshal(rawMarker, &prior); err != nil || prior.SchemaVersion != provider.ProductSurfaceDiagnosticReservationSchema || prior.AuthorizationID != authorization.AuthorizationID || prior.ReservationNumber != 1 || prior.ProviderAttemptLimit != 1 || prior.RetryLimit != 0 {
		guard.SecondReservationStatus = "PRIOR_MARKER_INVALID"
		return guard, false
	}
	rawAuthorization, err := json.Marshal(authorization)
	if err != nil {
		guard.SecondReservationStatus = "AUTHORIZATION_UNREADABLE"
		return guard, false
	}
	digest := sha256.Sum256(rawAuthorization)
	if prior.AuthorizationDigest != hex.EncodeToString(digest[:]) {
		guard.SecondReservationStatus = "PRIOR_AUTHORIZATION_MISMATCH"
		return guard, false
	}
	guard.PriorReservationNumber = prior.ReservationNumber
	_, err = provider.ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath, authorization, time.Now().UTC())
	if err != nil && strings.Contains(err.Error(), "reservation already consumed") {
		guard.SecondReservationStatus = "DENIED"
		return guard, true
	}
	guard.SecondReservationStatus = "UNEXPECTEDLY_RESERVED"
	return guard, false
}

func diagnosticPassBoundary(terminal providerTransportTerminal, authorization provider.ProductSurfaceDiagnosticAuthorization, offline offlineL2Record) bool {
	return offline.Status == qualificationPassed && terminal.DiagnosticReservationCount == 1 && terminal.ProviderEgress == 1 && terminal.RegisteredToolCount == 7 && terminal.Protocol.RegisteredToolsManifestDigest == authorization.ExecutionIdentity.ManifestDigest && terminal.FirstOutput == provider.ProductSurfaceDiagnosticCanaryOutput && terminal.ToolCalls == 0 && terminal.ReconnectCount == 0 && terminal.TurnCompletedCount == 1 && terminal.TurnTerminal == "completed" && terminal.CleanStop && terminal.CredentialSnapshotRemoved && terminal.SecondReservationDenied && !terminal.CredentialSnapshotCleanupFailed && !terminal.InitializationFailed && !terminal.ThreadStartFailed && !terminal.TurnFailed && !terminal.ProtocolEvidenceError && !terminal.StopError && terminal.CompanyCount == 0 && terminal.MissionCount == 0 && terminal.TaskCount == 0 && terminal.WorkerSessionCount == 0 && terminal.SuccessorCount == 0 && terminal.BusinessMutationCount == 0 && authorization.ExpectedToolCalls == 0
}

func loadDiagnosticConfig(root string) (provider.CodexRuntimeConfig, error) {
	if transport := os.Getenv("POLIS_PROVIDER_TRANSPORT"); transport != "" && transport != "codex" {
		return provider.CodexRuntimeConfig{}, errors.New("R0.5B4 requires the current Codex provider runtime")
	}
	if os.Getenv("POLIS_PROVIDER_ALLOWANCE_PATH") != "" || os.Getenv("POLIS_PROVIDER_TOOL_CALL_LIMIT") != "" {
		return provider.CodexRuntimeConfig{}, errors.New("diagnostic runtime must not bind a business allowance or tool-call budget")
	}
	model := environmentOr("POLIS_PROVIDER_MODEL", "gpt-5.6-luna")
	effort := environmentOr("POLIS_PROVIDER_EFFORT", "medium")
	version := environmentOr("POLIS_PROVIDER_EXPECTED_VERSION", os.Getenv("CODEX_VERSION"))
	qualification := environmentOr("POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION", provider.ProductToolSurfaceQualification)
	if qualification != provider.ProductToolSurfaceQualification {
		return provider.CodexRuntimeConfig{}, errors.New("provider product surface qualification is not polis-product-tool-surface@2")
	}
	rootPath := filepath.Join(".runtime", "windows", "r0.5b4-product-tool-surface-v2")
	evidencePath := filepath.Join(root, "provider")
	if configured := os.Getenv("POLIS_PROVIDER_ROOT"); configured != "" && filepath.Clean(configured) != filepath.Clean(rootPath) {
		return provider.CodexRuntimeConfig{}, errors.New("provider runtime root is not the dedicated R0.5B4 runtime directory")
	}
	if configured := os.Getenv("POLIS_PROVIDER_EVIDENCE"); configured != "" && filepath.Clean(configured) != filepath.Clean(evidencePath) {
		return provider.CodexRuntimeConfig{}, errors.New("provider evidence path is not the dedicated R0.5B4 evidence directory")
	}
	for _, path := range []string{filepath.Join(rootPath, diagnosticSessionID), filepath.Join(evidencePath, diagnosticSessionID)} {
		if _, err := os.Stat(path); err == nil {
			return provider.CodexRuntimeConfig{}, errors.New("R0.5B4 provider session directory already exists; no retry")
		} else if !errors.Is(err, os.ErrNotExist) {
			return provider.CodexRuntimeConfig{}, err
		}
	}
	return provider.CodexRuntimeConfig{
		Binary: os.Getenv("POLIS_PROVIDER_BINARY"), AuthFile: os.Getenv("POLIS_PROVIDER_AUTH_FILE"),
		Root: rootPath, EvidenceRoot: evidencePath, Model: model, Effort: effort, ExpectedVersion: version,
		TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: provider.ProductToolSurface(),
		ToolSurfaceQualification: qualification, MediumLimit: 0, HighLimit: 0, ToolCallLimit: 0,
		DiagnosticOnly: true, DiagnosticAuthorizationPath: filepath.Join(root, "diagnostic-authorization.json"),
		DiagnosticReservationPath: filepath.Join(root, "diagnostic-reservation.json"),
	}, nil
}

func currentSurface() exactProductSurface {
	surface := provider.ProductToolSurface()
	return exactProductSurface{
		RecordType: "polis-product-tool-surface@2", SurfaceID: provider.ProductToolSurfaceQualification,
		ToolCount: surface.ToolCount, Tools: surface.Tools, ManifestDigest: surface.ManifestDigest,
		AggregateSchemaBytes: surface.AggregateSchemaBytes, AggregateSchemaDigest: surface.AggregateSchemaDigest,
		HistoricalB2SurfaceStatus: "STALE", HistoricalR03ASurface: "HISTORICAL / NOT_REUSABLE",
	}
}

func validateExactSurface(surface exactProductSurface) error {
	if surface.SurfaceID != provider.ProductToolSurfaceQualification || surface.ToolCount != 7 || surface.ManifestDigest != "2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9" || surface.AggregateSchemaBytes != 1470 || surface.AggregateSchemaDigest != "8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f" {
		return errors.New("exact current product surface has unexplained drift; canary must not start")
	}
	return nil
}

func sameSurfaceIdentity(before, after exactProductSurface) bool {
	return before.SurfaceID == after.SurfaceID && before.ToolCount == after.ToolCount && before.ManifestDigest == after.ManifestDigest && before.AggregateSchemaBytes == after.AggregateSchemaBytes && before.AggregateSchemaDigest == after.AggregateSchemaDigest
}

func readSurface(path string) (exactProductSurface, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return exactProductSurface{}, err
	}
	var surface exactProductSurface
	if err = json.Unmarshal(raw, &surface); err != nil {
		return exactProductSurface{}, err
	}
	return surface, nil
}

func readOfflineL2(root string) ([]byte, offlineL2Record, error) {
	raw, err := os.ReadFile(filepath.Join(root, "offline-l2-final.json"))
	if err != nil {
		return nil, offlineL2Record{}, err
	}
	var offline offlineL2Record
	if err = json.Unmarshal(raw, &offline); err != nil {
		return nil, offlineL2Record{}, err
	}
	if offline.RecordType != "polis-product-surface-offline-l2-final@1" || offline.Status != qualificationPassed || !allGatesPass(offline.Gates) {
		return raw, offline, errors.New("product surface offline L2 gates are incomplete")
	}
	finalTestsPath := filepath.Join(root, "precanary-provider-tests.txt")
	if err = verifyGoTestArtifact(finalTestsPath); err != nil {
		return nil, offlineL2Record{}, err
	}
	finalTests, err := os.ReadFile(finalTestsPath)
	if err != nil {
		return nil, offlineL2Record{}, err
	}
	offlineDigest := sha256.Sum256(raw)
	testsDigest := sha256.Sum256(finalTests)
	binding := offlineQualificationDigestBinding{
		SchemaVersion:            "polis-product-offline-l2-digest-binding@1",
		OfflineL2ResultSHA256:    hex.EncodeToString(offlineDigest[:]),
		FinalProviderTestsSHA256: hex.EncodeToString(testsDigest[:]),
	}
	bindingRaw, err := json.Marshal(binding)
	if err != nil {
		return nil, offlineL2Record{}, err
	}
	return bindingRaw, offline, nil
}

func allGatesPass(gates map[string]bool) bool {
	want := []string{"registration_validity", "deterministic_manifest", "schema_parseability", "dispatcher_coverage", "authorization_coverage", "task_validation_binding_integration", "candidate_state_fencing_integration", "product_adapter_compatibility"}
	for _, gate := range want {
		if !gates[gate] {
			return false
		}
	}
	return true
}

func verifyGoTestArtifact(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	passed := false
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "FAIL" {
			return fmt.Errorf("offline test artifact contains a failed Go package result: %s", path)
		}
		if len(fields) > 1 && fields[0] == "ok" && strings.HasPrefix(fields[1], "polis/") {
			passed = true
		}
	}
	if !passed {
		return fmt.Errorf("offline test artifact is not a passing Go test result: %s", path)
	}
	return nil
}

func removeDiagnosticAuthSnapshot(config provider.CodexRuntimeConfig, providerSessionID string) bool {
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return false
	}
	target := filepath.Join(root, providerSessionID, "home", "auth.json")
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return false
	}
	err = os.Remove(target)
	return err == nil || errors.Is(err, os.ErrNotExist)
}

func writeJSONExclusive(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create evidence %s: %w", filepath.Base(path), err)
	}
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func evidenceRoot() string {
	return environmentOr("POLIS_B4_EVIDENCE_ROOT", evidenceRootDefault)
}

func environmentOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
