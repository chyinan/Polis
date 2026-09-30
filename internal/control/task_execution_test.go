// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
)

func TestRealProductStartHasOneProviderExecutableTaskAndOneBoundWorker(t *testing.T) {
	ctx := context.Background()
	k, companyID, service, runtime := newR05B5ControlFixture(t, nil)
	defer k.Close()
	defer service.Close()

	contract := r05b5AcceptanceContract()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title:              "one product work Task",
		Goal:               "write the public smoke artifact",
		RequestID:          "r05b5-create",
		AcceptanceContract: &contract,
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-start"})
	if err != nil {
		t.Fatalf("product Start failed: %v", err)
	}
	if !started.Accepted || started.ResultingState != "active" {
		t.Fatalf("unexpected Start receipt: %+v", started)
	}

	pool, err := pgxpool.New(ctx, r05b5TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var sessionState, taskState string
	var artifactCount, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifactCount, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifactCount != 1 || terminalEvents != 1 {
		t.Fatalf("unexpected offline product terminal state: session=%s task=%s artifacts=%d terminal_events=%d", sessionState, taskState, artifactCount, terminalEvents)
	}

	var taskRows, bootstrapRows, compatRows, providerTaskRows, sessions int
	if err = pool.QueryRow(ctx, `SELECT count(*),
count(*) FILTER (WHERE kind='bootstrap_plan'),
count(*) FILTER (WHERE kind='compat'),
count(*) FILTER (WHERE kind='compat' AND owner='emp-backend')
FROM tasks WHERE company_id=$1 AND mission_id=$2`, companyID, created.TargetID).Scan(&taskRows, &bootstrapRows, &compatRows, &providerTaskRows); err != nil {
		t.Fatal(err)
	}
	if taskRows != 2 || bootstrapRows != 1 || compatRows != 1 || providerTaskRows != 1 {
		t.Fatalf("Mission Task rows total/bootstrap/compat/provider-executable = %d/%d/%d/%d, want 2/1/1/1", taskRows, bootstrapRows, compatRows, providerTaskRows)
	}
	var workerTaskID, employeeID, workerSessionID, workerState, profile, incarnation string
	var epoch int64
	if err = pool.QueryRow(ctx, `SELECT s.task_id,s.employee_id,s.id,s.state,s.profile,s.epoch,s.incarnation
FROM worker_sessions s WHERE s.company_id=$1`, companyID).Scan(&workerTaskID, &employeeID, &workerSessionID, &workerState, &profile, &epoch, &incarnation); err != nil {
		t.Fatal(err)
	}
	if employeeID != core.EmployeeBackendID || workerState != "stopped" || profile != "gpt-5.6-luna/medium" || workerTaskID == "" || workerSessionID == "" || epoch < 1 || incarnation == "" {
		t.Fatalf("invalid provider WorkerSession binding: task=%s employee=%s session=%s state=%s profile=%s epoch=%d incarnation=%s", workerTaskID, employeeID, workerSessionID, workerState, profile, epoch, incarnation)
	}
	var boundTaskID, boundMissionID, boundDigest, bindingContractJSON string
	var acceptanceRevision, runnerKind, runnerRevision string
	var bindingCount int
	if err = pool.QueryRow(ctx, `SELECT count(*),min(v.task_id),min(v.mission_id),min(v.configuration_digest),min(v.contract::text),
min(v.acceptance_revision),min(v.runner_kind),min(v.runner_revision)
FROM task_validation_bindings v JOIN tasks t ON t.company_id=v.company_id AND t.id=v.task_id
WHERE v.company_id=$1 AND t.mission_id=$2 AND t.kind='compat' AND t.owner='emp-backend'`, companyID, created.TargetID).Scan(&bindingCount, &boundTaskID, &boundMissionID, &boundDigest, &bindingContractJSON, &acceptanceRevision, &runnerKind, &runnerRevision); err != nil {
		t.Fatal(err)
	}
	if bindingCount != 1 || boundTaskID != workerTaskID || boundMissionID != created.TargetID || len(boundDigest) != 64 {
		t.Fatalf("TaskValidationBinding does not target the execution Task: count=%d task=%s worker_task=%s mission=%s digest=%s", bindingCount, boundTaskID, workerTaskID, boundMissionID, boundDigest)
	}
	var boundContract taskvalidation.AcceptanceContract
	if err = json.Unmarshal([]byte(bindingContractJSON), &boundContract); err != nil {
		t.Fatal(err)
	}
	expectedBinding, err := taskvalidation.Bind(workerTaskID, created.TargetID, &contract)
	if err != nil {
		t.Fatal(err)
	}
	if acceptanceRevision != expectedBinding.AcceptanceRevision || runnerKind != expectedBinding.RunnerKind || runnerRevision != expectedBinding.RunnerRevision || boundDigest != expectedBinding.ConfigurationDigest || boundContract.Revision != expectedBinding.Contract.Revision || fmt.Sprint(boundContract.RequiredText) != fmt.Sprint(expectedBinding.Contract.RequiredText) {
		t.Fatalf("TaskValidationBinding contract differs from the public Mission contract: got=%+v want=%+v", boundContract, expectedBinding.Contract)
	}
	var passedProductChecks int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_checks WHERE company_id=$1 AND session_id=$2 AND phase='product' AND passed", companyID, workerSessionID).Scan(&passedProductChecks); err != nil {
		t.Fatal(err)
	}
	if passedProductChecks != 1 {
		t.Fatalf("public contract/checker produced %d passing product checks, want exactly 1", passedProductChecks)
	}
	var checkRaw []byte
	if err = pool.QueryRow(ctx, "SELECT report FROM worker_checks WHERE company_id=$1 AND session_id=$2 AND phase='product' AND passed", companyID, workerSessionID).Scan(&checkRaw); err != nil {
		t.Fatal(err)
	}
	var check taskvalidation.Result
	if err = json.Unmarshal(checkRaw, &check); err != nil {
		t.Fatal(err)
	}
	if check.Status != taskvalidation.StatusPass || check.TaskID != workerTaskID || check.MissionID != created.TargetID || check.ConfigurationDigest != boundDigest || check.SessionID != workerSessionID || check.Epoch != epoch {
		t.Fatalf("passing product check is not coherent with its TaskValidationBinding and WorkerSession: %+v", check)
	}
	if runtime.reserveCalls.Load() != 1 || runtime.startCalls.Load() != 1 || runtime.turnCalls.Load() != 1 || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline execution fan-out reserve/start/turn/egress = %d/%d/%d/%d, want 1/1/1/0", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load(), runtime.Stats().ProviderEgress)
	}
	var terminalRaw []byte
	if err = pool.QueryRow(ctx, "SELECT data FROM worker_observations WHERE company_id=$1 AND session_id=$2 AND reason='provider_terminal'", companyID, workerSessionID).Scan(&terminalRaw); err != nil {
		t.Fatal(err)
	}
	var terminal struct {
		Usage struct {
			Authorization provider.ExecutionAuthorization `json:"authorization"`
		} `json:"usage"`
	}
	if err = json.Unmarshal(terminalRaw, &terminal); err != nil {
		t.Fatal(err)
	}
	authorization := terminal.Usage.Authorization
	if authorization.CompanyID != companyID || authorization.MissionID != created.TargetID || authorization.TaskID != workerTaskID || authorization.TaskKind != core.TaskKindCompat || authorization.TaskOwnerID != employeeID || authorization.EmployeeID != employeeID || authorization.SessionID != workerSessionID || authorization.Epoch != epoch || authorization.Incarnation != incarnation {
		t.Fatalf("provider authorization is not bound to the actual execution Task and WorkerSession: %+v", authorization)
	}
	if authorization.Purpose != provider.Live2AuthorizationPurpose || authorization.ToolSurfaceQualification != provider.ProductToolSurfaceQualification || authorization.ToolSurfaceDigest != provider.ProductToolSurface().ManifestDigest || authorization.ToolCount != 7 || authorization.AggregateSchemaBytes != provider.ProductToolSurfaceV4AggregateSchemaBytes || authorization.AggregateSchemaDigest != provider.ProductToolSurfaceV4AggregateSchemaDigest || authorization.ExactSurfaceExecutionFingerprint != provider.ProductExactSurfaceExecutionFingerprint || authorization.ProductProviderL2Fingerprint != provider.ProductProviderL2Fingerprint || authorization.TaskValidationBindingDigest != boundDigest || len(authorization.WorkspaceDigest) != 64 || authorization.WorkspaceRevision != 1 {
		t.Fatalf("provider authorization does not bind the LIVE_2 purpose, qualified surface, TaskValidationBinding, and initial workspace CAS: %+v", authorization)
	}

	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-start"}); err != nil {
		t.Fatalf("same-request Start replay should return idempotently after terminal completion: %v", err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-start-duplicate"}); !errors.Is(err, core.Conflict) {
		t.Fatalf("different-request duplicate Start error=%v, want %s", err, core.Conflict)
	}
	if runtime.reserveCalls.Load() != 1 || runtime.startCalls.Load() != 1 || runtime.turnCalls.Load() != 1 {
		t.Fatalf("duplicate Start caused another provider attempt: reserve/start/turn=%d/%d/%d", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load())
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2", companyID, created.TargetID).Scan(&taskRows); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1", companyID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if taskRows != 2 || sessions != 1 {
		t.Fatalf("duplicate Start changed Task/WorkerSession counts to %d/%d, want 2/1", taskRows, sessions)
	}
	var missionState, bootstrapState string
	if err = pool.QueryRow(ctx, `SELECT m.state,
	(SELECT t.state FROM tasks t WHERE t.company_id=m.company_id AND t.mission_id=m.id AND t.kind='bootstrap_plan'),
	(SELECT t.state FROM tasks t WHERE t.company_id=m.company_id AND t.mission_id=m.id AND t.kind='compat')
FROM missions m WHERE m.company_id=$1 AND m.id=$2`, companyID, created.TargetID).Scan(&missionState, &bootstrapState, &taskState); err != nil {
		t.Fatal(err)
	}
	if missionState != "active" || bootstrapState != "completed" || taskState != "candidate" {
		t.Fatalf("unexpected Mission/Task lifecycle states after offline Start: mission=%s bootstrap=%s compat=%s", missionState, bootstrapState, taskState)
	}
	t.Logf("offline product Start: company=%s mission=%s mission_state=%s task_rows=%d bootstrap_plan=%d/%s compat=%d/%s provider_executable_tasks=%d worker_sessions=%d worker_task=%s worker_employee=%s task_validation_binding=%s fake_reserve_calls=%d fake_start_calls=%d fake_turn_calls=%d real_provider_reservations=0 provider_egress=%d live_medium=0 live_high=0",
		companyID, created.TargetID, missionState, taskRows, bootstrapRows, bootstrapState, compatRows, taskState, providerTaskRows, sessions, workerTaskID, employeeID, boundDigest, runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load(), runtime.Stats().ProviderEgress)
}

func TestRealProductStartWithoutTaskValidationBindingStopsBeforeProviderAuthorization(t *testing.T) {
	ctx := context.Background()
	k, companyID, service, runtime := newR05B5ControlFixture(t, nil)
	defer k.Close()
	defer service.Close()

	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title:     "missing public acceptance binding",
		Goal:      "provider execution must not begin without a task-scoped public contract",
		RequestID: "r05b5-no-binding-create",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-no-binding-start"}); err == nil {
		t.Fatal("Start accepted a product Task without TaskValidationBinding")
	}
	if runtime.reserveCalls.Load() != 0 || runtime.startCalls.Load() != 0 || runtime.turnCalls.Load() != 0 || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("Task without TaskValidationBinding reached provider runtime: reserve/start/turn/egress=%d/%d/%d/%d", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load(), runtime.Stats().ProviderEgress)
	}
	pool, err := pgxpool.New(ctx, r05b5TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var sessions int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1", companyID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("Task without TaskValidationBinding created %d WorkerSessions, want 0", sessions)
	}
}

func TestRealProductStartRejectsMissionWithMultipleExecutableTasksBeforeReserve(t *testing.T) {
	ctx := context.Background()
	k, companyID, service, runtime := newR05B5ControlFixture(t, nil)
	defer k.Close()
	defer service.Close()
	pool, err := pgxpool.New(ctx, r05b5TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	contract := r05b5AcceptanceContract()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{Title: "malformed task fan-out", Goal: "fail closed on multiple provider Tasks", RequestID: "r05b5-malformed-create", AcceptanceContract: &contract})
	if err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"preexisting-compat-a"} {
		if _, err = pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
	VALUES($1,$2,$3,$4,$5,'ready')`, companyID, taskID, created.TargetID, core.EmployeeBackendID, core.TaskKindCompat); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-malformed-start"}); err == nil {
		t.Fatal("Start accepted a Mission with multiple provider-executable Tasks")
	}
	if runtime.reserveCalls.Load() != 0 || runtime.startCalls.Load() != 0 || runtime.turnCalls.Load() != 0 {
		t.Fatalf("malformed Mission reached provider boundary: reserve/start/turn=%d/%d/%d", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load())
	}
	var providerTasks, sessions int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat' AND owner='emp-backend'", companyID, created.TargetID).Scan(&providerTasks); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1", companyID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if providerTasks != 1 || sessions != 0 {
		t.Fatalf("malformed Mission product Task/WorkerSession count=%d/%d, want existing 1 and no WorkerSession", providerTasks, sessions)
	}
}

func TestRealProductStartRejectsWorkspaceDriftBeforeReserve(t *testing.T) {
	ctx := context.Background()
	k, companyID, _, observed := newR05B5ControlFixture(t, nil)
	defer k.Close()
	pool, err := pgxpool.New(ctx, r05b5TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mutatingRuntime := &workspaceDriftProviderRuntime{Runtime: observed.Runtime, pool: pool, companyID: companyID}
	adapter, err := NewRealProviderWorkerAdapter(k, mutatingRuntime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()

	contract := r05b5AcceptanceContract()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title:              "stale initial workspace snapshot",
		Goal:               "reject CAS drift before provider reservation",
		RequestID:          "live2-stale-workspace-create",
		AcceptanceContract: &contract,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "live2-stale-workspace-start"}); err == nil {
		t.Fatal("Start accepted provider workspace drift introduced after WorkerSession creation")
	}
	if mutatingRuntime.mutationErr != nil || mutatingRuntime.surfaceCalls.Load() != 3 {
		t.Fatalf("failed to inject workspace CAS drift at the pre-reservation boundary: calls=%d error=%v", mutatingRuntime.surfaceCalls.Load(), mutatingRuntime.mutationErr)
	}
	if observed.reserveCalls.Load() != 0 || observed.startCalls.Load() != 0 || observed.turnCalls.Load() != 0 {
		t.Fatalf("stale workspace reached provider boundary: reserve/start/turn=%d/%d/%d", observed.reserveCalls.Load(), observed.startCalls.Load(), observed.turnCalls.Load())
	}
	var providerSessions, stopped int
	if err = pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE s.state='stopped')
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND t.kind='compat' AND t.owner='emp-backend'`, companyID).Scan(&providerSessions, &stopped); err != nil {
		t.Fatal(err)
	}
	if providerSessions != 1 || stopped != 1 {
		t.Fatalf("stale-workspace provider session count/stopped=%d/%d, want 1/1", providerSessions, stopped)
	}
}

type workspaceDriftProviderRuntime struct {
	provider.Runtime
	pool         *pgxpool.Pool
	companyID    string
	surfaceCalls atomic.Int64
	mutationErr  error
}

func (r *workspaceDriftProviderRuntime) ToolSurface() provider.ToolSurface {
	if r.surfaceCalls.Add(1) == 3 {
		_, r.mutationErr = r.pool.Exec(context.Background(), `UPDATE worker_workspaces w SET revision=revision+1
FROM tasks t WHERE t.company_id=$1 AND t.id=w.task_id AND t.company_id=w.company_id
AND t.kind='compat' AND t.owner='emp-backend'`, r.companyID)
	}
	return r.Runtime.ToolSurface()
}

func TestRealProductStartFailureCannotReserveAgainOnIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	startError := errors.New("offline fake provider process start failure")
	k, companyID, service, runtime := newR05B5ControlFixture(t, startError)
	defer k.Close()
	defer service.Close()
	pool, err := pgxpool.New(ctx, r05b5TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	contract := r05b5AcceptanceContract()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{Title: "one failed start attempt", Goal: "do not repeat a consumed provider attempt", RequestID: "r05b5-failure-create", AcceptanceContract: &contract})
	if err != nil {
		t.Fatal(err)
	}
	startRequest := MissionCommandRequest{MissionID: created.TargetID, RequestID: "r05b5-failure-start"}
	if _, err = service.StartMission(ctx, companyID, startRequest); err == nil {
		t.Fatal("fake process-start failure was reported as a successful Start")
	}
	if runtime.reserveCalls.Load() != 1 || runtime.startCalls.Load() != 1 || runtime.turnCalls.Load() != 0 {
		t.Fatalf("first failed attempt reserve/start/turn=%d/%d/%d, want 1/1/0", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load())
	}
	if _, err = service.StartMission(ctx, companyID, startRequest); err == nil {
		t.Fatal("same-request replay created another WorkerSession after a failed provider start")
	}
	if runtime.reserveCalls.Load() != 1 || runtime.startCalls.Load() != 1 || runtime.turnCalls.Load() != 0 {
		t.Fatalf("idempotent Start replay consumed another attempt: reserve/start/turn=%d/%d/%d", runtime.reserveCalls.Load(), runtime.startCalls.Load(), runtime.turnCalls.Load())
	}
	var sessions, stopped int
	if err = pool.QueryRow(ctx, "SELECT count(*),count(*) FILTER (WHERE state='stopped') FROM worker_sessions WHERE company_id=$1", companyID).Scan(&sessions, &stopped); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || stopped != 1 {
		t.Fatalf("failed-attempt WorkerSession count/stopped=%d/%d, want 1/1", sessions, stopped)
	}
}

func newR05B5ControlFixture(t *testing.T, startError error) (*kernel.Kernel, string, *Service, *observedProviderRuntime) {
	t.Helper()
	k, err := kernel.Open(context.Background(), r05b5TestDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("kernel.Open: %v", err)
	}
	companyID := fmt.Sprintf("r05b5-control-%x", time.Now().UnixNano())
	_, err = k.TXCreateCompany(context.Background(), companyID)
	if err != nil {
		k.Close()
		t.Fatalf("TXCreateCompany: %v", err)
	}
	runtime := &observedProviderRuntime{Runtime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond, Model: "gpt-5.6-luna", Effort: "medium", Purpose: provider.Live2AuthorizationPurpose, ExecutionEnvelope: provider.ProductProviderRuntimeEnvelopeFingerprintV2}), startError: startError}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	return k, companyID, NewService(k, adapter), runtime
}

func r05b5TestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POLIS_R05B5_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B5 PostgreSQL required")
	}
	return dsn
}

func r05b5AcceptanceContract() taskvalidation.AcceptanceContract {
	return taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision,
		RequiredText: []string{
			"Mission ID: {{mission_id}}",
			"Task ID: {{task_id}}",
			"Acknowledgement: This artifact was produced through the Polis real-provider product path.",
			"Task summary: This artifact records one small product integration smoke.",
		},
	}
}

type observedProviderRuntime struct {
	provider.Runtime
	reserveCalls atomic.Int64
	startCalls   atomic.Int64
	turnCalls    atomic.Int64
	startError   error
}

func (r *observedProviderRuntime) Reserve(ctx context.Context, authorization provider.ExecutionAuthorization) (provider.Reservation, error) {
	r.reserveCalls.Add(1)
	return r.Runtime.Reserve(ctx, authorization)
}

func (r *observedProviderRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	r.startCalls.Add(1)
	if r.startError != nil {
		return nil, r.startError
	}
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return &observedProviderSession{Session: session, turnCalls: &r.turnCalls}, nil
}

type observedProviderSession struct {
	provider.Session
	turnCalls *atomic.Int64
}

func (s *observedProviderSession) Turn(ctx context.Context, threadID, prompt string, options codex.TurnOptions, handler provider.ToolHandler) (provider.TurnResult, error) {
	s.turnCalls.Add(1)
	return s.Session.Turn(ctx, threadID, prompt, options, handler)
}
