// pattern: Imperative Shell
package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"polis/internal/runner"
	"testing"
)

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
				fmt.Println(`{"method":"warning","params":{"message":"Code Mode is unavailable because failed to spawn code-mode host"}}`)
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
