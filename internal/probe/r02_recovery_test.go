// pattern: Imperative Shell
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveredCandidateUsesSuccessfulCheckedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "protocol.jsonl")
	a := "package formatter\nfunc Render(v float64) string { return \"A\" }\n"
	b := "package formatter\nfunc Render(v float64) string { return \"B\" }\n"
	da := sha256.Sum256([]byte(a))
	db := sha256.Sum256([]byte(b))
	digestA, digestB := hex.EncodeToString(da[:]), hex.EncodeToString(db[:])
	type entry struct {
		Direction string `json:"direction"`
		Data      any    `json:"data"`
	}
	type call struct {
		CallID, Tool string
		Arguments    any
	}
	type tr struct {
		CallID, Tool string
		Result       any
	}
	lines := []entry{
		{"receive", map[string]any{"method": "item/tool/call", "params": map[string]any{"callId": "replace-a", "tool": "polis_workspace_replace", "arguments": map[string]any{"content": a, "expected_digest": "old"}}}},
		{"tool_result", map[string]any{"call_id": "replace-a", "tool": "polis_workspace_replace", "result": map[string]any{"receipt": map[string]any{"id": digestA, "status": "persisted"}}}},
		{"receive", map[string]any{"method": "item/tool/call", "params": map[string]any{"callId": "check-a", "tool": "polis_workspace_check", "arguments": map[string]any{}}}},
		{"tool_result", map[string]any{"call_id": "check-a", "tool": "polis_workspace_check", "result": map[string]any{"receipt": map[string]any{"id": "check-receipt", "status": "persisted"}, "data": map[string]any{"passed": true, "digest": digestA}}}},
		{"receive", map[string]any{"method": "item/tool/call", "params": map[string]any{"callId": "checkpoint-a", "tool": "polis_work_checkpoint", "arguments": map[string]any{"summary": "checkpoint", "facts": []string{"fact"}, "decisions": []string{"decision"}, "rejected": []string{"rejected"}, "evidence": []string{"check-receipt"}}}}},
		{"tool_result", map[string]any{"call_id": "checkpoint-a", "tool": "polis_work_checkpoint", "result": map[string]any{"receipt": map[string]any{"id": "checkpoint-receipt", "status": "persisted"}}}},
		{"receive", map[string]any{"method": "item/tool/call", "params": map[string]any{"callId": "replace-b", "tool": "polis_workspace_replace", "arguments": map[string]any{"content": b, "expected_digest": digestA}}}},
		{"tool_result", map[string]any{"call_id": "replace-b", "tool": "polis_workspace_replace", "result": map[string]any{"error": "REVISION_CONFLICT"}}},
		{"receive", map[string]any{"method": "item/tool/call", "params": map[string]any{"callId": "artifact", "tool": "polis_artifact_submit", "arguments": map[string]any{}}}},
		{"tool_result", map[string]any{"call_id": "artifact", "tool": "polis_artifact_submit", "result": map[string]any{"receipt": map[string]any{"id": "artifact-1", "status": "candidate"}}}},
	}
	_ = call{}
	_ = tr{}
	_ = digestB
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	enc := json.NewEncoder(f)
	for _, line := range lines {
		if e = enc.Encode(line); e != nil {
			t.Fatal(e)
		}
	}
	f.Close()
	content, cp, e := parseRecoveredCandidate(path, "artifact-1")
	if e != nil {
		t.Fatal(e)
	}
	if content != a || cp.Summary != "checkpoint" {
		t.Fatalf("recovered content/checkpoint mismatch: %q %+v", content, cp)
	}
}

func TestLoadRecoveredRealEvidenceFixture(t *testing.T){
	root:=os.Getenv("POLIS_R02_EVIDENCE");if root==""{t.Skip("POLIS_R02_EVIDENCE required")}
	r,e:=loadRecoveredReal(root);if e!=nil{t.Fatal(e)}
	if r.Artifact==""||r.Content==""||r.Checkpoint.Summary==""{t.Fatal("incomplete recovered evidence")}
}
