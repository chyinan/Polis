// pattern: Imperative Shell
package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/runner"
	"strings"
	"testing"
	"time"
)

const (
	t10FixtureThread = "01a086cb-dae3-7252-b4eb-c78b39a2189a"
	t10FixtureTurn   = "01a086cb-dae3-7252-b4eb-c78b39a2189b"
)

func TestR03AT10Codex1534NativeCompatibilityWithoutProvider(t *testing.T) {
	binary := os.Getenv("POLIS_CODEX_T10_BINARY")
	if binary == "" {
		t.Skip("independent 0.153.4 Linux binary required")
	}
	root := t.TempDir()
	if persistentRoot := os.Getenv("POLIS_T10_EVIDENCE"); persistentRoot != "" {
		if err := os.MkdirAll(persistentRoot, 0700); err != nil {
			t.Fatal(err)
		}
		root = persistentRoot
	}
	home := filepath.Join(root, "home")
	args, _, err := runner.NativeArgs(binary, home, "")
	if err != nil {
		t.Fatal(err)
	}

	helperArgs := append([]string(nil), args...)
	helperArgs[len(helperArgs)-3] = "/codex-code-mode-host"
	helperArgs[len(helperArgs)-2] = "--help"
	helperArgs = helperArgs[:len(helperArgs)-1]
	if _, err = runner.Run(helperArgs, []string{"PATH=/usr/bin:/bin"}, 10*time.Second); err != nil {
		t.Fatalf("0.153.4 code-mode host cannot execute in unchanged bwrap envelope: %v", err)
	}

	process, err := runner.Start("r03a-t10-native-compatibility", args, []string{"PATH=/usr/bin:/bin"})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewWithModelAndVersion(process, filepath.Join(root, "evidence"), "gpt-5.6-luna", "0.153.4")
	if err != nil {
		_, _ = process.Stop()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = client.Initialize(ctx); err != nil {
		client.Close()
		t.Fatal(err)
	}
	if _, err = client.StartThreadWithTools(ctx, "medium", Tools(), "T10 local no-provider registration qualification."); err != nil {
		client.Close()
		t.Fatal(err)
	}
	client.Close()
	proof, err := process.Stop()
	if err != nil || !proof.For("r03a-t10-native-compatibility") {
		t.Fatalf("0.153.4 app-server termination was not confirmed: proof=%s err=%v", proof.Description(), err)
	}

	protocol, err := os.ReadFile(filepath.Join(root, "evidence", "protocol.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"initialize", "initialized", "thread/start"} {
		if !strings.Contains(string(protocol), `"`+method+`"`) {
			t.Fatalf("native protocol log lacks %s: %s", method, protocol)
		}
	}
	if !strings.Contains(string(protocol), `"dynamicTools"`) {
		t.Fatal("native protocol log lacks dynamic-tool registration payload")
	}
	var line map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(string(protocol)), "\n") {
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatal(err)
		}
		if line["direction"] == "receive" {
			data, _ := line["data"].(map[string]any)
			if data["method"] == "thread/started" {
				return
			}
		}
	}
	t.Fatal("native protocol log lacks thread/started")
}

func TestR03AT10Codex1534StructuredReconnectLifecycle(t *testing.T) {
	if os.Getenv("POLIS_T10_PROTOCOL_HELPER") == "1" {
		t10ProtocolHelper(t)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start("r03a-t10-structured-reconnect", []string{exe, "-test.run=^TestR03AT10Codex1534StructuredReconnectLifecycle$"}, append(os.Environ(), "POLIS_T10_PROTOCOL_HELPER=1"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewWithModelAndVersion(process, t.TempDir(), "gpt-5.6-luna", "0.153.4")
	if err != nil {
		_, _ = process.Stop()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = client.Initialize(ctx); err != nil {
		client.Close()
		t.Fatal(err)
	}
	thread, err := client.StartThread(ctx, "medium")
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	result, err := client.TurnWithOptions(ctx, thread, "medium", "T10 local structured reconnect fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 200 * time.Millisecond, StreamingIdle: 200 * time.Millisecond, ReconnectGrace: 100 * time.Millisecond, Total: 500 * time.Millisecond}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		t.Fatal("0.153.4 structured reconnect fixture unexpectedly invoked a tool")
		return nil, false
	})
	client.Close()
	if err != nil || result.State != "completed" || !hasLifecycle(result, TurnReconnecting) || !hasLifecycle(result, TurnRecovered) {
		t.Fatalf("0.153.4 structured reconnect lifecycle was not recovered: result=%+v err=%v", result, err)
	}
	proof, err := process.Stop()
	if err != nil || !proof.For("r03a-t10-structured-reconnect") {
		t.Fatalf("structured reconnect helper termination was not confirmed: proof=%s err=%v", proof.Description(), err)
	}
}

func t10ProtocolHelper(t *testing.T) {
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		var message Message
		if err := json.Unmarshal(scan.Bytes(), &message); err != nil {
			continue
		}
		switch message.Method {
		case "initialize":
			fmt.Printf("{\"id\":%s,\"result\":{\"userAgent\":\"polis/0.153.4 (Ubuntu 22.4.0; x86_64)\",\"codexHome\":\"/home/codex\",\"platformFamily\":\"unix\",\"platformOs\":\"linux\"}}\n", message.ID)
		case "thread/start":
			fmt.Printf("{\"id\":%s,\"result\":{\"thread\":{\"id\":\"%s\",\"model\":\"gpt-5.6-luna\",\"reasoningEffort\":\"medium\",\"status\":{\"type\":\"idle\"}},\"model\":\"gpt-5.6-luna\",\"reasoningEffort\":\"medium\",\"approvalPolicy\":\"never\",\"sandbox\":{\"type\":\"readOnly\"}}}\n", message.ID, t10FixtureThread)
			fmt.Printf("{\"method\":\"thread/started\",\"params\":{\"thread\":{\"id\":\"%s\",\"model\":\"gpt-5.6-luna\",\"reasoningEffort\":\"medium\",\"status\":{\"type\":\"idle\"}}}}\n", t10FixtureThread)
		case "turn/start":
			fmt.Printf("{\"id\":%s,\"result\":{\"turn\":{\"id\":\"%s\"}}}\n", message.ID, t10FixtureTurn)
			fmt.Printf("{\"method\":\"turn/started\",\"params\":{\"threadId\":\"%s\",\"turn\":{\"id\":\"%s\",\"items\":[],\"itemsView\":\"notLoaded\",\"status\":\"inProgress\",\"error\":null,\"startedAt\":1788967902,\"completedAt\":null,\"durationMs\":null}}}\n", t10FixtureThread, t10FixtureTurn)
			fmt.Printf("{\"method\":\"error\",\"params\":{\"error\":{\"message\":\"Reconnecting... waiting for network\",\"codexErrorInfo\":{\"responseStreamDisconnected\":{\"httpStatusCode\":null}},\"additionalDetails\":\"local structured fixture\",\"misalignment\":null},\"willRetry\":true,\"threadId\":\"%s\",\"turnId\":\"%s\"}}\n", t10FixtureThread, t10FixtureTurn)
			fmt.Printf("{\"method\":\"item/agentMessage/delta\",\"params\":{\"threadId\":\"%s\",\"turnId\":\"%s\",\"itemId\":\"item-1\",\"delta\":\"ok\"}}\n", t10FixtureThread, t10FixtureTurn)
			fmt.Printf("{\"method\":\"turn/completed\",\"params\":{\"threadId\":\"%s\",\"turn\":{\"id\":\"%s\",\"status\":\"completed\",\"error\":null}}}\n", t10FixtureThread, t10FixtureTurn)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
}
