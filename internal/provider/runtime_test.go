// pattern: Imperative Shell
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
)

func TestCodexRuntimeReservationLifecycleClosesWithoutReleasingEligibility(t *testing.T) {
	allowance := t.TempDir() + "/reservation.json"
	runtime := NewCodexRuntime(CodexRuntimeConfig{
		Model: "gpt-5.6-luna", Effort: "medium", ExpectedVersion: ProductProviderRuntimeVersionV2,
		ExecutionEnvelope: "offline-reservation-bridge@1", ToolSurfaceQualification: ProductToolSurfaceQualification,
		ToolSurface: ProductToolSurface(), TransportPolicy: codex.DefaultTransportPolicy(),
		Purpose: ProductReservationBridgeQualificationPurpose, AllowancePath: allowance,
		MediumLimit: 1, HighLimit: 0, ToolCallLimit: 16,
	})
	authorization := validProviderExecutionAuthorization("gpt-5.6-luna", "real", ProductReservationBridgeQualificationPurpose, "offline-reservation-bridge@1")
	reservation, err := runtime.Reserve(context.Background(), authorization)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Stats().Reservations != 1 || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("reservation was not recorded without provider egress: %+v", runtime.Stats())
	}
	if got := namedRuntimeStat(runtime.Stats(), "ActiveReservations"); got != 1 {
		t.Fatalf("active reservations=%d, want 1", got)
	}
	closer, ok := any(runtime).(interface {
		CloseReservation(context.Context, Reservation, string) error
	})
	if !ok {
		t.Fatal("business runtime has no deterministic reservation close operation")
	}
	if err = closer.CloseReservation(context.Background(), reservation, "preflight_before_turn"); err != nil {
		t.Fatal(err)
	}
	if got := namedRuntimeStat(runtime.Stats(), "ActiveReservations"); got != 0 {
		t.Fatalf("active reservations after close=%d, want 0", got)
	}
	if got := namedRuntimeStat(runtime.Stats(), "ClosedReservations"); got != 1 {
		t.Fatalf("closed reservations=%d, want 1", got)
	}
	if _, err = runtime.Reserve(context.Background(), authorization); !errors.Is(err, errBusinessReservationAlreadyUsed) {
		t.Fatalf("reservation was reusable after close: %v", err)
	}
}

func namedRuntimeStat(value RuntimeStats, name string) int {
	field := reflect.ValueOf(value).FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.Int {
		return -1
	}
	return int(field.Int())
}

func TestFakeRuntimeExercisesProductProtocolWithoutProvider(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{TurnDelay: time.Millisecond})
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := runtime.Start(context.Background(), SessionStartOptions{SessionID: "fake-session", Model: "offline-model", Effort: "medium", Profile: "offline-model/medium", ToolSurface: ProductToolSurface()})
	if err != nil {
		t.Fatal(err)
	}
	if err = session.Initialize(context.Background(), codex.DefaultTransportPolicy()); err != nil {
		t.Fatal(err)
	}
	thread, err := session.StartThread(context.Background(), ThreadStartOptions{Model: "offline-model", Effort: "medium", Tools: ProductToolSurface().Tools})
	if err != nil {
		t.Fatal(err)
	}
	toolNames := make([]string, 0, 7)
	turn, err := session.Turn(context.Background(), thread, "complete the product task", codex.TurnOptions{ToolCallLimit: 16}, func(name, _ string, arguments json.RawMessage) (json.RawMessage, bool) {
		toolNames = append(toolNames, name)
		result := map[string]any{"receipt": map[string]string{"id": "offline-receipt", "status": "persisted"}}
		switch name {
		case "work_current":
			result["data"] = map[string]any{"Task": map[string]any{"ID": "task", "validation_binding": map[string]any{"contract": map[string]any{"required_text": []string{"Mission ID: mission", "Task ID: task", "Acknowledgement:", "Task summary:"}}}}}
		case "workspace_read":
			result["data"] = map[string]any{"Digest": "workspace-digest", "Revision": 1}
		case "workspace_check":
			result["data"] = map[string]any{"status": "PASS"}
		case "workspace_replace":
			var request map[string]any
			_ = json.Unmarshal(arguments, &request)
			if request["content"] == "stale writer must be rejected" {
				result = map[string]any{"error": "REVISION_CONFLICT"}
			}
		}
		raw, _ := json.Marshal(result)
		return raw, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.State != "completed" || turn.ProviderEgress != 0 || turn.ToolCalls != 6 {
		t.Fatalf("unexpected offline turn: %+v", turn)
	}
	if want := []string{"work_current", "workspace_read", "workspace_replace", "workspace_replace", "workspace_check", "task_submit"}; !reflect.DeepEqual(toolNames, want) {
		t.Fatalf("offline runtime tool path=%v, want %v", toolNames, want)
	}
	if _, err = session.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stats := runtime.Stats(); stats.Reservations != 0 || stats.ProviderEgress != 0 {
		t.Fatalf("offline runtime performed provider work: %+v", stats)
	}
}

func TestExecutionAuthorizationBindsProductIdentity(t *testing.T) {
	authorization := validProviderExecutionAuthorization("offline-model", "fake", "product-artifact", "test@1")
	if err := ValidateExecutionAuthorization(authorization); err != nil {
		t.Fatal(err)
	}
	changed := authorization
	changed.TaskID = "other-task"
	if err := ValidateExecutionAuthorizationMatch(changed, authorization); err == nil {
		t.Fatal("changed task identity was accepted")
	}
}

func TestExecutionAuthorizationDeniesMissionInternalTask(t *testing.T) {
	authorization := validProviderExecutionAuthorization("offline-model", "fake", "product-artifact", "test@1")
	authorization.TaskID = "bootstrap-task"
	authorization.TaskKind = core.TaskKindBootstrapPlan
	authorization.TaskOwnerID = core.EmployeePlanningID
	authorization.EmployeeID = core.EmployeePlanningID
	authorization.EmployeeRole = "planning"
	if err := ValidateExecutionAuthorization(authorization); err == nil {
		t.Fatal("provider authorization accepted a Mission-internal bootstrap Task")
	}
}

func TestCodexReservationDeniesInternalTaskBeforeCreatingAllowance(t *testing.T) {
	allowance := t.TempDir() + "/allowance.json"
	runtime := NewCodexRuntime(CodexRuntimeConfig{
		Model: "gpt-5.6-luna", Effort: "medium", ExpectedVersion: "0.154.0-alpha.6.2",
		ExecutionEnvelope: "offline-envelope@1", ToolSurfaceQualification: ProductToolSurfaceQualification,
		ToolSurface: ProductToolSurface(), TransportPolicy: codex.DefaultTransportPolicy(),
		AllowancePath: allowance, MediumLimit: 1, HighLimit: 0, ToolCallLimit: 16,
	})
	authorization := validProviderExecutionAuthorization("gpt-5.6-luna", "real", "product-artifact", "offline-envelope@1")
	authorization.TaskID = "bootstrap-task"
	authorization.TaskKind = core.TaskKindBootstrapPlan
	authorization.TaskOwnerID = core.EmployeePlanningID
	authorization.EmployeeID = core.EmployeePlanningID
	authorization.EmployeeRole = "planning"
	if _, err := runtime.Reserve(context.Background(), authorization); err == nil {
		t.Fatal("Codex runtime reserved provider capacity for a Mission-internal Task")
	}
	if _, err := os.Stat(allowance); !os.IsNotExist(err) {
		t.Fatalf("internal Task authorization created allowance file: stat error = %v", err)
	}
}

func validProviderExecutionAuthorization(model, providerMode, purpose, envelope string) ExecutionAuthorization {
	surface := ProductToolSurface()
	return ExecutionAuthorization{
		CompanyID: "company", MissionID: "mission", TaskID: "task", TaskKind: core.TaskKindCompat,
		TaskOwnerID: core.EmployeeBackendID, EmployeeID: core.EmployeeBackendID, EmployeeRole: "backend",
		SessionID: "session", Epoch: 2, Incarnation: "incarnation", Model: model, Profile: model + "/medium",
		Effort: "medium", ProviderMode: providerMode, Purpose: purpose,
		ToolSurfaceDigest: surface.ManifestDigest, ToolSurfaceQualification: ProductToolSurfaceQualification,
		ToolCount: surface.ToolCount, AggregateSchemaBytes: surface.AggregateSchemaBytes, AggregateSchemaDigest: surface.AggregateSchemaDigest,
		ExactSurfaceExecutionFingerprint: ProductExactSurfaceExecutionFingerprint, ProductProviderL2Fingerprint: ProductProviderL2Fingerprint,
		TaskValidationBindingDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		WorkspaceDigest:             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WorkspaceRevision: 1,
		ExecutionEnvelope: envelope, TransportPolicyRevision: codex.TransportPolicyRevision, ToolCallLimit: 16,
	}
}

func TestCodexRuntimeReadinessFailsClosedWhenProviderConfigurationIsMissing(t *testing.T) {
	runtime := NewCodexRuntime(CodexRuntimeConfig{TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: ProductToolSurface()})
	if err := runtime.Readiness(context.Background()); err == nil {
		t.Fatal("missing real provider configuration was accepted")
	}
}

func TestFakeRuntimeReadinessRejectsInvalidModelProfile(t *testing.T) {
	for name, config := range map[string]FakeRuntimeConfig{
		"invalid effort":  {Model: "offline-model", Effort: "high", Profile: "offline-model/high", ToolCallLimit: 16},
		"invalid profile": {Model: "offline-model", Effort: "medium", Profile: "wrong-profile", ToolCallLimit: 16},
		"invalid budget":  {Model: "offline-model", Effort: "medium", Profile: "offline-model/medium", ToolCallLimit: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if err := NewFakeRuntime(config).Readiness(context.Background()); err == nil {
				t.Fatal("invalid offline provider configuration was accepted")
			}
		})
	}
}
