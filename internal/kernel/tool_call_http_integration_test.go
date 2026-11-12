package kernel

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildFakeHTTPMCP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "httpmcp")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	pkg := filepath.Join(filepath.Dir(file), "testdata", "httpmcp")
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = os.Environ()
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake http mcp: %v\n%s", err, b)
	}
	return out
}

func startFakeHTTPMCP(t *testing.T, bin string) (endpoint string, stop func()) {
	t.Helper()
	cmd := exec.Command(bin, "127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	addr := strings.TrimSpace(line)
	endpoint = "http://" + addr + "/mcp"
	stop = func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	// brief readiness wait
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		// probe attempt will validate; server should be ready immediately after printing addr
		break
	}
	return endpoint, stop
}

func TestMCPStreamableHTTPToolCallGateAndExecution(t *testing.T) {
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
	bin := buildFakeHTTPMCP(t)
	endpoint, stop := startFakeHTTPMCP(t, bin)
	defer stop()

	scope, err := k.TXCreateCompany(ctx, "tool-call-http-co")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO employees(company_id,id,display_name,role_name,model_profile) VALUES($1,'emp-backend','Backend','backend','deterministic/fake') ON CONFLICT (company_id,id) DO NOTHING`, scope.company); err != nil {
		t.Fatal(err)
	}

	def, err := k.TXRegisterMCPServerDefinition(ctx, scope.company, MCPServerDefinitionInput{ID: "mcp-http", Name: "http", Transport: "streamable_http", Endpoint: &endpoint, DescriptorDigest: "sha256:" + repeatHex("cd", 32)})
	if err != nil {
		t.Fatal(err)
	}
	def, err = k.QualifyRegisteredMCP(ctx, scope.company, def.ID)
	if err != nil {
		t.Fatalf("qualify http mcp: %v", err)
	}
	if def.Status != "candidate" || len(def.DescriptorDigest) == 0 {
		t.Fatalf("expected qualified candidate definition, got %+v", def)
	}

	binding, err := k.TXUpsertEmployeeCapabilityBinding(ctx, scope.company, CapabilityBindingInput{EmployeeID: "emp-backend", MCPDefinitionIDs: []string{def.ID}, Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}

	// Discover cached descriptor for echo.
	var qualRaw []byte
	if err = k.pool.QueryRow(ctx, `SELECT qualification FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, scope.company, def.ID).Scan(&qualRaw); err != nil {
		t.Fatal(err)
	}
	desc, _, derr := cachedDescriptorFromQualification(qualRaw, "echo")
	if derr != nil || desc == nil {
		t.Fatalf("expected cached echo descriptor, err=%v desc=%v", derr, desc)
	}

	base := ToolCallRequest{CompanyID: scope.company, EmployeeID: "emp-backend", BindingID: binding.ID, Handle: def.ID + ":echo", DescriptorDigest: desc.Digest, Arguments: json.RawMessage(`{"text":"hello-http"}`), IdempotencyKey: "http-k1", PolicyRevision: "r15-tools-call@1", AuthorizationRef: "auth-op"}

	// Deny: no explicit call qualification.
	denied, err := k.TXCallMCPTool(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if denied.Outcome != "denied" || denied.Code != "CALL_NOT_QUALIFIED" {
		t.Fatalf("expected CALL_NOT_QUALIFIED, got %+v", denied)
	}

	// Approve explicit call qualification (streamable_http, read/no auth required).
	qual, err := k.TXUpsertMCPToolCallQualification(ctx, scope.company, MCPToolCallQualificationInput{BindingID: binding.ID, DefinitionID: def.ID, Handle: def.ID + ":echo", DescriptorDigest: desc.Digest, PolicyRevision: "r15-tools-call@1", EffectClass: "read", Status: "approved", AuthorizationRef: "auth-op", QualifiedBy: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if qual.Status != "approved" || qual.Transport != "streamable_http" || qual.Handle != def.ID+":echo" || qual.DescriptorDigest != desc.Digest || qual.PolicyRevision != "r15-tools-call@1" || qual.BindingID != binding.ID || qual.DefinitionID != def.ID {
		t.Fatalf("unexpected qualification: %+v", qual)
	}


	approvedReq := base
	approvedReq.IdempotencyKey = "http-k1-approved"
	okRec, err := k.TXCallMCPTool(ctx, approvedReq)
	if err != nil {
		t.Fatalf("http tool call: %v", err)
	}
	if okRec.Outcome != "completed" || okRec.Code != "" {
		t.Fatalf("expected completed http call, got %+v", okRec)
	}
	if okRec.ResultDigest == "" || len(okRec.ResultJSON) == 0 {
		t.Fatalf("expected result evidence, got %+v", okRec)
	}
	if !strings.Contains(string(okRec.EgressObserved), "streamable_http") || !strings.Contains(string(okRec.EgressObserved), "localOnly") {
		t.Fatalf("expected local streamable_http egress evidence, got %s", string(okRec.EgressObserved))
	}
	if !strings.Contains(string(okRec.ProcessObserved), "streamable_http") {
		t.Fatalf("expected streamable_http process evidence, got %s", string(okRec.ProcessObserved))
	}

	// Replay by idempotency key.
	replay, err := k.TXCallMCPTool(ctx, approvedReq)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != okRec.ID || replay.RequestDigest != okRec.RequestDigest {
		t.Fatalf("expected idempotent replay, got ok=%+v replay=%+v", okRec, replay)
	}

	// Sandbox: non-local endpoint must be denied by transport guard (no external egress).
	badEndpoint := "http://example.com/mcp"
	badDef, err := k.TXRegisterMCPServerDefinition(ctx, scope.company, MCPServerDefinitionInput{ID: "mcp-http-bad", Name: "bad", Transport: "streamable_http", Endpoint: &badEndpoint, DescriptorDigest: "sha256:" + repeatHex("ef", 32)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.QualifyRegisteredMCP(ctx, scope.company, badDef.ID); err == nil {
		t.Fatal("expected non-local endpoint qualification to be denied")
	}
}

