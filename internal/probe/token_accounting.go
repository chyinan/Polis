// pattern: Functional Core
package probe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"polis/internal/codex"
)

type TokenAccounting struct {
	ProtocolBytes                 int64 `json:"protocol_bytes"`
	RepeatedWorkspacePayloadBytes int64 `json:"repeated_workspace_payload_bytes"`
	RepeatedToolResultBytes       int64 `json:"repeated_tool_result_bytes"`
	AccumulatedConversationBytes  int64 `json:"accumulated_conversation_bytes"`
	LastInputTokens               int64 `json:"last_input_tokens"`
	LastCachedInputTokens         int64 `json:"last_cached_input_tokens"`
	LastUncachedInputTokens       int64 `json:"last_uncached_input_tokens"`
	LastOutputTokens              int64 `json:"last_output_tokens"`
	LastReasoningOutputTokens     int64 `json:"last_reasoning_output_tokens"`
	LastTotalTokens               int64 `json:"last_total_tokens"`
}

// AccountProtocolJSONL counts observable serialized components only. It does
// not infer provider tokenization or alter the conversation/prompt protocol.
func AccountProtocolJSONL(raw []byte, usage codex.TokenUsage) (TokenAccounting, error) {
	accounting := TokenAccounting{
		ProtocolBytes:             int64(len(raw)),
		LastInputTokens:           usage.Last.InputTokens,
		LastCachedInputTokens:     usage.Last.CachedInputTokens,
		LastOutputTokens:          usage.Last.OutputTokens,
		LastReasoningOutputTokens: usage.Last.ReasoningOutputTokens,
		LastTotalTokens:           usage.Last.TotalTokens,
	}
	accounting.LastUncachedInputTokens = accounting.LastInputTokens - accounting.LastCachedInputTokens
	if accounting.LastUncachedInputTokens < 0 {
		return TokenAccounting{}, fmt.Errorf("cached input tokens exceed input tokens")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		var envelope struct {
			Direction string          `json:"direction"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			return TokenAccounting{}, fmt.Errorf("invalid protocol line: %w", err)
		}
		if len(envelope.Data) == 0 || envelope.Data[0] != '{' {
			continue
		}
		var data struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return TokenAccounting{}, fmt.Errorf("invalid protocol data: %w", err)
		}
		accounting.AccumulatedConversationBytes += int64(len(data.Params) + len(data.Result))
		if data.Method != "item/tool/call" {
			if envelope.Direction == "tool_result" && len(data.Result) > 0 {
				accounting.RepeatedToolResultBytes += int64(len(data.Result))
			}
			continue
		}
		var params struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(data.Params, &params); err != nil {
			return TokenAccounting{}, fmt.Errorf("invalid tool-call params: %w", err)
		}
		if params.Tool == "polis_workspace_replace" {
			var arguments struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(params.Arguments, &arguments); err != nil {
				return TokenAccounting{}, fmt.Errorf("invalid workspace replacement: %w", err)
			}
			accounting.RepeatedWorkspacePayloadBytes += int64(len(arguments.Content))
		}
	}
	if err := scanner.Err(); err != nil {
		return TokenAccounting{}, err
	}
	return accounting, nil
}
