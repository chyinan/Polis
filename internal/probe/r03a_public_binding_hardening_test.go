// pattern: Imperative Shell
package probe

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/fixture"
	"polis/internal/kernel"
)

func TestFrozenV3NonConvergenceGetsPublicBindingFeedbackWithoutBehaviorBypass(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		root = filepath.Dir(root)
	}
	protocolPath := filepath.Join(root, "evidence", "development", "r0.3a-pagination-v3-real-backend-v2", "e96c09670af9b98c8fb14a236c8f340f", "protocol.jsonl")
	file, err := os.Open(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	type protocolEntry struct {
		Direction string          `json:"direction"`
		Data      json.RawMessage `json:"data"`
	}
	type protocolData struct {
		Method string `json:"method"`
		Params struct {
			Item struct {
				Type      string          `json:"type"`
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
				Content   []struct {
					Text string `json:"text"`
				} `json:"contentItems"`
			} `json:"item"`
		} `json:"params"`
	}
	var candidates []string
	oldCoarseFeedback := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry protocolEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		var data protocolData
		if len(entry.Data) == 0 || json.Unmarshal(entry.Data, &data) != nil {
			continue
		}
		if entry.Direction != "receive" || data.Method != "item/completed" || data.Params.Item.Type != "dynamicToolCall" {
			continue
		}
		for _, content := range data.Params.Item.Content {
			if strings.Contains(content.Text, "OUTCOME_UNKNOWN") {
				oldCoarseFeedback = true
			}
		}
		if data.Params.Item.Tool != "polis_workspace_replace" {
			continue
		}
		var args struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(data.Params.Item.Arguments, &args); err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, args.Content)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 15 || !oldCoarseFeedback {
		t.Fatalf("frozen V3 evidence did not reproduce 15 candidates and coarse feedback: candidates=%d coarse=%v", len(candidates), oldCoarseFeedback)
	}

	bindingFailures := 0
	for _, candidate := range candidates {
		report := fixture.CheckBackendPublicBinding(candidate)
		if report.Passed {
			t.Fatalf("frozen wrong candidate unexpectedly passed public binding: %+v", report)
		} else {
			bindingFailures++
			if report.ReasonCode == "OUTCOME_UNKNOWN" || report.ReasonCode == "" {
				t.Fatalf("new binding checker retained coarse reason code: %+v", report)
			}
		}
	}
	if bindingFailures != 15 {
		t.Fatalf("frozen candidates did not all fail the new binding contract: failures=%d", bindingFailures)
	}
	behaviorCandidate := `package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "items,next_cursor" }
`
	if !fixture.CheckBackendPublicBinding(behaviorCandidate).Passed {
		t.Fatal("binding-correct behavior-negative fixture was rejected by the binding layer")
	}
	behavior, err := kernel.VerifyPaginationBackendCandidateSource(context.Background(), t.TempDir(), behaviorCandidate, fixture.MustPeerPaginationContract())
	if err != nil {
		t.Fatal(err)
	}
	if behavior.Passed {
		t.Fatalf("binding-correct frozen candidate unexpectedly passed pagination behavior: %+v", behavior)
	}
}
