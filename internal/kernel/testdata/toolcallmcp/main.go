package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func write(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 256*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var r req
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		switch r.Method {
		case "initialize":
			write(map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "polis-fake-mcp", "version": "0.15-test"},
			}})
		case "notifications/initialized":
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(r.Params, &p)
			write(map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": map[string]any{
				"content": []any{map[string]any{"type": "text", "text": "ok:" + p.Name}},
				"isError": false,
			}})
		case "tools/list":
			write(map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": map[string]any{"tools": []any{}}})
		case "resources/list":
			write(map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": map[string]any{"resources": []any{}}})
		default:
			if r.ID != nil {
				write(map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
			}
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		// ignore
	}
}
