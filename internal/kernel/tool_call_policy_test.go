// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
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
	httpOK := base
	httpQual := *qual
	httpQual.Transport = "streamable_http"
	httpOK.Qualification = &httpQual
	if got := toolCallGate(httpOK); !got.OK {
		t.Fatalf("expected streamable_http qualified call ok, got %#v", got)
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
	// Effectful/unknown calls no longer require an authorization reference.
	effectful := base
	eq := *qual
	eq.EffectClass = "effectful"
	effectful.Qualification = &eq
	if got := toolCallGate(effectful); !got.OK {
		t.Fatalf("effectful without auth should pass, got %#v", got)
	}
	unknown := base
	uq := *qual
	uq.EffectClass = "unknown"
	unknown.Qualification = &uq
	if got := toolCallGate(unknown); !got.OK {
		t.Fatalf("unknown without auth should pass, got %#v", got)
	}
}

func TestToolCallGateSchemaRequired(t *testing.T) {
	desc := &CachedToolDescriptor{Name: "t", InputSchema: json.RawMessage(`{"type":"object","required":["x"],"properties":{"x":{"type":"string"}}}`), Digest: "sha256:abc"}
	req := ToolCallRequest{BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	def := &toolCatalogSource{ID: "mcp1", Status: "candidate"}
	qual := &MCPToolCallQualification{BindingID: "bind1", DefinitionID: "mcp1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", PolicyRevision: toolCallPolicyRevision, Status: "approved", Transport: "stdio", EffectClass: "read"}
	got := toolCallGate(ToolCallGateInput{Request: req, BindingFound: true, BindingStatus: "approved", BindingMCPIDs: []string{"mcp1"}, Definition: def, Descriptor: desc, Qualification: qual})
	if got.OK || got.Code != "ARGUMENTS_SCHEMA_VIOLATION" {
		t.Fatalf("got %#v", got)
	}
}

func TestToolCallGateAdditionalPropertiesFalse(t *testing.T) {
	desc := &CachedToolDescriptor{Name: "t", InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}},"additionalProperties":false}`), Digest: "sha256:abc"}
	req := ToolCallRequest{BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{"x":"1","y":"2"}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	def := &toolCatalogSource{ID: "mcp1", Status: "candidate"}
	qual := &MCPToolCallQualification{BindingID: "bind1", DefinitionID: "mcp1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", PolicyRevision: toolCallPolicyRevision, Status: "approved", Transport: "stdio", EffectClass: "read"}
	got := toolCallGate(ToolCallGateInput{Request: req, BindingFound: true, BindingStatus: "approved", BindingMCPIDs: []string{"mcp1"}, Definition: def, Descriptor: desc, Qualification: qual})
	if got.OK || got.Code != "ARGUMENTS_SCHEMA_VIOLATION" {
		t.Fatalf("got %#v", got)
	}
}

func TestToolCallGateMalformedSchema(t *testing.T) {
	desc := &CachedToolDescriptor{Name: "t", InputSchema: json.RawMessage(`{"type":"object","required":`), Digest: "sha256:abc"}
	req := ToolCallRequest{BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	def := &toolCatalogSource{ID: "mcp1", Status: "candidate"}
	qual := &MCPToolCallQualification{BindingID: "bind1", DefinitionID: "mcp1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", PolicyRevision: toolCallPolicyRevision, Status: "approved", Transport: "stdio", EffectClass: "read"}
	got := toolCallGate(ToolCallGateInput{Request: req, BindingFound: true, BindingStatus: "approved", BindingMCPIDs: []string{"mcp1"}, Definition: def, Descriptor: desc, Qualification: qual})
	if got.OK || got.Code != "SCHEMA_INVALID" {
		t.Fatalf("got %#v", got)
	}
}

func TestValidateToolCallRequest(t *testing.T) {
	ok := ToolCallRequest{CompanyID: "co1", EmployeeID: "emp-backend", BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{"x":1}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	if _, err := validateToolCallRequest(ok); err != nil {
		t.Fatal(err)
	}
	badHandle := ok
	badHandle.Handle = "bad"
	if _, err := validateToolCallRequest(badHandle); err == nil {
		t.Fatal("expected error")
	}
	badArgs := ok
	badArgs.Arguments = json.RawMessage(`[]`)
	if _, err := validateToolCallRequest(badArgs); !errors.Is(err, core.Malformed) {
		t.Fatalf("got %v", err)
	}
	badJSON := ok
	badJSON.Arguments = json.RawMessage(`{`)
	if _, err := validateToolCallRequest(badJSON); !errors.Is(err, core.Malformed) {
		t.Fatalf("got %v", err)
	}
}

func TestToolCallDigestsStable(t *testing.T) {
	req := ToolCallRequest{CompanyID: "co1", EmployeeID: "emp-backend", BindingID: "bind1", Handle: "mcp1:t", DescriptorDigest: "sha256:abc", Arguments: json.RawMessage(`{"b":2,"a":1}`), IdempotencyKey: "k1", PolicyRevision: toolCallPolicyRevision}
	a := toolCallRequestDigest(req)
	b := toolCallRequestDigest(req)
	if a == "" || a != b {
		t.Fatalf("unstable digest: %q %q", a, b)
	}
	if toolCallArgumentsDigest(json.RawMessage(`{"a":1}`)) == "" {
		t.Fatal("empty args digest")
	}
}

func TestRedactBoundedJSONMasksSensitiveKeys(t *testing.T) {
	raw := json.RawMessage(`{"ok":1,"authorization":"Bearer secret","nested":{"api_key":"k"},"items":[{"token":"t"}]}`)
	out := redactBoundedJSON(raw, 4096)
	s := string(out)
	if strings.Contains(s, "Bearer secret") || strings.Contains(s, `"api_key":"k"`) || strings.Contains(s, `"token":"t"`) {
		t.Fatalf("sensitive values not redacted: %s", s)
	}
	if !strings.Contains(s, "[REDACTED]") || !strings.Contains(s, `"ok":1`) {
		t.Fatalf("unexpected redaction output: %s", s)
	}
}

func TestValidateArgumentsAgainstSchemaSubset(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		args    string
		wantErr bool
	}{
		{"empty schema", `{}`, `{"x":1}`, false},
		{"type string ok", `{"type":"object","properties":{"x":{"type":"string"}}}`, `{"x":"a"}`, false},
		{"type string bad", `{"type":"object","properties":{"x":{"type":"string"}}}`, `{"x":1}`, true},
		{"integer ok", `{"type":"object","properties":{"n":{"type":"integer"}}}`, `{"n":3}`, false},
		{"integer rejects float", `{"type":"object","properties":{"n":{"type":"integer"}}}`, `{"n":3.5}`, true},
		{"enum ok", `{"type":"object","properties":{"m":{"enum":["a","b"]}}}`, `{"m":"a"}`, false},
		{"enum bad", `{"type":"object","properties":{"m":{"enum":["a","b"]}}}`, `{"m":"c"}`, true},
		{"array items", `{"type":"object","properties":{"a":{"type":"array","items":{"type":"string"}}}}`, `{"a":["x","y"]}`, false},
		{"array items bad", `{"type":"object","properties":{"a":{"type":"array","items":{"type":"string"}}}}`, `{"a":["x",1]}`, true},
		{"nested required", `{"type":"object","properties":{"o":{"type":"object","required":["y"],"properties":{"y":{"type":"number"}}}}}`, `{"o":{}}`, true},
		{"pattern ok", `{"type":"object","properties":{"id":{"type":"string","pattern":"^[a-z]+$"}}}`, `{"id":"abc"}`, false},
		{"pattern bad", `{"type":"object","properties":{"id":{"type":"string","pattern":"^[a-z]+$"}}}`, `{"id":"a1"}`, true},
		{"local ref ok", `{"type":"object","properties":{"x":{"$ref":"#/$defs/s"}},"$defs":{"s":{"type":"string","minLength":2}}}`, `{"x":"ab"}`, false},
		{"local ref bad", `{"type":"object","properties":{"x":{"$ref":"#/$defs/s"}},"$defs":{"s":{"type":"string","minLength":2}}}`, `{"x":"a"}`, true},
		{"external ref rejected", `{"type":"object","properties":{"x":{"$ref":"https://example.test/schema"}}}`, `{"x":"a"}`, true},
		{"format uuid ok", `{"type":"object","properties":{"id":{"type":"string","format":"uuid"}}}`, `{"id":"123e4567-e89b-12d3-a456-426614174000"}`, false},
		{"format uuid bad", `{"type":"object","properties":{"id":{"type":"string","format":"uuid"}}}`, `{"id":"not-a-uuid"}`, true},
		{"format date-time ok", `{"type":"object","properties":{"t":{"type":"string","format":"date-time"}}}`, `{"t":"2026-11-08T12:00:00Z"}`, false},
		{"format ipv4 ok", `{"type":"object","properties":{"ip":{"type":"string","format":"ipv4"}}}`, `{"ip":"127.0.0.1"}`, false},
		{"format ipv6 ok", `{"type":"object","properties":{"ip":{"type":"string","format":"ipv6"}}}`, `{"ip":"::1"}`, false},
		{"format unknown passes", `{"type":"object","properties":{"x":{"type":"string","format":"vendor-custom"}}}`, `{"x":"anything"}`, false},
		{"boolean schema false", `false`, `{}`, true},
		{"malformed schema", `{"type":`, `{}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateArgumentsAgainstSchema(json.RawMessage(tc.schema), json.RawMessage(tc.args))
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected %v", err)
			}
			if tc.name == "malformed schema" && !errors.Is(err, core.Malformed) {
				t.Fatalf("expected malformed, got %v", err)
			}
		})
	}
}
