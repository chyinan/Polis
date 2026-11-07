// pattern: Functional Core
package provider

import (
	"context"
	"errors"
	"testing"

	"polis/internal/codex"
	"polis/internal/core"
)

func TestLive2AuthorizationRequiresQualifiedSurfaceAndTaskSnapshot(t *testing.T) {
	authorization := live2AuthorizationFixture()
	if err := ValidateExecutionAuthorization(authorization); err != nil {
		t.Fatalf("valid LIVE_2 authorization rejected: %v", err)
	}

	for name, mutate := range map[string]func(*ExecutionAuthorization){
		"wrong surface":                func(a *ExecutionAuthorization) { a.ToolSurfaceDigest = "stale-surface" },
		"wrong schema digest":          func(a *ExecutionAuthorization) { a.AggregateSchemaDigest = "stale-schema" },
		"wrong schema byte count":      func(a *ExecutionAuthorization) { a.AggregateSchemaBytes++ },
		"wrong surface fingerprint":    func(a *ExecutionAuthorization) { a.ExactSurfaceExecutionFingerprint = "stale-execution" },
		"wrong provider L2":            func(a *ExecutionAuthorization) { a.ProductProviderL2Fingerprint = "stale-l2" },
		"missing validation binding":   func(a *ExecutionAuthorization) { a.TaskValidationBindingDigest = "" },
		"invalid validation binding":   func(a *ExecutionAuthorization) { a.TaskValidationBindingDigest = "not-a-digest" },
		"missing workspace CAS digest": func(a *ExecutionAuthorization) { a.WorkspaceDigest = "" },
		"invalid workspace CAS digest": func(a *ExecutionAuthorization) { a.WorkspaceDigest = "not-a-digest" },
		"invalid workspace revision":   func(a *ExecutionAuthorization) { a.WorkspaceRevision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := authorization
			mutate(&changed)
			if err := ValidateExecutionAuthorization(changed); err == nil {
				t.Fatal("invalid LIVE_2 authorization was accepted")
			}
		})
	}
}

func TestCodexBusinessStartRequiresOneUseAuthorizationReservation(t *testing.T) {
	options := SessionStartOptions{
		SessionID: "session", Model: "gpt-5.6-luna", Effort: "medium", Profile: "gpt-5.6-luna/medium",
		MissionID: "mission", TaskID: "task", ToolSurface: ProductToolSurface(),
	}
	unauthorized := NewCodexRuntime(CodexRuntimeConfig{})
	if _, err := unauthorized.Start(context.Background(), options); !errors.Is(err, errBusinessReservationRequired) {
		t.Fatalf("business Start without reservation error = %v, want reservation-required denial", err)
	}

	runtime, _, _ := live2CodexRuntimeWithAuth(t, live2AuthFile(t, "access-a", "refresh-a"))
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("valid LIVE_2 provider preflight rejected: %v", err)
	}
	authorization := live2AuthorizationFixture()
	authorization.ProviderMode = "real"
	authorization.Purpose = runtime.ExecutionProfile().Purpose
	if _, err := runtime.Reserve(context.Background(), authorization); err != nil {
		t.Fatalf("valid one-use test authorization could not reserve the local budget: %v", err)
	}
	if _, err := runtime.Reserve(context.Background(), authorization); !errors.Is(err, errBusinessReservationAlreadyUsed) {
		t.Fatalf("second business reservation error = %v, want one-use denial", err)
	}
	if _, err := runtime.Start(context.Background(), options); err == nil {
		t.Fatal("Codex runtime started with missing binary/auth configuration")
	}
	if _, err := runtime.Start(context.Background(), options); !errors.Is(err, errBusinessReservationAlreadyUsed) {
		t.Fatalf("replayed business Start error = %v, want one-use denial", err)
	}
}

func TestLive2RuntimeBindingMatchesPurposeAndQualification(t *testing.T) {
	surface := ProductToolSurface()
	profile := ExecutionProfile{
		Model: "gpt-5.6-luna", Effort: "medium", Profile: "gpt-5.6-luna/medium", ToolCallLimit: 16,
		Purpose: Live2AuthorizationPurpose, ToolSurfaceQualification: ProductToolSurfaceQualification,
		ExecutionEnvelope: ProductProviderRuntimeEnvelopeFingerprintV2, ExactSurfaceExecutionFingerprint: ProductExactSurfaceExecutionFingerprint,
		ProductProviderL2Fingerprint: ProductProviderL2Fingerprint, TransportPolicy: codex.DefaultTransportPolicy(),
	}
	authorization := live2AuthorizationFixture()
	if err := validateRuntimeBinding(authorization, "fake", profile, surface); err != nil {
		t.Fatalf("valid LIVE_2 runtime binding rejected: %v", err)
	}

	for name, mutate := range map[string]func(*ExecutionAuthorization){
		"wrong purpose":            func(a *ExecutionAuthorization) { a.Purpose = "product-artifact" },
		"wrong exact execution fp": func(a *ExecutionAuthorization) { a.ExactSurfaceExecutionFingerprint = "stale-execution" },
		"wrong provider L2 fp":     func(a *ExecutionAuthorization) { a.ProductProviderL2Fingerprint = "stale-l2" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := authorization
			mutate(&changed)
			if err := validateRuntimeBinding(changed, "fake", profile, surface); err == nil {
				t.Fatal("runtime accepted an authorization with drifted purpose or qualification")
			}
		})
	}
}

func live2AuthorizationFixture() ExecutionAuthorization {
	surface := ProductToolSurface()
	return ExecutionAuthorization{
		CompanyID: "company", MissionID: "mission", TaskID: "task", TaskKind: core.TaskKindCompat,
		TaskOwnerID: core.EmployeeBackendID, EmployeeID: core.EmployeeBackendID, EmployeeRole: "backend",
		SessionID: "session", Epoch: 2, Incarnation: "incarnation", Model: "gpt-5.6-luna",
		Profile: "gpt-5.6-luna/medium", Effort: "medium", ProviderMode: "fake", Purpose: Live2AuthorizationPurpose,
		ToolSurfaceDigest: surface.ManifestDigest, ToolSurfaceQualification: ProductToolSurfaceQualification,
		ToolCount: surface.ToolCount, AggregateSchemaBytes: surface.AggregateSchemaBytes, AggregateSchemaDigest: surface.AggregateSchemaDigest,
		ExactSurfaceExecutionFingerprint: ProductExactSurfaceExecutionFingerprint,
		ProductProviderL2Fingerprint:     ProductProviderL2Fingerprint,
		TaskValidationBindingDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		WorkspaceDigest:                  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WorkspaceRevision: 1,
		ExecutionEnvelope: ProductProviderRuntimeEnvelopeFingerprintV2, TransportPolicyRevision: codex.TransportPolicyRevision, ToolCallLimit: 16,
	}
}
