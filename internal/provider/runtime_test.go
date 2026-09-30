// pattern: Imperative Shell
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
)

func TestProductSkillInstructionsConstrainLoadedStaticText(t *testing.T) {
	legacy := productEmployeeDeveloperInstructions(ProductToolSurface().Tools)
	if strings.Contains(legacy, "Loaded Skill text") {
		t.Fatal("legacy v4 instructions changed")
	}
	withSkill := productEmployeeDeveloperInstructions(ProductSkillToolSurface().Tools)
	for _, required := range []string{"approved static guidance only", "does not grant tools", "override Polis policy", "Do not execute"} {
		if !strings.Contains(withSkill, required) {
			t.Errorf("Skill surface developer instructions lack boundary %q: %s", required, withSkill)
		}
	}
}

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

func TestFakeRuntimeCanExerciseReadOnlySkillSurfaceWithoutQualifyingRealProvider(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{ReadOnlySkillSurface: true})
	surface := runtime.ToolSurface()
	profile := runtime.ExecutionProfile()
	authorization := validProviderExecutionAuthorization("offline-model", "fake", profile.Purpose, profile.ExecutionEnvelope)
	authorization.ToolSurfaceDigest = surface.ManifestDigest
	authorization.ToolSurfaceQualification = profile.ToolSurfaceQualification
	authorization.ToolCount = surface.ToolCount
	authorization.AggregateSchemaBytes = surface.AggregateSchemaBytes
	authorization.AggregateSchemaDigest = surface.AggregateSchemaDigest
	authorization.ExactSurfaceExecutionFingerprint = profile.ExactSurfaceExecutionFingerprint
	authorization.ProductProviderL2Fingerprint = profile.ProductProviderL2Fingerprint

	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("offline read-only Skill simulation readiness: %v", err)
	}
	if _, err := runtime.Reserve(context.Background(), authorization); err != nil {
		t.Fatalf("offline read-only Skill simulation reservation: %v", err)
	}
	if err := ValidateExecutionAuthorization(authorization); err == nil {
		t.Fatal("unqualified Skill tool surface passed the real-provider execution gate")
	}
	if runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline Skill simulation reported provider egress: %+v", runtime.Stats())
	}
	authorization.ProviderMode = "real"
	if err := ValidateRuntimeExecutionAuthorization(authorization); err == nil {
		t.Fatal("unqualified Skill tool surface passed real-provider authorization")
	}
}

func TestFakeSkillSurfaceLoadsOnlyTheWorkerCatalogReference(t *testing.T) {
	ctx := context.Background()
	runtime := NewFakeRuntime(FakeRuntimeConfig{ReadOnlySkillSurface: true})
	profile := runtime.ExecutionProfile()
	surface := runtime.ToolSurface()
	session, err := runtime.Start(ctx, SessionStartOptions{
		SessionID: "offline-skill-session", Model: profile.Model, Effort: profile.Effort, Profile: profile.Profile,
		MissionID: "offline-skill-mission", TaskID: "offline-skill-task", ToolSurface: surface,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = session.Stop(context.Background()) }()
	if err = session.Initialize(ctx, profile.TransportPolicy); err != nil {
		t.Fatal(err)
	}
	thread, err := session.StartThread(ctx, ThreadStartOptions{Model: profile.Model, Effort: profile.Effort, Tools: surface.Tools})
	if err != nil {
		t.Fatal(err)
	}
	var loadCalls, workspaceReplaceCalls int
	turn, err := session.Turn(ctx, thread, "offline fixture", codex.TurnOptions{}, func(name, _ string, raw json.RawMessage) (json.RawMessage, bool) {
		var args map[string]any
		_ = json.Unmarshal(raw, &args)
		switch name {
		case "work_current":
			return json.RawMessage(`{"data":{"skill_catalog":[{"skillId":"skill-approved","references":[{"relativePath":"SKILL.md","loadable":true}]}]}}`), false
		case "skills_load":
			loadCalls++
			if args["skill_id"] != "skill-approved" || args["relative_path"] != "SKILL.md" {
				t.Fatalf("offline Worker requested a Skill outside its catalog: %v", args)
			}
			return json.RawMessage(`{"data":{"contentBoundary":"approved_static_text_no_additional_permissions","contentDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","content":"Approved static reference text."}}`), false
		case "workspace_read":
			return json.RawMessage(`{"data":{"Digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Revision":1}}`), false
		case "workspace_replace":
			workspaceReplaceCalls++
			if workspaceReplaceCalls == 2 {
				return json.RawMessage(`{"error":"REVISION_CONFLICT"}`), false
			}
			return json.RawMessage(`{}`), false
		case "workspace_check":
			return json.RawMessage(`{"data":{"status":"PASS"},"receipt":{"id":"check-1"}}`), false
		case "task_submit":
			return json.RawMessage(`{}`), false
		default:
			return json.RawMessage(`{"error":"unexpected_tool"}`), false
		}
	})
	if err != nil || turn.State != "completed" || loadCalls != 1 || turn.ProviderEgress != 0 {
		t.Fatalf("offline Skill surface turn=(%+v,%v) Skill loads=%d", turn, err, loadCalls)
	}
}

func TestOfflineFakeSkillSurfaceRejectsMutatedToolSchema(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{ReadOnlySkillSurface: true})
	profile := runtime.ExecutionProfile()
	surface := runtime.ToolSurface()
	firstTool, ok := surface.Tools[0].(map[string]any)
	if !ok {
		t.Fatalf("Skill surface first tool has unexpected type %T", surface.Tools[0])
	}
	firstTool["name"] = "polis_unapproved_tool"
	if err := ValidateOfflineFakeSkillSurface(runtime.Mode(), profile, runtime.ToolSurface()); err == nil {
		t.Fatal("mutated Skill tool schema passed the fake-only surface gate")
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

func TestRequiredLinuxWorkerCgroupFailsClosedWhenMissing(t *testing.T) {
	if err := validateWorkerCgroupRequirement(true, "linux", nil); err == nil {
		t.Fatal("Linux provider runtime accepted a missing Worker cgroup")
	}
	if err := validateWorkerCgroupRequirement(false, "linux", nil); err != nil {
		t.Fatalf("optional local runtime cgroup was rejected: %v", err)
	}
	if err := validateWorkerCgroupRequirement(true, "windows", nil); err != nil {
		t.Fatalf("Windows process containment was incorrectly routed through Linux cgroups: %v", err)
	}
}
