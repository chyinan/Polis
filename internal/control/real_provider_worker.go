// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

type RealProviderWorkerAdapter struct {
	runtime              provider.Runtime
	kernel               *kernel.Kernel
	mcpFactory           controlledMCPProcessFactory
	mcpOwnerLease        *stdioMCPProcessLease
	mu                   sync.Mutex
	workers              map[string]*providerWorker
	unresolvedWorkers    map[string]*providerWorker
	localCleanupSessions map[string]provider.Session
	localHostReconcile   map[string]bool
	workerCgroupManager  environment.LinuxWorkerCgroupManager
}

type providerWorker struct {
	companyID              string
	missionID              string
	binding                kernel.Binding
	inputContext           kernel.ProductTaskInputContext
	authorization          provider.ExecutionAuthorization
	reservation            provider.Reservation
	reservationCloseReason string
	session                provider.Session
	cancel                 context.CancelFunc
	cleanupMu              sync.Mutex
	cleanupBegun           bool
	processStopped         bool
	stopConfirmed          bool
	reservationClosed      bool
	hostReconcileOnly      bool
	noProcessFailure       bool
	startFailureReason     string
	startFailureRequestID  string
	turnToolCallLimit      int
	taskToolBudget         kernel.ToolCallBudget
	problemKey             string
	stopProof              runner.StopProof
	done                   chan struct{}
	runDone                chan struct{}
	ready                  chan struct{}
	readyOnce              sync.Once
	readyErr               error
	mu                     sync.Mutex
	cleanupErr             error
	mcpState               *stdioMCPWorkerState
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

func NewRealProviderWorkerAdapter(runtime *kernel.Kernel, providerRuntime provider.Runtime, workerCgroupManagers ...environment.LinuxWorkerCgroupManager) (*RealProviderWorkerAdapter, error) {
	if runtime == nil || providerRuntime == nil {
		return nil, errors.New("real provider worker adapter dependencies are incomplete")
	}
	if len(workerCgroupManagers) > 1 || (len(workerCgroupManagers) == 1 && workerCgroupManagers[0] == nil) {
		return nil, errors.New("Linux Worker cgroup manager configuration is invalid")
	}
	adapter := &RealProviderWorkerAdapter{runtime: providerRuntime, kernel: runtime, workers: make(map[string]*providerWorker), unresolvedWorkers: make(map[string]*providerWorker), localCleanupSessions: make(map[string]provider.Session), localHostReconcile: make(map[string]bool)}
	if len(workerCgroupManagers) == 1 {
		adapter.workerCgroupManager = workerCgroupManagers[0]
	}
	return adapter, nil
}

// NewRealProviderWorkerAdapterWithMCPAppContainer enables the separately
// versioned controlled-MCP path with a concrete deny-all AppContainer. Close
// stops WorkerSession owners first and closes the sandbox only after cleanup.
func NewRealProviderWorkerAdapterWithMCPAppContainer(runtime *kernel.Kernel, providerRuntime provider.Runtime, sandbox *runner.AppContainerSandbox, workerCgroupManagers ...environment.LinuxWorkerCgroupManager) (*RealProviderWorkerAdapter, error) {
	if sandbox == nil {
		return nil, errors.New("controlled MCP requires a concrete AppContainer sandbox")
	}
	adapter, err := NewRealProviderWorkerAdapter(runtime, providerRuntime, workerCgroupManagers...)
	if err != nil {
		return nil, err
	}
	adapter.mcpFactory = &appContainerControlledMCPFactory{sandbox: sandbox}
	adapter.mcpOwnerLease = newStdioMCPProcessLease()
	return adapter, nil
}

// ControlledMCPSandbox returns the adapter's deny-all AppContainer when the
// separate fake-only MCP surface is configured, so Workbench runtime
// observation can share its package root during this service lifetime.
func (a *RealProviderWorkerAdapter) ControlledMCPSandbox() *runner.AppContainerSandbox {
	if a == nil {
		return nil
	}
	factory, ok := a.mcpFactory.(*appContainerControlledMCPFactory)
	if !ok {
		return nil
	}
	return factory.sandbox
}

// NewRealProviderWorkerLaunchQualificationAdapter keeps the exact real
// adapter/runtime boundary available for local no-turn process qualification.
// It intentionally has no kernel and cannot start a business WorkerSession.
func NewRealProviderWorkerLaunchQualificationAdapter(providerRuntime provider.Runtime) (*RealProviderWorkerAdapter, error) {
	if providerRuntime == nil {
		return nil, errors.New("real provider launch qualification runtime is incomplete")
	}
	return &RealProviderWorkerAdapter{runtime: providerRuntime, workers: make(map[string]*providerWorker), unresolvedWorkers: make(map[string]*providerWorker), localCleanupSessions: make(map[string]provider.Session), localHostReconcile: make(map[string]bool)}, nil
}

func (a *RealProviderWorkerAdapter) Mode() string { return "real" }
func (a *RealProviderWorkerAdapter) ToolSurface() provider.ToolSurface {
	return a.runtime.ToolSurface()
}
func (a *RealProviderWorkerAdapter) Readiness(ctx context.Context) error {
	if err := a.runtime.Readiness(ctx); err != nil {
		return fmt.Errorf("real provider runtime unavailable: %w", err)
	}
	profile := a.runtime.ExecutionProfile()
	if profile.ToolSurfaceQualification == provider.ProductWorkspaceTreeToolSurfaceQualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeWorkspaceTreeSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductWorkspaceSnapshotRevocationToolSurfaceQualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeWorkspaceSnapshotRevocationSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductSkillToolSurfaceQualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeSkillSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductDirectMessagingToolSurfaceQualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeProductDirectMessagingSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductSharedMissionArtifactToolSurfaceQualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeSharedMissionArtifactSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductControlledMCPToolSurfaceQualification && a.runtime.Mode() == "fake" {
		if a.mcpFactory == nil {
			return errors.New("controlled MCP AppContainer is not configured")
		}
		return provider.ValidateOfflineFakeControlledMCPSurface(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	if profile.ToolSurfaceQualification == provider.ProductControlledMCPToolSurfaceV2Qualification && a.runtime.Mode() == "fake" {
		return provider.ValidateOfflineFakeControlledMCPSurfaceV2(a.runtime.Mode(), profile, a.runtime.ToolSurface())
	}
	surface := a.runtime.ToolSurface()
	if surface.ToolCount != 7 || surface.ManifestDigest != provider.ProductToolSurfaceV4ManifestDigest || surface.AggregateSchemaBytes != provider.ProductToolSurfaceV4AggregateSchemaBytes || surface.AggregateSchemaDigest != provider.ProductToolSurfaceV4AggregateSchemaDigest {
		return errors.New("real provider product tool surface is not the current @4 surface; live qualification is required")
	}
	if profile.Model == "" || profile.Effort != "medium" || (profile.ToolCallLimit <= 0 && profile.Purpose != provider.ProductLocalProcessLaunchQualificationPurpose) || profile.Purpose == "" || profile.ExecutionEnvelope == "" || profile.ToolSurfaceQualification != provider.ProductToolSurfaceQualification || profile.ExactSurfaceExecutionFingerprint != provider.ProductExactSurfaceExecutionFingerprint || profile.ProductProviderL2Fingerprint != provider.ProductProviderL2Fingerprint {
		return errors.New("real provider execution profile is unavailable")
	}
	if profile.Purpose == provider.Live2AuthorizationPurpose && (profile.Model != "gpt-5.6-luna" || profile.Effort != "medium" || profile.ToolCallLimit != 16 || profile.ExecutionEnvelope != provider.ProductProviderRuntimeEnvelopeFingerprintV2) {
		return errors.New("real provider LIVE_2 profile differs from the qualified B4 execution")
	}
	requiresLinuxCgroup, declaredLinuxCgroupRequirement := a.runtime.(provider.LinuxWorkerCgroupRequirement)
	if runtime.GOOS == "linux" && a.runtime.Mode() == "real" && (!declaredLinuxCgroupRequirement || !requiresLinuxCgroup.RequiresLinuxWorkerCgroup()) {
		return errors.New("Linux real Worker runtime must declare the cgroup-contained native sandbox profile")
	}
	if runtime.GOOS == "linux" && declaredLinuxCgroupRequirement && requiresLinuxCgroup.RequiresLinuxWorkerCgroup() && a.workerCgroupManager == nil {
		return errors.New("Linux real Worker runtime requires the delegated cgroup v2 manager")
	}
	if runtime.GOOS == "linux" && declaredLinuxCgroupRequirement && requiresLinuxCgroup.RequiresLinuxWorkerCgroup() {
		if err := a.workerCgroupManager.WorkerContainmentReady(); err != nil {
			return fmt.Errorf("Linux real Worker containment is unavailable: %w", err)
		}
	}
	return nil
}

// AutomaticProductDispatchReadiness is a strict opt-in gate for the
// unqualified, zero-egress Fake @7 simulation. Automatic admission is not
// available to the real provider, other fake surfaces, or launch-only adapters.
func (a *RealProviderWorkerAdapter) AutomaticProductDispatchReadiness(ctx context.Context) error {
	if a == nil || a.kernel == nil || a.runtime == nil {
		return errors.New("automatic product dispatch requires a business Worker adapter")
	}
	if err := a.runtime.Readiness(ctx); err != nil {
		return fmt.Errorf("automatic product dispatch runtime is unavailable: %w", err)
	}
	if a.runtime.Mode() != "fake" {
		return errors.New("automatic product dispatch requires the zero-egress Fake @7 runtime")
	}
	return provider.ValidateOfflineFakeProductDirectMessagingSurface(a.runtime.Mode(), a.runtime.ExecutionProfile(), a.runtime.ToolSurface())
}

func (a *RealProviderWorkerAdapter) bindWorkerContainment(ctx context.Context, binding kernel.Binding) (runner.WorkerProcessCgroup, error) {
	requiresLinuxCgroup, declaredLinuxCgroupRequirement := a.runtime.(provider.LinuxWorkerCgroupRequirement)
	requiresCgroup := runtime.GOOS == "linux" && a.runtime.Mode() == "real"
	if requiresCgroup && (!declaredLinuxCgroupRequirement || !requiresLinuxCgroup.RequiresLinuxWorkerCgroup()) {
		return nil, errors.New("Linux real Worker runtime must declare the cgroup-contained native sandbox profile")
	}
	if !requiresCgroup || a.workerCgroupManager == nil {
		if requiresCgroup && declaredLinuxCgroupRequirement && requiresLinuxCgroup.RequiresLinuxWorkerCgroup() {
			return nil, errors.New("Linux real Worker runtime requires the delegated cgroup v2 manager")
		}
		if err := a.kernel.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
			return nil, err
		}
		return nil, nil
	}
	cgroup, err := a.workerCgroupManager.CreateWorker(binding.SessionID())
	if err != nil {
		return nil, err
	}
	metadata := runner.ProcessContainmentMetadata{
		HostOS: "linux", Profile: "linux_worker_cgroup_v2@1", CgroupID: cgroup.Name(),
		CgroupHostID: cgroup.HostIdentity(), CgroupRootID: cgroup.RootIdentity(), CgroupBootID: cgroup.BootID(),
	}
	if !metadata.Valid() {
		cleanupErr := cgroup.Cleanup()
		return nil, errors.Join(errors.New("Linux Worker cgroup metadata is invalid"), cleanupErr)
	}
	if err = a.kernel.TXBindWorkerProcessHost(ctx, binding, metadata); err != nil {
		return nil, errors.Join(err, cgroup.Cleanup())
	}
	return cgroup, nil
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
	if !core.ValidID(sessionID) {
		return core.Malformed
	}
	a.mu.Lock()
	_, cleanupPending := a.localCleanupSessions[sessionID]
	cleanupPending = cleanupPending || a.localHostReconcile[sessionID]
	a.mu.Unlock()
	if cleanupPending {
		return runner.ErrProcessTreeStopUnconfirmed
	}
	if err := a.Readiness(ctx); err != nil {
		return err
	}
	local, ok := a.runtime.(localQualificationRuntime)
	if !ok {
		return errors.New("provider runtime does not expose the zero-turn local qualification boundary")
	}
	profile := a.runtime.ExecutionProfile()
	var workerCgroup runner.WorkerProcessCgroup
	if runtime.GOOS == "linux" && a.workerCgroupManager != nil {
		group, createErr := a.workerCgroupManager.CreateWorker(sessionID)
		if createErr != nil {
			return createErr
		}
		workerCgroup = group
	}
	session, err := local.StartLocalQualification(ctx, provider.SessionStartOptions{SessionID: sessionID, Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, ToolSurface: a.runtime.ToolSurface(), WorkerCgroup: workerCgroup})
	if err != nil {
		if workerCgroup != nil {
			proof, reconcileErr := a.workerCgroupManager.ReconcileWorker(sessionID)
			if reconcileErr == nil && proof.ForWorkerSession(sessionID, workerCgroup.Name()) {
				return err
			}
			if reconcileErr == nil {
				reconcileErr = runner.ErrProcessTreeStopUnconfirmed
			}
			a.mu.Lock()
			a.localHostReconcile[sessionID] = true
			a.mu.Unlock()
			return errors.Join(err, fmt.Errorf("local qualification cgroup cleanup is unresolved: %w", reconcileErr))
		}
		if launchFailureHasCreatedProcess(err) {
			proof, reconcileErr := runner.ReconcileWindowsWorkerProcessTree(ctx, sessionID, 0)
			if reconcileErr == nil && proof.ForWorkerSession(sessionID, 0) {
				return err
			}
			if reconcileErr == nil {
				reconcileErr = runner.ErrProcessTreeStopUnconfirmed
			}
			a.mu.Lock()
			a.localHostReconcile[sessionID] = true
			a.mu.Unlock()
			return errors.Join(err, fmt.Errorf("local qualification host process cleanup is unresolved: %w", reconcileErr))
		}
		return err
	}
	if session.Process() == nil {
		if workerCgroup != nil {
			proof, cleanupErr := a.workerCgroupManager.ReconcileWorker(sessionID)
			if cleanupErr != nil || !proof.ForWorkerSession(sessionID, workerCgroup.Name()) {
				return errors.Join(errors.New("local qualification session has no process handle"), cleanupErr, runner.ErrProcessTreeStopUnconfirmed)
			}
		}
		return a.stopLocalQualificationAfterFailure(ctx, sessionID, session, errors.New("local qualification session has no process handle"))
	}
	if err = session.Initialize(ctx, profile.TransportPolicy); err != nil {
		return a.stopLocalQualificationAfterFailure(ctx, sessionID, session, err)
	}
	thread, err := session.StartThread(ctx, provider.ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: a.runtime.ToolSurface().Tools})
	if err != nil {
		return a.stopLocalQualificationAfterFailure(ctx, sessionID, session, err)
	}
	if thread == "" {
		return a.stopLocalQualificationAfterFailure(ctx, sessionID, session, errors.New("local qualification thread/start returned empty thread ID"))
	}
	return a.stopLocalQualificationSession(ctx, sessionID, session)
}

func (a *RealProviderWorkerAdapter) stopLocalQualificationAfterFailure(ctx context.Context, sessionID string, session provider.Session, cause error) error {
	if cleanupErr := a.stopLocalQualificationSession(ctx, sessionID, session); cleanupErr != nil {
		return errors.Join(cause, fmt.Errorf("local qualification cleanup is unresolved: %w", cleanupErr))
	}
	return cause
}

func (a *RealProviderWorkerAdapter) stopLocalQualificationSession(ctx context.Context, sessionID string, session provider.Session) error {
	if session == nil {
		return errors.New("local qualification session is unavailable for cleanup")
	}
	proof, err := session.Stop(ctx)
	if err == nil && !proof.For(sessionID) {
		err = runner.ErrProcessTreeStopUnconfirmed
	}
	if err != nil {
		a.mu.Lock()
		a.localCleanupSessions[sessionID] = session
		a.mu.Unlock()
		return err
	}
	a.mu.Lock()
	if a.localCleanupSessions[sessionID] == session {
		delete(a.localCleanupSessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

// RetryLocalLaunchCleanup retries cleanup for a failed zero-turn local launch.
func (a *RealProviderWorkerAdapter) RetryLocalLaunchCleanup(ctx context.Context, sessionID string) error {
	if !core.ValidID(sessionID) {
		return core.Malformed
	}
	a.mu.Lock()
	session := a.localCleanupSessions[sessionID]
	hostReconcile := a.localHostReconcile[sessionID]
	a.mu.Unlock()
	if hostReconcile {
		proof, err := runner.ReconcileWindowsWorkerProcessTree(ctx, sessionID, 0)
		if err != nil {
			return err
		}
		if !proof.ForWorkerSession(sessionID, 0) {
			return runner.ErrProcessTreeStopUnconfirmed
		}
		a.mu.Lock()
		delete(a.localHostReconcile, sessionID)
		a.mu.Unlock()
		return nil
	}
	if session == nil {
		return nil
	}
	return a.stopLocalQualificationSession(ctx, sessionID, session)
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
	finalizeBeforeProcess := func(cause error, reason, requestID string) error {
		cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, reason, requestID, "")
		if cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("business preflight WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return cause
	}
	reconcileUnknownProcess := func(cause error) error {
		worker := newStartupProviderWorker(companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, nil, true, "", a.mcpOwnerLease)
		ownerKey := a.retainUnresolvedWorker(worker)
		if cleanupErr := a.cleanup(context.Background(), ownerKey, worker); cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("business preflight host process cleanup is unresolved: %w", cleanupErr))
		}
		return cause
	}
	stopUnactivated := func(cause error, session provider.Session) error {
		if cleanupErr := a.stopUnactivatedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, session, "business_preflight_cleanup"); cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("business preflight WorkerSession cleanup is unresolved: %w", cleanupErr))
		}
		return cause
	}
	if !result.BusinessBindingPassed {
		cause := errors.New("business preflight binding does not match Task")
		return result, finalizeBeforeProcess(cause, "business preflight binding mismatch", "business-preflight-binding-failed-"+binding.SessionID())
	}
	workerCgroup, err := a.bindWorkerContainment(ctx, binding)
	if err != nil {
		cause := fmt.Errorf("business preflight process containment binding failed: %w", err)
		return result, finalizeBeforeProcess(cause, "business preflight process containment could not be bound", "business-preflight-containment-failed-"+binding.SessionID())
	}
	session, err := local.StartLocalQualification(ctx, provider.SessionStartOptions{SessionID: binding.SessionID(), Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, MissionID: missionID, TaskID: task.ID, ToolSurface: a.runtime.ToolSurface(), WorkerCgroup: workerCgroup})
	if err != nil {
		if workerCgroup != nil {
			err = errors.Join(err, workerCgroup.Cleanup())
		}
		if launchFailureHasCreatedProcess(err) {
			return result, reconcileUnknownProcess(err)
		}
		return result, finalizeBeforeProcess(err, err.Error(), "business-preflight-process-start-failed-"+binding.SessionID())
	}
	if session.Process() == nil {
		noProcessErr := errors.New("business preflight session has no process")
		if workerCgroup != nil {
			noProcessErr = errors.Join(noProcessErr, workerCgroup.Cleanup())
		}
		return result, reconcileUnknownProcess(noProcessErr)
	}
	result.ProcessStarted = true
	result.ProcessPID = session.Process().PID()
	if err = a.kernel.TXAttachWorker(ctx, binding, session.Process()); err != nil {
		return result, stopUnactivated(fmt.Errorf("business preflight process attach failed: %w", err), session)
	}
	if err = a.kernel.TXValidateWorker(ctx, binding); err != nil {
		return result, stopUnactivated(fmt.Errorf("business preflight WorkerSession validation failed: %w", err), session)
	}
	if err = a.kernel.TXActivateWorker(ctx, binding, profile.Profile); err != nil {
		return result, stopUnactivated(fmt.Errorf("business preflight WorkerSession activation failed: %w", err), session)
	}
	if err = session.Initialize(ctx, profile.TransportPolicy); err != nil {
		result.Initialization = initializationEvidence(session, err)
		_, _ = a.kernel.TXRecordProviderInitializationFailure(context.Background(), binding, result.Initialization, "business-preflight-initialize-failed-"+binding.SessionID())
		return result, stopUnactivated(err, session)
	}
	result.InitializePassed = true
	result.Initialization = initializationEvidence(session, nil)
	thread, err := session.StartThread(ctx, provider.ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: a.runtime.ToolSurface().Tools})
	if err != nil || thread == "" {
		if err == nil {
			err = errors.New("business preflight thread/start returned empty thread ID")
		}
		return result, stopUnactivated(err, session)
	}
	result.ThreadStarted = true
	result.RegisteredToolCount = a.runtime.ToolSurface().ToolCount
	result.RegisteredToolDigest = a.runtime.ToolSurface().ManifestDigest
	if cleanupErr := a.stopUnactivatedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, session, "business_preflight_complete"); cleanupErr != nil {
		return result, fmt.Errorf("business preflight stop is unresolved: %w", cleanupErr)
	}
	result.CleanStop = true
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
	if _, manifestErr := a.kernel.TaskInputManifest(ctx, scope, task.ID); errors.Is(manifestErr, core.OutOfScope) {
		if err = a.kernel.EnsureTaskInputManifest(ctx, scope, missionID, task.ID, "task-input-bind-"+task.ID); err != nil {
			return nil, fmt.Errorf("product Task input manifest binding failed: %w", err)
		}
	} else if manifestErr != nil {
		return nil, fmt.Errorf("product Task input manifest verification failed: %w", manifestErr)
	}
	profile := a.runtime.ExecutionProfile()
	binding, err := a.kernel.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, profile.Profile, int64(profile.ToolCallLimit))
	if err != nil {
		return nil, err
	}
	identitySnapshot, err := provider.CaptureProviderAuthIdentitySnapshot(ctx, a.runtime)
	if err != nil {
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, "provider auth identity snapshot could not be captured", "provider-auth-snapshot-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, fmt.Errorf("provider auth identity snapshot could not be captured: %w", err)
	}
	if _, err = a.kernel.TXRecordWorkerProviderAuthIdentity(ctx, binding, kernel.WorkerProviderAuthIdentitySnapshot{
		SchemaVersion: identitySnapshot.SchemaVersion, SourceClass: identitySnapshot.SourceClass,
		Status: identitySnapshot.Status, Fingerprint: identitySnapshot.Fingerprint, ReasonCode: identitySnapshot.ReasonCode,
	}); err != nil {
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, "provider auth identity snapshot could not be persisted", "provider-auth-snapshot-persist-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, fmt.Errorf("provider auth identity snapshot could not be persisted: %w", err)
	}
	accountIdentitySnapshot, err := provider.CaptureProviderAccountIdentitySnapshot(ctx, a.runtime)
	if err != nil {
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, "provider account identity snapshot could not be captured", "provider-account-identity-snapshot-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, fmt.Errorf("provider account identity snapshot could not be captured: %w", err)
	}
	if _, err = a.kernel.TXRecordWorkerProviderAccountIdentity(ctx, binding, kernel.WorkerProviderAccountIdentitySnapshot{
		SchemaVersion: accountIdentitySnapshot.SchemaVersion, ProviderClass: accountIdentitySnapshot.ProviderClass,
		Status: accountIdentitySnapshot.Status, Fingerprint: accountIdentitySnapshot.Fingerprint, ReasonCode: accountIdentitySnapshot.ReasonCode,
	}); err != nil {
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, "provider account identity snapshot could not be persisted", "provider-account-identity-snapshot-persist-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, fmt.Errorf("provider account identity snapshot could not be persisted: %w", err)
	}
	inputContext, err := a.kernel.ProductTaskInputContext(ctx, binding)
	if err != nil {
		inputErr := fmt.Errorf("product Task input context failed integrity checks: %w", err)
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, kernel.ProductTaskInputContext{}, provider.Reservation{}, "Task input context could not be verified", "task-input-context-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(inputErr, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, inputErr
	}
	taskToolBudget, err := a.kernel.WorkerToolCallBudget(ctx, binding)
	if err != nil {
		budgetErr := fmt.Errorf("product Task tool-call budget could not be verified: %w", err)
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, inputContext, provider.Reservation{}, "Task tool-call budget could not be verified", "task-tool-budget-failed-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(budgetErr, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, budgetErr
	}
	turnToolCallLimit := profile.ToolCallLimit
	if taskToolBudget.Remaining >= 0 && (turnToolCallLimit <= 0 || int64(turnToolCallLimit) > taskToolBudget.Remaining) {
		turnToolCallLimit = int(taskToolBudget.Remaining)
	}
	if allowTurn && turnToolCallLimit < 1 {
		budgetErr := errors.New("product Task has no remaining tool-call budget")
		if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, inputContext, provider.Reservation{}, budgetErr.Error(), "task-tool-budget-exhausted-"+binding.SessionID(), ""); cleanupErr != nil {
			return nil, errors.Join(budgetErr, fmt.Errorf("WorkerSession finalization is unresolved: %w", cleanupErr))
		}
		return nil, budgetErr
	}
	var reservation provider.Reservation
	var workerCgroup runner.WorkerProcessCgroup
	failStart := func(startErr error, session provider.Session) error {
		if session == nil {
			if workerCgroup != nil {
				if cleanupErr := workerCgroup.Cleanup(); cleanupErr != nil {
					return errors.Join(startErr, fmt.Errorf("WorkerSession cgroup cleanup is unresolved: %w", cleanupErr))
				}
				workerCgroup = nil
			}
			if launchFailureHasCreatedProcess(startErr) {
				worker := newStartupProviderWorker(companyID, missionID, binding, inputContext, reservation, nil, true, "start_process_reconciliation_required", a.mcpOwnerLease)
				ownerKey := a.retainUnresolvedWorker(worker)
				if reconcileErr := a.cleanup(context.Background(), ownerKey, worker); reconcileErr != nil {
					return errors.Join(startErr, fmt.Errorf("provider host process cleanup is unresolved: %w", reconcileErr))
				}
				return startErr
			}
			if cleanupErr := a.finalizeUnstartedOwned(context.Background(), companyID, missionID, binding, inputContext, reservation, startErr.Error(), "worker-start-failed-"+binding.SessionID(), "start_failed"); cleanupErr != nil {
				return fmt.Errorf("%v; worker startup cleanup is unresolved: %w", startErr, cleanupErr)
			}
			return startErr
		}
		if cleanupErr := a.stopUnactivatedOwned(context.Background(), companyID, missionID, binding, inputContext, reservation, session, "start_failed"); cleanupErr != nil {
			return fmt.Errorf("%v; worker cleanup failed: %w", startErr, cleanupErr)
		}
		return startErr
	}
	workerCgroup, err = a.bindWorkerContainment(ctx, binding)
	if err != nil {
		return nil, failStart(fmt.Errorf("provider process containment binding failed: %w", err), nil)
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
	if err = provider.ValidateRuntimeExecutionAuthorization(authorization); err != nil {
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
	session, err := a.runtime.Start(ctx, provider.SessionStartOptions{SessionID: binding.SessionID(), Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile, MissionID: missionID, TaskID: task.ID, ToolSurface: a.runtime.ToolSurface(), WorkerCgroup: workerCgroup})
	if err != nil {
		if workerCgroup != nil {
			err = errors.Join(err, workerCgroup.Cleanup())
		}
		_ = a.recordLifecycleFailure(binding, lifecycleFailureCategory(err, "process_handle_lost"), "provider process was not attached")
		return nil, failStart(err, nil)
	}
	if session.Process() == nil {
		_ = a.recordLifecycleFailure(binding, "process_handle_lost", "provider session returned no process handle")
		noProcessErr := errors.New("real provider session has no process handle")
		if workerCgroup != nil {
			noProcessErr = errors.Join(noProcessErr, workerCgroup.Cleanup())
		}
		return nil, failStart(noProcessErr, nil)
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
	worker := newStartupProviderWorker(companyID, missionID, binding, inputContext, reservation, session, false, "worker_cleanup", a.mcpOwnerLease)
	worker.mcpState.allowStreamableHTTP = profile.ToolSurfaceQualification == provider.ProductControlledMCPToolSurfaceV2Qualification && os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") == "1"
	worker.turnToolCallLimit = turnToolCallLimit
	worker.taskToolBudget = taskToolBudget
	worker.problemKey = task.ProblemKey
	worker.authorization = authorization
	worker.cancel = cancel
	a.mu.Lock()
	if _, exists := a.workers[key]; exists {
		a.mu.Unlock()
		_ = a.recordLifecycleFailure(binding, "duplicate_stop", "duplicate worker ownership was detected before initialize")
		cancel()
		if cleanupErr := a.stopUnactivatedOwned(context.Background(), companyID, missionID, binding, inputContext, reservation, session, "duplicate_stop"); cleanupErr != nil {
			return nil, fmt.Errorf("duplicate provider worker cleanup is unresolved: %w", cleanupErr)
		}
		return nil, nil
	}
	a.workers[key] = worker
	a.mu.Unlock()
	go func() {
		defer close(worker.runDone)
		a.run(runCtx, key, worker, profile, allowTurn)
	}()
	return worker, nil
}

func launchFailureHasCreatedProcess(err error) bool {
	var failure *runner.LaunchFailure
	return errors.As(err, &failure) && failure.ProcessCreated
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
	<-worker.runDone
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
	if !allowTurn {
		worker.signalReady(nil)
		_ = a.cleanup(context.Background(), key, worker)
		return
	}
	deliveryID, verifiedInputPrompt, deliveryErr := a.kernel.TXPrepareProductTaskInputDelivery(ctx, worker.binding, worker.inputContext)
	if deliveryErr != nil {
		worker.signalReady(deliveryErr)
		_, _ = a.kernel.TXCreateHumanIntervention(ctx, a.kernel.LocalScope(worker.authorization.CompanyID), kernel.HumanInterventionInput{
			MissionID: worker.authorization.MissionID, TaskID: worker.binding.TaskID(),
			ProblemKey: "input-delivery-" + worker.binding.TaskID(), Severity: "high", ReasonCode: "environment_not_ready",
			AffectedScope: "task", ProtectionAction: "execution_paused", RequiredAction: "supply_missing_input",
		}, "task-input-delivery-preparation-"+worker.binding.SessionID())
		_ = a.cleanup(context.Background(), key, worker)
		return
	}
	worker.signalReady(nil)
	prompt := "Complete the assigned product task using only the registered Polis tools. Persist the requested artifact and checkpoint, then stop." + verifiedInputPrompt
	turnOptions := codexTurnOptions(profile)
	if worker.turnToolCallLimit > 0 && (turnOptions.ToolCallLimit <= 0 || worker.turnToolCallLimit < turnOptions.ToolCallLimit) {
		turnOptions.ToolCallLimit = worker.turnToolCallLimit
	}
	turnOptions.Images = make([]codex.TurnImage, len(worker.inputContext.Payload.Images))
	for index, image := range worker.inputContext.Payload.Images {
		turnOptions.Images[index] = codex.TurnImage{MediaType: image.MediaType, Content: append([]byte(nil), image.Content...)}
	}
	controlledMCPV1Surface := profile.ToolSurfaceQualification == provider.ProductControlledMCPToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductControlledMCPToolSurface().ManifestDigest
	controlledMCPV2Surface := profile.ToolSurfaceQualification == provider.ProductControlledMCPToolSurfaceV2Qualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductControlledMCPToolSurfaceV2().ManifestDigest
	controlledMCPSurface := controlledMCPV1Surface || controlledMCPV2Surface
	workspaceSnapshotRevocationSurface := profile.ToolSurfaceQualification == provider.ProductWorkspaceSnapshotRevocationToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductWorkspaceSnapshotRevocationToolSurface().ManifestDigest
	workspaceTreeSurface := (profile.ToolSurfaceQualification == provider.ProductWorkspaceTreeToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductWorkspaceTreeToolSurface().ManifestDigest) || workspaceSnapshotRevocationSurface
	directMessagingSurface := (profile.ToolSurfaceQualification == provider.ProductDirectMessagingToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductDirectMessagingToolSurface().ManifestDigest) || workspaceTreeSurface
	sharedMissionArtifactSurface := (profile.ToolSurfaceQualification == provider.ProductSharedMissionArtifactToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductSharedMissionArtifactToolSurface().ManifestDigest) || workspaceTreeSurface
	if sharedMissionArtifactSurface {
		directMessagingSurface = true
	}
	turn, turnErr := worker.session.Turn(ctx, thread, prompt, turnOptions, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		if controlledMCPSurface && name == "mcp_call" {
			encoded, callErr := worker.mcpState.call(ctx, a.kernel, a.mcpFactory, worker.companyID, worker.binding, callID, raw)
			if callErr == nil {
				return encoded, false
			}
			if worker.mcpState.shouldStopAfterFailure() {
				worker.cancel()
			}
			toolError, _ := json.Marshal(kernel.ToolResult{Error: "CONTROLLED_MCP_CALL_FAILED", Detail: "the approved call was denied or its result is unresolved; stop this WorkerSession"})
			return toolError, false
		}
		skillLoadSurface := (profile.ToolSurfaceQualification == provider.ProductSkillToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductSkillToolSurface().ManifestDigest) ||
			(profile.ToolSurfaceQualification == provider.ProductSkillDirectoryToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductSkillDirectoryToolSurface().ManifestDigest)
		skillDirectorySurface := profile.ToolSurfaceQualification == provider.ProductSkillDirectoryToolSurfaceQualification && a.runtime.ToolSurface().ManifestDigest == provider.ProductSkillDirectoryToolSurface().ManifestDigest
		tools := kernel.EmployeeTools{Kernel: a.kernel, Binding: worker.binding, ProductSurface: true, SkillLoadSurface: skillLoadSurface, SkillDirectorySurface: skillDirectorySurface, DirectMessagingSurface: directMessagingSurface, SharedArtifactSurface: sharedMissionArtifactSurface, WorkspaceTreeSurface: workspaceTreeSurface, WorkspaceSnapshotRevocationSurface: workspaceSnapshotRevocationSurface, ControlledMCPSurface: controlledMCPSurface, ControlledStdioMCPEnabled: controlledMCPV1Surface || (controlledMCPV2Surface && a.mcpFactory != nil), StreamableHTTPMCPEnabled: controlledMCPV2Surface && os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") == "1"}
		result := tools.Call(ctx, name, callID, raw)
		encoded, _ := json.Marshal(result)
		return encoded, false
	})
	if deliveryID != "" {
		deliveryOutcome := "not_sent"
		if turn.ProviderEgress > 0 {
			if turnErr != nil {
				deliveryOutcome = "outcome_unknown"
			} else {
				deliveryOutcome = "provider_delivered"
			}
		} else if turnErr == nil && a.runtime.Mode() == "fake" {
			deliveryOutcome = "local_context_loaded"
		}
		if receiptErr := a.kernel.TXCompleteProductTaskInputDelivery(context.Background(), worker.binding, deliveryID, deliveryOutcome, turn.ProviderEgress); receiptErr != nil && turnErr == nil {
			turnErr = fmt.Errorf("provider turn completed but Task input delivery evidence could not be saved: %w", receiptErr)
		}
	}
	usageRecord := map[string]any{"authorization": worker.authorization, "problem_key": worker.problemKey, "task_tool_call_budget_at_start": worker.taskToolBudget, "effective_turn_tool_call_limit": turnOptions.ToolCallLimit, "token_usage": turn.Usage, "tool_calls": turn.ToolCalls, "provider_egress": turn.ProviderEgress, "reconnect_attempt_count": turn.ReconnectAttemptCount, "reconnect_recovered": turn.ReconnectRecovered, "retry_visibility": turn.RetryVisibility, "started_at": turn.StartedAt, "finished_at": turn.FinishedAt}
	if turn.RetryObservationScope != "" {
		usageRecord["retry_observation_scope"] = turn.RetryObservationScope
		usageRecord["unobserved_provider_retry_count"] = nil
	}
	usage = usageRecord
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
	owners := make([]struct {
		key    string
		worker *providerWorker
	}, 0)
	for ownerKey, candidate := range a.unresolvedWorkers {
		if candidate.companyID == companyID && candidate.missionID == missionID {
			owners = append(owners, struct {
				key    string
				worker *providerWorker
			}{ownerKey, candidate})
		}
	}
	a.mu.Unlock()
	if worker == nil && len(owners) == 0 {
		return nil
	}
	var failures []error
	if worker != nil {
		worker.cancel()
		if err := a.cleanup(ctx, key, worker); err != nil {
			failures = append(failures, err)
		}
	}
	for _, owner := range owners {
		owner.worker.cancel()
		if err := a.cleanup(ctx, owner.key, owner.worker); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (a *RealProviderWorkerAdapter) StopSession(ctx context.Context, companyID, sessionID string) error {
	var ownerKey string
	var worker *providerWorker
	a.mu.Lock()
	for key, candidate := range a.workers {
		if candidate.companyID == companyID && candidate.binding.SessionID() == sessionID {
			ownerKey, worker = key, candidate
			break
		}
	}
	if worker == nil {
		for key, candidate := range a.unresolvedWorkers {
			if candidate.companyID == companyID && candidate.binding.SessionID() == sessionID {
				ownerKey, worker = key, candidate
				break
			}
		}
	}
	a.mu.Unlock()
	if worker == nil {
		if a.kernel == nil {
			return errors.New("WorkerSession host reconciliation requires a business kernel")
		}
		return a.kernel.ReconcileWorkerSession(ctx, companyID, sessionID, a.workerCgroupManager)
	}
	worker.cancel()
	return a.cleanup(ctx, ownerKey, worker)
}

func (a *RealProviderWorkerAdapter) cleanup(ctx context.Context, key string, worker *providerWorker) error {
	worker.cleanupMu.Lock()
	defer worker.cleanupMu.Unlock()
	select {
	case <-worker.done:
		return nil
	default:
	}
	if worker.cancel != nil {
		worker.cancel()
	}
	worker.mcpState.quiesce()
	worker.mu.Lock()
	cleanupBegun := worker.cleanupBegun
	processStopped := worker.processStopped
	stopConfirmed := worker.stopConfirmed
	reservationClosed := worker.reservationClosed
	proof := worker.stopProof
	worker.mu.Unlock()
	var err error
	if !stopConfirmed {
		if worker.noProcessFailure {
			if err = a.kernel.TXFinalizeWorkerBeforeProcess(ctx, worker.binding, worker.startFailureReason, worker.startFailureRequestID); err != nil {
				return a.recordCleanupError(worker, err)
			}
			worker.mu.Lock()
			worker.stopConfirmed = true
			worker.mu.Unlock()
		} else if worker.hostReconcileOnly {
			if err = a.kernel.ReconcileWorkerSession(ctx, worker.companyID, worker.binding.SessionID()); err != nil {
				return a.recordCleanupError(worker, err)
			}
			worker.mu.Lock()
			worker.stopConfirmed = true
			worker.mu.Unlock()
		} else if processStopped {
			if err = a.kernel.TXConfirmStopped(ctx, worker.binding, proof); err != nil {
				return a.recordCleanupError(worker, err)
			}
			worker.mu.Lock()
			worker.stopConfirmed = true
			worker.mu.Unlock()
		} else {
			if worker.session != nil && worker.session.Process() != nil {
				if err = a.kernel.TXAttachWorker(ctx, worker.binding, worker.session.Process()); err != nil {
					return a.recordCleanupError(worker, err)
				}
			}
			if !cleanupBegun {
				if err = a.kernel.TXBeginStop(ctx, worker.binding); err != nil {
					return a.recordCleanupError(worker, err)
				}
				worker.mu.Lock()
				worker.cleanupBegun = true
				worker.mu.Unlock()
			}
			if !processStopped {
				proof, err = worker.session.Stop(ctx)
				if err != nil {
					return a.recordCleanupError(worker, err)
				}
				worker.mu.Lock()
				worker.processStopped = true
				worker.stopProof = proof
				worker.mu.Unlock()
			}
			if err = a.kernel.TXConfirmStopped(ctx, worker.binding, proof); err != nil {
				return a.recordCleanupError(worker, err)
			}
			worker.mu.Lock()
			worker.stopConfirmed = true
			worker.mu.Unlock()
		}
	}
	if err = worker.mcpState.stopAfterWorkerStop(context.Background(), a.kernel, worker.companyID, worker.binding.SessionID()); err != nil {
		return a.recordCleanupError(worker, err)
	}
	if !reservationClosed {
		if worker.reservation.ID != "" {
			reason := worker.reservationCloseReason
			if reason == "" {
				reason = "worker_cleanup"
			}
			if err = a.runtime.CloseReservation(context.Background(), worker.reservation, reason); err != nil {
				return a.recordCleanupError(worker, err)
			}
		}
		worker.mu.Lock()
		worker.reservationClosed = true
		worker.mu.Unlock()
	}
	worker.mu.Lock()
	worker.cleanupErr = nil
	worker.mu.Unlock()
	close(worker.done)
	a.mu.Lock()
	if a.workers[key] == worker {
		delete(a.workers, key)
	}
	for ownerKey, owner := range a.unresolvedWorkers {
		if owner == worker {
			delete(a.unresolvedWorkers, ownerKey)
		}
	}
	a.mu.Unlock()
	return nil
}

func newStartupProviderWorker(companyID, missionID string, binding kernel.Binding, inputContext kernel.ProductTaskInputContext, reservation provider.Reservation, session provider.Session, hostReconcileOnly bool, reservationCloseReason string, ownerLease *stdioMCPProcessLease) *providerWorker {
	return &providerWorker{
		companyID: companyID, missionID: missionID, binding: binding, inputContext: inputContext,
		reservation: reservation, session: session, cancel: func() {}, done: make(chan struct{}),
		runDone: make(chan struct{}), ready: make(chan struct{}), hostReconcileOnly: hostReconcileOnly,
		reservationCloseReason: reservationCloseReason, mcpState: &stdioMCPWorkerState{ownerLease: ownerLease},
	}
}

func (a *RealProviderWorkerAdapter) retainUnresolvedWorker(worker *providerWorker) string {
	key := worker.binding.SessionID()
	a.mu.Lock()
	a.unresolvedWorkers[key] = worker
	a.mu.Unlock()
	return key
}

func (a *RealProviderWorkerAdapter) stopUnactivatedOwned(ctx context.Context, companyID, missionID string, binding kernel.Binding, inputContext kernel.ProductTaskInputContext, reservation provider.Reservation, session provider.Session, closeReason string) error {
	worker := newStartupProviderWorker(companyID, missionID, binding, inputContext, reservation, session, false, closeReason, a.mcpOwnerLease)
	ownerKey := a.retainUnresolvedWorker(worker)
	return a.cleanup(ctx, ownerKey, worker)
}

func (a *RealProviderWorkerAdapter) finalizeUnstartedOwned(ctx context.Context, companyID, missionID string, binding kernel.Binding, inputContext kernel.ProductTaskInputContext, reservation provider.Reservation, reason, requestID, closeReason string) error {
	worker := newStartupProviderWorker(companyID, missionID, binding, inputContext, reservation, nil, false, closeReason, a.mcpOwnerLease)
	worker.noProcessFailure = true
	worker.startFailureReason = reason
	worker.startFailureRequestID = requestID
	ownerKey := a.retainUnresolvedWorker(worker)
	return a.cleanup(ctx, ownerKey, worker)
}

func (a *RealProviderWorkerAdapter) recordCleanupError(worker *providerWorker, err error) error {
	worker.mu.Lock()
	worker.cleanupErr = err
	worker.mu.Unlock()
	return err
}

func (a *RealProviderWorkerAdapter) Close() {
	if err := a.QuiesceForMCPObserver(); err != nil {
		return
	}
	_ = a.CloseAfterMCPObserver()
}

// QuiesceForMCPObserver stops WorkerSessions but leaves the shared sandbox
// alive so the observer can stop its owner and remove its pinned package tree.
func (a *RealProviderWorkerAdapter) QuiesceForMCPObserver() error {
	var closeErrors []error
	a.mu.Lock()
	keys := make([]string, 0, len(a.workers))
	seen := make(map[string]struct{}, len(a.workers)+len(a.unresolvedWorkers))
	for key := range a.workers {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for _, worker := range a.unresolvedWorkers {
		key := worker.companyID + "/" + worker.missionID
		if _, exists := seen[key]; !exists {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	a.mu.Unlock()
	for _, key := range keys {
		company, mission, ok := splitProviderWorkerKey(key)
		if ok {
			if err := a.Stop(context.Background(), company, mission); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
	}
	a.mu.Lock()
	localSessionIDs := make([]string, 0, len(a.localCleanupSessions))
	for sessionID := range a.localCleanupSessions {
		localSessionIDs = append(localSessionIDs, sessionID)
	}
	for sessionID := range a.localHostReconcile {
		localSessionIDs = append(localSessionIDs, sessionID)
	}
	a.mu.Unlock()
	for _, sessionID := range localSessionIDs {
		if err := a.RetryLocalLaunchCleanup(context.Background(), sessionID); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	a.mu.Lock()
	cleanupUnresolved := len(a.workers) != 0 || len(a.unresolvedWorkers) != 0 || len(a.localCleanupSessions) != 0 || len(a.localHostReconcile) != 0
	a.mu.Unlock()
	if cleanupUnresolved {
		closeErrors = append(closeErrors, errors.New("real provider worker adapter still owns unresolved process cleanup"))
	}
	return errors.Join(closeErrors...)
}

func (a *RealProviderWorkerAdapter) CloseAfterMCPObserver() error {
	a.mu.Lock()
	cleanupUnresolved := len(a.workers) != 0 || len(a.unresolvedWorkers) != 0 || len(a.localCleanupSessions) != 0 || len(a.localHostReconcile) != 0
	factory := a.mcpFactory
	a.mu.Unlock()
	if cleanupUnresolved {
		return errors.New("cannot close controlled MCP sandbox while Worker cleanup is unresolved")
	}
	if closer, ok := factory.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
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
var _ WorkerSessionStopper = (*RealProviderWorkerAdapter)(nil)
