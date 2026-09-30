// pattern: Imperative Shell
package codex

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

type Budget struct {
	Started       time.Time `json:"started"`
	Medium        int       `json:"medium_turns"`
	High          int       `json:"high_turns"`
	MediumLimit   int       `json:"medium_limit"`
	HighLimit     int       `json:"high_limit"`
	ToolCallLimit int       `json:"tool_call_limit"`
	ToolCallsUsed int       `json:"tool_calls_used"`
	path          string
	mu            sync.Mutex
}

type ToolCallBudget struct {
	Limit     int `json:"tool_call_limit"`
	Used      int `json:"tool_calls_used"`
	Remaining int `json:"tool_calls_remaining"`
}

func NewBudget(path string, limits ...int) (*Budget, error) {
	m, h := 3, 3
	toolCalls := 0
	if len(limits) > 0 {
		if len(limits) != 2 && len(limits) != 3 {
			return nil, errors.New("two profile limits or two profile limits plus tool-call limit required")
		}
		m, h = limits[0], limits[1]
		if len(limits) == 3 {
			toolCalls = limits[2]
		}
	}
	return newBudget(path, m, h, toolCalls)
}

func NewBusinessBudget(path string, mediumLimit, highLimit, toolCallLimit int) (*Budget, error) {
	return newBudget(path, mediumLimit, highLimit, toolCallLimit)
}

func NewHighOnlyBudget(path string) (*Budget, error) {
	return newBudget(path, 0, 1, 0)
}

func newBudget(path string, m, h, toolCalls int) (*Budget, error) {
	if m < 0 || m > 3 || h < 0 || h > 3 || toolCalls < 0 || toolCalls > RuntimeToolCallSafetyCap {
		return nil, errors.New("profile limits must be medium 0..3 and high 0..3")
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, errors.New("probe allowance already exists; no automatic retry/reset")
	}
	defer f.Close()
	b := &Budget{Started: time.Now().UTC(), path: path, MediumLimit: m, HighLimit: h, ToolCallLimit: toolCalls}
	data, _ := json.Marshal(b)
	if _, e = f.Write(data); e != nil {
		return nil, e
	}
	return b, f.Sync()
}

// OpenBudget resumes exactly one previously created allowance. It never
// initializes missing limits or changes the start time, so a retest cannot reset
// the original experiment by choosing a new process.
func OpenBudget(path string) (*Budget, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var b Budget
	if e = json.Unmarshal(raw, &b); e != nil {
		return nil, e
	}
	if b.Started.IsZero() || b.path != "" && b.path != path {
		return nil, errors.New("invalid persisted allowance")
	}
	if b.MediumLimit < 0 || b.MediumLimit > 3 || b.HighLimit < 0 || b.HighLimit > 3 || b.ToolCallLimit < 0 || b.ToolCallLimit > RuntimeToolCallSafetyCap || b.ToolCallsUsed < 0 || b.ToolCallLimit > 0 && b.ToolCallsUsed > b.ToolCallLimit {
		return nil, errors.New("invalid persisted allowance limits")
	}
	b.path = path
	return &b, nil
}

func (b *Budget) ToolBudget() ToolCallBudget {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.toolBudgetLocked()
}

func (b *Budget) RecordToolCall() (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ToolCallLimit > 0 && b.ToolCallsUsed >= b.ToolCallLimit {
		return false, nil
	}
	b.ToolCallsUsed++
	return true, b.persistLocked()
}
func (b *Budget) Reserve(effort string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Since(b.Started) >= 10*time.Minute || b.Medium+b.High >= b.MediumLimit+b.HighLimit {
		return errors.New("probe allowance exhausted")
	}
	switch effort {
	case "medium":
		if b.Medium >= b.MediumLimit {
			return errors.New("medium turn cap reached")
		}
		b.Medium++
	case "high":
		if b.High >= b.HighLimit {
			return errors.New("high turn cap reached")
		}
		b.High++
	default:
		return errors.New("unapproved profile")
	}
	return b.persistLocked()
}

func (b *Budget) toolBudgetLocked() ToolCallBudget {
	remaining := -1
	if b.ToolCallLimit > 0 {
		remaining = b.ToolCallLimit - b.ToolCallsUsed
	}
	return ToolCallBudget{Limit: b.ToolCallLimit, Used: b.ToolCallsUsed, Remaining: remaining}
}

func (b *Budget) persistLocked() error {
	f, e := os.OpenFile(b.path, os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	raw, _ := json.Marshal(b)
	if _, e = f.Write(raw); e != nil {
		return e
	}
	return f.Sync()
}
