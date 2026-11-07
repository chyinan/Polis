// pattern: Functional Core
package probe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

type canaryProtocolSummary struct {
	FirstValidOutput        bool
	FirstValidOutputDeltaMS int64
	FirstDisconnectDeltaMS  int64
	RecoveryDeltaMS         int64
	ReconnectCount          int
	ReconnectPhases         []string
	TerminalState           string
	AssistantOutput         string
	SentinelMatch           bool
}

func analyzeCanaryProtocol(raw []byte, sentinel string) (canaryProtocolSummary, error) {
	var out canaryProtocolSummary
	var turnStarted, firstDisconnect time.Time
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var envelope struct {
			Time      time.Time `json:"time"`
			Direction string    `json:"direction"`
			Data      struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scan.Bytes(), &envelope); err != nil {
			return out, err
		}
		if envelope.Direction != "receive" {
			continue
		}
		switch envelope.Data.Method {
		case "turn/started":
			turnStarted = envelope.Time
		case "error":
			var p struct {
				WillRetry bool `json:"willRetry"`
				Error     struct {
					CodexErrorInfo struct {
						ResponseStreamDisconnected json.RawMessage `json:"responseStreamDisconnected"`
					} `json:"codexErrorInfo"`
				} `json:"error"`
			}
			if err := json.Unmarshal(envelope.Data.Params, &p); err != nil {
				return out, err
			}
			if p.WillRetry && len(p.Error.CodexErrorInfo.ResponseStreamDisconnected) > 0 {
				out.ReconnectCount++
				phase := "pre_first_output_reconnecting"
				if out.FirstValidOutput {
					phase = "post_first_output_reconnecting"
				}
				out.ReconnectPhases = append(out.ReconnectPhases, phase)
				if firstDisconnect.IsZero() {
					firstDisconnect = envelope.Time
					if !turnStarted.IsZero() {
						out.FirstDisconnectDeltaMS = envelope.Time.Sub(turnStarted).Milliseconds()
					}
				}
			}
		case "item/agentMessage/delta":
			var p struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(envelope.Data.Params, &p); err != nil {
				return out, err
			}
			out.AssistantOutput += p.Delta
			markCanaryFirstOutput(&out, turnStarted, firstDisconnect, envelope.Time)
		case "item/completed":
			var p struct {
				Item struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
			}
			if err := json.Unmarshal(envelope.Data.Params, &p); err != nil {
				return out, err
			}
			if p.Item.Type == "agentMessage" {
				out.AssistantOutput = p.Item.Text
				markCanaryFirstOutput(&out, turnStarted, firstDisconnect, envelope.Time)
			}
		case "turn/completed":
			var p struct {
				Turn struct {
					Status string `json:"status"`
				} `json:"turn"`
			}
			if err := json.Unmarshal(envelope.Data.Params, &p); err != nil {
				return out, err
			}
			out.TerminalState = p.Turn.Status
		}
	}
	if err := scan.Err(); err != nil {
		return out, err
	}
	out.SentinelMatch = strings.TrimSpace(out.AssistantOutput) == sentinel
	return out, nil
}

func markCanaryFirstOutput(out *canaryProtocolSummary, turnStarted, firstDisconnect, at time.Time) {
	if out.FirstValidOutput {
		return
	}
	out.FirstValidOutput = true
	if !turnStarted.IsZero() {
		out.FirstValidOutputDeltaMS = at.Sub(turnStarted).Milliseconds()
	}
	if !firstDisconnect.IsZero() {
		out.RecoveryDeltaMS = at.Sub(firstDisconnect).Milliseconds()
	}
}
