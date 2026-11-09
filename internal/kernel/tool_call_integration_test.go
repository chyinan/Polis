package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"polis/internal/core"
)

func buildFakeToolMCP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "toolcallmcp")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	pkg := filepath.Join(filepath.Dir(file), "testdata", "toolcallmcp")
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = os.Environ()
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake mcp: %v\n%s", err, b)
	}
	return out
}

func TestMCPToolCallGateAndExecution(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	fakeCmd := buildFakeToolMCP(t)

	scope, err := k.TXCreateCompany(ctx, "tool-call-co")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO employees(company_id,id,display_name,role_name,model_profile) VALUES($1,'emp-backend','Backend','backend','deterministic/fake') ON CONFLICT (company_id,id) DO NOTHING`, scope.company); err != nil {
		t.Fatal(err)
	}
	def, err := k.TXRegisterMCPServerDefinition(ctx, scope.company, MCPServerDefinitionInput{ID: "mcp-local", Name: "local", Transport: "stdio", Command: &fakeCmd, Args: []string{}, DescriptorDigest: "sha256:" + repeatHex("ab", 32)})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := freezeToolDescriptor("echo", "Echo", json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	qual, err := json.Marshal(map[string]any{
		"probeStatus": "passed", "toolNames": []string{"echo"}, "toolsCalled": 0,
		"toolDescriptors": []CachedToolDescriptor{frozen},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_server_definitions SET status='candidate', descriptor_digest=$1, qualification=$2 WHERE company_id=$3 AND id=$4`, frozen.Digest, qual, scope.company, def.ID); err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXUpsertEmployeeCapabilityBinding(ctx, scope.company, CapabilityBindingInput{EmployeeID: "emp-backend", MCPDefinitionIDs: []string{"mcp-local"}, Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}

	base := ToolCallRequest{CompanyID: scope.company, EmployeeID: "emp-backend", BindingID: binding.ID, Handle: "mcp-local:echo", DescriptorDigest: frozen.Digest, Arguments: json.RawMessage(`{"x":"1"}`), IdempotencyKey: "k-denied", PolicyRevision: toolCallPolicyRevision}

	denied, err := k.TXCallMCPTool(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if denied.Outcome != "denied" || denied.Code != "CALL_NOT_QUALIFIED" {
		t.Fatalf("expected CALL_NOT_QUALIFIED, got %+v", denied)
	}

	_, err = k.TXUpsertMCPToolCallQualification(ctx, scope.company, MCPToolCallQualificationInput{BindingID: binding.ID, DefinitionID: "mcp-local", Handle: "mcp-local:echo", DescriptorDigest: frozen.Digest, PolicyRevision: toolCallPolicyRevision, EffectClass: "read", Status: "approved", AuthorizationRef: "op-1"})
	if err != nil {
		t.Fatal(err)
	}
	okReq := base
	okReq.IdempotencyKey = "k-ok"
	receipt, err := k.TXCallMCPTool(ctx, okReq)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != "completed" || receipt.Code != "" {
		t.Fatalf("expected completed, got %+v", receipt)
	}
	if receipt.ResultDigest == "" || len(receipt.ResultJSON) == 0 {
		t.Fatalf("missing result: %+v", receipt)
	}
	replay, err := k.TXCallMCPTool(ctx, okReq)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != receipt.ID || replay.Outcome != "completed" {
		t.Fatalf("idempotent replay mismatch: %+v vs %+v", receipt, replay)
	}

	bad := base
	bad.IdempotencyKey = "k-bad"
	bad.DescriptorDigest = "sha256:" + repeatHex("cd", 32)
	mismatch, err := k.TXCallMCPTool(ctx, bad)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Outcome != "denied" || mismatch.Code != "DESCRIPTOR_DIGEST_MISMATCH" {
		t.Fatalf("expected digest mismatch, got %+v", mismatch)
	}

	schemaBad := base
	schemaBad.IdempotencyKey = "k-schema-bad"
	schemaBad.Arguments = json.RawMessage(`{"x":1}`)
	schemaDenied, err := k.TXCallMCPTool(ctx, schemaBad)
	if err != nil {
		t.Fatal(err)
	}
	if schemaDenied.Outcome != "denied" || schemaDenied.Code != "ARGUMENTS_SCHEMA_VIOLATION" {
		t.Fatalf("expected ARGUMENTS_SCHEMA_VIOLATION, got %+v", schemaDenied)
	}
	if len(schemaDenied.ErrorJSON) == 0 || !strings.Contains(string(schemaDenied.ErrorJSON), "$.x") {
		t.Fatalf("expected bounded schema violation detail, got %s", string(schemaDenied.ErrorJSON))
	}

	if _, err = k.pool.Exec(ctx, `INSERT INTO employees(company_id,id,display_name,role_name,model_profile) VALUES($1,'emp-frontend','Frontend','frontend','deterministic/fake') ON CONFLICT (company_id,id) DO NOTHING`, scope.company); err != nil {
		t.Fatal(err)
	}
	if err = k.TXCreateMission(ctx, scope, "mission-tc"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,'task-owned-by-frontend','mission-tc','emp-frontend','bootstrap_plan','ready') ON CONFLICT DO NOTHING`, scope.company); err != nil {
		t.Logf("skip task owner mismatch setup due to schema constraints: %v", err)
	} else {
		ownerMismatch := base
		ownerMismatch.IdempotencyKey = "k-owner-mismatch"
		ownerMismatch.TaskID = "task-owned-by-frontend"
		_, err = k.TXCallMCPTool(ctx, ownerMismatch)
		if !errors.Is(err, core.Denied) {
			t.Fatalf("expected core.Denied for task owner mismatch, got %v", err)
		}
	}
}

func repeatHex(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
