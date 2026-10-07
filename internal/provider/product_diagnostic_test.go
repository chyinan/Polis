// pattern: Imperative Shell
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

func TestProductSurfaceDiagnosticAuthorizationBindsExactIdentityAndZeroBusinessScope(t *testing.T) {
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	if err := ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		t.Fatal(err)
	}
	if authorization.Purpose != "PRODUCT_SURFACE_V4_LIVE_QUALIFICATION" || authorization.ReservationLimit != 1 || authorization.MediumLimit != 1 || authorization.HighLimit != 0 || authorization.RetryLimit != 0 || authorization.ExpectedToolCalls != 0 || authorization.BusinessObjectsAllowed {
		t.Fatalf("diagnostic authorization has an unsafe scope: %+v", authorization)
	}
}

func TestProductSurfaceDiagnosticAuthorizationRejectsBindingDrift(t *testing.T) {
	base := productSurfaceDiagnosticAuthorizationFixture(t)
	changed := base
	changed.ExecutionIdentity.AggregateSchemaBytes++
	if err := ValidateProductSurfaceDiagnosticAuthorization(changed, base); err == nil {
		t.Fatal("changed schema identity was accepted")
	}

	changed = base
	changed.Purpose = "product-artifact"
	if err := ValidateProductSurfaceDiagnosticAuthorization(changed, base); err == nil {
		t.Fatal("business purpose was accepted as a diagnostic authorization")
	}

	changed = base
	changed.ExpectedToolCalls = 1
	if err := ValidateProductSurfaceDiagnosticAuthorization(changed, base); err == nil {
		t.Fatal("nonzero expected tool calls were accepted")
	}
}

func TestReserveProductSurfaceDiagnosticOnceDeniesSecondReservation(t *testing.T) {
	root := t.TempDir()
	authorizationPath := filepath.Join(root, "diagnostic-authorization.json")
	reservationPath := filepath.Join(root, "diagnostic-reservation.json")
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	raw, err := json.Marshal(authorization)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(authorizationPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reserved, err := ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath, authorization, time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if reserved.ReservationNumber != 1 {
		t.Fatalf("reservation=%+v want reservation 1", reserved)
	}
	if _, err = ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath, authorization, time.Date(2026, 9, 17, 1, 0, 1, 0, time.UTC)); err == nil {
		t.Fatal("second diagnostic reservation was accepted")
	}
	if _, err = os.Stat(reservationPath); err != nil {
		t.Fatalf("terminal reservation marker missing: %v", err)
	}
}

func TestReserveProductSurfaceDiagnosticOnceAllowsOnlyOneConcurrentClaim(t *testing.T) {
	root := t.TempDir()
	authorizationPath := filepath.Join(root, "diagnostic-authorization.json")
	reservationPath := filepath.Join(root, "diagnostic-reservation.json")
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	raw, err := json.Marshal(authorization)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(authorizationPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	const contenders = 8
	start := make(chan struct{})
	results := make(chan error, contenders)
	for i := 0; i < contenders; i++ {
		go func() {
			<-start
			_, reserveErr := ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath, authorization, time.Now().UTC())
			results <- reserveErr
		}()
	}
	close(start)
	successes := 0
	for i := 0; i < contenders; i++ {
		if err = <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent diagnostic reservations succeeded %d times, want 1", successes)
	}
}

func TestReserveProductSurfaceDiagnosticOnceRejectsDriftBeforeConsumingMarker(t *testing.T) {
	root := t.TempDir()
	authorizationPath := filepath.Join(root, "diagnostic-authorization.json")
	reservationPath := filepath.Join(root, "diagnostic-reservation.json")
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	changed := authorization
	changed.ExecutionIdentity.Model = "gpt-5.6-sol"
	raw, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(authorizationPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath, authorization, time.Now().UTC()); err == nil {
		t.Fatal("authorization drift was accepted")
	}
	if _, err = os.Stat(reservationPath); !os.IsNotExist(err) {
		t.Fatalf("invalid authorization consumed a reservation marker: stat err=%v", err)
	}
}

func TestDiagnosticOnlyCodexRuntimeCannotReserveBusinessExecution(t *testing.T) {
	runtime := NewCodexRuntime(CodexRuntimeConfig{DiagnosticOnly: true})
	if _, err := runtime.Reserve(context.Background(), ExecutionAuthorization{}); err == nil {
		t.Fatal("diagnostic-only runtime accepted a business reservation")
	}
}

func TestDiagnosticOnlyCodexRuntimeCannotStartWithoutItsReservation(t *testing.T) {
	runtime := NewCodexRuntime(CodexRuntimeConfig{DiagnosticOnly: true})
	_, err := runtime.Start(context.Background(), SessionStartOptions{SessionID: "r05b8-product-surface-v3-canary"})
	if err == nil || !strings.Contains(err.Error(), "diagnostic reservation") {
		t.Fatalf("start without a one-use reservation was not rejected: %v", err)
	}
}

func TestProductSurfaceDiagnosticPromptHasOnlyTheCanaryRequest(t *testing.T) {
	if ProductSurfaceDiagnosticPrompt != "Respond with exactly POLIS_PRODUCT_SURFACE_V4_CANARY_OK." {
		t.Fatalf("unexpected diagnostic prompt: %q", ProductSurfaceDiagnosticPrompt)
	}
	if ProductSurfaceDiagnosticInstructionRevision != "product-surface-v4-diagnostic-instructions@1" || !strings.Contains(productSurfaceDiagnosticDeveloperInstructions, "Do not call tools") {
		t.Fatal("diagnostic thread lacks its fixed no-tool instruction")
	}
}

func TestProductDiagnosticSessionNeverDispatchesAndStopsAfterOneTurn(t *testing.T) {
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	inner := &diagnosticSessionStub{toolCall: true}
	threadStarts := 0
	session := &productSurfaceDiagnosticSession{
		inner: inner, surface: ProductToolSurface(), identity: authorization.ExecutionIdentity,
		startThread: func(context.Context, ThreadStartOptions) (string, error) {
			threadStarts++
			return "diagnostic-thread", nil
		},
	}
	if _, err := session.StartThread(context.Background(), ThreadStartOptions{}); err == nil {
		t.Fatal("diagnostic session allowed the business employee thread path")
	}
	policy := codex.DefaultTransportPolicy()
	if err := session.Initialize(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	thread, err := session.StartDiagnosticThread(context.Background(), ThreadStartOptions{Model: authorization.ExecutionIdentity.Model, Effort: authorization.ExecutionIdentity.Effort, Tools: ProductToolSurface().Tools})
	if err != nil {
		t.Fatal(err)
	}
	if thread != "diagnostic-thread" || threadStarts != 1 {
		t.Fatalf("diagnostic thread start=%q count=%d", thread, threadStarts)
	}
	options := codex.TurnOptions{Policy: &policy, ToolCallLimit: 0}
	result, err := session.Turn(context.Background(), thread, ProductSurfaceDiagnosticPrompt, options, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ToolCalls != 1 || inner.toolDispatchBoundary != true || inner.businessDispatches != 0 {
		t.Fatalf("diagnostic tool call was not stopped before business dispatch: result=%+v inner=%+v", result, inner)
	}
	if _, err = session.Turn(context.Background(), thread, ProductSurfaceDiagnosticPrompt, options, nil); err == nil || inner.turnCalls != 1 {
		t.Fatalf("second diagnostic turn was not denied: err=%v calls=%d", err, inner.turnCalls)
	}
}

func TestBuildProductSurfaceExecutionIdentityBindsCurrentRuntimeBytes(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	helperName := "codex-code-mode-host"
	if strings.EqualFold(filepath.Ext(binary), ".exe") {
		helperName += ".exe"
	}
	helper := filepath.Join(root, helperName)
	if err := os.WriteFile(binary, []byte("current-codex-binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("current-codex-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	config := CodexRuntimeConfig{
		Binary: binary, AuthFile: filepath.Join(root, "auth.json"), Root: filepath.Join(root, "runtime"),
		EvidenceRoot: filepath.Join(root, "evidence"), Model: "gpt-5.6-luna", Effort: "medium",
		ExpectedVersion: "0.154.0-alpha.6.2", TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: ProductToolSurface(),
		ToolSurfaceQualification: ProductToolSurfaceQualification,
	}
	identity, err := BuildProductSurfaceExecutionIdentity(config, "r05b8-product-surface-v3-canary")
	if err != nil {
		t.Fatal(err)
	}
	first, err := ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || identity.ProviderRuntime.NativeVersion != config.ExpectedVersion || identity.TransportPolicy != config.TransportPolicy.Snapshot() {
		t.Fatalf("incomplete product execution identity: %+v", identity)
	}
	if err = os.WriteFile(helper, []byte("changed-codex-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := BuildProductSurfaceExecutionIdentity(config, "r05b8-product-surface-v3-canary")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ComputeProductSurfaceExecutionFingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("runtime helper drift did not change the product execution fingerprint")
	}
}

func TestCodexRuntimeDiagnosticReservationIsIsolatedFromBusinessBudget(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	helperName := "codex-code-mode-host"
	if strings.EqualFold(filepath.Ext(binary), ".exe") {
		helperName += ".exe"
	}
	helper := filepath.Join(root, helperName)
	for path, content := range map[string]string{binary: "provider binary", helper: "provider helper", filepath.Join(root, "auth.json"): `{"auth_mode":"chatgpt","tokens":{"access_token":"fixture-secret","refresh_token":"fixture-refresh"}}`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	providerSessionID := "r05b8-product-surface-v3-canary"
	config := CodexRuntimeConfig{
		Binary: binary, AuthFile: filepath.Join(root, "auth.json"), Root: filepath.Join(root, "runtime"),
		EvidenceRoot: filepath.Join(root, "provider-evidence"), Model: "gpt-5.6-luna", Effort: "medium",
		ExpectedVersion: "0.154.0-alpha.6.2", TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: ProductToolSurface(),
		ToolSurfaceQualification: ProductToolSurfaceQualification, DiagnosticOnly: true,
		DiagnosticAuthorizationPath: filepath.Join(root, "authorization.json"), DiagnosticReservationPath: filepath.Join(root, "reservation.json"),
	}
	identity, err := BuildProductSurfaceExecutionIdentity(config, providerSessionID)
	if err != nil {
		t.Fatal(err)
	}
	config.ExecutionEnvelope = identity.LaunchEnvelope.Fingerprint
	authorization, err := NewProductSurfaceDiagnosticAuthorization("r0.5b8-live-canary-1", providerSessionID, identity, strings.Repeat("d", 64), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = WriteProductSurfaceDiagnosticAuthorization(config.DiagnosticAuthorizationPath, authorization); err != nil {
		t.Fatal(err)
	}
	runtime := NewCodexRuntime(config)
	if _, err = runtime.ReserveProductSurfaceDiagnostic(context.Background(), authorization); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.Reserve(context.Background(), ExecutionAuthorization{}); err == nil {
		t.Fatal("diagnostic reservation also enabled the business budget path")
	}
	secondRuntime := NewCodexRuntime(config)
	if _, err = secondRuntime.ReserveProductSurfaceDiagnostic(context.Background(), authorization); err == nil {
		t.Fatal("second runtime reserved an already-consumed diagnostic marker")
	}
}

func TestProductProviderL2FingerprintRequiresEveryLivePassBoundary(t *testing.T) {
	evidence := productProviderL2EvidenceFixture(t)
	fingerprint, err := ComputeProductProviderL2Fingerprint(evidence)
	if err != nil || fingerprint == "" {
		t.Fatalf("valid product provider L2 evidence was rejected: fingerprint=%q err=%v", fingerprint, err)
	}

	changed := evidence
	changed.ReconnectCount = 1
	if _, err = ComputeProductProviderL2Fingerprint(changed); err == nil {
		t.Fatal("reconnected diagnostic received a qualified fingerprint")
	}
	changed = evidence
	changed.WorkerSessionCount = 1
	if _, err = ComputeProductProviderL2Fingerprint(changed); err == nil {
		t.Fatal("business WorkerSession side effect received a qualified fingerprint")
	}
	changed = evidence
	changed.AggregateSchemaDigestAfter = strings.Repeat("e", 64)
	if _, err = ComputeProductProviderL2Fingerprint(changed); err == nil {
		t.Fatal("stale post-canary schema received a qualified fingerprint")
	}
	changed = evidence
	changed.SecondReservationDenied = false
	if _, err = ComputeProductProviderL2Fingerprint(changed); err == nil {
		t.Fatal("missing actual duplicate-reservation denial received a qualified fingerprint")
	}
}

type diagnosticSessionStub struct {
	toolCall             bool
	turnCalls            int
	toolDispatchBoundary bool
	businessDispatches   int
}

func (s *diagnosticSessionStub) Process() *runner.Process                                { return nil }
func (s *diagnosticSessionStub) Initialize(context.Context, codex.TransportPolicy) error { return nil }
func (s *diagnosticSessionStub) StartThread(context.Context, ThreadStartOptions) (string, error) {
	return "", errors.New("unexpected business thread")
}
func (s *diagnosticSessionStub) Turn(ctx context.Context, _ string, _ string, _ codex.TurnOptions, handler ToolHandler) (TurnResult, error) {
	s.turnCalls++
	if s.toolCall {
		_, s.toolDispatchBoundary = handler("workspace_replace", "call-1", json.RawMessage(`{"content":"must not dispatch"}`))
		if s.toolDispatchBoundary {
			return TurnResult{State: "interrupted_after_checkpoint"}, nil
		}
		s.businessDispatches++
	}
	return TurnResult{State: "completed"}, nil
}
func (s *diagnosticSessionStub) Stop(context.Context) (runner.StopProof, error) {
	return runner.StopProof{}, nil
}

func productSurfaceDiagnosticAuthorizationFixture(t *testing.T) ProductSurfaceDiagnosticAuthorization {
	t.Helper()
	policy := codex.DefaultTransportPolicy().Snapshot()
	launchPolicy := runner.NativeTransportPolicyBinding{
		Revision: policy.Revision, InitializeTimeoutMS: policy.InitializeTimeoutMS,
		StartAcknowledgementMS: policy.StartAcknowledgementMS, FirstOutputDeadlineMS: policy.FirstOutputDeadlineMS,
		ReconnectGraceMS: policy.ReconnectGraceMS, StreamingIdleMS: policy.StreamingIdleMS,
		TotalTurnDeadlineMS: policy.TotalTurnDeadlineMS, ReconciliationMS: policy.ReconciliationMS,
	}
	binaryDigest := strings.Repeat("a", 64)
	helperDigest := strings.Repeat("b", 64)
	identity := ProductSurfaceExecutionIdentity{
		SchemaVersion: ProductSurfaceExecutionIdentitySchema,
		SurfaceID:     ProductToolSurfaceQualification, ToolCount: 7,
		ManifestDigest:        ProductToolSurfaceV4ManifestDigest,
		AggregateSchemaBytes:  ProductToolSurfaceV4AggregateSchemaBytes,
		AggregateSchemaDigest: ProductToolSurfaceV4AggregateSchemaDigest,
		Model:                 "gpt-5.6-luna", Effort: "medium", Profile: "gpt-5.6-luna/medium", ProviderMode: "real",
		ProviderRuntime: ProductProviderRuntimeIdentity{RuntimeImplementation: ProductProviderRuntimeImplementation, NativeVersion: "0.154.0-alpha.6.2", BinarySHA256: binaryDigest, HelperSHA256: helperDigest, ProtocolCompatibility: ProductProviderProtocolCompatibility},
		LaunchEnvelope: runner.NativeLaunchEnvelope{
			SchemaVersion: runner.NativeLaunchEnvelopeSchema, LaunchMode: runner.NativeLaunchModeWindowsDirect, RuntimeOS: "windows",
			ProcessExecutable: "codex.exe", BinaryPath: `C:\controlled-runtime\codex.exe`, HelperPath: `C:\controlled-runtime\codex-code-mode-host.exe`,
			BinarySHA256: binaryDigest, HelperSHA256: helperDigest, Argv: []string{"<codex-binary>", "app-server", "--stdio"},
			WorkingDirectory: "", Home: "<controlled-runtime-home>", CodexHome: "<controlled-runtime-home>",
			Stdio: "stdin_stdout_jsonl;stderr_separate_bounded", Boundary: "Windows native process handle",
			EnvironmentPolicy: runner.ControlledEnvironmentPolicy, InheritedEnvironmentPolicy: runner.InheritedEnvironmentPolicy,
			Environment: []runner.SafeEnvironmentEntry{{Name: "HOME", ValueHash: strings.Repeat("d", 64)}},
			StdinMode:   "redirected_pipe_write", StdoutMode: "redirected_pipe_read", StderrMode: "redirected_pipe_read_bounded", PipeSetupMode: "stdin_stdout_stderr_separate_pipes",
			ProcessCreationFlags: []string{"windows_hide_window"}, ProcessGroupPolicy: "windows_process_handle", JobObjectPolicy: "none",
			RuntimeRootRule: "dedicated runtime root per qualification", RuntimeHomeRule: "runtime_root/session_id/home", TempDirectoryRule: "runtime_home/tmp", EvidenceRootRule: "dedicated evidence root per qualification/session",
			TransportPolicy: launchPolicy, Fingerprint: strings.Repeat("c", 64),
		},
		RuntimePolicy: ProductSurfaceRuntimePolicy{
			NativeConfigSHA256: digestHex([]byte(runner.NativeDefaultConfig)), NativeTransportPolicy: string(runner.NativeTransportPolicyNativeDefault),
			ApprovalPolicy: "never", SandboxPolicy: "read-only", ThreadWorkingDirectory: "/work", ThreadEphemeral: true,
		},
		TransportPolicy: policy, DiagnosticInstructionRevision: ProductSurfaceDiagnosticInstructionRevision,
	}
	authorization, err := NewProductSurfaceDiagnosticAuthorization("r0.5b4-live-canary-1", "r05b4-product-surface-v2-canary", identity, strings.Repeat("d", 64), time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return authorization
}

func productProviderL2EvidenceFixture(t *testing.T) ProductProviderL2Evidence {
	t.Helper()
	authorization := productSurfaceDiagnosticAuthorizationFixture(t)
	return ProductProviderL2Evidence{
		SchemaVersion:     ProductProviderL2EvidenceSchema,
		ExecutionIdentity: authorization.ExecutionIdentity, ExecutionFingerprint: authorization.ExecutionFingerprint,
		DiagnosticAuthorizationID: authorization.AuthorizationID, OfflineQualificationDigest: authorization.OfflineQualificationDigest,
		DiagnosticReservationCount: 1, ProviderEgress: 1, RegisteredToolCount: 7,
		FirstOutput: ProductSurfaceDiagnosticCanaryOutput, ToolCalls: 0, ReconnectCount: 0,
		TurnTerminal: "completed", CleanStop: true,
		CompanyCount: 0, MissionCount: 0, TaskCount: 0, WorkerSessionCount: 0, SuccessorCount: 0, BusinessMutationCount: 0,
		AggregateManifestDigestAfter: authorization.ExecutionIdentity.ManifestDigest,
		AggregateSchemaDigestAfter:   authorization.ExecutionIdentity.AggregateSchemaDigest,
		FirstOutputLatencyMS:         250, ElapsedMS: 1000,
		SecondReservationDenied: true,
	}
}
