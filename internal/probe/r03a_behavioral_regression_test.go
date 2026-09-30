// pattern: Imperative Shell
package probe

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"polis/internal/fixture"
	"runtime"
	"strings"
	"testing"
)

func TestContinuation4RawEvidenceRemainsReadOnlyRegressionFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	protocolPath := filepath.Join(root, "evidence", "development", "r0.3a-real-backend-employee", "continuation-4", "8bfd303beafe6fefd25337a2bd1ef77d", "protocol.jsonl")
	resultPath := filepath.Join(root, "evidence", "development", "r0.3a-real-backend-employee", "continuation-4", "result.json")
	raw, err := os.Open(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	genericFeedback := false
	checkpointDenied := false
	directRev3 := false
	acceptedRev4 := false
	scanner := bufio.NewScanner(raw)
	for scanner.Scan() {
		var entry struct {
			Data struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		line := string(scanner.Bytes())
		if strings.Contains(line, "peer candidate checks failed") {
			genericFeedback = true
		}
		if strings.Contains(line, "POLICY_DENIED") {
			checkpointDenied = true
		}
		if strings.Contains(line, "0580220bd9831aeb8b41d2c1268d69d1") {
			directRev3 = true
		}
		if strings.Contains(line, "d56e82def48cbba4d8624f832d79e42c") {
			acceptedRev4 = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !genericFeedback || !checkpointDenied || !directRev3 || !acceptedRev4 {
		t.Fatalf("continuation-4 fixture did not preserve expected feedback/revision evidence: generic=%v checkpoint-denied=%v rev3=%v rev4=%v", genericFeedback, checkpointDenied, directRev3, acceptedRev4)
	}
	result, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "tool-call limit") {
		t.Fatalf("continuation-4 result did not preserve hidden tool-call exhaustion: %s", result)
	}
}

func TestHandoverV4RawInteractionFailureRemainsReadOnlyRegressionFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	protocolPath := filepath.Join(root, "evidence", "development", "r0.3a-real-frontend-handover-v4", "2b7a6fd917d335459b0dbf32dda34158", "protocol.jsonl")
	raw, err := os.Open(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	collabApplyCalls := 0
	legacyContentCalls := 0
	naturalLanguageCheckpointCalls := 0
	scanner := bufio.NewScanner(raw)
	for scanner.Scan() {
		var entry struct {
			Direction string `json:"direction"`
			Data      struct {
				Method string `json:"method"`
				Params struct {
					Tool      string          `json:"tool"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"params"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil || entry.Direction != "receive" || entry.Data.Method != "item/tool/call" {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(entry.Data.Params.Arguments, &fields) != nil {
			continue
		}
		switch entry.Data.Params.Tool {
		case "polis_collab_apply":
			collabApplyCalls++
			if _, present := fields["content"]; present {
				legacyContentCalls++
			}
		case "polis_work_checkpoint":
			if evidence, present := fields["evidence"]; present {
				var values []string
				if json.Unmarshal(evidence, &values) == nil && len(values) > 0 {
					naturalLanguageCheckpointCalls++
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if collabApplyCalls != 8 || legacyContentCalls != 6 || naturalLanguageCheckpointCalls != 2 {
		t.Fatalf("handover-v4 fixture changed: collab_apply=%d legacy_content=%d natural_language_checkpoint=%d", collabApplyCalls, legacyContentCalls, naturalLanguageCheckpointCalls)
	}
}

func TestRevisedSuccessorEvidenceGetsRicherPublicCheckerFeedback(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	protocolPath := filepath.Join(root, "evidence", "development", "r0.3a-real-frontend-handover-revised", "115be34ae3cf91e1a760d8dd8034e3e9", "protocol.jsonl")
	raw, err := os.Open(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	failedChecks := 0
	legacyDetailCount := 0
	latestCandidate := ""
	scanner := bufio.NewScanner(raw)
	for scanner.Scan() {
		var entry struct {
			Direction string `json:"direction"`
			Data      struct {
				Method string `json:"method"`
				Params struct {
					Tool      string          `json:"tool"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"params"`
				Result struct {
					Data struct {
						Passed   bool `json:"passed"`
						Criteria []struct {
							ParserDiagnostics     []any `json:"parser_diagnostics"`
							MissingRequiredFields []any `json:"missing_required_fields"`
						} `json:"criteria"`
					} `json:"data"`
				} `json:"result"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.Direction == "receive" && entry.Data.Method == "item/tool/call" && entry.Data.Params.Tool == "polis_workspace_replace" {
			var args struct {
				Content string `json:"content"`
			}
			if json.Unmarshal(entry.Data.Params.Arguments, &args) == nil {
				latestCandidate = args.Content
			}
		}
		if entry.Direction != "tool_result" || entry.Data.Result.Data.Passed || len(entry.Data.Result.Data.Criteria) == 0 {
			continue
		}
		failedChecks++
		for _, criterion := range entry.Data.Result.Data.Criteria {
			if len(criterion.ParserDiagnostics) != 0 || len(criterion.MissingRequiredFields) != 0 {
				legacyDetailCount++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if failedChecks != 7 || legacyDetailCount != 0 || latestCandidate == "" {
		t.Fatalf("frozen successor fixture changed or was not coarse: checks=%d legacy_details=%d candidate=%v", failedChecks, legacyDetailCount, latestCandidate != "")
	}
	criteria := fixture.CheckPeerFrontendCandidate(latestCandidate, fixture.PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`})
	if fixture.CandidateCriteriaPassed(criteria) {
		t.Fatalf("the frozen invalid candidate unexpectedly passed the revised checker: %+v", criteria)
	}
	var hasRicherFeedback bool
	for _, criterion := range criteria {
		if len(criterion.ParserDiagnostics) != 0 || len(criterion.MissingRequiredFields) != 0 || len(criterion.TypeMismatches) != 0 || criterion.ExpectedPublicShape != nil {
			hasRicherFeedback = true
		}
	}
	if !hasRicherFeedback {
		t.Fatalf("revised checker did not add public diagnostics: %+v", criteria)
	}
	rich, err := json.Marshal(criteria)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"hidden verifier", "oracle implementation", "sentinel literal", "contract-v2"} {
		if strings.Contains(string(rich), forbidden) {
			t.Fatalf("revised checker leaked hidden detail %q: %s", forbidden, rich)
		}
	}
}
