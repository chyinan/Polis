// pattern: Imperative Shell
package codex

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"polis/internal/runner"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/r03a_t1_disconnect_protocol.jsonl
var r03aDisconnectFixture []byte

const r03aFixtureThread = "01a08132-be2d-7ec3-b64e-3532231f9d02"
const r03aFixtureTurn = "01a08132-be80-7e11-97f7-02012cb901d1"

func TestR03AT1ActualDisconnectFixtureIsStructuredAndStable(t *testing.T) {
	var methods []string
	scan := bufio.NewScanner(strings.NewReader(string(r03aDisconnectFixture)))
	for scan.Scan() {
		var envelope struct {
			Data struct {
				Method string `json:"method"`
				Params struct {
					WillRetry bool `json:"willRetry"`
					Error     struct {
						CodexErrorInfo struct {
							ResponseStreamDisconnected json.RawMessage `json:"responseStreamDisconnected"`
						} `json:"codexErrorInfo"`
					} `json:"error"`
				} `json:"params"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scan.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, envelope.Data.Method)
		if envelope.Data.Method == "error" && (!envelope.Data.Params.WillRetry || len(envelope.Data.Params.Error.CodexErrorInfo.ResponseStreamDisconnected) == 0) {
			t.Fatal("fixture lost structured reconnect fields")
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 7 || methods[0] != "turn/started" || methods[1] != "error" || methods[6] != "error" {
		t.Fatalf("fixture event sequence changed: %v", methods)
	}
}

func TestR03AT1WarningClassificationUsesStructuredCode(t *testing.T) {
	if err := warningError(Message{Method: "warning", Params: json.RawMessage(`{"message":"Code Mode is unavailable"}`)}); err != nil {
		t.Fatalf("unstructured warning was treated as terminal: %v", err)
	}
	if err := warningError(Message{Method: "warning", Params: json.RawMessage(`{"code":"code_mode_unavailable"}`)}); err == nil {
		t.Fatal("structured capability warning was not classified")
	}
}

func TestR03AT1DisconnectStopsAtPhaseAwareDeadline(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "disconnect")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "disconnect fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 200 * time.Millisecond, StreamingIdle: 200 * time.Millisecond, ReconnectGrace: 30 * time.Millisecond, Total: 500 * time.Millisecond}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		t.Fatal("disconnect fixture unexpectedly invoked a tool")
		return nil, false
	})
	if err == nil || !strings.Contains(err.Error(), "first_valid_output_deadline_exceeded") {
		t.Fatalf("pre-first-output disconnect was not bounded by first-output deadline: %v", err)
	}
}

func TestR03AT1DuplicateToolCallIsNotDeliveredTwice(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "duplicate")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	_, err := c.Turn(ctx, r03aFixtureThread, "medium", "duplicate fixture", func(name, id string, raw json.RawMessage) (json.RawMessage, bool) {
		calls++
		if name != "workspace_read" || id != "replayed-call" {
			t.Fatalf("unexpected tool identity: %s %s", name, id)
		}
		return []byte(`{"data":"one invocation"}`), false
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("duplicate native tool call reached Polis handler %d times", calls)
	}
}

func TestSkillLoadReplayRechecksCurrentAuthorization(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "skill_duplicate")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	_, err := c.Turn(ctx, r03aFixtureThread, "medium", "skill replay fixture", func(name, id string, raw json.RawMessage) (json.RawMessage, bool) {
		calls++
		if name != "skills_load" || id != "replayed-skill-call" {
			t.Fatalf("unexpected Skill replay identity: %s %s", name, id)
		}
		if calls == 1 {
			return []byte(`{"data":{"content":"first read"}}`), false
		}
		return []byte(`{"error":"DENIED"}`), false
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("a replayed Skill load did not recheck its grant: handler calls=%d want=2", calls)
	}
}

func TestExplicitBusinessToolCallLimitReplacesHiddenThirtyTwoBoundary(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "many")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	_, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "explicit business budget", TurnOptions{ToolCallLimit: 5, Timeouts: TurnTimeouts{FirstValidOutput: time.Second, StreamingIdle: time.Second, Total: time.Second}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		calls++
		return []byte(`{"ok":true}`), false
	})
	if err == nil || !strings.Contains(err.Error(), "business tool-call limit exhausted") || calls != 5 {
		t.Fatalf("explicit business limit was not the terminal boundary: err=%v calls=%d", err, calls)
	}

	c, cleanup = r03aProtocolClient(t, "many")
	defer cleanup()
	calls = 0
	result, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "no hidden thirty-two boundary", TurnOptions{ToolCallLimit: 40, Timeouts: TurnTimeouts{FirstValidOutput: time.Second, StreamingIdle: time.Second, Total: time.Second}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		calls++
		return []byte(`{"ok":true}`), false
	})
	if err != nil || result.State != "completed" || calls != 33 {
		t.Fatalf("explicit limit did not permit calls beyond historical boundary: result=%+v err=%v calls=%d", result, err, calls)
	}
}

func TestR03AT1ReconnectRecoveryResumesSameTurn(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "recover")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	result, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "recovery fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 200 * time.Millisecond, StreamingIdle: 200 * time.Millisecond, ReconnectGrace: 100 * time.Millisecond, Total: 500 * time.Millisecond}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		calls++
		return []byte(`{"data":"recovered"}`), false
	})
	if err != nil || result.State != "completed" || calls != 1 || !hasLifecycle(result, TurnReconnecting) || !hasLifecycle(result, TurnRecovered) {
		t.Fatalf("reconnect recovery was not normal completion: result=%+v err=%v calls=%d", result, err, calls)
	}
}

func TestR03AT1DisconnectAfterToolCallReplaysWithoutDuplicateMutation(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "after_tool")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	result, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "post-tool disconnect fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 300 * time.Millisecond, StreamingIdle: 300 * time.Millisecond, ReconnectGrace: 300 * time.Millisecond, Total: 800 * time.Millisecond}}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
		calls++
		return []byte(`{"data":"one durable action"}`), false
	})
	if err != nil || result.State != "completed" || calls != 1 || !hasLifecycle(result, TurnRecovered) {
		t.Fatalf("post-tool reconnect was not idempotent recovery: result=%+v err=%v calls=%d", result, err, calls)
	}
}

func TestR03AT1ExplicitTerminalProviderFailureIsNotReconnect(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "terminal")
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "terminal fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 200 * time.Millisecond, StreamingIdle: 200 * time.Millisecond, ReconnectGrace: 100 * time.Millisecond, Total: 500 * time.Millisecond}}, func(string, string, json.RawMessage) (json.RawMessage, bool) { return []byte(`{}`), false })
	var terminal ProviderTerminalError
	if !errors.As(err, &terminal) {
		t.Fatalf("explicit terminal provider event was misclassified: %v", err)
	}
}

func TestR03AT1StopDuringReconnectRequiresTerminationConfirmation(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "stop")
	defer cleanup()
	stop := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct {
		result TurnResult
		err    error
	}, 1)
	go func() {
		result, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "stop fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 500 * time.Millisecond, StreamingIdle: 500 * time.Millisecond, ReconnectGrace: 200 * time.Millisecond, Total: 800 * time.Millisecond, StopAcknowledgement: 100 * time.Millisecond}, Stop: stop}, func(string, string, json.RawMessage) (json.RawMessage, bool) { return []byte(`{}`), false })
		done <- struct {
			result TurnResult
			err    error
		}{result, err}
	}()
	time.Sleep(30 * time.Millisecond)
	close(stop)
	outcome := <-done
	if outcome.err != nil || outcome.result.State != "stopped" || !outcome.result.StopAcknowledged || !outcome.result.TerminationConfirmed || !hasLifecycle(outcome.result, TurnStopRequested) || !hasLifecycle(outcome.result, TurnTerminationConfirmed) {
		t.Fatalf("stop during reconnect was not confirmed: result=%+v err=%v", outcome.result, outcome.err)
	}
}

func TestR03AT21StopDuringPostOutputReconnectRequiresTerminationConfirmation(t *testing.T) {
	c, cleanup := r03aProtocolClient(t, "stop_post")
	defer cleanup()
	stop := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct {
		result TurnResult
		err    error
	}, 1)
	go func() {
		result, err := c.TurnWithOptions(ctx, r03aFixtureThread, "medium", "post-output stop fixture", TurnOptions{Timeouts: TurnTimeouts{FirstValidOutput: 500 * time.Millisecond, StreamingIdle: 500 * time.Millisecond, ReconnectGrace: 500 * time.Millisecond, Total: 1200 * time.Millisecond, StopAcknowledgement: 200 * time.Millisecond}, Stop: stop}, func(string, string, json.RawMessage) (json.RawMessage, bool) {
			return []byte(`{"data":"output"}`), false
		})
		done <- struct {
			result TurnResult
			err    error
		}{result, err}
	}()
	time.Sleep(30 * time.Millisecond)
	close(stop)
	outcome := <-done
	if outcome.err != nil || outcome.result.State != "stopped" || !outcome.result.StopAcknowledged || !outcome.result.TerminationConfirmed || !hasLifecycle(outcome.result, TurnRecovered) || !hasLifecycle(outcome.result, TurnStopRequested) || !hasLifecycle(outcome.result, TurnTerminationConfirmed) {
		t.Fatalf("post-output stop was not confirmed: result=%+v err=%v", outcome.result, outcome.err)
	}
}

func hasLifecycle(result TurnResult, state TurnLifecycleState) bool {
	for _, event := range result.Lifecycle {
		if event.State == state {
			return true
		}
	}
	return false
}

func r03aProtocolClient(t *testing.T, mode string) (*Client, func()) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "POLIS_T1_PROTOCOL_HELPER=1", "POLIS_T1_MODE="+mode)
	p, err := runner.Start("r03a-t1-"+mode, []string{exe, "-test.run=^TestR03AT1ProtocolHelper$"}, env)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewWithModel(p, t.TempDir(), "gpt-5.6-luna")
	if err != nil {
		_, _ = p.Stop()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = c.Initialize(ctx); err != nil {
		c.Close()
		t.Fatal(err)
	}
	if _, err = c.StartThread(ctx, "medium"); err != nil {
		c.Close()
		t.Fatal(err)
	}
	return c, func() { c.Close() }
}

func TestR03AT1ProtocolHelper(t *testing.T) {
	if os.Getenv("POLIS_T1_PROTOCOL_HELPER") != "1" {
		return
	}
	mode := os.Getenv("POLIS_T1_MODE")
	postToolDisconnectSent := false
	manyResponses := 0
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		var msg Message
		if json.Unmarshal(scan.Bytes(), &msg) != nil {
			continue
		}
		switch msg.Method {
		case "initialize":
			fmt.Printf("{\"id\":%s,\"result\":{\"userAgent\":\"codex-cli 0.151.0\"}}\n", msg.ID)
		case "thread/start":
			fmt.Printf("{\"id\":%s,\"result\":{\"thread\":{\"id\":\"%s\"},\"model\":\"gpt-5.6-luna\",\"reasoningEffort\":\"medium\",\"approvalPolicy\":\"never\",\"sandbox\":{\"type\":\"readOnly\"}}}\n", msg.ID, r03aFixtureThread)
		case "turn/start":
			fmt.Printf("{\"id\":%s,\"result\":{\"turn\":{\"id\":\"%s\"}}}\n", msg.ID, r03aFixtureTurn)
			fmt.Printf("{\"method\":\"turn/started\",\"params\":{\"threadId\":\"%s\",\"turn\":{\"id\":\"%s\",\"status\":\"inProgress\"}}}\n", r03aFixtureThread, r03aFixtureTurn)
			switch mode {
			case "disconnect":
				for _, line := range strings.Split(strings.TrimSpace(string(r03aDisconnectFixture)), "\n")[1:] {
					var envelope struct {
						Data json.RawMessage `json:"data"`
					}
					_ = json.Unmarshal([]byte(line), &envelope)
					fmt.Println(string(envelope.Data))
				}
			case "duplicate":
				for i := 0; i < 2; i++ {
					fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-call","tool":"polis_workspace_read","arguments":{}}}`)
				}
			case "skill_duplicate":
				for i := 0; i < 2; i++ {
					fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-skill-call","tool":"polis_skills_load","arguments":{"skill_id":"skill","relative_path":"SKILL.md"}}}`)
				}
			case "recover":
				fmt.Println(`{"method":"error","params":{"error":{"codexErrorInfo":{"responseStreamDisconnected":{"httpStatusCode":null}}},"willRetry":true,"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1"}}`)
				fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-call","tool":"polis_workspace_read","arguments":{}}}`)
			case "after_tool":
				fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-call","tool":"polis_workspace_read","arguments":{}}}`)
			case "terminal":
				fmt.Println(`{"method":"error","params":{"error":{"codexErrorInfo":{"explicitTerminalFailure":true}},"willRetry":false,"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1"}}`)
			case "stop":
				fmt.Println(`{"method":"error","params":{"error":{"codexErrorInfo":{"responseStreamDisconnected":{"httpStatusCode":null}}},"willRetry":true,"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1"}}`)
			case "stop_post":
				fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-call","tool":"polis_workspace_read","arguments":{}}}`)
			case "many":
				for i := 0; i < 33; i++ {
					fmt.Printf(`{"id":%d,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"many-%d","tool":"polis_workspace_read","arguments":{}}}`+"\n", 100+i, i)
				}
			}
		case "":
			if msg.ID != nil {
				if mode == "many" {
					manyResponses++
					if manyResponses == 33 {
						fmt.Println(`{"method":"turn/completed","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turn":{"id":"01a08132-be80-7e11-97f7-02012cb901d1","status":"completed"}}}`)
					}
				} else if mode == "after_tool" && !postToolDisconnectSent {
					postToolDisconnectSent = true
					fmt.Println(`{"method":"error","params":{"error":{"codexErrorInfo":{"responseStreamDisconnected":{"httpStatusCode":null}}},"willRetry":true,"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1"}}`)
					fmt.Println(`{"id":99,"method":"item/tool/call","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1","callId":"replayed-call","tool":"polis_workspace_read","arguments":{}}}`)
				} else if mode == "stop_post" && !postToolDisconnectSent {
					postToolDisconnectSent = true
					fmt.Println(`{"method":"error","params":{"error":{"codexErrorInfo":{"responseStreamDisconnected":{"httpStatusCode":null}}},"willRetry":true,"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turnId":"01a08132-be80-7e11-97f7-02012cb901d1"}}`)
				} else {
					fmt.Println(`{"method":"turn/completed","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turn":{"id":"01a08132-be80-7e11-97f7-02012cb901d1","status":"completed"}}}`)
				}
			}
		case "turn/interrupt":
			fmt.Printf("{\"id\":%s,\"result\":{\"accepted\":true}}\n", msg.ID)
			fmt.Println(`{"method":"turn/completed","params":{"threadId":"01a08132-be2d-7ec3-b64e-3532231f9d02","turn":{"id":"01a08132-be80-7e11-97f7-02012cb901d1","status":"interrupted"}}}`)
		}
	}
}
