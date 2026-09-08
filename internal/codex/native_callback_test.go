// pattern: Imperative Shell
package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"polis/internal/runner"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The provider is a deterministic local SSE fixture, not a model. The real
// pinned app-server and code-mode host must dispatch a dynamic tool through it.
func TestNativeDynamicCallbackWithScriptedProvider(t *testing.T) {
	binary := os.Getenv("POLIS_CODEX_BINARY")
	if binary == "" {
		t.Skip("pinned native binary required")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/responses") {
			http.Error(w, "scripted provider only", 404)
			return
		}
		n := requests.Add(1)
		if n > 2 {
			http.Error(w, "fixture request limit", 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(event string, data map[string]any) {
			data["type"] = event
			b, _ := json.Marshal(data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
			w.(http.Flusher).Flush()
		}
		id := fmt.Sprintf("resp_fixture_%d", n)
		emit("response.created", map[string]any{"response": map[string]any{"id": id, "status": "in_progress", "output": []any{}}})
		var item map[string]any
		if n == 1 {
			item = map[string]any{"id": "fc_fixture", "type": "function_call", "call_id": "call_fixture", "name": "polis_work_current", "arguments": "{}"}
			emit("response.output_item.added", map[string]any{"output_index": 0, "item": item})
		} else {
			item = map[string]any{"id": "msg_fixture", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "scripted provider completed", "annotations": []any{}}}}
			emit("response.output_item.added", map[string]any{"output_index": 0, "item": item})
		}
		emit("response.output_item.done", map[string]any{"output_index": 0, "item": item})
		emit("response.completed", map[string]any{"response": map[string]any{"id": id, "status": "completed", "output": []any{item}, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}}})
	}))
	defer server.Close()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	args, _, e := runner.NativeArgs(binary, home, "")
	if e != nil {
		t.Fatal(e)
	}
	config := strings.Replace(runner.NativeConfig, "model_provider = \"polis-openai\"", "model_provider = \"polis-fixture\"", 1)
	config += fmt.Sprintf("\n[model_providers.polis-fixture]\nname = \"ScriptedNoModel\"\nbase_url = %q\nrequires_openai_auth = false\nrequest_max_retries = 0\nstream_max_retries = 0\nsupports_websockets = false\n", server.URL)
	if e = os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); e != nil {
		t.Fatal(e)
	}
	p, e := runner.Start("native-fixture", args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stop()
	c, e := New(p, filepath.Join(root, "evidence"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = c.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	thread, e := c.StartThread(ctx, "medium")
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	result, e := c.Turn(ctx, thread, "medium", "scripted transport fixture", func(name, id string, raw json.RawMessage) (json.RawMessage, bool) {
		if name != "work_current" || id != "call_fixture" {
			t.Errorf("unexpected callback: %s %s", name, id)
		}
		calls++
		return []byte(`{"data":"authoritative fixture state"}`), false
	})
	if e != nil {
		t.Fatal(e)
	}
	if calls != 1 || requests.Load() != 2 || result.State != "completed" {
		t.Fatalf("native callback not exercised: calls=%d requests=%d result=%+v", calls, requests.Load(), result)
	}
}
