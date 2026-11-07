// pattern: Functional Core
package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ProductSurfaceDiagnosticProtocolSummary struct {
	InitializeRequestCount        int       `json:"initialize_request_count"`
	InitializeResponseCount       int       `json:"initialize_response_count"`
	InitializeSentAt              time.Time `json:"initialize_sent_at,omitempty"`
	InitializeReceivedAt          time.Time `json:"initialize_received_at,omitempty"`
	ThreadStartRequestCount       int       `json:"thread_start_request_count"`
	ThreadStartResponseCount      int       `json:"thread_start_response_count"`
	ThreadStartSentAt             time.Time `json:"thread_start_sent_at,omitempty"`
	ThreadStartReceivedAt         time.Time `json:"thread_start_received_at,omitempty"`
	RegisteredToolCount           int       `json:"registered_tool_count"`
	RegisteredToolsManifestDigest string    `json:"registered_tools_manifest_digest"`
	TurnStartCount                int       `json:"turn_start_count"`
	TurnStartAt                   time.Time `json:"turn_start_at,omitempty"`
	FirstOutputAt                 time.Time `json:"first_output_at,omitempty"`
	FirstOutput                   string    `json:"first_output"`
	TurnCompletedCount            int       `json:"turn_completed_count"`
	TurnCompletedAt               time.Time `json:"turn_completed_at,omitempty"`
	ToolCallEventCount            int       `json:"tool_call_event_count"`
	LifecyclePhases               []string  `json:"lifecycle_phases,omitempty"`
}

type productProtocolRequest struct {
	Method string
	SentAt time.Time
}

func SummarizeProductSurfaceDiagnosticProtocol(raw []byte) (ProductSurfaceDiagnosticProtocolSummary, error) {
	var summary ProductSurfaceDiagnosticProtocolSummary
	requests := map[string]productProtocolRequest{}
	var deltas strings.Builder
	var completedText string
	lines := bytes.Split(raw, []byte{'\n'})
	for lineNumber, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record struct {
			Time      time.Time       `json:"time"`
			Direction string          `json:"direction"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return ProductSurfaceDiagnosticProtocolSummary{}, fmt.Errorf("decode protocol line %d: %w", lineNumber+1, err)
		}
		if record.Direction == "lifecycle" {
			var event struct {
				Phase string `json:"phase"`
			}
			if err := json.Unmarshal(record.Data, &event); err != nil {
				return ProductSurfaceDiagnosticProtocolSummary{}, fmt.Errorf("decode lifecycle line %d: %w", lineNumber+1, err)
			}
			if event.Phase != "" {
				summary.LifecyclePhases = append(summary.LifecyclePhases, event.Phase)
			}
			continue
		}
		var message struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(record.Data, &message); err != nil {
			continue // bounded stderr records are strings, not protocol messages.
		}
		requestID := string(message.ID)
		if record.Direction == "send" && message.Method != "" {
			if requestID != "" {
				requests[requestID] = productProtocolRequest{Method: message.Method, SentAt: record.Time}
			}
			switch message.Method {
			case "initialize":
				summary.InitializeRequestCount++
				if summary.InitializeSentAt.IsZero() {
					summary.InitializeSentAt = record.Time
				}
			case "thread/start":
				summary.ThreadStartRequestCount++
				if summary.ThreadStartSentAt.IsZero() {
					summary.ThreadStartSentAt = record.Time
				}
				var params struct {
					DynamicTools []json.RawMessage `json:"dynamicTools"`
				}
				if err := json.Unmarshal(message.Params, &params); err != nil {
					return ProductSurfaceDiagnosticProtocolSummary{}, fmt.Errorf("decode thread/start tools: %w", err)
				}
				summary.RegisteredToolCount = len(params.DynamicTools)
				definitions, err := json.Marshal(params.DynamicTools)
				if err != nil {
					return ProductSurfaceDiagnosticProtocolSummary{}, fmt.Errorf("marshal thread/start tools: %w", err)
				}
				digest := sha256.Sum256(definitions)
				summary.RegisteredToolsManifestDigest = hex.EncodeToString(digest[:])
			case "turn/start":
				summary.TurnStartCount++
				if summary.TurnStartAt.IsZero() {
					summary.TurnStartAt = record.Time
				}
			}
			continue
		}
		if record.Direction == "receive" && requestID != "" && len(message.Result) > 0 {
			if request, exists := requests[requestID]; exists {
				switch request.Method {
				case "initialize":
					summary.InitializeResponseCount++
					if summary.InitializeReceivedAt.IsZero() {
						summary.InitializeReceivedAt = record.Time
					}
				case "thread/start":
					summary.ThreadStartResponseCount++
					if summary.ThreadStartReceivedAt.IsZero() {
						summary.ThreadStartReceivedAt = record.Time
					}
				}
			}
			continue
		}
		if record.Direction != "receive" {
			continue
		}
		switch message.Method {
		case "item/agentMessage/delta":
			var params struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(message.Params, &params); err != nil {
				return ProductSurfaceDiagnosticProtocolSummary{}, fmt.Errorf("decode agent message delta: %w", err)
			}
			if params.Delta != "" {
				if summary.FirstOutputAt.IsZero() {
					summary.FirstOutputAt = record.Time
				}
				deltas.WriteString(params.Delta)
			}
		case "item/agentMessage/completed":
			text := completedAgentMessageText(message.Params)
			if text != "" {
				if summary.FirstOutputAt.IsZero() {
					summary.FirstOutputAt = record.Time
				}
				completedText = text
			}
		case "item/tool/call":
			summary.ToolCallEventCount++
		case "turn/completed":
			summary.TurnCompletedCount++
			if summary.TurnCompletedAt.IsZero() {
				summary.TurnCompletedAt = record.Time
			}
		}
	}
	summary.FirstOutput = deltas.String()
	if summary.FirstOutput == "" {
		summary.FirstOutput = completedText
	}
	if summary.TurnStartCount > 1 {
		return summary, errors.New("diagnostic protocol contains more than one turn/start")
	}
	return summary, nil
}

func completedAgentMessageText(raw json.RawMessage) string {
	var params struct {
		Text string `json:"text"`
		Item struct {
			Text string `json:"text"`
		} `json:"item"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return ""
	}
	if params.Text != "" {
		return params.Text
	}
	return params.Item.Text
}
