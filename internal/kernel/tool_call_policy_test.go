// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"testing"
)

func TestToolCallGateDenyByDefault(t *testing.T) {
	desc := &CachedToolDescriptor{Name: "t", Description: "d", InputSchema: json.RawMessage(`{"type":"object"}`), Digest: "sha256:abc"}
	req := ToolCallRequest{CompanyID: "co1", EmployeeID: "emp-backend", BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	def := &toolCatalogSource{ID: "mcp1", Name: "s", Status: "candidate", DescriptorDigest: "d"}
	qual := &MCPToolCallQualification{BindingID: "bind1", DefinitionID: "mcp1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", PolicyRevision: toolCallPolicyRevision, Status: "approved", Transport: "stdio", EffectClass: "read"}
	base := ToolCallGateInput{Request: req, BindingFound: true, BindingStatus: "approved", BindingMCPIDs: []string{"mcp1"}, Definition: def, Descriptor: desc, Qualification: qual}
	if got := toolCallGate(base); !got.OK {
		t.Fatalf("expected ok, got %#v", got)
	}
	cases := []struct {
		name string
		mut  func(*ToolCallGateInput)
		code string
	}{
		{"binding missing", func(in *ToolCallGateInput) { in.BindingFound = false }, "BINDING_NOT_APPROVED"},
		{"binding draft", func(in *ToolCallGateInput) { in.BindingStatus = "draft" }, "BINDING_NOT_APPROVED"},
		{"binding revoked", func(in *ToolCallGateInput) { in.BindingStatus = "revoked" }, "BINDING_REVOKED"},
		{"target missing", func(in *ToolCallGateInput) { in.BindingMCPIDs = []string{"other"} }, "BINDING_TARGET_MISSING"},
		{"definition revoked", func(in *ToolCallGateInput) { d := *def; d.Status = "revoked"; in.Definition = &d }, "DEFINITION_REVOKED"},
		{"definition unverified", func(in *ToolCallGateInput) { d := *def; d.Status = "unverified"; in.Definition = &d }, "DEFINITION_UNVERIFIED"},
		{"descriptor missing", func(in *ToolCallGateInput) { in.Descriptor = nil }, "DESCRIPTOR_MISSING"},
		{"digest mismatch", func(in *ToolCallGateInput) { in.Request.DescriptorDigest = "sha256:other" }, "DESCRIPTOR_DIGEST_MISMATCH"},
		{"qualification missing", func(in *ToolCallGateInput) { in.Qualification = nil }, "CALL_NOT_QUALIFIED"},
		{"qualification revoked", func(in *ToolCallGateInput) { q := *qual; q.Status = "revoked"; in.Qualification = &q }, "CALL_QUALIFICATION_REVOKED"},
		{"policy mismatch", func(in *ToolCallGateInput) { q := *qual; q.PolicyRevision = "other"; in.Qualification = &q }, "CALL_NOT_QUALIFIED"},
		{"transport", func(in *ToolCallGateInput) { q := *qual; q.Transport = "streamable_http"; in.Qualification = &q }, "TRANSPORT_NOT_QUALIFIED"},
		{"effectful auth required", func(in *ToolCallGateInput) { q := *qual; q.EffectClass = "effectful"; in.Qualification = &q }, "AUTHORIZATION_REQUIRED"},
	}
	for _, tc := range cases {
		in := base
		in.Request = req
		in.Definition = def
		in.Descriptor = desc
		in.Qualification = qual
		tc.mut(&in)
		got := toolCallGate(in)
		if got.OK || got.Code != tc.code {
			t.Fatalf("%s: got %#v", tc.name, got)
		}
	}
}

func TestToolCallGateSchemaRequired(t *testing.T) {
	desc := &CachedToolDescriptor{Name: "t", InputSchema: json.RawMessage(`{"type":"object","required":["x"],"properties":{"x":{"type":"string"}}}`), Digest: "sha256:abc"}
	req := ToolCallRequest{BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	def := &toolCatalogSource{ID: "mcp1", Status: "candidate"}
	qual := &MCPToolCallQualification{BindingID: "bind1", DefinitionID: "mcp1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", PolicyRevision: toolCallPolicyRevision, Status: "approved", Transport: "stdio", EffectClass: "read"}
	got := toolCallGate(ToolCallGateInput{Request: req, BindingFound: true, BindingStatus: "approved", BindingMCPIDs: []string{"mcp1"}, Definition: def, Descriptor: desc, Qualification: qual})
	if got.OK || got.Code != "SCHEMA_INVALID" {
		t.Fatalf("got %#v", got)
	}
}
