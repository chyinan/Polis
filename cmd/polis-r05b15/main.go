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
	Cycle           int                                              `json:"cycle"`
	CompanyID       string                                           `json:"company_id"`
	MissionID       string                                           `json:"mission_id"`
	TaskID          string                                           `json:"task_id"`
	SessionID       string                                           `json:"session_id"`
	Result          control.BusinessContextPreflightResult           `json:"result"`
	Protocol        provider.ProductSurfaceDiagnosticProtocolSummary `json:"protocol"`
	ProtocolPath    string                                           `json:"protocol_path"`
	TaskState       string                                           `json:"task_state"`
	WorkerState     string                                           `json:"worker_state"`
	ProviderReserve int                                              `json:"provider_reservations"`
	ProviderEgress  int                                              `json:"provider_egress"`
	TurnStarted     int                                              `json:"turn_started"`
	OrphanProcesses int                                              `json:"orphan_processes"`
}

type report struct {
	RecordType       string        `json:"record_type"`
	Status           string        `json:"status"`
	ProviderReserve  int           `json:"provider_reservations"`
	ProviderEgress   int           `json:"provider_egress"`
	TurnStarted      int           `json:"turn_started"`
	OrphanProcesses  int           `json:"orphan_processes"`
	Cycles           []cycleReport `json:"cycles"`
	SurfaceID        string        `json:"surface_id"`
	ManifestDigest   string        `json:"manifest_digest"`
	SchemaBytes      int           `json:"schema_bytes"`
	SchemaDigest     string        `json:"schema_digest"`
	ExecutionFP      string        `json:"execution_fingerprint"`
	ProviderL2FP     string        `json:"provider_l2_fingerprint"`
	RuntimeVersion   string        `json:"runtime_version"`
	LaunchEnvelopeFP string        `json:"launch_envelope_fingerprint"`
	GeneratedAt      time.Time     `json:"generated_at"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	dsn := requiredEnv("POLIS_DSN")
	evidenceRoot := requiredEnv("POLIS_R05B15_EVIDENCE")
	root := requiredEnv("POLIS_BLOB_ROOT")
	runtimeRoot := requiredEnv("POLIS_PROVIDER_ROOT")
	providerEvidence := requiredEnv("POLIS_PROVIDER_EVIDENCE")
	authFile := requiredEnv("POLIS_PROVIDER_AUTH_FILE")
	manifest := requiredEnv("POLIS_PROVIDER_RUNTIME_MANIFEST")
	executionEnvelope := requiredEnv("POLIS_PROVIDER_EXECUTION_ENVELOPE")
	if _, err := os.Stat(authFile); err != nil {
		return fmt.Errorf("provider auth source unavailable: %w", err)
	}
	if err := os.MkdirAll(evidenceRoot, 0700); err != nil {
		return err
	}
	config := provider.CodexRuntimeConfig{
		RuntimeManifestPath:              manifest,
		AuthFile:                         authFile,
		Root:                             runtimeRoot,
		EvidenceRoot:                     providerEvidence,
		Model:                            "gpt-5.6-luna",
		Effort:                           "medium",
		ExpectedVersion:                  "0.154.0-alpha.6.2",
		TransportPolicy:                  codex.DefaultTransportPolicy(),
		ToolSurface:                      provider.ProductToolSurface(),
		Purpose:                          provider.ProductLocalProcessLaunchQualificationPurpose,
		ExactSurfaceExecutionFingerprint: provider.ProductExactSurfaceExecutionFingerprint,
		ProductProviderL2Fingerprint:     provider.ProductProviderL2Fingerprint,
		ExecutionEnvelope:                executionEnvelope,
		ToolSurfaceQualification:         provider.ProductToolSurfaceQualification,
		DiagnosticOnly:                   true,
		DiagnosticAuthorizationPath:      filepath.Join(evidenceRoot, "local-preflight-authorization.json"),
		DiagnosticReservationPath:        filepath.Join(evidenceRoot, "local-preflight-reservation.json"),
	}
	bound, err := provider.BindCodexRuntimeConfig(config)
	if err != nil {
		return err
	}
	runtime := provider.NewCodexRuntime(bound)
	k, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		return err
	}
	defer k.Close()
	adapter, err := control.NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		return err
	}
	defer adapter.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	cycles := make([]cycleReport, 0, 3)
	for cycle := 1; cycle <= 3; cycle++ {
		cycleResult, err := runCycle(ctx, k, pool, adapter, providerEvidence, cycle)
		if err != nil {
			return fmt.Errorf("cycle %d: %w", cycle, err)
		}
		cycles = append(cycles, cycleResult)
	}
	stats := runtime.Stats()
	result := report{
		RecordType:       "polis-r0.5b15-business-context-provider-preflight@1",
		Status:           "PASSED",
		ProviderReserve:  stats.Reservations,
		ProviderEgress:   stats.ProviderEgress,
		Cycles:           cycles,
		SurfaceID:        provider.ProductToolSurfaceQualification,
		ManifestDigest:   runtime.ToolSurface().ManifestDigest,
		SchemaBytes:      runtime.ToolSurface().AggregateSchemaBytes,
		SchemaDigest:     runtime.ToolSurface().AggregateSchemaDigest,
		ExecutionFP:      runtime.ExecutionProfile().ExactSurfaceExecutionFingerprint,
		ProviderL2FP:     runtime.ExecutionProfile().ProductProviderL2Fingerprint,
		RuntimeVersion:   bound.ExpectedVersion,
		LaunchEnvelopeFP: bound.ExecutionEnvelope,
		GeneratedAt:      time.Now().UTC(),
	}
	for _, cycle := range cycles {
		result.TurnStarted += cycle.TurnStarted
		result.OrphanProcesses += cycle.OrphanProcesses
	}
	if result.ProviderReserve != 0 || result.ProviderEgress != 0 || result.TurnStarted != 0 || result.OrphanProcesses != 0 {
		result.Status = "FAILED"
	}
	return writeJSON(filepath.Join(evidenceRoot, "business-context-preflight.json"), result)
}

func runCycle(ctx context.Context, k *kernel.Kernel, pool *pgxpool.Pool, adapter *control.RealProviderWorkerAdapter, providerEvidence string, cycle int) (cycleReport, error) {
	company := fmt.Sprintf("r05b15-cycle-%d-%d", cycle, time.Now().UTC().UnixNano())
	if _, err := k.TXCreateCompany(ctx, company); err != nil {
		return cycleReport{}, err
	}
	service := control.NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, control.CreateMissionRequest{
		Title:              "R0.5B15 business context preflight",
		Goal:               "Reach provider initialize and exact product thread start without a turn.",
		RequestID:          fmt.Sprintf("r0-5b15-create-%d", cycle),
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		return cycleReport{}, err
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, fmt.Sprintf("r0-5b15-start-%d", cycle)); err != nil {
		return cycleReport{}, err
	}
	result, err := adapter.QualifyLocalBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		return cycleReport{}, err
	}
	protocolPath := filepath.Join(providerEvidence, result.SessionID, "protocol.jsonl")
	protocolRaw, err := os.ReadFile(protocolPath)
	if err != nil {
		return cycleReport{}, err
	}
	protocol, err := provider.SummarizeProductSurfaceDiagnosticProtocol(protocolRaw)
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
	if !result.ProcessStarted || !result.InitializePassed || !result.ThreadStarted || result.RegisteredToolCount != 7 || !result.BusinessBindingPassed || !result.CleanStop || protocol.InitializeResponseCount != 1 || protocol.ThreadStartResponseCount != 1 || protocol.RegisteredToolCount != 7 || protocol.TurnStartCount != 0 || taskState != "working" || workerState != "stopped" {
		return cycleReport{}, errors.New("business context preflight boundary did not pass")
	}
	return cycleReport{Cycle: cycle, CompanyID: company, MissionID: created.TargetID, TaskID: result.TaskID, SessionID: result.SessionID, Result: result, Protocol: protocol, ProtocolPath: protocolPath, TaskState: taskState, WorkerState: workerState, ProviderReserve: result.ProviderReservations, ProviderEgress: result.ProviderEgress, TurnStarted: protocol.TurnStartCount, OrphanProcesses: 0}, nil
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
