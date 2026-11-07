// pattern: Imperative Shell
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

const productSurfaceDiagnosticDeveloperInstructions = "This is an isolated provider diagnostic for Polis product tool registration. No business objects or WorkerSession exist. Do not call tools, shell, web, delegation, external MCP, or account tools. Reply exactly as requested in the user turn."

func BuildProductSurfaceExecutionIdentity(config CodexRuntimeConfig, providerSessionID string) (ProductSurfaceExecutionIdentity, error) {
	if config.Model == "" || config.Effort == "" || config.AuthFile == "" || config.Root == "" {
		return ProductSurfaceExecutionIdentity{}, errors.New("product execution identity configuration is incomplete")
	}
	if config.RuntimeManifestPath == "" && (config.ExpectedVersion == "" || config.Binary == "") {
		return ProductSurfaceExecutionIdentity{}, errors.New("product execution identity configuration is incomplete")
	}
	if config.RuntimeManifestPath != "" {
		bound, err := BindCodexRuntimeConfig(config)
		if err != nil {
			return ProductSurfaceExecutionIdentity{}, err
		}
		config = bound
	}
	if config.ToolSurface.ToolCount == 0 {
		config.ToolSurface = ProductToolSurface()
	}
	launch, err := describeProductNativeLaunch(config, providerSessionID)
	if err != nil {
		return ProductSurfaceExecutionIdentity{}, err
	}
	surface := config.ToolSurface
	identity := ProductSurfaceExecutionIdentity{
		SchemaVersion: ProductSurfaceExecutionIdentitySchema,
		SurfaceID:     ProductToolSurfaceQualification, ToolCount: surface.ToolCount,
		ManifestDigest: surface.ManifestDigest, AggregateSchemaBytes: surface.AggregateSchemaBytes, AggregateSchemaDigest: surface.AggregateSchemaDigest,
		Model: config.Model, Effort: config.Effort, Profile: config.Model + "/" + config.Effort, ProviderMode: "real",
		ProviderRuntime: ProductProviderRuntimeIdentity{RuntimeImplementation: ProductProviderRuntimeImplementation, NativeVersion: config.ExpectedVersion, BinarySHA256: launch.BinarySHA256, HelperSHA256: launch.HelperSHA256, ProtocolCompatibility: ProductProviderProtocolCompatibility},
		LaunchEnvelope:  launch,
		RuntimePolicy: ProductSurfaceRuntimePolicy{
			NativeConfigSHA256: digestHex([]byte(runner.NativeDefaultConfig)), NativeTransportPolicy: string(runner.NativeTransportPolicyNativeDefault),
			ApprovalPolicy: "never", SandboxPolicy: "read-only", ThreadWorkingDirectory: "/work", ThreadEphemeral: true, ModelFallbackAllowed: false,
		},
		TransportPolicy: config.TransportPolicy.Snapshot(), DiagnosticInstructionRevision: ProductSurfaceDiagnosticInstructionRevision,
	}
	if _, err = ComputeProductSurfaceExecutionFingerprint(identity); err != nil {
		return ProductSurfaceExecutionIdentity{}, err
	}
	return identity, nil
}

func (r *CodexRuntime) ReserveProductSurfaceDiagnostic(ctx context.Context, expected ProductSurfaceDiagnosticAuthorization) (Reservation, error) {
	if !r.config.DiagnosticOnly {
		return Reservation{}, errors.New("product surface diagnostic reservation requires a diagnostic-only provider runtime")
	}
	if err := r.Readiness(ctx); err != nil {
		return Reservation{}, err
	}
	if err := r.validateProductDiagnosticRuntimeBinding(expected); err != nil {
		return Reservation{}, err
	}
	r.diagnosticMu.Lock()
	defer r.diagnosticMu.Unlock()
	if r.diagnosticReservation != nil {
		return Reservation{}, errors.New("product surface diagnostic reservation already consumed; no retry")
	}
	reservation, err := ReserveProductSurfaceDiagnosticOnce(r.config.DiagnosticAuthorizationPath, r.config.DiagnosticReservationPath, expected, time.Now().UTC())
	if err != nil {
		return Reservation{}, err
	}
	r.diagnosticReservation = &reservation
	r.diagnosticAuthorization = expected
	return Reservation{ID: expected.AuthorizationID}, nil
}

func (r *CodexRuntime) validateProductDiagnosticRuntimeBinding(authorization ProductSurfaceDiagnosticAuthorization) error {
	if err := ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		return err
	}
	identity := authorization.ExecutionIdentity
	profile := r.ExecutionProfile()
	if identity.Model != profile.Model || identity.Effort != profile.Effort || identity.Profile != profile.Profile || identity.ProviderMode != r.Mode() || identity.ProviderRuntime.NativeVersion != r.config.ExpectedVersion || identity.LaunchEnvelope.Fingerprint != profile.ExecutionEnvelope || identity.SurfaceID != profile.ToolSurfaceQualification || identity.TransportPolicy != profile.TransportPolicy.Snapshot() {
		return errors.New("product surface diagnostic authorization does not match the selected provider runtime")
	}
	if identity.ManifestDigest != r.surface.ManifestDigest || identity.ToolCount != r.surface.ToolCount || identity.AggregateSchemaBytes != r.surface.AggregateSchemaBytes || identity.AggregateSchemaDigest != r.surface.AggregateSchemaDigest {
		return errors.New("product surface diagnostic authorization does not match the registered product surface")
	}
	current, err := BuildProductSurfaceExecutionIdentity(r.config, authorization.ProviderSessionID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, identity) {
		return errors.New("product surface diagnostic execution identity is stale")
	}
	return nil
}

func (r *CodexRuntime) beginProductDiagnosticStart(options SessionStartOptions) error {
	r.diagnosticMu.Lock()
	defer r.diagnosticMu.Unlock()
	if r.diagnosticReservation == nil {
		return errors.New("diagnostic reservation is required before provider process start")
	}
	if r.diagnosticStarted {
		return errors.New("diagnostic provider process start already consumed; no retry")
	}
	r.diagnosticStarted = true
	authorization := r.diagnosticAuthorization
	profile := r.ExecutionProfile()
	if options.SessionID != authorization.ProviderSessionID || options.Model != profile.Model || options.Effort != profile.Effort || options.Profile != profile.Profile || options.ToolSurface.ManifestDigest != r.surface.ManifestDigest || options.MissionID != "" || options.TaskID != "" {
		return errors.New("diagnostic session options do not match the zero-business authorization")
	}
	return nil
}

func describeProductNativeLaunch(config CodexRuntimeConfig, providerSessionID string) (runner.NativeLaunchEnvelope, error) {
	if providerSessionID == "" {
		return runner.NativeLaunchEnvelope{}, errors.New("provider diagnostic session ID is required")
	}
	home := filepath.Join(config.Root, providerSessionID, "home")
	helper := config.HelperBinary
	if helper == "" {
		helper = filepath.Join(filepath.Dir(config.Binary), "codex-code-mode-host")
		if (runtime.GOOS == "windows" || strings.HasSuffix(strings.ToLower(config.Binary), ".exe")) && !isRegularFile(helper) {
			helper += ".exe"
		}
	}
	helperBytes, err := os.ReadFile(helper)
	if err != nil {
		return runner.NativeLaunchEnvelope{}, fmt.Errorf("read native provider helper: %w", err)
	}
	helperDigest := sha256.Sum256(helperBytes)
	var args, environment []string
	if runtime.GOOS == "windows" {
		args = []string{config.Binary, "app-server", "--stdio"}
		environment = runner.NativeEnvironment(home)
	} else {
		launch, buildErr := runner.BuildNativeLaunch(config.Binary, helper, home, config.AuthFile, "", runner.NativeTransportPolicyNativeDefault, hex.EncodeToString(helperDigest[:]))
		if buildErr != nil {
			return runner.NativeLaunchEnvelope{}, buildErr
		}
		args, environment = launch.Args, launch.Environment
	}
	envelope, err := runner.DescribeNativeLaunchWithDirectories(config.Binary, helper, home, args, environment, "", runner.ProcessLaunchDirectories{
		RuntimeRoot: config.Root, RuntimeHome: home, TempDirectory: filepath.Join(home, "tmp"), EvidenceRoot: filepath.Join(config.EvidenceRoot, providerSessionID),
	})
	if err != nil {
		return runner.NativeLaunchEnvelope{}, err
	}
	snapshot := config.TransportPolicy.Snapshot()
	return runner.BindTransportPolicy(envelope, runner.NativeTransportPolicyBinding{
		Revision: snapshot.Revision, InitializeTimeoutMS: snapshot.InitializeTimeoutMS,
		StartAcknowledgementMS: snapshot.StartAcknowledgementMS, FirstOutputDeadlineMS: snapshot.FirstOutputDeadlineMS,
		ReconnectGraceMS: snapshot.ReconnectGraceMS, StreamingIdleMS: snapshot.StreamingIdleMS,
		TotalTurnDeadlineMS: snapshot.TotalTurnDeadlineMS, ReconciliationMS: snapshot.ReconciliationMS,
	}), nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

type productSurfaceDiagnosticSession struct {
	inner       Session
	surface     ToolSurface
	identity    ProductSurfaceExecutionIdentity
	startThread func(context.Context, ThreadStartOptions) (string, error)
	mu          sync.Mutex
	initialized bool
	threadStart bool
	turnUsed    bool
	stopped     bool
	threadID    string
}

func newProductSurfaceDiagnosticSession(inner *codexSession, surface ToolSurface, identity ProductSurfaceExecutionIdentity) *productSurfaceDiagnosticSession {
	return &productSurfaceDiagnosticSession{
		inner: inner, surface: surface, identity: identity,
		startThread: func(ctx context.Context, options ThreadStartOptions) (string, error) {
			return inner.startThreadWithInstructions(ctx, options, productSurfaceDiagnosticDeveloperInstructions)
		},
	}
}

func (s *productSurfaceDiagnosticSession) Process() *runner.Process { return s.inner.Process() }

func (s *productSurfaceDiagnosticSession) InitializationEvidence() codex.InitializeLifecycleEvidence {
	if observed, ok := s.inner.(interface {
		InitializationEvidence() codex.InitializeLifecycleEvidence
	}); ok {
		return observed.InitializationEvidence()
	}
	return codex.InitializeLifecycleEvidence{}
}

func (s *productSurfaceDiagnosticSession) Initialize(ctx context.Context, policy codex.TransportPolicy) error {
	s.mu.Lock()
	if s.stopped || s.initialized {
		s.mu.Unlock()
		return errors.New("diagnostic session initialization already consumed")
	}
	if policy.Snapshot() != s.identity.TransportPolicy {
		s.mu.Unlock()
		return errors.New("diagnostic session transport policy changed")
	}
	s.initialized = true
	s.mu.Unlock()
	return s.inner.Initialize(ctx, policy)
}

func (s *productSurfaceDiagnosticSession) StartThread(context.Context, ThreadStartOptions) (string, error) {
	return "", errors.New("diagnostic session requires its dedicated no-tool thread start")
}

func (s *productSurfaceDiagnosticSession) StartDiagnosticThread(ctx context.Context, options ThreadStartOptions) (string, error) {
	s.mu.Lock()
	if s.stopped || !s.initialized || s.threadStart {
		s.mu.Unlock()
		return "", errors.New("diagnostic thread start is unavailable or already consumed")
	}
	if options.Model != s.identity.Model || options.Effort != s.identity.Effort || !sameProductToolSurface(ToolSurfaceFromTools(options.Tools), s.surface) {
		s.mu.Unlock()
		return "", errors.New("diagnostic thread tools or profile do not match the exact product authorization")
	}
	s.threadStart = true
	s.mu.Unlock()
	threadID, err := s.startThread(ctx, options)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.threadID = threadID
	s.mu.Unlock()
	return threadID, nil
}

func (s *productSurfaceDiagnosticSession) Turn(ctx context.Context, thread, prompt string, options codex.TurnOptions, handler ToolHandler) (TurnResult, error) {
	s.mu.Lock()
	if s.stopped || !s.threadStart || s.turnUsed || thread == "" || thread != s.threadID {
		s.mu.Unlock()
		return TurnResult{}, errors.New("diagnostic turn is unavailable or already consumed")
	}
	if prompt != ProductSurfaceDiagnosticPrompt || handler != nil || options.ToolCallLimit != 0 || options.Policy == nil || options.Policy.Snapshot() != s.identity.TransportPolicy {
		s.mu.Unlock()
		return TurnResult{}, errors.New("diagnostic turn request exceeds the authorized prompt, tool, or transport scope")
	}
	s.turnUsed = true
	s.mu.Unlock()
	toolCalls := 0
	result, err := s.inner.Turn(ctx, thread, prompt, options, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		toolCalls++
		return json.RawMessage(`{"error":"tool calls are disabled for this diagnostic"}`), true
	})
	result.ToolCalls = toolCalls
	return result, err
}

func (s *productSurfaceDiagnosticSession) Stop(ctx context.Context) (runner.StopProof, error) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return runner.StopProof{}, errors.New("diagnostic session stop already consumed")
	}
	s.stopped = true
	s.mu.Unlock()
	return s.inner.Stop(ctx)
}

func sameProductToolSurface(left, right ToolSurface) bool {
	return left.ToolCount == right.ToolCount && left.ManifestDigest == right.ManifestDigest && left.AggregateSchemaBytes == right.AggregateSchemaBytes && left.AggregateSchemaDigest == right.AggregateSchemaDigest
}
