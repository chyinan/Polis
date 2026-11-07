// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/runner"
	"strings"
	"testing"
)

type peerHardeningEnv struct {
	ctx             context.Context
	k               *Kernel
	scope           Scope
	fixture         PeerFixture
	backend         Binding
	frontend        Binding
	process         *runner.Process
	frontendProcess *runner.Process
}

func newPeerHardeningEnv(t *testing.T, toolLimit int64) peerHardeningEnv {
	t.Helper()
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	env := peerHardeningEnv{ctx: context.Background()}
	var err error
	env.k, err = Open(env.ctx, dsn, t.TempDir())
	must(t, err)
	env.scope, err = env.k.TXCreateCompany(env.ctx, "r03a-hardening-"+newID())
	must(t, err)
	env.fixture, err = env.k.TXCreatePeerFixture(env.ctx, env.scope, "r03a-hardening-"+newID())
	must(t, err)
	env.backend, err = env.k.TXNewWorkerWithToolBudget(env.ctx, env.scope, env.fixture.Backend.ID, "gpt-5.6-luna/medium", toolLimit)
	must(t, err)
	env.process, err = runner.Start(env.backend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	must(t, env.k.TXAttachWorker(env.ctx, env.backend, env.process))
	must(t, env.k.TXValidateWorker(env.ctx, env.backend))
	must(t, env.k.TXActivateWorker(env.ctx, env.backend, "hardening-test"))
	env.frontend, err = env.k.TXNewWorkerWithToolBudget(env.ctx, env.scope, env.fixture.Frontend.ID, "gpt-5.6-luna/medium", 20)
	must(t, err)
	env.frontendProcess, err = runner.Start(env.frontend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	must(t, env.k.TXAttachWorker(env.ctx, env.frontend, env.frontendProcess))
	must(t, env.k.TXValidateWorker(env.ctx, env.frontend))
	must(t, env.k.TXActivateWorker(env.ctx, env.frontend, "hardening-frontend-test"))
	t.Cleanup(func() {
		_ = env.k.TXBeginStop(context.Background(), env.backend)
		proof, stopErr := env.process.Stop()
		if stopErr == nil {
			_ = env.k.TXConfirmStopped(context.Background(), env.backend, proof)
		}
		_ = env.k.TXBeginStop(context.Background(), env.frontend)
		frontendProof, frontendStopErr := env.frontendProcess.Stop()
		if frontendStopErr == nil {
			_ = env.k.TXConfirmStopped(context.Background(), env.frontend, frontendProof)
		}
		env.k.Close()
	})
	return env
}

func acceptPeerRevision(t *testing.T, env peerHardeningEnv, endpoint, schema, key string) PeerContractRevision {
	t.Helper()
	h, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	revision, err := env.k.TXProposePeerContract(env.ctx, env.backend, h.Task, PeerContractProposal{Endpoint: endpoint, Schema: schema}, key+"-propose")
	must(t, err)
	must(t, env.k.TXAcceptPeerContract(env.ctx, env.backend, revision.ID, key+"-accept"))
	return revision
}

func TestPeerWorkspaceCheckReturnsActionableSemanticCriteria(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	view, err := env.k.PeerWorkCurrent(env.ctx, env.backend)
	must(t, err)
	if view.ToolCallLimit != 20 || view.ToolCallsUsed != 0 || view.ToolCallsRemaining != 20 {
		t.Fatalf("worker context did not expose explicit tool budget: %+v", view)
	}
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "semantic")
	h, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	wrong := `package backend
const Endpoint = "GET /items?cursor=..."
func FetchItems() string { return "items" }
`
	mustReceipt := func(result ToolResult) *Receipt {
		t.Helper()
		if result.Error != "" || result.Receipt == nil {
			t.Fatalf("tool failed: %+v", result)
		}
		return result.Receipt
	}
	mustReceipt(ToolResult{Receipt: func() *Receipt {
		r, e := env.k.TXReplace(env.ctx, env.backend, "semantic-wrong", h.Workspace.Digest, wrong)
		must(t, e)
		return &r
	}()})
	report, err := env.k.TXPeerCheck(env.ctx, env.backend, "semantic-check")
	must(t, err)
	if report.Passed || report.CheckReceiptID == "" || report.Receipt == nil || report.CheckReceiptID != report.Receipt.ID || len(report.Criteria) < 2 || report.RelevantContractRevision != revision.ID || report.RelevantWorkspaceRevision == 0 {
		t.Fatalf("unexpected semantic failure report: %+v", report)
	}
	for _, criterion := range report.Criteria {
		if criterion.Passed {
			continue
		}
		if criterion.PublicReasonCode == "" || criterion.ActionableSummary == "" || criterion.ContractRevisionID != revision.ID || criterion.CheckReceiptID != report.CheckReceiptID || criterion.RelevantContractRevision != revision.ID || criterion.RelevantWorkspaceRevision == 0 {
			t.Fatalf("criterion is not actionable: %+v", criterion)
		}
	}
	shape := report.Criteria[2]
	if shape.CriterionID != "response_shape_compatibility" || len(shape.MissingRequiredFields) != 1 || shape.MissingRequiredFields[0] != "next_cursor" || shape.ExpectedPublicShape["next_cursor"] != "string" || shape.ActualShapeSummary == "" {
		t.Fatalf("response shape feedback did not identify the public mismatch: %+v", shape)
	}
	raw, err := json.Marshal(report)
	must(t, err)
	if strings.Contains(string(raw), "contract-v2") || strings.Contains(string(raw), "oracle") || strings.Contains(string(raw), "sentinel") {
		t.Fatalf("hidden checker implementation leaked: %s", raw)
	}

	neighbor := `package backend
const Endpoint = "GET /items?offset=..."
func FetchItems() string { return "items,next_cursor" }
`
	current, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "semantic-neighbor", current.Digest, neighbor)
	must(t, err)
	neighborReport, err := env.k.TXPeerCheck(env.ctx, env.backend, "semantic-neighbor-check")
	must(t, err)
	if neighborReport.Passed {
		t.Fatalf("neighboring wrong candidate passed: %+v", neighborReport)
	}

	current, err = env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "semantic-correct", current.Digest, fixture.PeerBackendV2)
	must(t, err)
	correct, err := env.k.TXPeerCheck(env.ctx, env.backend, "semantic-correct-check")
	must(t, err)
	if !correct.Passed {
		t.Fatalf("semantic correct candidate failed: %+v", correct)
	}
}

func TestProgressCheckpointIsHandoverStateButNotArtifactAcceptance(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "checkpoint")
	h, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: h.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply after progress", Actionable: true}, "checkpoint-message")
	must(t, err)
	wrong := `package backend
const Endpoint = "GET /items?cursor=..."
func FetchItems() string { return "items" }
`
	_, err = env.k.TXReplace(env.ctx, env.backend, "checkpoint-wrong", h.Workspace.Digest, wrong)
	must(t, err)
	check := env.k.TXPeerCheck
	report, err := check(env.ctx, env.backend, "checkpoint-check")
	must(t, err)
	if report.Passed || report.Receipt == nil {
		t.Fatalf("failed check did not create evidence: %+v", report)
	}
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	progress := Checkpoint{Kind: CheckpointProgress, Summary: "backend endpoint is bound but response shape is incomplete", Facts: []string{"accepted revision is persisted"}, Decisions: []string{"retain cursor endpoint"}, Rejected: []string{"submit before response compatibility"}, EvidenceRefs: []string{report.CheckReceiptID}, NextAction: "add the required response fields"}
	raw, err := json.Marshal(progress)
	must(t, err)
	checkpointResult := tools.Call(env.ctx, "work_checkpoint", "progress-checkpoint", raw)
	if checkpointResult.Error != "" || checkpointResult.Receipt == nil {
		t.Fatalf("progress checkpoint denied: %+v", checkpointResult)
	}
	handover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	var saved Checkpoint
	must(t, json.Unmarshal(mustJSON(t, handover.Checkpoints[len(handover.Checkpoints)-1]), &saved))
	if saved.Kind != CheckpointProgress || saved.WorkspaceRevision == 0 || saved.ContractRevisionID != revision.ID || saved.SessionID == "" || saved.Epoch == 0 || saved.PendingMessageID == "" || saved.PendingObligationID == "" || saved.PendingMessageState != "persisted" || saved.PendingObligationState != "pending" || saved.NextAction == "" {
		t.Fatalf("progress metadata incomplete: %+v", saved)
	}
	if ok, err := env.k.CheckpointEvidence(env.ctx, env.backend, checkpointResult.Receipt.ID); err != nil || ok {
		t.Fatalf("progress checkpoint was treated as qualified evidence: ok=%v err=%v", ok, err)
	}
	artifact := tools.Call(env.ctx, "artifact_submit", "progress-artifact", []byte(`{}`))
	if artifact.Error != string(core.Denied) {
		t.Fatalf("progress checkpoint authorized artifact submission: %+v", artifact)
	}

	current, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "checkpoint-correct", current.Digest, fixture.PeerBackendV2)
	must(t, err)
	passed, err := env.k.TXPeerCheck(env.ctx, env.backend, "checkpoint-passed-check")
	must(t, err)
	if !passed.Passed || passed.Receipt == nil {
		t.Fatalf("correct check failed: %+v", passed)
	}
	qualified := Checkpoint{Kind: CheckpointQualified, Summary: "backend candidate is qualified", Facts: []string{"response shape matches accepted revision"}, Decisions: []string{"submit candidate"}, Rejected: []string{"partial delivery"}, EvidenceRefs: []string{passed.Receipt.ID}}
	raw, err = json.Marshal(qualified)
	must(t, err)
	qualifiedResult := tools.Call(env.ctx, "work_checkpoint", "qualified-checkpoint", raw)
	if qualifiedResult.Error != "" || qualifiedResult.Receipt == nil {
		t.Fatalf("qualified checkpoint denied: %+v", qualifiedResult)
	}
	artifact = tools.Call(env.ctx, "artifact_submit", "qualified-artifact", []byte(`{}`))
	if artifact.Error != "" || artifact.Receipt == nil {
		t.Fatalf("qualified checkpoint did not authorize artifact: %+v", artifact)
	}
}

func TestContractBoundObligationIsSupersededAndReplacementIsCurrent(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision3 := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "supersession-3")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	message3, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision3.ID, Body: "apply revision 3", Actionable: true}, "send-revision-3")
	must(t, err)
	must(t, env.k.TXPeerDeliver(env.ctx, env.frontend, message3.ID, "deliver-revision-3"))
	must(t, env.k.TXPeerObserve(env.ctx, env.frontend, message3.ID, "observe-revision-3"))
	must(t, env.k.TXPeerAck(env.ctx, env.frontend, message3.ID, "ack-revision-3"))
	revision4 := acceptPeerRevision(t, env, "GET /items?cursor={cursor}", `{"items":["id","name"],"next_cursor":""}`, "supersession-4")
	messageState, obligationState, signalState := peerStates(t, env.k, env.scope, message3.ID)
	if messageState != "superseded" || obligationState != "superseded" || signalState != "superseded" {
		t.Fatalf("revision 3 state was not superseded: (%s,%s,%s)", messageState, obligationState, signalState)
	}
	if _, err := env.k.TXPeerApply(env.ctx, env.frontend, PeerApplyRequest{ObligationID: message3.ObligationID, ContractRevisionID: revision3.ID, WorkspaceRevision: 2, EvidenceRefs: []string{"stale-receipt"}}, "apply-stale-revision-3"); !errors.Is(err, core.Denied) {
		t.Fatalf("stale revision 3 apply was accepted: %v", err)
	}
	if err := env.k.TXPeerResolve(env.ctx, env.frontend, message3.ObligationID, "artifact-that-does-not-exist", "resolve-stale-revision-3"); err != core.Denied {
		t.Fatalf("stale revision 3 resolve was accepted: %v", err)
	}
	message4, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision4.ID, Body: "apply revision 4", Actionable: true}, "send-revision-4")
	must(t, err)
	duplicate, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision4.ID, Body: "apply revision 4", Actionable: true}, "send-revision-4")
	must(t, err)
	if duplicate.ID != message4.ID || duplicate.ObligationID != message4.ObligationID {
		t.Fatalf("replacement was not idempotent: first=%+v duplicate=%+v", message4, duplicate)
	}
	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision3.ID, Body: "late revision 3", Actionable: true}, "late-revision-3")
	if !errors.Is(err, core.Denied) {
		t.Fatalf("late revision 3 message was accepted: %v", err)
	}
	view, err := env.k.PeerWorkCurrent(env.ctx, env.frontend)
	must(t, err)
	if view.ObligationID != message4.ObligationID {
		t.Fatalf("frontend current obligation rolled back to stale revision: %+v", view)
	}
}

func TestPeerSendMissingTaskTargetReturnsActionableFailureAndNoRows(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items", fixture.PeerPaginationContractV3, "send-target")
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	raw, err := json.Marshal(map[string]any{
		"to_task":              "emp-frontend",
		"contract_revision_id": revision.ID,
		"body":                 "apply the accepted contract",
		"actionable":           true,
	})
	must(t, err)
	result := tools.Call(env.ctx, "collab_send", "send-target-invalid", raw)
	if result.Error != string(core.Malformed) {
		t.Fatalf("missing peer task target was not a deterministic denial: %+v", result)
	}
	var rejection PeerToolRejection
	must(t, json.Unmarshal(mustJSON(t, result.Data), &rejection))
	if rejection.ReasonCode != "target_fields_required" || rejection.TransactionOutcome != "not_started" {
		t.Fatalf("target failure feedback was not actionable: %+v", rejection)
	}
	var messageCount int
	must(t, env.k.pool.QueryRow(env.ctx, "SELECT count(*) FROM messages WHERE company_id=$1", env.scope.company).Scan(&messageCount))
	if messageCount != 0 {
		t.Fatalf("failed peer send created message rows: %d", messageCount)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	must(t, err)
	return raw
}
