// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"polis/internal/codex"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

type RealProviderWorkerAdapter struct {
	runtime provider.Runtime
	kernel  *kernel.Kernel
	mu      sync.Mutex
	workers map[string]*providerWorker
}

type providerWorker struct {
	binding       kernel.Binding
	authorization provider.ExecutionAuthorization
	reservation   provider.Reservation
	session       provider.Session
	cancel        context.CancelFunc
	cleanupOnce   sync.Once
	done          chan struct{}
	ready         chan struct{}
	readyOnce     sync.Once
	readyErr      error
	mu            sync.Mutex
	cleanupErr    error
}

func (w *providerWorker) signalReady(err error) {
	w.readyOnce.Do(func() {
		w.mu.Lock()
		w.readyErr = err
		w.mu.Unlock()
		close(w.ready)
	})
}

// BusinessContextPreflightResult is the zero-provider proof for the real
// product business binding. It intentionally stops before any reservation or
// turn and is only valid for a local qualification runtime.
type BusinessContextPreflightResult struct {
	CompanyID             string
	MissionID             string
	TaskID                string
	SessionID             string
	Epoch                 int64
	Incarnation           string
	ProcessPID            int
	ProcessExited         bool
	ProcessStarted        bool
	InitializePassed      bool
	ThreadStarted         bool
	RegisteredToolCount   int
	RegisteredToolDigest  string
	BusinessBindingPassed bool
	CleanStop             bool
	ProviderReservations  int
	ActiveReservations    int
	ClosedReservations    int
	ProviderEgress        int
	Initialization        codex.InitializeLifecycleEvidence
}

func NewRealProviderWorkerAdapter(runtime *kernel.Kernel, providerRuntime provider.Runtime) (*RealProviderWorkerAdapter, error) {
	if runtime == nil || providerRuntime == nil {
		return nil, errors.New("real provider worker adapter dependencies are incomplete")
	}
	return &RealProviderWorkerAdapter{runtime: providerRuntime, kernel: runtime, workers: make(map[string]*providerWorker)}, nil
}

// NewRealProviderWorkerLaunchQualificationAdapter keeps the exact real
// adapter/runtime boundary available for local no-turn process qualification.
// It intentionally has no kernel and cannot start a business WorkerSession.
func NewRealProviderWorkerLaunchQualificationAdapter(providerRuntime provider.Runtime) (*RealProviderWorkerAdapter, error) {
	if providerRuntime == nil {
		return nil, errors.New("real provider launch qualification runtime is incomplete")
	}
	return &RealProviderWorkerAdapter{runtime: providerRuntime, workers: make(map[string]*providerWorker)}, nil
}

func (a *RealProviderWorkerAdapter) Mode() string { return "real" }
func (a *RealProviderWorkerAdapter) ToolSurface() provider.ToolSurface {
	return a.runtime.ToolSurface()
}
func (a *RealProviderWorkerAdapter) Readiness(ctx context.Context) error {
	if err := a.runtime.Readiness(ctx); err != nil {
		return fmt.Errorf("real provider runtime unavailable: %w", err)
	}
	surface := a.runtime.ToolSurface()
	if surface.ToolCount != 7 || surface.ManifestDigest != provider.ProductToolSurfaceV4ManifestDigest || surface.AggregateSchemaBytes != provider.ProductToolSurfaceV4AggregateSchemaBytes || surface.AggregateSchemaDigest != provider.ProductToolSurfaceV4AggregateSchemaDigest {
		return errors.New("real provider product tool surface is not the current @4 surface; live qualification is required")
	}
	profile := a.runtime.ExecutionProfile()
	if profile.Model == "" || profile.Effort != "medium" || (profile.ToolCallLimit <= 0 && profile.Purpose != provider.ProductLocalProcessLaunchQualificationPurpose) || profile.Purpose == "" || profile.ExecutionEnvelope == "" || profile.ToolSurfaceQualification != provider.ProductToolSurfaceQualification || profile.ExactSurfaceExecutionFingerprint != provider.ProductExactSurfaceExecutionFingerprint || profile.ProductProviderL2Fingerprint != provider.ProductProviderL2Fingerprint {
		return errors.New("real provider execution profile is unavailable")
	}
	if profile.Purpose == provider.Live2AuthorizationPurpose && (profile.Model != "gpt-5.6-luna" || profile.Effort != "medium" || profile.ToolCallLimit != 16 || profile.ExecutionEnvelope != provider.ProductProviderRuntimeEnvelopeFingerprintV2) {
		return errors.New("real provider LIVE_2 profile differs from the qualified B4 execution")
	}
	return nil
}

type localQualificationRuntime interface {
	StartLocalQualification(context.Context, provider.SessionStartOptions) (provider.Session, error)
}

// QualifyLocalLaunch runs the same RealProviderWorkerAdapter-to-runtime
// handshake used by serve, ending after exact thread/start and before turn.
func (a *RealProviderWorkerAdapter) QualifyLocalLaunch(ctx context.Context, sessionID string) error {
	if a.kernel != nil {
		return errors.New("local launch qualification adapter must not own a business kernel")
	}
	if err := a.Readiness(ctx); err != nil {
		return err
	}
	local, ok := a.runtime.(localQualificationRuntime)
	if !ok {
		return errors.New("provider runtime does not expose the zero-turn local qualification boundary")
	}
	profile := a.runtime.ExecutionProfile()
	session, err := local.StartLocalQualification(ctx, provider.SessionStartOptions{SessionID: sessionID, Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, ToolSurface: a.runtime.ToolSurface()})
	if err != nil {
		return err
	}
	if session.Process() == nil {
		return errors.New("local qualification session has no process handle")
	}
	if err = session.Initialize(ctx, profile.TransportPolicy); err != nil {
		_, _ = session.Stop(ctx)
		return err
	}
	thread, err := session.StartThread(ctx, provider.ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: a.runtime.ToolSurface().Tools})
	if err != nil {
		_, _ = session.Stop(ctx)
		return err
	}
	if thread == "" {
		_, _ = session.Stop(ctx)
		return errors.New("local qualification thread/start returned empty thread ID")
	}
	proof, err := session.Stop(ctx)
	if err != nil || !proof.For(sessionID) {
		return fmt.Errorf("local qualification clean stop failed: %v", err)
	}
	return nil
}

// QualifyLocalBusinessContext exercises the production business binding and
// canonical process/session lifecycle without reserving business allowance or
// starting a provider turn.
func (a *RealProviderWorkerAdapter) QualifyLocalBusinessContext(ctx context.Context, companyID, missionID string) (BusinessContextPreflightResult, error) {
	result := BusinessContextPreflightResult{CompanyID: companyID, MissionID: missionID}
	if a.kernel == nil {
		return result, errors.New("business preflight requires a kernel")
	}
	if err := a.Readiness(ctx); err != nil {
		return result, fmt.Errorf("business preflight readiness failed: %w", err)
	}
	local, ok := a.runtime.(localQualificationRuntime)
	if !ok {
		return result, errors.New("provider runtime does not expose local qualification")
	}
	profile := a.runtime.ExecutionProfile()
	if profile.Purpose != provider.ProductLocalProcessLaunchQualificationPurpose || profile.ToolCallLimit != 0 {
		return result, errors.New("business preflight requires the zero-budget local qualification profile")
	}
	scope := a.kernel.LocalScope(companyID)
	details, err := a.kernel.MissionDetails(ctx, scope, missionID)
	if err != nil {
		return result, fmt.Errorf("business preflight MissionDetails failed: %w", err)
	}
	task, err := a.kernel.TXPrepareProductTask(ctx, scope, missionID, details.Goal, "product-preflight-task-"+missionID)
	if err != nil {
		return result, fmt.Errorf("business preflight task preparation failed: %w", err)
	}
	if !task.IsProductProviderExecutable() || task.Mission != missionID || !task.HasValidProductValidationBinding() || task.State != "ready" {
		return result, fmt.Errorf("business preflight Task is not the unique bound product Task: task=%+v", task)
	}
	binding, err := a.kernel.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, profile.Profile, 0)
	if err != nil {
		return result, fmt.Errorf("business preflight WorkerSession creation failed: %w", err)
	}
	result.TaskID, result.SessionID, result.Epoch, result.Incarnation = binding.TaskID(), binding.SessionID(), binding.Epoch(), binding.Incarnation()
	result.BusinessBindingPassed = binding.TaskID() == task.ID && binding.EmployeeID() == task.Owner && binding.TaskValidationBindingDigest() == task.ValidationBinding.ConfigurationDigest
	if !result.BusinessBindingPassed {
		_ = a.kernel.TXFinalizeWorkerBeforeProcess(context.Background(), binding, "business preflight binding mismatch", "business-preflight-binding-failed-"+binding.SessionID())
		return result, errors.New("business preflight binding does not match Task")
	}
	session, err := local.StartLocalQualification(ctx, provider.SessionStartOptions{SessionID: binding.SessionID(), Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, MissionID: missionID, TaskID: task.ID, ToolSurface: a.runtime.ToolSurface()})
	if err != nil {
		_ = a.kernel.TXFinalizeWorkerBeforeProcess(context.Background(), binding, err.Error(), "business-preflight-process-start-failed-"+binding.SessionID())
		return result, err
	}
	if session.Process() == nil {
		_, _ = session.Stop(context.Background())
		_ = a.kernel.TXFinalizeWorkerBeforeProcess(context.Background(), binding, "business preflight session has no process", "business-preflight-no-process-"+binding.SessionID())
		return result, errors.New("business preflight session has no process")
	}
	result.ProcessStarted = true
	result.ProcessPID = session.Process().PID()
	if err = a.kernel.TXAttachWorker(ctx, binding, session.Process()); err != nil {
		_, _ = session.Stop(context.Background())
		return result, fmt.Errorf("business preflight process attach failed: %w", err)
	}
	if err = a.kernel.TXValidateWorker(ctx, binding); err != nil {
		_, _ = session.Stop(context.Background())
		return result, fmt.Errorf("business preflight WorkerSession validation failed: %w", err)
	}
	if err = a.kernel.TXActivateWorker(ctx, binding, profile.Profile); err != nil {
		_, _ = session.Stop(context.Background())
		return result, fmt.Errorf("business preflight WorkerSession activation failed: %w", err)
	}
	if err = session.Initialize(ctx, profile.TransportPolicy); err != nil {
		result.Initialization = initializationEvidence(session, err)
		_, _ = a.kernel.TXRecordProviderInitializationFailure(context.Background(), binding, result.Initialization, "business-preflight-initialize-failed-"+binding.SessionID())
		_ = a.stopUnactivated(context.Background(), binding, session)
		return result, err
	}
	result.InitializePassed = true
	result.Initialization = initializationEvidence(session, nil)
	thread, err := session.StartThread(ctx, provider.ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: a.runtime.ToolSurface().Tools})
	if err != nil || thread == "" {
		if err == nil {
			err = errors.New("business preflight thread/start returned empty thread ID")
		}
		_ = a.stopUnactivated(context.Background(), binding, session)
		return result, err
	}
	result.ThreadStarted = true
	result.RegisteredToolCount = a.runtime.ToolSurface().ToolCount
	result.RegisteredToolDigest = a.runtime.ToolSurface().ManifestDigest
	if err = a.kernel.TXBeginStop(ctx, binding); err != nil {
		_, _ = session.Stop(context.Background())
		return result, fmt.Errorf("business preflight stop transition failed: %w", err)
	}
	proof, stopErr := session.Stop(ctx)
	if stopErr != nil {
		return result, stopErr
	}
	if err = a.kernel.TXConfirmStopped(ctx, binding, proof); err != nil {
		return result, err
	}
	result.CleanStop = proof.For(binding.SessionID())
	return result, nil
}

func (a *RealProviderWorkerAdapter) Start(ctx context.Context, companyID, missionID string) error {
	_, err := a.startBusinessWorker(ctx, companyID, missionID, true)
	return err
}

func (a *RealProviderWorkerAdapter) startBusinessWorker(ctx context.Context, companyID, missionID string, allowTurn bool) (*providerWorker, error) {
	a.mu.Lock()
	key := companyID + "/" + missionID
	if _, exists := a.workers[key]; exists {
		a.mu.Unlock()
		return nil, nil
	}
	a.mu.Unlock()
	if err := a.Readiness(ctx); err != nil {
		return nil, err
	}
	scope := a.kernel.LocalScope(companyID)
	details, err := a.kernel.MissionDetails(ctx, scope, missionID)
	if err != nil {
		return nil, err
	}
	task, err := a.kernel.TXPrepareProductTask(ctx, scope, missionID, details.Goal, "product-task-"+missionID)
	if err != nil {
		return nil, err
	}
	if !task.IsProductProviderExecutable() || task.Mission != missionID || !task.HasValidProductValidationBinding() {
		return nil, errors.New("prepared product Task is not provider-executable for this Mission")
	}
	if task.State != "ready" {
		return nil, nil
	}
	profile := a.runtime.ExecutionProfile()
	binding, err := a.kernel.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, profile.Profile, int64(profile.ToolCallLimit))
	if err != nil {
		return nil, err
	}
	var reservation provider.Reservation
	closeReservation := func(reason string) error {
		if reservation.ID == "" {
			return nil
		}
		return a.runtime.CloseReservation(context.Background(), reservation, reason)
	}
	failStart := func(startErr error, session provider.Session) error {
		if session == nil {
			finalizeErr := a.kernel.TXFinalizeWorkerBeforeProcess(context.Background(), binding, startErr.Error(), "worker-start-failed-"+binding.SessionID())
			closeErr := closeReservation("start_failed")
			if finalizeErr != nil {
				return fmt.Errorf("%v; worker finalization failed: %w", startErr, finalizeErr)
			}
			if closeErr != nil {
				return fmt.Errorf("%v; reservation cleanup failed: %w", startErr, closeErr)
			}
			return startErr
		}
		if cleanupErr := a.stopUnactivated(context.Background(), binding, session); cleanupErr != nil {
			return fmt.Errorf("%v; worker cleanup failed: %w", startErr, cleanupErr)
		}
		if closeErr := closeReservation("start_failed"); closeErr != nil {
			return fmt.Errorf("%v; reservation cleanup failed: %w", startErr, closeErr)
		}
		return startErr
	}
	if err = a.recordLifecycle(binding, "adapter_start_entered", "", 0, false, false); err != nil {
		return nil, failStart(fmt.Errorf("provider start lifecycle persistence failed: %w", err), nil)
	}
	if binding.TaskID() != task.ID || binding.EmployeeID() != task.Owner || binding.TaskValidationBindingDigest() != task.ValidationBinding.ConfigurationDigest {
		_ = a.recordLifecycleFailure(binding, "authorization_became_invalid", "provider WorkerSession binding does not match its Task")
		return nil, failStart(errors.New("provider WorkerSession binding does not match its Task assignment"), nil)
	}
	surface := a.runtime.ToolSurface()
	authorization := provider.ExecutionAuthorization{CompanyID: companyID, MissionID: missionID, TaskID: binding.TaskID(), TaskKind: task.Kind, TaskOwnerID: task.Owner, EmployeeID: binding.EmployeeID(), EmployeeRole: "backend", SessionID: binding.SessionID(), Epoch: binding.Epoch(), Incarnation: binding.Incarnation(), Model: profile.Model, Profile: profile.Profile, Effort: profile.Effort, ProviderMode: a.runtime.Mode(), Purpose: profile.Purpose, ToolSurfaceDigest: surface.ManifestDigest, ToolSurfaceQualification: profile.ToolSurfaceQualification, ToolCount: surface.ToolCount, AggregateSchemaBytes: surface.AggregateSchemaBytes, AggregateSchemaDigest: surface.AggregateSchemaDigest, ExactSurfaceExecutionFingerprint: profile.ExactSurfaceExecutionFingerprint, ProductProviderL2Fingerprint: profile.ProductProviderL2Fingerprint, TaskValidationBindingDigest: binding.TaskValidationBindingDigest(), WorkspaceDigest: binding.WorkspaceDigest(), WorkspaceRevision: binding.WorkspaceRevision(), ExecutionEnvelope: profile.ExecutionEnvelope, TransportPolicyRevision: profile.TransportPolicy.Revision, ToolCallLimit: profile.ToolCallLimit}
	if err = provider.ValidateExecutionAuthorization(authorization); err != nil {
		_ = a.recordLifecycleFailure(binding, "authorization_became_invalid", "provider authorization validation failed")
		return nil, failStart(err, nil)
	}
	if err = a.kernel.ValidateProductProviderAuthorizationBinding(ctx, binding, profile.Profile, int64(profile.ToolCallLimit)); err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "authorization_became_invalid"), "provider authorization binding became invalid before reservation")
		return nil, failStart(fmt.Errorf("provider authorization binding became stale before reservation: %w", err), nil)
	}
	reservation, err = a.runtime.Reserve(ctx, authorization)
	if err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "reservation_invalid"), "provider reservation was not accepted")
		return nil, failStart(err, nil)
	}
	if err = a.recordLifecycle(binding, "reservation_bound", "", 0, false, false); err != nil {
		return nil, failStart(fmt.Errorf("provider reservation lifecycle persistence failed: %w", err), nil)
	}
	session, err := a.runtime.Start(ctx, provider.SessionStartOptions{SessionID: binding.SessionID(), Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, MissionID: missionID, TaskID: task.ID, ToolSurface: a.runtime.ToolSurface()})
	if err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "process_handle_lost"), "provider process was not attached")
		return nil, failStart(err, nil)
	}
	if session.Process() == nil {
		_ = a.recordLifecycleFailure(binding, "process_handle_lost", "provider session returned no process handle")
		return nil, failStart(errors.New("real provider session has no process handle"), nil)
	}
	if err = a.recordLifecycle(binding, "process_created", "", session.Process().PID(), true, false); err != nil {
		return nil, failStart(fmt.Errorf("provider process lifecycle persistence failed: %w", err), session)
	}
	if err = a.kernel.TXAttachWorker(ctx, binding, session.Process()); err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "writer_fenced"), "provider process attachment was fenced")
		return nil, failStart(err, session)
	}
	if err = a.recordLifecycle(binding, "process_attached", "", session.Process().PID(), true, true); err != nil {
		return nil, failStart(fmt.Errorf("provider process attach lifecycle persistence failed: %w", err), session)
	}
	if err = a.kernel.TXValidateWorker(ctx, binding); err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "writer_fenced"), "WorkerSession validation was fenced before initialize")
		return nil, failStart(err, session)
	}
	if err = a.kernel.TXActivateWorker(ctx, binding, profile.Model+"/"+profile.Effort); err != nil {
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "writer_fenced"), "WorkerSession activation was fenced before initialize")
		return nil, failStart(err, session)
	}
	if err = a.recordLifecycle(binding, "worker_activated", "", session.Process().PID(), true, true); err != nil {
		return nil, failStart(fmt.Errorf("provider activation lifecycle persistence failed: %w", err), session)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	worker := &providerWorker{binding: binding, authorization: authorization, reservation: reservation, session: session, cancel: cancel, done: make(chan struct{}), ready: make(chan struct{})}
	a.mu.Lock()
	if _, exists := a.workers[key]; exists {
		a.mu.Unlock()
		_ = a.recordLifecycleFailure(binding, "duplicate_stop", "duplicate worker ownership was detected before initialize")
		cancel()
		_, _ = session.Stop(context.Background())
		_ = closeReservation("duplicate_stop")
		return nil, nil
	}
	a.workers[key] = worker
	a.mu.Unlock()
	go a.run(runCtx, key, worker, profile, allowTurn)
	return worker, nil
}

// QualifyLocalReservedBusinessContext exercises the reservation-bearing
// business start path and stops after thread/start, before any turn/start.
// It is a local qualification boundary and is not a production task command.
func (a *RealProviderWorkerAdapter) QualifyLocalReservedBusinessContext(ctx context.Context, companyID, missionID string) (BusinessContextPreflightResult, error) {
	result := BusinessContextPreflightResult{CompanyID: companyID, MissionID: missionID}
	if a.kernel == nil {
		return result, errors.New("reservation-bearing preflight requires a kernel")
	}
	profile := a.runtime.ExecutionProfile()
	if profile.Purpose != provider.ProductReservationBridgeQualificationPurpose || profile.ToolCallLimit <= 0 {
		return result, errors.New("reservation-bearing preflight requires its bounded qualification profile")
	}
	worker, err := a.startBusinessWorker(ctx, companyID, missionID, false)
	if err != nil {
		return result, err
	}
	if worker == nil {
		return result, errors.New("reservation-bearing preflight did not create a WorkerSession")
	}
	select {
	case <-worker.ready:
	case <-ctx.Done():
		worker.cancel()
		_ = a.cleanup(context.Background(), companyID+"/"+missionID, worker)
		return result, ctx.Err()
	}
	worker.mu.Lock()
	readyErr := worker.readyErr
	worker.mu.Unlock()
	<-worker.done
	worker.mu.Lock()
	cleanupErr := worker.cleanupErr
	worker.mu.Unlock()
	if worker.session.Process() != nil {
		result.ProcessStarted = true
		result.ProcessPID = worker.session.Process().PID()
		result.ProcessExited = worker.session.Process().HasExited()
	}
	result.TaskID = worker.binding.TaskID()
	result.SessionID = worker.binding.SessionID()
	result.Epoch = worker.binding.Epoch()
	result.Incarnation = worker.binding.Incarnation()
	result.BusinessBindingPassed = worker.binding.TaskValidationBindingDigest() == worker.authorization.TaskValidationBindingDigest && worker.authorization.Purpose == profile.Purpose
	result.Initialization = initializationEvidence(worker.session, readyErr)
	result.InitializePassed = readyErr == nil
	result.ThreadStarted = readyErr == nil
	result.RegisteredToolCount = a.runtime.ToolSurface().ToolCount
	result.RegisteredToolDigest = a.runtime.ToolSurface().ManifestDigest
	result.CleanStop = cleanupErr == nil
	stats := a.runtime.Stats()
	result.ProviderReservations = stats.Reservations
	result.ActiveReservations = stats.ActiveReservations
	result.ClosedReservations = stats.ClosedReservations
	result.ProviderEgress = stats.ProviderEgress
	if readyErr != nil {
		return result, readyErr
	}
	if cleanupErr != nil {
		return result, cleanupErr
	}
	return result, nil
}

func (a *RealProviderWorkerAdapter) run(ctx context.Context, key string, worker *providerWorker, profile provider.ExecutionProfile, allowTurn bool) {
	state, outcome := "inconclusive", "runtime_failure"
	var usage any
	if err := worker.session.Initialize(ctx, profile.TransportPolicy); err != nil {
		worker.signalReady(err)
		_, _ = a.kernel.TXRecordProviderInitializationFailure(context.Background(), worker.binding, initializationFailureEvidence(worker.session, err), "provider-initialization-failed-"+worker.binding.SessionID())
		_ = a.cleanup(context.Background(), key, worker)
		return
	}
	thread, threadErr := worker.session.StartThread(ctx, provider.ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: a.runtime.ToolSurface().Tools})
	if threadErr != nil {
		worker.signalReady(threadErr)
		_, _ = a.kernel.TXRecordProviderThreadStartFailure(context.Background(), worker.binding, map[string]any{
			"phase":        "thread_start",
			"reason_code":  "thread_start_failed",
			"safe_message": "provider thread/start failed before a turn was started",
		}, "provider-thread-start-failed-"+worker.binding.SessionID())
		_ = a.cleanup(context.Background(), key, worker)
		return
	}
	worker.signalReady(nil)
	if !allowTurn {
		_ = a.cleanup(context.Background(), key, worker)
		return
	}
	prompt := "Complete the assigned product task using only the registered Polis tools. Persist the requested artifact and checkpoint, then stop."
	turn, turnErr := worker.session.Turn(ctx, thread, prompt, codexTurnOptions(profile), func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		tools := kernel.EmployeeTools{Kernel: a.kernel, Binding: worker.binding, ProductSurface: true}
		result := tools.Call(ctx, name, callID, raw)
		encoded, _ := json.Marshal(result)
		return encoded, false
	})
	usage = map[string]any{"authorization": worker.authorization, "token_usage": turn.Usage, "tool_calls": turn.ToolCalls, "provider_egress": turn.ProviderEgress, "started_at": turn.StartedAt, "finished_at": turn.FinishedAt}
	if turnErr != nil {
		state, outcome = "inconclusive", "transport_failure"
		if ctx.Err() != nil {
			outcome = "cancelled"
		}
	} else if turn.State == "completed" {
		state, outcome = "completed", turn.Outcome
	} else {
		state, outcome = "failed", turn.Outcome
	}
	_, _ = a.kernel.TXRecordProviderTerminal(context.Background(), worker.binding, state, outcome, usage, "provider-terminal-"+worker.binding.SessionID())
	_ = a.cleanup(context.Background(), key, worker)
}

func initializationFailureEvidence(session provider.Session, err error) map[string]any {
	evidence := initializationEvidence(session, err)
	return map[string]any{
		"phase":                         evidence.Phase,
		"reason_code":                   evidence.ReasonCode,
		"failure_category":              evidence.FailureCategory,
		"safe_message":                  evidence.SafeMessage,
		"process_created":               evidence.ProcessCreated,
		"pid_present":                   evidence.PIDPresent,
		"child_exit_observed":           evidence.ChildExitObserved,
		"initialize_request_sent":       evidence.InitializeRequestSent,
		"initialize_ack_received":       evidence.InitializeAckReceived,
		"initialized_notification_sent": evidence.InitializedNotificationSent,
		"stdout_pipe_state":             evidence.StdoutPipeState,
		"stderr_category":               evidence.StderrCategory,
		"lifecycle":                     evidence.Lifecycle,
	}
}

func initializationEvidence(session provider.Session, err error) codex.InitializeLifecycleEvidence {
	evidence := codex.InitializeLifecycleEvidence{Phase: "initialize", ReasonCode: "initialization_failed", SafeMessage: "provider initialization failed before a turn was started"}
	if observed, ok := session.(interface {
		InitializationEvidence() codex.InitializeLifecycleEvidence
	}); ok {
		evidence = observed.InitializationEvidence()
	}
	var failure *codex.InitializationFailure
	if errors.As(err, &failure) {
		evidence = failure.InitializeLifecycleEvidence
	}
	if evidence.FailureCategory == "" && err != nil {
		evidence.FailureCategory = codex.ClassifyInitializationFailureCategory(err, evidence)
	}
	return evidence
}

func (a *RealProviderWorkerAdapter) Stop(ctx context.Context, companyID, missionID string) error {
	key := companyID + "/" + missionID
	a.mu.Lock()
	worker := a.workers[key]
	a.mu.Unlock()
	if worker == nil {
		return nil
	}
	worker.cancel()
	return a.cleanup(ctx, key, worker)
}

func (a *RealProviderWorkerAdapter) cleanup(ctx context.Context, key string, worker *providerWorker) error {
	worker.cleanupOnce.Do(func() {
		var err error
		if err = a.kernel.TXBeginStop(ctx, worker.binding); err == nil {
			var proofErr error
			var proof runner.StopProof
			proof, proofErr = worker.session.Stop(ctx)
			if proofErr != nil {
				err = proofErr
			} else {
				err = a.kernel.TXConfirmStopped(ctx, worker.binding, proof)
			}
		}
		if closeErr := a.runtime.CloseReservation(context.Background(), worker.reservation, "worker_cleanup"); err == nil {
			err = closeErr
		}
		worker.mu.Lock()
		worker.cleanupErr = err
		worker.mu.Unlock()
		close(worker.done)
	})
	select {
	case <-worker.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	a.mu.Lock()
	delete(a.workers, key)
	a.mu.Unlock()
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.cleanupErr
}

func (a *RealProviderWorkerAdapter) stopUnactivated(ctx context.Context, binding kernel.Binding, session provider.Session) error {
	if err := a.kernel.TXBeginStop(ctx, binding); err != nil {
		return err
	}
	proof, err := session.Stop(ctx)
	if err != nil {
		return err
	}
	return a.kernel.TXConfirmStopped(ctx, binding, proof)
}

func (a *RealProviderWorkerAdapter) Close() {
	a.mu.Lock()
	keys := make([]string, 0, len(a.workers))
	for key := range a.workers {
		keys = append(keys, key)
	}
	a.mu.Unlock()
	for _, key := range keys {
		company, mission, ok := splitProviderWorkerKey(key)
		if ok {
			_ = a.Stop(context.Background(), company, mission)
		}
	}
}

func splitProviderWorkerKey(key string) (string, string, bool) { return splitWorkerKey(key) }

func codexTurnOptions(profile provider.ExecutionProfile) codex.TurnOptions {
	return codex.TurnOptions{Policy: &profile.TransportPolicy, ToolCallLimit: profile.ToolCallLimit}
}

func (a *RealProviderWorkerAdapter) recordLifecycle(binding kernel.Binding, phase, reason string, pid int, processCreated, pidAttached bool) error {
	_, err := a.kernel.TXRecordProviderLifecycle(context.Background(), binding, kernel.ProviderLifecycleEvent{Phase: phase, ReasonCode: reason, ProcessPID: pid, ProcessCreated: processCreated, PIDAttached: pidAttached}, "provider-lifecycle-"+phase+"-"+binding.SessionID())
	return err
}

func (a *RealProviderWorkerAdapter) recordLifecycleFailure(binding kernel.Binding, category, safeMessage string) error {
	_, err := a.kernel.TXRecordProviderLifecycle(context.Background(), binding, kernel.ProviderLifecycleEvent{Phase: "pre_initialize_failure", ReasonCode: category, SafeMessage: safeMessage}, "lifecycle-failure-"+category+"-"+binding.SessionID())
	return err
}

func lifecycleFailureCategory(err error, fallback string) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "context_cancelled"
	}
	return fallback
}

var _ WorkerAdapter = (*RealProviderWorkerAdapter)(nil)
