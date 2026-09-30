// pattern: Functional Core
package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInboundProtocolLogRedactsToolArgumentsAndUnknownFields(t *testing.T) {
	const raw = `{"id":99,"method":"item/tool/call","params":{"threadId":"thread-1","turnId":"turn-1","callId":"call-1","tool":"polis_mcp_call","arguments":{"credential":"private-argument"},"opaque":"private-extra"}}`
	var message Message
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		t.Fatal(err)
	}
	logged := string(redactInboundProtocolMessageForLog(message, []byte(raw)))
	for _, privateValue := range []string{"private-argument", "private-extra", `"arguments"`} {
		if strings.Contains(logged, privateValue) {
			t.Fatalf("tool-call log contains %q: %s", privateValue, logged)
		}
	}
	var safe Message
	if err := json.Unmarshal([]byte(logged), &safe); err != nil {
		t.Fatal(err)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(safe.Params, &params); err != nil {
		t.Fatal(err)
	}
	var redacted bool
	if err := json.Unmarshal(params["argumentsRedacted"], &redacted); err != nil || !redacted {
		t.Fatalf("tool-call redaction marker=%s err=%v", params["argumentsRedacted"], err)
	}
	if string(params["tool"]) != `"polis_mcp_call"` || string(params["callId"]) != `"call-1"` {
		t.Fatalf("safe call metadata was lost: %s", safe.Params)
	}
}

func TestInboundProtocolLogPreservesNonToolProtocolFrame(t *testing.T) {
	const raw = `{"method":"turn/completed","params":{"turn":{"id":"turn-1","status":"completed"}}}`
	var message Message
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		t.Fatal(err)
	}
	if got := string(redactInboundProtocolMessageForLog(message, []byte(raw))); got != raw {
		t.Fatalf("non-tool frame changed: %s", got)
	}
}
