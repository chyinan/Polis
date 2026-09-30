// pattern: Imperative Shell
package kernel

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"polis/internal/core"
)

func TestBeginMCPCallReturnsIntentStateWithoutReusableRuntimeAuthorization(t *testing.T) {
	method, ok := reflect.TypeOf((*Kernel)(nil)).MethodByName("TXBeginStdioMCPToolCall")
	if !ok {
		t.Fatal("one-shot MCP intent reservation API is missing")
	}
	resultType := method.Type.Out(0)
	if resultType.Kind() != reflect.Struct {
		t.Fatalf("intent reservation result type=%s, want struct", resultType)
	}
	if _, exposesAuthorization := resultType.FieldByName("Authorization"); exposesAuthorization {
		t.Fatal("intent reservation exposes a reusable runtime authorization outside the Worker owner lifecycle")
	}
}

func TestValidateStdioMCPToolCallStartRequiresCurrentOneShotIntent(t *testing.T) {
	arguments := json.RawMessage(`{"key":"needle"}`)
	input := StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-1", CapabilityID: "capability-1", ToolName: "lookup",
		ToolSchemaSHA256: repeatDigest('a'), Arguments: arguments,
	}
	authorization := StdioMCPToolAuthorization{RuntimeQualification: StdioMCPRuntimeQualification{CompanyID: "company-1", RuntimeQualificationID: "runtime-1"}}
	record := StdioMCPToolCallRecord{
		CompanyID: "company-1", SessionID: "session-1", EmployeeID: "employee-1",
		Status: "dispatching", ProviderCallID: input.ProviderCallID, CapabilityID: input.CapabilityID,
		ToolName: input.ToolName, ToolSchemaSHA256: input.ToolSchemaSHA256,
		ArgumentsSHA256: digestCapabilityBytes(arguments), RuntimeQualificationID: "runtime-1",
	}
	binding := Binding{scope: Scope{company: "company-1"}, employee: "employee-1", session: "session-1"}
	if err := validateStdioMCPToolCallStart(record, input, authorization, binding); err != nil {
		t.Fatalf("valid one-shot dispatch record rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*StdioMCPToolCallRecord, *StdioMCPToolCallIntentInput, *StdioMCPToolAuthorization)
	}{
		{name: "unknown outcome", mutate: func(record *StdioMCPToolCallRecord, _ *StdioMCPToolCallIntentInput, _ *StdioMCPToolAuthorization) {
			record.Status = "outcome_unknown"
		}},
		{name: "capability changed", mutate: func(record *StdioMCPToolCallRecord, _ *StdioMCPToolCallIntentInput, _ *StdioMCPToolAuthorization) {
			record.CapabilityID = "other-capability"
		}},
		{name: "provider call changed", mutate: func(record *StdioMCPToolCallRecord, _ *StdioMCPToolCallIntentInput, _ *StdioMCPToolAuthorization) {
			record.ProviderCallID = "other-call"
		}},
		{name: "arguments changed", mutate: func(record *StdioMCPToolCallRecord, _ *StdioMCPToolCallIntentInput, _ *StdioMCPToolAuthorization) {
			record.ArgumentsSHA256 = repeatDigest('b')
		}},
		{name: "runtime changed", mutate: func(_ *StdioMCPToolCallRecord, _ *StdioMCPToolCallIntentInput, authorization *StdioMCPToolAuthorization) {
			authorization.RuntimeQualification.RuntimeQualificationID = "other-runtime"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			currentRecord, currentInput, currentAuthorization := record, input, authorization
			test.mutate(&currentRecord, &currentInput, &currentAuthorization)
			if err := validateStdioMCPToolCallStart(currentRecord, currentInput, currentAuthorization, binding); !errors.Is(err, core.Denied) {
				t.Fatalf("mismatched one-shot dispatch state error=%v, want %s", err, core.Denied)
			}
		})
	}
}

func TestValidateStdioMCPToolCallOwnerRequiresExactEmployeeAndSession(t *testing.T) {
	binding := Binding{scope: Scope{company: "company-1"}, employee: "employee-1", session: "session-1"}
	record := StdioMCPToolCallRecord{CompanyID: "company-1", EmployeeID: "employee-1", SessionID: "session-1"}
	if err := validateStdioMCPToolCallOwner(record, binding); err != nil {
		t.Fatalf("exact call owner rejected: %v", err)
	}
	record.EmployeeID = "employee-2"
	if err := validateStdioMCPToolCallOwner(record, binding); !errors.Is(err, core.Denied) {
		t.Fatalf("cross-Employee completion error=%v, want %s", err, core.Denied)
	}
	record.EmployeeID = "employee-1"
	record.SessionID = "session-2"
	if err := validateStdioMCPToolCallOwner(record, binding); !errors.Is(err, core.Denied) {
		t.Fatalf("cross-session completion error=%v, want %s", err, core.Denied)
	}
}
