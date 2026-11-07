// pattern: Imperative Shell
// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
)

func TestRealProviderWorkerAdapterUsesOfflineRuntimeAndFinalizesWorkerLifecycle(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 5 * time.Millisecond})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-provider-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	productSurface := provider.ProductToolSurface()
	adapterSurface := adapter.ToolSurface()
	if adapterSurface.ToolCount != productSurface.ToolCount || adapterSurface.ManifestDigest != productSurface.ManifestDigest || adapterSurface.AggregateSchemaBytes != productSurface.AggregateSchemaBytes || adapterSurface.AggregateSchemaDigest != productSurface.AggregateSchemaDigest {
		t.Fatalf("production adapter surface differs from generic product registry: adapter=%+v product=%+v", adapterSurface, productSurface)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "offline provider bridge", Goal: "persist one deterministic product artifact", RequestID: "bridge-create", AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}}})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "bridge-start"})
	if err != nil {
		t.Fatal(err)
	}
	if started.ResultingState != "active" || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("unexpected start/runtime state: %+v/%+v", started, runtime.Stats())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, company, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("unexpected product bridge state: session=%s task=%s artifacts=%d terminal_events=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	if runtime.Stats().Reservations != 0 || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline bridge used provider: %+v", runtime.Stats())
	}
	var bindingDigest, runnerKind, runnerRevision string
	if err = pool.QueryRow(ctx, `SELECT v.configuration_digest,v.runner_kind,v.runner_revision
FROM task_validation_bindings v JOIN tasks t ON t.company_id=v.company_id AND t.id=v.task_id
WHERE v.company_id=$1 AND t.mission_id=$2`, company, created.TargetID).Scan(&bindingDigest, &runnerKind, &runnerRevision); err != nil {
		t.Fatal(err)
	}
	if len(bindingDigest) != 64 || runnerKind != taskvalidation.TextContainsAllRunnerKind || runnerRevision != taskvalidation.TextContainsAllRunnerRevision {
		t.Fatalf("unexpected task validation binding %s/%s/%s", bindingDigest, runnerKind, runnerRevision)
	}
	var checks, passedChecks, qualifications int
	if err = pool.QueryRow(ctx, "SELECT count(*),count(*) FILTER (WHERE passed) FROM worker_checks WHERE company_id=$1 AND phase='product'", company).Scan(&checks, &passedChecks); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1", company).Scan(&qualifications); err != nil {
		t.Fatal(err)
	}
	if checks != 1 || passedChecks != 1 || qualifications != 1 {
		t.Fatalf("product validation evidence checks=%d passed=%d artifact_qualifications=%d", checks, passedChecks, qualifications)
	}
	var checkReportRaw []byte
	if err = pool.QueryRow(ctx, "SELECT report FROM worker_checks WHERE company_id=$1 AND phase='product'", company).Scan(&checkReportRaw); err != nil {
		t.Fatal(err)
	}
	var checkReport taskvalidation.Result
	if json.Unmarshal(checkReportRaw, &checkReport) != nil || checkReport.Status != taskvalidation.StatusPass || checkReport.ConfigurationDigest != bindingDigest || checkReport.WorkspaceRevision != 2 {
		t.Fatalf("product check report is not bound to current Task: %+v", checkReport)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "bridge-start-duplicate"}); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate start error=%v, want %s", err, core.Conflict)
	}
	service.Close()
}

func TestRealProviderWorkerAdapterMissingBindingCannotQualifyOrSubmit(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B3_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated offline product PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b3-no-validation-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "exploratory task", Goal: "explore without a public acceptance contract", RequestID: "no-validation-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "no-validation-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var sessionState, taskState string
		var artifacts, checkpoints, checks int
		err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&sessionState)
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND kind='compat' ORDER BY id DESC LIMIT 1", company).Scan(&taskState)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM artifacts WHERE company_id=$1", company).Scan(&artifacts)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_checkpoints c JOIN worker_sessions s ON s.company_id=c.company_id AND s.id=c.session_id WHERE c.company_id=$1", company).Scan(&checkpoints)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_checks WHERE company_id=$1 AND phase='product' AND NOT passed AND report->>'status'='VALIDATION_NOT_CONFIGURED'", company).Scan(&checks)
		}
		if err == nil && sessionState == "stopped" && taskState == "working" && artifacts == 0 && checkpoints == 0 && checks == 1 {
			if runtime.Stats().ProviderEgress != 0 || runtime.Stats().Reservations != 0 {
				t.Fatalf("missing-binding offline run used provider: %+v", runtime.Stats())
			}
			service.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("missing-binding task did not fail closed: err=%v", err)
}

func TestRealProviderWorkerAdapterCancelsOfflineTurnWithoutOrphan(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-cancel-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "cancel bridge", Goal: "cancel during an offline turn", RequestID: "cancel-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "cancel-start"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CancelMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "cancel-command"}); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var live int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND state!='stopped'", company).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("orphan live sessions = %d", live)
	}
	if runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline cancellation used provider: %+v", runtime.Stats())
	}
}

func TestRealProviderWorkerAdapterClosesReservationAfterPreTurnStop(t *testing.T) {
	dsn := os.Getenv("POLIS_R0_5B16_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B16 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b16-reservation-stop-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := &reservationTrackingRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "reservation lifecycle stop",
		Goal:               "close the reservation when a no-turn worker is stopped",
		RequestID:          "r0-5b16-reservation-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b16-reservation-start"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CancelMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b16-reservation-stop"}); err != nil {
		t.Fatal(err)
	}
	if got := runtime.closeReasons(); len(got) != 1 || got[0] == "" {
		t.Fatalf("reservation close calls=%v, want one classified close", got)
	}
}

func TestRealProviderWorkerAdapterReservationBearingPreflightStopsBeforeTurn(t *testing.T) {
	dsn := os.Getenv("POLIS_R0_5B16_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B16 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b16-reservation-preflight-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := &reservationTrackingRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "reservation bridge preflight",
		Goal:               "reach initialize and thread start without a provider turn",
		RequestID:          "r0-5b16-preflight-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, "r0-5b16-preflight-start"); err != nil {
		t.Fatal(err)
	}
	qualifier, ok := any(adapter).(interface {
		QualifyLocalReservedBusinessContext(context.Context, string, string) (BusinessContextPreflightResult, error)
	})
	if !ok {
		t.Fatal("real provider adapter has no reservation-bearing local qualification boundary")
	}
	result, err := qualifier.QualifyLocalReservedBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		t.Fatalf("reservation-bearing preflight result=%+v error=%v", result, err)
	}
	if !result.ProcessStarted || !result.InitializePassed || !result.ThreadStarted || !result.CleanStop {
		t.Fatalf("reservation-bearing preflight did not reach clean thread boundary: %+v", result)
	}
	if got := runtime.closeReasons(); len(got) != 1 || got[0] == "" {
		t.Fatalf("reservation close calls=%v, want one classified close", got)
	}
	if got := runtime.turnCalls(); got != 0 {
		t.Fatalf("reservation-bearing preflight started %d turns", got)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, "SELECT payload->>'status' FROM events WHERE company_id=$1 AND kind='provider.runtime.lifecycle' ORDER BY company_seq", company)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var phases []string
	for rows.Next() {
		var phase string
		if err = rows.Scan(&phase); err != nil {
			t.Fatal(err)
		}
		phases = append(phases, phase)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantPhases := []string{"adapter_start_entered", "reservation_bound", "process_created", "process_attached"}
	if len(phases) < len(wantPhases) {
		t.Fatalf("lifecycle phases=%v, want at least %v", phases, wantPhases)
	}
	for index, phase := range wantPhases {
		if phases[index] != phase {
			t.Fatalf("lifecycle phases=%v, want prefix %v", phases, wantPhases)
		}
	}
}

type reservationTrackingRuntime struct {
	*provider.FakeRuntime
	mu        sync.Mutex
	closed    []string
	turnCount int
}

func (r *reservationTrackingRuntime) ExecutionProfile() provider.ExecutionProfile {
	profile := r.FakeRuntime.ExecutionProfile()
	profile.Purpose = provider.ProductReservationBridgeQualificationPurpose
	return profile
}

func (r *reservationTrackingRuntime) Reserve(ctx context.Context, _ provider.ExecutionAuthorization) (provider.Reservation, error) {
	if err := ctx.Err(); err != nil {
		return provider.Reservation{}, err
	}
	return provider.Reservation{ID: "offline-reservation-bridge"}, nil
}

func (r *reservationTrackingRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.FakeRuntime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return reservationTrackingSession{Session: session, runtime: r}, nil
}

func (r *reservationTrackingRuntime) CloseReservation(_ context.Context, _ provider.Reservation, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = append(r.closed, reason)
	return nil
}

func (r *reservationTrackingRuntime) closeReasons() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.closed...)
}

func (r *reservationTrackingRuntime) turnCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.turnCount
}

type reservationTrackingSession struct {
	provider.Session
	runtime *reservationTrackingRuntime
}

func (s reservationTrackingSession) Turn(ctx context.Context, thread, prompt string, options codex.TurnOptions, handler provider.ToolHandler) (provider.TurnResult, error) {
	s.runtime.mu.Lock()
	s.runtime.turnCount++
	s.runtime.mu.Unlock()
	return s.Session.Turn(ctx, thread, prompt, options, handler)
}

func TestRealProviderWorkerAdapterProcessStartFailureFinalizesSession(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-start-failure-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, startFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "process failure bridge", Goal: "fail before provider process start", RequestID: "failure-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "failure-start"}); err == nil {
		t.Fatal("provider process start failure was accepted")
	}
	var live, stopped int
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE state!='stopped'), count(*) FILTER (WHERE state='stopped') FROM worker_sessions WHERE company_id=$1", company).Scan(&live, &stopped); err != nil {
		t.Fatal(err)
	}
	if live != 0 || stopped != 1 {
		t.Fatalf("failed provider start left invalid session state: live=%d stopped=%d", live, stopped)
	}
}

func TestRealProviderWorkerAdapterDisconnectIsInconclusiveAndStopped(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-disconnect-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, disconnectRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "disconnect bridge", Goal: "classify transport disconnect", RequestID: "disconnect-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "disconnect-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	waitForTerminalEvent(t, ctx, pool, company, "provider.turn.inconclusive")
	var state string
	if err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "stopped" {
		t.Fatalf("disconnect session state=%s, want stopped", state)
	}
}

func TestRealProviderWorkerAdapterInitializationFailureUsesInitializationEvent(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-initialize-failure-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, initializationFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "initialize failure classification",
		Goal:               "exercise the business provider initialization boundary",
		RequestID:          "r0-5b15-initialize-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b15-initialize-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var expected, turnFailure int
		if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='provider.runtime.initialization_failed'), count(*) FILTER (WHERE kind='provider.turn.inconclusive') FROM events WHERE company_id=$1", company).Scan(&expected, &turnFailure); err != nil {
			t.Fatal(err)
		}
		if turnFailure > 0 {
			t.Fatalf("initialize failure was reported as a provider turn failure: expected=%d turn_failure=%d", expected, turnFailure)
		}
		if expected == 1 {
			var failureCategory string
			if err = pool.QueryRow(ctx, "SELECT COALESCE(data->>'failure_category','') FROM worker_observations WHERE company_id=$1 AND reason='provider_initialization' ORDER BY id DESC LIMIT 1", company).Scan(&failureCategory); err != nil {
				t.Fatal(err)
			}
			if failureCategory == "" {
				t.Fatal("initialize failure observation has no safe failure_category")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for provider.runtime.initialization_failed")
}

func TestRealProviderWorkerAdapterBusinessContextLocalPreflightAvoidsReservation(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-business-preflight-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := newLocalQualificationFakeRuntime()
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "business context local preflight",
		Goal:               "reach initialize and thread start without a provider turn",
		RequestID:          "r0-5b15-preflight-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, "r0-5b15-preflight-start"); err != nil {
		t.Fatal(err)
	}
	result, err := adapter.QualifyLocalBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		t.Fatalf("business preflight result=%+v error type=%T error=%q detail=%+v", result, err, err, err)
	}
	if !result.ProcessStarted || !result.InitializePassed || !result.ThreadStarted || result.RegisteredToolCount != 7 || !result.BusinessBindingPassed || !result.CleanStop {
		t.Fatalf("local business preflight did not pass its boundary: %+v", result)
	}
	if stats := runtime.Stats(); stats.Reservations != 0 || stats.ProviderEgress != 0 {
		t.Fatalf("local business preflight used provider accounting: %+v", stats)
	}
}

func TestRealProviderWorkerAdapterThreadStartFailureUsesThreadStartEvent(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-thread-failure-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, threadStartFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "thread start failure classification", Goal: "classify thread start before a turn", RequestID: "r0-5b15-thread-create", AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b15-thread-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var expected, turnFailure int
		if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='provider.runtime.thread_start_failed'), count(*) FILTER (WHERE kind='provider.turn.inconclusive') FROM events WHERE company_id=$1", company).Scan(&expected, &turnFailure); err != nil {
			t.Fatal(err)
		}
		if turnFailure > 0 {
			t.Fatalf("thread start failure was reported as a provider turn failure: expected=%d turn_failure=%d", expected, turnFailure)
		}
		if expected == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for provider.runtime.thread_start_failed")
}

type startFailureRuntime struct{ provider.Runtime }

func (startFailureRuntime) Mode() string { return "fake" }
func (startFailureRuntime) Start(context.Context, provider.SessionStartOptions) (provider.Session, error) {
	return nil, errors.New("offline provider process start failed")
}

type disconnectRuntime struct{ provider.Runtime }

func (disconnectRuntime) Mode() string { return "fake" }
func (r disconnectRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return disconnectSession{Session: session}, nil
}

type disconnectSession struct{ provider.Session }

func (disconnectSession) Turn(context.Context, string, string, codex.TurnOptions, provider.ToolHandler) (provider.TurnResult, error) {
	return provider.TurnResult{State: "failed", Outcome: "transport_disconnect", ProviderEgress: 0}, errors.New("offline transport disconnected before terminal")
}

type initializationFailureRuntime struct{ provider.Runtime }

func (r initializationFailureRuntime) Mode() string { return "fake" }
func (r initializationFailureRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return initializationFailureSession{Session: session}, nil
}

type initializationFailureSession struct{ provider.Session }

func (initializationFailureSession) Initialize(context.Context, codex.TransportPolicy) error {
	return errors.New("offline initialize failed before protocol acknowledgement")
}

type threadStartFailureRuntime struct{ provider.Runtime }

func (threadStartFailureRuntime) Mode() string { return "fake" }
func (r threadStartFailureRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return threadStartFailureSession{Session: session}, nil
}

type threadStartFailureSession struct{ provider.Session }

func (threadStartFailureSession) StartThread(context.Context, provider.ThreadStartOptions) (string, error) {
	return "", errors.New("offline thread start failed before a turn")
}

type localQualificationFakeRuntime struct {
	*provider.FakeRuntime
	profile provider.ExecutionProfile
}

func newLocalQualificationFakeRuntime() *localQualificationFakeRuntime {
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	profile := base.ExecutionProfile()
	profile.Purpose = provider.ProductLocalProcessLaunchQualificationPurpose
	profile.ToolCallLimit = 0
	return &localQualificationFakeRuntime{FakeRuntime: base, profile: profile}
}

func (r *localQualificationFakeRuntime) ExecutionProfile() provider.ExecutionProfile {
	return r.profile
}
func (r *localQualificationFakeRuntime) StartLocalQualification(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	return r.FakeRuntime.Start(ctx, options)
}

func waitForTerminalEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, company, kind string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind=$2", company, kind).Scan(&count); err == nil && count == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", kind)
}

func waitForProviderTerminal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, company string, sessionState, taskState *string, artifacts, terminalEvents *int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var eventCount int
		var artifactCount int
		var currentSession, currentTask string
		err := pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&currentSession)
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND kind='compat' ORDER BY id DESC LIMIT 1", company).Scan(&currentTask)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM artifacts WHERE company_id=$1", company).Scan(&artifactCount)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind='provider.turn.completed'", company).Scan(&eventCount)
		}
		if err == nil && currentSession == "stopped" && currentTask == "candidate" && artifactCount == 1 && eventCount == 1 {
			*sessionState, *taskState, *artifacts, *terminalEvents = currentSession, currentTask, artifactCount, eventCount
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	var sessions, tasks, checks, checkpoints, terminal, events []byte
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'task_id',task_id,'state',state,'profile',profile)), '[]'::json) FROM worker_sessions WHERE company_id=$1", company).Scan(&sessions)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'state',state,'kind',kind)), '[]'::json) FROM tasks WHERE company_id=$1", company).Scan(&tasks)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'phase',phase,'passed',passed,'report',report)), '[]'::json) FROM worker_checks WHERE company_id=$1", company).Scan(&checks)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(data), '[]'::json) FROM worker_checkpoints WHERE company_id=$1", company).Scan(&checkpoints)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(data), '[]'::json) FROM worker_observations WHERE company_id=$1 AND reason='provider_terminal'", company).Scan(&terminal)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('kind',kind,'payload',payload)), '[]'::json) FROM events WHERE company_id=$1", company).Scan(&events)
	t.Fatalf("timed out waiting for offline provider terminal state: sessions=%s tasks=%s checks=%s checkpoints=%s terminal=%s events=%s", sessions, tasks, checks, checkpoints, terminal, events)
}
