// pattern: Imperative Shell
package kernel

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/runner"
	"strings"
	"testing"
)

func TestReviewerNativeProtocolHelper(t *testing.T) {
	if os.Getenv("POLIS_REVIEWER_NATIVE_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var message codex.Message
		if json.Unmarshal(scanner.Bytes(), &message) != nil {
			continue
		}
		switch message.Method {
		case "initialize":
			fmt.Fprintf(os.Stdout, "{\"id\":%s,\"result\":{\"userAgent\":\"codex-cli 0.151.0\"}}\n", message.ID)
		case "thread/start":
			fmt.Fprintf(os.Stdout, "{\"id\":%s,\"result\":{\"thread\":{\"id\":\"h3-review-thread\"},\"model\":\"gpt-5.6-luna\",\"reasoningEffort\":\"high\",\"approvalPolicy\":\"never\",\"sandbox\":{\"type\":\"readOnly\"}}}\n", message.ID)
		case "turn/start":
			var params struct {
				Thread string `json:"threadId"`
			}
			_ = json.Unmarshal(message.Params, &params)
			fmt.Fprintf(os.Stdout, "{\"id\":%s,\"result\":{\"turn\":{\"id\":\"h3-review-turn\"}}}\n", message.ID)
			args := map[string]any{
				"verdict":     "passed",
				"findings":    []string{"candidate reviewed"},
				"evidence":    []string{os.Getenv("POLIS_H3_ARTIFACT"), os.Getenv("POLIS_H3_DIGEST"), "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c", "signed-zero@1", "formatter.go"},
				"confidence":  "high",
				"limitations": []string{"scripted local protocol"},
			}
			toolParams := map[string]any{"threadId": params.Thread, "turnId": "h3-review-turn", "callId": "h3-review-call", "tool": "polis_review_submit", "arguments": args}
			toolMessage := map[string]any{"id": 99, "method": "item/tool/call", "params": toolParams}
			encoded, _ := json.Marshal(toolMessage)
			fmt.Fprintln(os.Stdout, string(encoded))
		case "":
		default:
			if strings.HasPrefix(message.Method, "item/") || message.Method == "initialized" {
				continue
			}
		}
		if message.Method == "turn/start" {
			fmt.Fprintln(os.Stdout, `{"method":"turn/completed","params":{"threadId":"h3-review-thread","turn":{"id":"h3-review-turn","status":"completed"}}}`)
		}
	}
}

func TestReviewerGrantPersistsReviewSubmitWithoutBehaviorVerifier(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	company := "h3-review-" + newID()
	mission := "h3-mission-" + newID()
	scope, err := k.TXCreateCompany(ctx, company)
	must(t, err)
	_, err = k.TXCreateProbe(ctx, scope, mission)
	must(t, err)
	artifactID := "57761d55c7b0dbf63668b9f8010de41d"
	_, err = k.TXRestoreCandidateForReview(ctx, scope, mission, artifactID, []byte(fixture.Source))
	must(t, err)
	reviewTask, err := k.TXCreateReviewProbe(ctx, scope, mission, artifactID)
	must(t, err)
	b, err := k.TXNewWorker(ctx, scope, reviewTask.ID, "gpt-5.6-luna/high")
	must(t, err)
	p, err := runner.Start(b.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, b, p))
	must(t, k.TXValidateWorker(ctx, b))
	must(t, k.TXActivateWorker(ctx, b, "h3-capability"))
	initialWorkspace, err := k.Workspace(ctx, b)
	must(t, err)
	oldTools := ReviewerTools{Kernel: k, Binding: b, Evidence: ReviewerEvidence{SubjectRevision: "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c", ArtifactID: artifactID, CandidateDigest: initialWorkspace.Digest, Contract: "signed-zero@1", TaskInput: "frozen task input", AllowedPath: "formatter.go"}}
	must(t, k.TXBeginStop(ctx, b))
	stopProof, err := p.Stop()
	must(t, err)
	must(t, k.TXConfirmStopped(ctx, b, stopProof))
	b, err = k.TXNewWorker(ctx, scope, reviewTask.ID, "gpt-5.6-luna/high")
	must(t, err)
	p2, err := runner.Start(b.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p2.Stop()
	must(t, k.TXAttachWorker(ctx, b, p2))
	must(t, k.TXValidateWorker(ctx, b))
	must(t, k.TXActivateWorker(ctx, b, "h3-capability-2"))
	workspace, err := k.Workspace(ctx, b)
	must(t, err)
	evidence := ReviewerEvidence{SubjectRevision: "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c", ArtifactID: artifactID, CandidateDigest: workspace.Digest, Contract: "signed-zero@1", TaskInput: "frozen task input", AllowedPath: "formatter.go"}
	if names := codex.ReviewerTools(); len(names) != 4 {
		t.Fatalf("minimal reviewer grant changed: got %d tools", len(names))
	}
	submission := ReviewerSubmission{Verdict: "passed", Findings: []string{"candidate reviewed"}, Evidence: []string{artifactID, workspace.Digest, evidence.SubjectRevision, evidence.Contract, evidence.AllowedPath}, Confidence: "high", Limitations: []string{"single frozen candidate"}}
	raw, err := json.Marshal(submission)
	must(t, err)
	stale := oldTools.Call(ctx, "review_submit", "stale-review-submit", raw)
	if stale.Error != string(core.StaleEpoch) {
		t.Fatalf("old reviewer epoch was not rejected: %s", stale.Error)
	}
	tools := ReviewerTools{Kernel: k, Binding: b, Evidence: evidence}
	badEvidence := submission
	badEvidence.Evidence = []string{"hidden-verifier-receipt"}
	badRaw, err := json.Marshal(badEvidence)
	must(t, err)
	if bad := tools.Call(ctx, "review_submit", "bad-evidence", badRaw); bad.Error != string(core.Denied) {
		t.Fatalf("wrong evidence ref was not rejected: %s", bad.Error)
	}
	wrongArtifact := evidence
	wrongArtifact.ArtifactID = "wrong-artifact"
	wrongTools := ReviewerTools{Kernel: k, Binding: b, Evidence: wrongArtifact}
	if bad := wrongTools.Call(ctx, "review_submit", "wrong-artifact", raw); bad.Error != string(core.Denied) {
		t.Fatalf("wrong artifact was not rejected: %s", bad.Error)
	}
	wrongRevision := evidence
	wrongRevision.SubjectRevision = "wrong-revision"
	if bad := (ReviewerTools{Kernel: k, Binding: b, Evidence: wrongRevision}).Call(ctx, "review_submit", "wrong-revision", raw); bad.Error != string(core.Malformed) {
		t.Fatalf("wrong subject revision was not rejected: %s", bad.Error)
	}
	exe, err := os.Executable()
	must(t, err)
	env := append(os.Environ(), "POLIS_REVIEWER_NATIVE_HELPER=1", "POLIS_H3_ARTIFACT="+artifactID, "POLIS_H3_DIGEST="+workspace.Digest)
	nativeProcess, err := runner.Start("h3-native-review", []string{exe, "-test.run=^TestReviewerNativeProtocolHelper$"}, env)
	must(t, err)
	protocolDir := t.TempDir()
	client, err := codex.NewWithModel(nativeProcess, protocolDir, "gpt-5.6-luna")
	must(t, err)
	closed := false
	defer func() {
		if !closed {
			client.Close()
		}
	}()
	must(t, client.Initialize(ctx))
	thread, err := client.StartReviewerThread(ctx, "high")
	must(t, err)
	called := []string{}
	turn, err := client.Turn(ctx, thread, "high", "frozen review input", func(name, callID string, payload json.RawMessage) (json.RawMessage, bool) {
		called = append(called, name)
		response := tools.Call(ctx, name, callID, payload)
		encoded, _ := json.Marshal(response)
		return encoded, false
	})
	must(t, err)
	if turn.State != "completed" || len(called) != 1 || called[0] != "review_submit" {
		t.Fatalf("native reviewer registration path did not submit exactly once: state=%s calls=%v", turn.State, called)
	}
	client.Close()
	closed = true
	protocol, err := os.ReadFile(filepath.Join(protocolDir, "protocol.jsonl"))
	must(t, err)
	protocolText := string(protocol)
	for _, forbidden := range []string{"workspace_check", "work_checkpoint", "hidden_verifier_result", `"passed":true`} {
		if strings.Contains(protocolText, forbidden) {
			t.Fatalf("Reviewer-visible protocol leaked forbidden verifier/checkpoint material %q", forbidden)
		}
	}
	snapshot, err := k.Snapshot(ctx, scope, mission)
	must(t, err)
	var completed bool
	for _, task := range snapshot.Tasks {
		if task.ID == reviewTask.ID {
			completed = task.State == "completed"
		}
	}
	if !completed {
		t.Fatal("formal review record did not complete the disposable review task")
	}
}
