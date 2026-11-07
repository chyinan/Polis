// pattern: Imperative Shell
package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/runner"
	"strings"
	"testing"
	"time"
)

func TestInitializeExitBeforeProtocolHelper(t *testing.T) {
	if os.Getenv("POLIS_INITIALIZE_EXIT_HELPER") == "1" {
		os.Exit(17)
	}
}

func TestInitializeFailureCapturesSafeLifecycleEvidence(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := runner.Start("initialize-evidence", []string{exe, "-test.run=^TestInitializeExitBeforeProtocolHelper$"}, append(os.Environ(), "POLIS_INITIALIZE_EXIT_HELPER=1"))
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := t.TempDir()
	client, err := NewWithModelAndVersion(p, evidenceRoot, "gpt-5.6-luna", runner.NativeVersionNumber)
	if err != nil {
		_, _ = p.Stop()
		t.Fatal(err)
	}
	defer client.Close()
	err = client.InitializeWithPolicy(context.Background(), DefaultTransportPolicy())
	var failure *InitializationFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Initialize error=%v, want structured InitializationFailure", err)
	}
	if failure.Phase != "initialize" || !failure.ProcessCreated || !failure.PIDPresent || !failure.ChildExitObserved || failure.InitializeAckReceived || failure.SafeMessage == "" {
		t.Fatalf("unexpected initialization lifecycle evidence: %+v", failure)
	}
	if failure.StderrCategory == "raw" || failure.SafeMessage == err.Error() && failure.SafeMessage != "child exited before initialize" {
		t.Fatalf("unsafe initialization evidence: %+v", failure)
	}
	raw, readErr := os.ReadFile(filepath.Join(evidenceRoot, "protocol.jsonl"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, phase := range []string{"initialize_prepare_started", "initialize_payload_ready"} {
		if !strings.Contains(string(raw), phase) {
			t.Fatalf("lifecycle phase %q missing from safe protocol evidence: %s", phase, raw)
		}
	}
}

func TestScriptedProtocolHelper(t *testing.T) {
	if os.Getenv("POLIS_PROTOCOL_HELPER") != "1" {
		return
	}
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		var m Message
		_ = json.Unmarshal(s.Bytes(), &m)
		if m.Method == "initialize" {
			fmt.Printf("{\"id\":%s,\"result\":{\"userAgent\":\"codex-cli 0.151.0\"}}\n", m.ID)
		}
		if m.Method == "thread/start" {
			if os.Getenv("POLIS_WARNING_HELPER") == "1" {
				fmt.Println(`{"method":"warning","params":{"code":"code_mode_unavailable","message":"Code Mode is unavailable because failed to spawn code-mode host"}}`)
			}
			fmt.Printf("{\"id\":%s,\"result\":{\"thread\":{\"id\":\"native-thread\"},\"model\":\"gpt-5.6-sol\",\"reasoningEffort\":\"medium\",\"approvalPolicy\":\"never\",\"sandbox\":{\"type\":\"readOnly\"}}}\n", m.ID)
		}
		if m.Method == "turn/start" {
			fmt.Printf("{\"id\":%s,\"result\":{\"turn\":{\"id\":\"turn-a\"}}}\n", m.ID)
			fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"foreign-thread","turnId":"turn-a","callId":"call-a","tool":"polis_workspace_replace","arguments":{}}}`)
		}
	}
	os.Exit(0)
}

func TestNativeCapabilityFailureStopsBeforeTurn(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	p, e := runner.Start("warning", []string{exe, "-test.run=^TestScriptedProtocolHelper$"}, append(os.Environ(), "POLIS_PROTOCOL_HELPER=1", "POLIS_WARNING_HELPER=1"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stop()
	c, e := New(p, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e = c.Initialize(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = c.StartThread(context.Background(), "medium"); e == nil {
		t.Fatal("unavailable tool host was treated as qualified")
	}
}
func TestForeignNativeCorrelationNeverReachesKernel(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	p, e := runner.Start("test", []string{exe, "-test.run=^TestScriptedProtocolHelper$"}, append(os.Environ(), "POLIS_PROTOCOL_HELPER=1"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stop()
	c, e := New(p, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	ctx := context.Background()
	if e = c.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	thread, e := c.StartThread(ctx, "medium")
	if e != nil {
		t.Fatal(e)
	}
	called := false
	_, e = c.Turn(ctx, thread, "medium", "test", func(string, string, json.RawMessage) (json.RawMessage, bool) { called = true; return nil, false })
	if e == nil || called {
		t.Fatal("foreign native identity reached worker")
	}
}
func TestBudgetPersistsReservations(t *testing.T) {
	path := t.TempDir() + "/budget.json"
	b, e := NewBudget(path)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e = b.Reserve("medium"); e != nil {
			t.Fatal(e)
		}
	}
	if e = b.Reserve("medium"); e == nil {
		t.Fatal("profile cap exceeded")
	}
	if _, e = NewBudget(path); e == nil {
		t.Fatal("restart reset allowance")
	}
}

func TestReducedRetestBudgetDoesNotExpandToDefaults(t *testing.T) {
	b, e := NewBudget(t.TempDir()+"/budget.json", 2, 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"medium", "medium", "high"} {
		if e = b.Reserve(p); e != nil {
			t.Fatal(e)
		}
	}
	if e = b.Reserve("medium"); e == nil {
		t.Fatal("medium cap expanded")
	}
	if e = b.Reserve("high"); e == nil {
		t.Fatal("high cap expanded")
	}
}

func TestBackendOnlyBudgetAllowsZeroHighTurns(t *testing.T) {
	b, e := NewBudget(t.TempDir()+"/backend-only.json", 1, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = b.Reserve("medium"); e != nil {
		t.Fatal(e)
	}
	if e = b.Reserve("high"); e == nil {
		t.Fatal("backend-only boundary unexpectedly reserved High")
	}
}

func TestPersistedBudgetKeepsOriginalDeadline(t *testing.T) {
	path := t.TempDir() + "/expired.json"
	started := time.Now().UTC().Add(-11 * time.Minute)
	raw, _ := json.Marshal(Budget{Started: started, MediumLimit: 2, HighLimit: 1})
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	b, e := OpenBudget(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = b.Reserve("high"); e == nil {
		t.Fatal("expired budget regained a fresh window")
	}
}
