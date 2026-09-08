// pattern: Imperative Shell
package probe

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"polis/internal/codex"
	"polis/internal/kernel"
	"polis/internal/runner"
)

type recoveredReal struct {
	Result     R02Result
	Content    string
	Checkpoint kernel.Checkpoint
	Bundle     kernel.HandoverBundle
	Artifact   string
}

func loadRecoveredReal(root string) (recoveredReal, error) {
	var out recoveredReal
	raw, e := os.ReadFile(filepath.Join(root, "result.json"))
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out.Result); e != nil {
		return out, e
	}
	if out.Result.Model != "gpt-5.6-luna" || len(out.Result.Turns) != 2 || out.Result.Turns[0].Status != "passed" || out.Result.Turns[1].Status != "passed" || out.Result.Turns[1].ArtifactID == "" {
		return out, errors.New("real R0.2 medium gates are not both recorded as passed")
	}
	out.Artifact = out.Result.Turns[1].ArtifactID
	raw, e = os.ReadFile(filepath.Join(root, "handover-bundle.json"))
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out.Bundle); e != nil {
		return out, e
	}
	out.Content, out.Checkpoint, e = parseRecoveredCandidate(filepath.Join(root, out.Result.Turns[1].SessionID, "protocol.jsonl"), out.Artifact)
	if e != nil {
		return out, e
	}
	if out.Content == "" || out.Checkpoint.Summary == "" || len(out.Checkpoint.Facts) == 0 {
		return out, errors.New("immutable real evidence lacks final content or checkpoint")
	}
	return out, nil
}

// parseRecoveredCandidate only accepts content observed in a successful
// workspace_replace, then successfully checked against the same digest, then
// checkpointed and submitted with the expected artifact receipt. A later
// failed or unverified replacement cannot overwrite accepted evidence.
func parseRecoveredCandidate(path, artifactID string) (string, kernel.Checkpoint, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", kernel.Checkpoint{}, e
	}
	defer f.Close()
	type callInfo struct {
		Tool      string
		Arguments json.RawMessage
	}
	calls := map[string]callInfo{}
	var content, checkedContent, checkedDigest, checkReceipt, currentContent, currentDigest, acceptedContent string
	var cp kernel.Checkpoint
	var cpReceipt string
	artifactOK := false
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 2<<20)
	for scan.Scan() {
		var entry struct {
			Direction string          `json:"direction"`
			Data      json.RawMessage `json:"data"`
		}
		if e = json.Unmarshal(scan.Bytes(), &entry); e != nil {
			return "", cp, e
		}
		if entry.Direction == "receive" {
			var m struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if json.Unmarshal(entry.Data, &m) != nil || m.Method != "item/tool/call" {
				continue
			}
			var p struct {
				CallID    string          `json:"callId"`
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if json.Unmarshal(m.Params, &p) == nil {
				calls[p.CallID] = callInfo{p.Tool, p.Arguments}
			}
			continue
		}
		if entry.Direction != "tool_result" {
			continue
		}
		var tr struct {
			CallID string          `json:"call_id"`
			Tool   string          `json:"tool"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(entry.Data, &tr) != nil {
			continue
		}
		var result struct {
			Receipt *struct{ ID, Status string } `json:"receipt"`
			Error   string                       `json:"error"`
			Data    json.RawMessage              `json:"data"`
		}
		if json.Unmarshal(tr.Result, &result) != nil || result.Error != "" || result.Receipt == nil {
			continue
		}
		call := calls[tr.CallID]
		if artifactOK {
			continue
		}
		switch tr.Tool {
		case "polis_workspace_replace":
			if call.Tool != tr.Tool || result.Receipt.Status != "persisted" {
				continue
			}
			var a struct {
				Content string `json:"content"`
			}
			if json.Unmarshal(call.Arguments, &a) != nil || a.Content == "" {
				continue
			}
			h := sha256.Sum256([]byte(a.Content))
			if hex.EncodeToString(h[:]) != result.Receipt.ID {
				continue
			}
			content = a.Content
			currentContent = content
			currentDigest = result.Receipt.ID
			// A successful replacement invalidates every prior check and checkpoint.
			checkedContent, checkedDigest, checkReceipt = "", "", ""
			cp = kernel.Checkpoint{}
			cpReceipt = ""
		case "polis_workspace_read", "polis_work_current", "polis_context_read":
			var data struct {
				Digest  string `json:"Digest"`
				Content string `json:"Content"`
			}
			_ = json.Unmarshal(result.Data, &data)
			if data.Content != "" && data.Digest != "" {
				currentContent, currentDigest = data.Content, data.Digest
			}
		case "polis_workspace_check":
			if result.Receipt.Status != "persisted" {
				continue
			}
			var report runner.Report
			if json.Unmarshal(result.Data, &report) != nil || !report.Passed || report.Digest == "" || result.Receipt.ID == "" {
				continue
			}
			if content == "" {
				continue
			}
			h := sha256.Sum256([]byte(content))
			if hex.EncodeToString(h[:]) != report.Digest {
				continue
			}
			checkedContent, checkedDigest, checkReceipt = content, report.Digest, result.Receipt.ID
		case "polis_work_checkpoint":
			if call.Tool != tr.Tool || result.Receipt.Status != "persisted" {
				continue
			}
			var candidate kernel.Checkpoint
			if json.Unmarshal(call.Arguments, &candidate) != nil || candidate.Summary == "" || len(candidate.Facts) == 0 || len(candidate.Decisions) == 0 || len(candidate.Rejected) == 0 || len(candidate.Evidence) == 0 || !contains(candidate.Evidence, checkReceipt) {
				continue
			}
			cp, cpReceipt = candidate, result.Receipt.ID
		case "polis_artifact_submit":
			if result.Receipt.Status == "candidate" && result.Receipt.ID == artifactID && cpReceipt != "" && checkedDigest != "" && checkedDigest == digestFor(checkedContent) && currentDigest == checkedDigest && currentContent == checkedContent {
				artifactOK = true
				acceptedContent = checkedContent
			}
		}
	}
	if e = scan.Err(); e != nil {
		return "", cp, e
	}
	if !artifactOK {
		return "", cp, errors.New("no artifact receipt bound to successful checked content and checkpoint")
	}
	return acceptedContent, cp, nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func digestFor(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// RunR02HighRecovery consumes only the single remaining High turn from the
// already-authorized 2 Medium + 1 High experiment. It never runs Medium again
// and never edits the old R0.2 result or its ProblemKey.
func RunR02HighRecovery(cfg Config, oldEvidence string) (result R02Result, err error) {
	if cfg.Model != "gpt-5.6-luna" || cfg.HighLimit != 1 {
		return result, errors.New("R0.2 High recovery requires Luna and High limit 1")
	}
	old, e := loadRecoveredReal(oldEvidence)
	if e != nil {
		return result, e
	}
	budget, e := codex.OpenBudget(filepath.Join(oldEvidence, "allowance.json"))
	if e != nil {
		return result, e
	}
	if budget.Medium != 2 || budget.High != 0 || budget.MediumLimit != 2 || budget.HighLimit != 1 {
		return result, errors.New("remaining R0.2 allowance is not exactly one unused High turn")
	}
	if e = os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	result = R02Result{Status: "inconclusive", Model: cfg.Model, Version: runner.NativeVersion, SchemaDigest: cfg.SchemaDigest, Started: time.Now().UTC(), Usage: R02Usage{Money: "not_estimated", Limit: "existing R0.2 allowance: 2 Medium consumed + 1 High remaining"}}
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	defer func() {
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "high-recovery-result.json"), result)
	}()
	deadline := budget.Started.Add(10 * time.Minute)
	if time.Now().UTC().After(deadline) {
		return result, errors.New("probe allowance exhausted before High reservation; no model turn started")
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	k, e := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if e != nil {
		return result, e
	}
	defer k.Close()
	s, e := k.TXCreateCompany(ctx, "r02-recovered")
	if e != nil {
		return result, e
	}
	_, e = k.TXCreateProbe(ctx, s, "mission-r02")
	if e != nil {
		return result, e
	}
	if _, e = k.TXRestoreCandidateForReview(ctx, s, "mission-r02", old.Artifact, []byte(old.Content)); e != nil {
		return result, e
	}
	reviewTask, e := k.TXCreateReviewProbe(ctx, s, "mission-r02", old.Artifact)
	if e != nil {
		return result, e
	}
	if e = budget.Reserve("high"); e != nil {
		return result, e
	}
	checker := runner.Verifier{GoRoot: cfg.GoRoot, Scratch: cfg.Root, Context: ctx}
	review, e := runR02ReviewSession(ctx, cfg, k, s, reviewTask.ID, old.Artifact, old.Bundle, checker, 3, "independent behavior acceptance after repaired packaging")
	result.CapabilityDigest = review.capability
	result.Turns = append(result.Turns, review.turn)
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	if e != nil {
		return result, e
	}
	if !review.turn.CheckPassed || review.turn.CheckpointID == "" {
		return result, errors.New("High review did not pass compiled signed-zero behavior check")
	}
	if ok, checkErr := k.CheckpointEvidence(ctx, review.binding, review.turn.CheckpointID); checkErr != nil || !ok {
		return result, errors.New("High checkpoint receipt not bound to session and digest")
	}
	if _, e = k.VerifyProbe(ctx, s, old.Artifact, "full", checker); e != nil {
		return result, e
	}
	if !hasAll(review.turn.ToolEvents, "workspace_read", "workspace_check", "work_checkpoint") || containsAny(review.turn.ToolEvents, "workspace_replace", "artifact_submit") {
		return result, errors.New("High behavior review did not remain independent/read-only")
	}
	result.Handover = "passed"
	result.Behavior = "passed"
	result.Status = "passed"
	return result, nil
}
func hasAll(items []string, need ...string) bool {
	set := map[string]bool{}
	for _, x := range items {
		set[x] = true
	}
	for _, x := range need {
		if !set[x] {
			return false
		}
	}
	return true
}
func containsAny(items []string, bad ...string) bool {
	set := map[string]bool{}
	for _, x := range items {
		set[x] = true
	}
	for _, x := range bad {
		if set[x] {
			return true
		}
	}
	return false
}
