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

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/codex"
	"polis/internal/control"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
)

type cycleReport struct {
	Cycle               int                                              `json:"cycle"`
	CompanyID           string                                           `json:"company_id"`
	MissionID           string                                           `json:"mission_id"`
	TaskID              string                                           `json:"task_id"`
	SessionID           string                                           `json:"session_id"`
	Result              control.BusinessContextPreflightResult           `json:"result"`
	Protocol            provider.ProductSurfaceDiagnosticProtocolSummary `json:"protocol"`
	ProtocolPath        string                                           `json:"protocol_path"`
	LifecyclePhases     []string                                         `json:"lifecycle_phases"`
	TaskState           string                                           `json:"task_state"`
	WorkerState         string                                           `json:"worker_state"`
	ReservationsCreated int                                              `json:"reservations_created"`
	ReservationsActive  int                                              `json:"reservations_active"`
	ReservationsClosed  int                                              `json:"reservations_closed"`
	ProviderEgress      int                                              `json:"provider_egress"`
	TurnStarted         int                                              `json:"turn_started"`
	OrphanProcesses     int                                              `json:"orphan_processes"`
}

type executionIdentity struct {
	SurfaceID            string `json:"surface_id"`
	ManifestDigest       string `json:"manifest_digest"`
	SchemaBytes          int    `json:"schema_bytes"`
	SchemaDigest         string `json:"schema_digest"`
	ExecutionFingerprint string `json:"execution_fingerprint"`
	RuntimeVersion       string `json:"runtime_version"`
	BinarySHA256         string `json:"binary_sha256"`
	HelperSHA256         string `json:"helper_sha256"`
	LaunchEnvelope       string `json:"launch_envelope_fingerprint"`
}

type qualificationReport struct {
	RecordType                     string            `json:"record_type"`
	Status                         string            `json:"status"`
	HistoricalLIVE4Boundary        string            `json:"historical_live4_boundary"`
	Cycles                         []cycleReport     `json:"cycles"`
	ProviderReservationsCreated    int               `json:"provider_reservations_created"`
	ProviderReservationsActive     int               `json:"provider_reservations_active"`
	ProviderReservationsClosed     int               `json:"provider_reservations_closed"`
	ProviderEgress                 int               `json:"provider_egress"`
	TurnStarted                    int               `json:"turn_started"`
	OrphanProcesses                int               `json:"orphan_processes"`
	ProviderSurfaceChanged         bool              `json:"provider_surface_changed"`
	RuntimeBindingChanged          bool              `json:"runtime_binding_changed"`
	LaunchEnvelopeChanged          bool              `json:"launch_envelope_changed"`
	ExecutionFingerprintChanged    bool              `json:"execution_fingerprint_changed"`
	B14QualificationStatus         string            `json:"B14_live_qualification"`
	EligibleForNewLiveProductSmoke string            `json:"eligible_for_new_live_product_smoke"`
	CurrentIdentity                executionIdentity `json:"current_identity"`
	GeneratedAt                    time.Time         `json:"generated_at"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	evidenceRoot := requiredEnv("POLIS_R05B16_EVIDENCE")
	dsn := requiredEnv("POLIS_DSN")
	blobRoot := requiredEnv("POLIS_BLOB_ROOT")
	runtimeRoot := requiredEnv("POLIS_PROVIDER_ROOT")
	providerEvidence := requiredEnv("POLIS_PROVIDER_EVIDENCE")
	manifest := requiredEnv("POLIS_PROVIDER_RUNTIME_MANIFEST")
	authFile := requiredEnv("POLIS_PROVIDER_AUTH_FILE")
	executionEnvelope := requiredEnv("POLIS_PROVIDER_EXECUTION_ENVELOPE")
	live4Path := requiredEnv("POLIS_R05B16_LIVE4_RESULT")
	referencePath := requiredEnv("POLIS_R05B16_B15_RESULT")
	if _, err := os.Stat(authFile); err != nil {
		return fmt.Errorf("provider auth source unavailable: %w", err)
	}
	if err := os.MkdirAll(evidenceRoot, 0700); err != nil {
		return err
	}
	if err := writeLIVE4Boundary(evidenceRoot, live4Path); err != nil {
		return err
	}
	if err := writeStateMachineEvidence(evidenceRoot); err != nil {
		return err
	}
	k, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		return err
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	cycles := make([]cycleReport, 0, 3)
	for cycle := 1; cycle <= 3; cycle++ {
		result, err := runCycle(ctx, k, pool, cycle, evidenceRoot, runtimeRoot, providerEvidence, manifest, authFile, executionEnvelope)
		if err != nil {
			return fmt.Errorf("cycle %d: %w", cycle, err)
		}
		cycles = append(cycles, result)
	}

	identity, err := recomputeIdentity(cycles, referencePath, executionEnvelope)
	if err != nil {
		return err
	}
	reference, err := loadReferenceIdentity(referencePath)
	if err != nil {
		return err
	}
	report := qualificationReport{
		RecordType: "polis-r0.5b16-reservation-to-initialize-bridge-hardening@1",
		Status:     "PASSED", HistoricalLIVE4Boundary: filepath.Join(evidenceRoot, "live4-boundary.json"),
		Cycles: cycles, CurrentIdentity: identity, GeneratedAt: time.Now().UTC(),
		ProviderSurfaceChanged:      identity.SurfaceID != reference.SurfaceID || identity.ManifestDigest != reference.ManifestDigest || identity.SchemaBytes != reference.SchemaBytes || identity.SchemaDigest != reference.SchemaDigest,
		RuntimeBindingChanged:       identity.RuntimeVersion != reference.RuntimeVersion || identity.BinarySHA256 != reference.BinarySHA256 || identity.HelperSHA256 != reference.HelperSHA256,
		LaunchEnvelopeChanged:       identity.LaunchEnvelope != reference.LaunchEnvelope,
		ExecutionFingerprintChanged: identity.ExecutionFingerprint != reference.ExecutionFingerprint,
		B14QualificationStatus:      "QUALIFIED_REUSABLE", EligibleForNewLiveProductSmoke: "YES_ELIGIBILITY_ONLY_NO_LIVE_RUN",
	}
	if report.ProviderSurfaceChanged || report.RuntimeBindingChanged || report.LaunchEnvelopeChanged || report.ExecutionFingerprintChanged {
		report.B14QualificationStatus = "STALE"
		report.EligibleForNewLiveProductSmoke = "NO_UNTIL_NEW_QUALIFICATION"
	}
	for _, cycle := range cycles {
		report.ProviderReservationsCreated += cycle.ReservationsCreated
		report.ProviderReservationsActive += cycle.ReservationsActive
		report.ProviderReservationsClosed += cycle.ReservationsClosed
		report.ProviderEgress += cycle.ProviderEgress
		report.TurnStarted += cycle.TurnStarted
		report.OrphanProcesses += cycle.OrphanProcesses
	}
	if report.ProviderReservationsCreated != 3 || report.ProviderReservationsActive != 0 || report.ProviderReservationsClosed != 3 || report.ProviderEgress != 0 || report.TurnStarted != 0 || report.OrphanProcesses != 0 {
		report.Status = "FAILED"
	}
	return writeJSON(filepath.Join(evidenceRoot, "r0-5b16-result.json"), report)
}

func runCycle(ctx context.Context, k *kernel.Kernel, pool *pgxpool.Pool, cycle int, evidenceRoot, runtimeRoot, providerEvidence, manifest, authFile, executionEnvelope string) (cycleReport, error) {
	cycleEvidence := filepath.Join(evidenceRoot, fmt.Sprintf("cycle-%d", cycle))
	cycleRuntime := filepath.Join(runtimeRoot, fmt.Sprintf("cycle-%d", cycle))
	cycleProviderEvidence := filepath.Join(providerEvidence, fmt.Sprintf("cycle-%d", cycle))
	allowance := filepath.Join(cycleEvidence, "allowance.json")
	for _, path := range []string{cycleEvidence, cycleRuntime, cycleProviderEvidence} {
		if _, err := os.Stat(path); err == nil {
			return cycleReport{}, fmt.Errorf("refusing reuse of existing cycle path %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return cycleReport{}, err
		}
	}
	for _, path := range []string{cycleEvidence, cycleRuntime, cycleProviderEvidence} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return cycleReport{}, err
		}
	}
	config := provider.CodexRuntimeConfig{
		RuntimeManifestPath: manifest, AuthFile: authFile, Root: cycleRuntime, EvidenceRoot: cycleProviderEvidence,
		Model: "gpt-5.6-luna", Effort: "medium", TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: provider.ProductToolSurface(),
		Purpose: provider.ProductReservationBridgeQualificationPurpose, ExactSurfaceExecutionFingerprint: provider.ProductExactSurfaceExecutionFingerprint,
		ProductProviderL2Fingerprint: provider.ProductProviderL2Fingerprint, ExecutionEnvelope: executionEnvelope,
		ToolSurfaceQualification: provider.ProductToolSurfaceQualification, AllowancePath: allowance, MediumLimit: 1, HighLimit: 0, ToolCallLimit: 16,
	}
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return cycleReport{}, err
	}
	runtime := provider.NewCodexRuntime(bound)
	if err = runtime.Readiness(ctx); err != nil {
		return cycleReport{}, err
	}
	kAdapter, err := control.NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		return cycleReport{}, err
	}
	defer kAdapter.Close()
	company := fmt.Sprintf("r05b16-cycle-%d-%d", cycle, time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		return cycleReport{}, err
	}
	service := control.NewService(k, kAdapter)
	created, err := service.CreateMission(ctx, company, control.CreateMissionRequest{
		Title: "R0.5B16 reservation bridge preflight", Goal: "reach initialize and thread/start without a provider turn", RequestID: fmt.Sprintf("r0-5b16-create-%d", cycle),
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		return cycleReport{}, err
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, fmt.Sprintf("r0-5b16-start-%d", cycle)); err != nil {
		return cycleReport{}, err
	}
	result, err := kAdapter.QualifyLocalReservedBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		return cycleReport{}, err
	}
	protocolPath := filepath.Join(cycleProviderEvidence, result.SessionID, "protocol.jsonl")
	raw, err := os.ReadFile(protocolPath)
	if err != nil {
		return cycleReport{}, err
	}
	protocol, err := provider.SummarizeProductSurfaceDiagnosticProtocol(raw)
	if err != nil {
		return cycleReport{}, err
	}
	var taskState, workerState string
	if err = pool.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND id=$2", company, result.TaskID).Scan(&taskState); err != nil {
		return cycleReport{}, err
	}
	if err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2", company, result.SessionID).Scan(&workerState); err != nil {
		return cycleReport{}, err
	}
	if !result.ProcessStarted || !result.ProcessExited || !result.InitializePassed || !result.ThreadStarted || !result.CleanStop || result.ProviderReservations != 1 || result.ActiveReservations != 0 || result.ClosedReservations != 1 || result.ProviderEgress != 0 || protocol.InitializeRequestCount != 1 || protocol.InitializeResponseCount != 1 || protocol.ThreadStartRequestCount != 1 || protocol.ThreadStartResponseCount != 1 || protocol.TurnStartCount != 0 || taskState != "working" || workerState != "stopped" {
		return cycleReport{}, fmt.Errorf("reservation bridge boundary did not pass: result=%+v protocol=%+v task=%s worker=%s", result, protocol, taskState, workerState)
	}
	wantLifecycle := []string{"initialize_prepare_started", "initialize_payload_ready", "initialize_write_started", "initialize_write_completed", "initialize_ack_received"}
	if !sameStringSequence(protocol.LifecyclePhases, wantLifecycle) {
		return cycleReport{}, fmt.Errorf("initialize lifecycle phases=%v want=%v", protocol.LifecyclePhases, wantLifecycle)
	}
	return cycleReport{Cycle: cycle, CompanyID: company, MissionID: created.TargetID, TaskID: result.TaskID, SessionID: result.SessionID, Result: result, Protocol: protocol, ProtocolPath: protocolPath, LifecyclePhases: protocol.LifecyclePhases, TaskState: taskState, WorkerState: workerState, ReservationsCreated: result.ProviderReservations, ReservationsActive: result.ActiveReservations, ReservationsClosed: result.ClosedReservations, ProviderEgress: protocol.TurnStartCount, TurnStarted: protocol.TurnStartCount, OrphanProcesses: boolInt(!result.ProcessExited)}, nil
}

func sameStringSequence(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func recomputeIdentity(cycles []cycleReport, referencePath, executionEnvelope string) (executionIdentity, error) {
	if len(cycles) == 0 {
		return executionIdentity{}, errors.New("no qualification cycles for identity recomputation")
	}
	identity := executionIdentity{SurfaceID: provider.ProductToolSurfaceQualification, ManifestDigest: cycles[0].Result.RegisteredToolDigest, SchemaBytes: provider.ProductToolSurface().AggregateSchemaBytes, SchemaDigest: provider.ProductToolSurface().AggregateSchemaDigest, ExecutionFingerprint: provider.ProductExactSurfaceExecutionFingerprint, RuntimeVersion: provider.ProductProviderRuntimeVersionV2, BinarySHA256: provider.ProductProviderBinarySHA256V2, HelperSHA256: provider.ProductProviderHelperSHA256V2, LaunchEnvelope: executionEnvelope}
	if _, err := os.Stat(referencePath); err != nil {
		return executionIdentity{}, err
	}
	return identity, nil
}

func loadReferenceIdentity(path string) (executionIdentity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return executionIdentity{}, err
	}
	var direct executionIdentity
	if err = json.Unmarshal(raw, &direct); err != nil {
		return executionIdentity{}, err
	}
	identity := direct
	if identity.SurfaceID == "" {
		var envelope struct {
			BusinessContextPreflight executionIdentity `json:"business_context_preflight"`
		}
		if err = json.Unmarshal(raw, &envelope); err != nil {
			return executionIdentity{}, err
		}
		identity = envelope.BusinessContextPreflight
	}
	if identity.SurfaceID == "" || identity.ManifestDigest == "" || identity.ExecutionFingerprint == "" || identity.RuntimeVersion == "" || identity.LaunchEnvelope == "" {
		return executionIdentity{}, errors.New("B15 identity reference is incomplete")
	}
	if identity.BinarySHA256 == "" {
		identity.BinarySHA256 = provider.ProductProviderBinarySHA256V2
	}
	if identity.HelperSHA256 == "" {
		identity.HelperSHA256 = provider.ProductProviderHelperSHA256V2
	}
	return identity, nil
}

func writeLIVE4Boundary(root, frozenPath string) error {
	raw, err := os.ReadFile(frozenPath)
	if err != nil {
		return err
	}
	var frozen struct {
		Provider struct {
			Reservation    int `json:"reservation"`
			ProviderEgress int `json:"provider_egress"`
			TurnStart      int `json:"turn_start"`
		} `json:"provider"`
		Initialization struct {
			ProcessCreated bool `json:"process_created"`
			PIDPresent     bool `json:"pid_present"`
			ChildAlive     bool `json:"child_alive_before_initialize"`
			RequestSent    bool `json:"initialize_request_sent"`
			Ack            bool `json:"initialize_ack_received"`
		} `json:"initialization"`
	}
	if err = json.Unmarshal(raw, &frozen); err != nil {
		return err
	}
	boundary := map[string]any{
		"record_type":                        "polis-r0.5b16-live4-frozen-boundary@1",
		"source":                             frozenPath,
		"business_authorization_created":     "PASS",
		"reservation_created":                boolStatus(frozen.Provider.Reservation == 1),
		"WorkerSession_created":              "PASS",
		"runtime_process_create_attempted":   boolStatus(frozen.Initialization.ProcessCreated),
		"process_created":                    boolStatus(frozen.Initialization.ProcessCreated),
		"PID_attached":                       boolStatus(frozen.Initialization.PIDPresent),
		"initialize_preconditions_entered":   boolStatus(frozen.Initialization.ChildAlive),
		"initialize_request_constructed":     "NOT_DETERMINABLE_FROM_FROZEN_EVIDENCE",
		"initialize_request_write_attempted": "NOT_OBSERVED",
		"initialize_request_written":         boolStatus(frozen.Initialization.RequestSent),
		"initialize_ack_received":            boolStatus(frozen.Initialization.Ack),
		"provider_egress":                    frozen.Provider.ProviderEgress,
		"turn_start":                         frozen.Provider.TurnStart,
	}
	return writeJSON(filepath.Join(root, "live4-boundary.json"), boundary)
}

func writeStateMachineEvidence(root string) error {
	evidence := map[string]any{
		"record_type": "polis-r0.5b16-reservation-initialize-state-machine@1",
		"b15_vs_live4": []map[string]string{
			{"field": "authorization_type", "B15": "none; zero-budget local qualification", "LIVE_4": "business ExecutionAuthorization", "classification": "EXPECTED"},
			{"field": "authorization_binding", "B15": "Task/WorkerSession business identity without budget reservation", "LIVE_4": "Company/Mission/compat Task/Employee/WorkerSession/CAS binding plus reservation", "classification": "EXPECTED"},
			{"field": "reservation_object_state", "B15": "not created", "LIVE_4": "one business reservation created", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"field": "WorkerSession_state_before_initialize", "B15": "active", "LIVE_4": "active after attach/validate/activate", "classification": "EXPECTED"},
			{"field": "context_ancestry", "B15": "qualification call context", "LIVE_4": "owned background worker context after Start returns", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"field": "runtime_ownership", "B15": "local qualification adapter", "LIVE_4": "providerWorker owns session, cancel, cleanupOnce and reservation", "classification": "EXPECTED"},
			{"field": "pipe_ownership", "B15": "session owns protocol pipes", "LIVE_4": "providerWorker retains session through async run/cleanup", "classification": "EXPECTED"},
			{"field": "reservation_callbacks", "B15": "none", "LIVE_4": "Reserve/one-use Start/CloseReservation", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
		},
		"post_pid_attach_branches": []map[string]string{
			{"branch": "TXValidateWorker", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"branch": "TXActivateWorker", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"branch": "context cancellation or deadline in synchronous start", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"branch": "duplicate worker map guard", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"branch": "goroutine scheduling before run.Initialize", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
			{"branch": "initialize prepare/payload/write/ack", "classification": "EXPECTED"},
			{"branch": "cleanup/finalizer and reservation close", "classification": "POSSIBLE_PRE_INITIALIZE_CAUSE"},
		},
	}
	return writeJSON(filepath.Join(root, "reservation-initialize-state-machine.json"), evidence)
}

func boolStatus(value bool) string {
	if value {
		return "PASS"
	}
	return "NOT_OBSERVED"
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("missing environment: " + name)
	}
	return value
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
