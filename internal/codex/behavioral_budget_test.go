// pattern: Imperative Shell
package codex

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBusinessBudgetCarriesExplicitToolCallLimit(t *testing.T) {
	b, err := NewBusinessBudget(t.TempDir()+"/allowance.json", 1, 0, 17)
	if err != nil {
		t.Fatal(err)
	}
	if b.ToolCallLimit != 17 {
		t.Fatalf("tool call limit=%d, want 17", b.ToolCallLimit)
	}
	if got := b.ToolBudget(); got.Limit != 17 || got.Used != 0 || got.Remaining != 17 {
		t.Fatalf("initial tool budget=%+v", got)
	}
	b.RecordToolCall()
	if got := b.ToolBudget(); got.Used != 1 || got.Remaining != 16 {
		t.Fatalf("updated tool budget=%+v", got)
	}
}

func TestBusinessAuthorizationBindsToolCallLimit(t *testing.T) {
	context := BusinessExecutionContext{
		ExecutionFingerprint: "fingerprint",
		EmployeeID:           "emp-backend",
		ProblemKey:           "r03a-real-peer-collaboration-v1",
		Purpose:              "real_backend_peer_collaboration",
		Model:                "gpt-5.6-luna",
		Profile:              "gpt-5.6-luna/medium",
		Effort:               "medium",
		Limits:               AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 17},
	}
	binding := BusinessAuthorizationBindingFromContext(context)
	if decision := AuthorizeBusinessExecution(binding, context); !decision.Allowed {
		t.Fatalf("exact tool budget binding denied: %+v", decision)
	}
	binding.Limits.ToolCallLimit = 18
	if decision := AuthorizeBusinessExecution(binding, context); decision.Allowed || decision.ReasonCode != AllowanceLimitsMismatchReasonCode {
		t.Fatalf("tool budget drift was accepted: %+v", decision)
	}
	binding = BusinessAuthorizationBindingFromContext(context)
	binding.Limits.ToolCallLimit = RuntimeToolCallSafetyCap + 1
	context.Limits.ToolCallLimit = RuntimeToolCallSafetyCap + 1
	if decision := AuthorizeBusinessExecution(binding, context); decision.Allowed || decision.ReasonCode != ToolCallLimitTooLargeReasonCode {
		t.Fatalf("tool budget beyond runtime cap was accepted: %+v", decision)
	}
}

func TestHighOnlyBudgetDoesNotAuthorizeMedium(t *testing.T) {
	b, err := NewHighOnlyBudget(t.TempDir() + "/high-only.json")
	if err != nil {
		t.Fatal(err)
	}
	if b.MediumLimit != 0 || b.HighLimit != 1 || b.Medium != 0 || b.High != 0 {
		t.Fatalf("unexpected High-only allowance: %+v", b)
	}
	if err := b.Reserve("medium"); err == nil {
		t.Fatal("High-only allowance authorized a Medium turn")
	}
	if err := b.Reserve("high"); err != nil {
		t.Fatal(err)
	}
}

func TestPeerBackendToolsRemainExactElevenWithExplicitCheckpointKind(t *testing.T) {
	tools := PeerBackendTools()
	if len(tools) != 11 {
		t.Fatalf("peer backend tool surface has %d tools, want 11", len(tools))
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(raw, "tool-call") || !containsJSONField(raw, "kind") || !containsJSONField(raw, "progress") || !containsJSONField(raw, "qualified") {
		t.Fatalf("formal surface does not expose the explicit budget/checkpoint contract: %s", raw)
	}
}

func containsJSONField(raw []byte, value string) bool {
	return len(raw) > 0 && string(raw) != "" && bytes.Contains(raw, []byte(value))
}
