// pattern: Functional Core
package codex

import "encoding/json"

// redactInboundProtocolMessageForLog preserves ordinary protocol evidence but
// records only safe correlation metadata for native tool calls.
func redactInboundProtocolMessageForLog(message Message, raw []byte) json.RawMessage {
	if message.Method != "item/tool/call" {
		return append(json.RawMessage(nil), raw...)
	}
	type safeToolCallParams struct {
		ThreadID          string `json:"threadId,omitempty"`
		TurnID            string `json:"turnId,omitempty"`
		CallID            string `json:"callId,omitempty"`
		Tool              string `json:"tool,omitempty"`
		ArgumentsRedacted bool   `json:"argumentsRedacted"`
	}
	var params struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		CallID   string `json:"callId"`
		Tool     string `json:"tool"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		params = struct {
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
			CallID   string `json:"callId"`
			Tool     string `json:"tool"`
		}{}
	}
	safeParams, err := json.Marshal(safeToolCallParams{
		ThreadID: params.ThreadID, TurnID: params.TurnID, CallID: params.CallID, Tool: params.Tool,
		ArgumentsRedacted: true,
	})
	if err != nil {
		return json.RawMessage(`{"method":"item/tool/call","params":{"argumentsRedacted":true}}`)
	}
	safeMessage, err := json.Marshal(Message{
		ID: append(json.RawMessage(nil), message.ID...), Method: message.Method, Params: safeParams,
	})
	if err != nil {
		return json.RawMessage(`{"method":"item/tool/call","params":{"argumentsRedacted":true}}`)
	}
	return safeMessage
}
