// pattern: Functional Core
package codex

import (
	"testing"
	"time"
)

func TestAuthorizeBusinessExecutionAllowsExactBinding(t *testing.T) {
	context := businessAuthorizationTestContext()
	decision := AuthorizeBusinessExecution(BusinessAuthorizationBindingFromContext(context), context)
	if !decision.Allowed || decision.ReasonCode != AuthorizationAllowedReasonCode {
		t.Fatalf("exact binding was denied: %+v", decision)
	}
}

func TestAuthorizeBusinessExecutionRejectsUntrustedBindings(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*BusinessAuthorizationBinding, *BusinessExecutionContext)
		reasonCode string
	}{
		{"empty fingerprint historical T21B fixture", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.ExecutionFingerprint = ""
		}, MissingExecutionFingerprintReasonCode},
		{"wrong fingerprint", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.ExecutionFingerprint = "wrong"
		}, ExecutionFingerprintMismatchReasonCode},
		{"stale qualification", func(_ *BusinessAuthorizationBinding, context *BusinessExecutionContext) {
			context.QualificationStale = true
		}, QualificationStaleReasonCode},
		{"wrong employee", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.EmployeeID = "emp-frontend"
		}, EmployeeIDMismatchReasonCode},
		{"wrong ProblemKey", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.ProblemKey = "other-problem"
		}, ProblemKeyMismatchReasonCode},
		{"wrong purpose", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.Purpose = "other-purpose"
		}, PurposeMismatchReasonCode},
		{"model drift", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) {
			binding.Model = "gpt-5.6-sol"
		}, ModelMismatchReasonCode},
		{"profile drift", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) { binding.Profile = "high" }, ProfileMismatchReasonCode},
		{"effort drift", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) { binding.Effort = "high" }, EffortMismatchReasonCode},
		{"allowance limits drift", func(binding *BusinessAuthorizationBinding, _ *BusinessExecutionContext) { binding.Limits.Medium = 2 }, AllowanceLimitsMismatchReasonCode},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			context := businessAuthorizationTestContext()
			binding := BusinessAuthorizationBindingFromContext(context)
			testCase.mutate(&binding, &context)
			decision := AuthorizeBusinessExecution(binding, context)
			if decision.Allowed || decision.ReasonCode != testCase.reasonCode {
				t.Fatalf("unexpected authorization decision: %+v", decision)
			}
		})
	}
}

func TestAuthorizeBusinessExecutionRejectsMissingRequiredBindingField(t *testing.T) {
	context := businessAuthorizationTestContext()
	binding := BusinessAuthorizationBindingFromContext(context)
	binding.Purpose = ""
	decision := AuthorizeBusinessExecution(binding, context)
	if decision.Allowed || decision.ReasonCode != AuthorizationFieldsMissingReasonCode {
		t.Fatalf("missing required binding field was accepted: %+v", decision)
	}
}

func TestAuthorizeBusinessExecutionRejectsTransportPolicyDrift(t *testing.T) {
	context := businessAuthorizationTestContext()
	policy := DefaultTransportPolicy()
	context.TransportPolicyRevision = policy.Revision
	context.TransportPolicy = policy.Snapshot()
	binding := BusinessAuthorizationBindingFromContext(context)
	if decision := AuthorizeBusinessExecution(binding, context); !decision.Allowed {
		t.Fatalf("exact transport policy binding denied: %+v", decision)
	}
	drifted := policy
	drifted.ReconnectGrace = 31 * time.Second
	context.TransportPolicy = drifted.Snapshot()
	if decision := AuthorizeBusinessExecution(binding, context); decision.Allowed || decision.ReasonCode != TransportPolicyMismatchReasonCode {
		t.Fatalf("transport policy drift was accepted: %+v", decision)
	}
}

func TestAuthorizeBusinessExecutionBindsDatabaseAccessView(t *testing.T) {
	context := businessAuthorizationTestContext()
	context.DatabaseBindingFingerprint = "3169383be36c40901474396f46171938ffc2987e522a5c4a5328f3ab953bb148"
	context.DatabaseAccessStrategyRevision = "r03a-windows-localhost-forwarding@1"
	binding := BusinessAuthorizationBindingFromContext(context)
	if decision := AuthorizeBusinessExecution(binding, context); !decision.Allowed {
		t.Fatalf("exact database binding denied: %+v", decision)
	}
	context.DatabaseBindingFingerprint = "wrong"
	if decision := AuthorizeBusinessExecution(binding, context); decision.Allowed || decision.ReasonCode != DatabaseBindingMismatchReasonCode {
		t.Fatalf("database binding drift was accepted: %+v", decision)
	}
	context = businessAuthorizationTestContext()
	context.DatabaseBindingFingerprint = "3169383be36c40901474396f46171938ffc2987e522a5c4a5328f3ab953bb148"
	binding = BusinessAuthorizationBindingFromContext(context)
	binding.DatabaseAccessStrategyRevision = ""
	if decision := AuthorizeBusinessExecution(binding, context); decision.Allowed || decision.ReasonCode != DatabaseBindingMissingReasonCode {
		t.Fatalf("incomplete database binding was accepted: %+v", decision)
	}
}

func businessAuthorizationTestContext() BusinessExecutionContext {
	return BusinessExecutionContext{
		ExecutionFingerprint: "f3f4e0f8dc64ed373bea7716237ad14932cf98c887acead7e6b5d193216b90af",
		EmployeeID:           "emp-backend",
		ProblemKey:           "r03a-real-peer-collaboration-v1",
		Purpose:              "real backend employee",
		Model:                "gpt-5.6-luna",
		Profile:              "gpt-5.6-luna/medium",
		Effort:               "medium",
		Limits:               AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 17},
	}
}
