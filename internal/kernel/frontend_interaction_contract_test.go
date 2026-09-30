// pattern: Imperative Shell
package kernel

import (
	"encoding/json"
	"polis/internal/core"
	"polis/internal/fixture"
	"testing"
)

func TestFrontendCollabApplyRejectsCandidateContentAndRequiresStructuredReferences(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "frontend-apply-contract")
	handover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: handover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply the accepted revision", Actionable: true}, "frontend-apply-message")
	must(t, err)
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	inbox := tools.Call(env.ctx, "collab_inbox", "frontend-inbox", []byte(`{}`))
	if inbox.Error != "" {
		t.Fatalf("collab_inbox: %+v", inbox)
	}
	ackArgs, err := json.Marshal(map[string]string{"message_id": message.ID})
	must(t, err)
	ack := tools.Call(env.ctx, "collab_ack", "frontend-ack", ackArgs)
	if ack.Error != "" {
		t.Fatalf("collab_ack: %+v", ack)
	}

	legacyArgs, err := json.Marshal(map[string]any{"message_id": message.ID, "content": fixture.PeerFrontendV2})
	must(t, err)
	legacy := tools.Call(env.ctx, "collab_apply", "frontend-legacy-apply", legacyArgs)
	if legacy.Error != string(core.Malformed) {
		t.Fatalf("candidate content was accepted or returned wrong code: %+v", legacy)
	}
	publicRaw, err := json.Marshal(legacy.Data)
	must(t, err)
	var public map[string]any
	must(t, json.Unmarshal(publicRaw, &public))
	if public["reason_code"] != "candidate_content_not_accepted" {
		t.Fatalf("legacy rejection was not actionable: %+v", legacy)
	}
	if public["current_obligation_id"] != message.ObligationID || public["current_contract_revision"] != revision.ID || public["current_workspace_revision"] != float64(1) {
		t.Fatalf("legacy rejection lost current references: %+v", public)
	}

	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	if workspace.Revision != 1 {
		t.Fatalf("legacy collab_apply changed workspace: %+v", workspace)
	}
}

func TestFrontendCollabApplyAndProgressCheckpointUseReceiptReferences(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "frontend-apply-positive")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply the accepted revision", Actionable: true}, "frontend-apply-positive-message")
	must(t, err)
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	if result := tools.Call(env.ctx, "collab_inbox", "frontend-positive-inbox", []byte(`{}`)); result.Error != "" {
		t.Fatalf("collab_inbox: %+v", result)
	}
	ackArgs, err := json.Marshal(map[string]string{"message_id": message.ID})
	must(t, err)
	if result := tools.Call(env.ctx, "collab_ack", "frontend-positive-ack", ackArgs); result.Error != "" {
		t.Fatalf("collab_ack: %+v", result)
	}

	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation := tools.Call(env.ctx, "workspace_replace", "frontend-positive-replace", mustJSON(t, map[string]any{"expected_digest": workspace.Digest, "content": fixture.PeerFrontendV2}))
	if mutation.Error != "" || mutation.Receipt == nil {
		t.Fatalf("workspace_replace: %+v", mutation)
	}
	applyArgs, err := json.Marshal(map[string]any{"obligation_id": message.ObligationID, "contract_revision_id": revision.ID, "workspace_revision": workspace.Revision + 1, "evidence_refs": []string{mutation.Receipt.ID}})
	must(t, err)
	applied := tools.Call(env.ctx, "collab_apply", "frontend-positive-apply", applyArgs)
	if applied.Error != "" || applied.Receipt == nil {
		t.Fatalf("structured collab_apply: %+v", applied)
	}
	handover, err := env.k.PeerHandover(env.ctx, env.frontend)
	must(t, err)
	if handover.ObligationState != "applied" || handover.MessageState != "applied" || handover.WorkspaceRevision != workspace.Revision+1 {
		t.Fatalf("apply did not persist current state: %+v", handover)
	}

	check := tools.Call(env.ctx, "workspace_check", "frontend-positive-check", []byte(`{}`))
	if check.Error != "" || check.Receipt == nil {
		t.Fatalf("workspace_check: %+v", check)
	}
	progressArgs, err := json.Marshal(Checkpoint{Kind: CheckpointProgress, Summary: "applied peer change and recorded the current check", Facts: []string{"workspace revision advanced"}, Decisions: []string{"retain the applied peer responsibility"}, Rejected: []string{"finalize before qualified acceptance"}, EvidenceRefs: []string{mutation.Receipt.ID, applied.Receipt.ID, check.Receipt.ID}, NextAction: "handover to successor for final qualification"})
	must(t, err)
	progress := tools.Call(env.ctx, "work_checkpoint", "frontend-positive-progress", progressArgs)
	if progress.Error != "" || progress.Receipt == nil {
		t.Fatalf("progress checkpoint: %+v", progress)
	}
	if artifact := tools.Call(env.ctx, "artifact_submit", "frontend-progress-artifact", []byte(`{}`)); artifact.Error != string(core.Denied) {
		t.Fatalf("progress checkpoint authorized artifact: %+v", artifact)
	}
	if current, err := env.k.PeerHandover(env.ctx, env.frontend); err != nil || current.ObligationState != "applied" {
		t.Fatalf("progress checkpoint changed obligation state: %+v, %v", current, err)
	}
}

func TestFrontendCollabApplyReportsUnsupportedCheckpointReceipt(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":null}`, "frontend-apply-feedback")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply the accepted revision", Actionable: true}, "frontend-apply-feedback-message")
	must(t, err)
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	if result := tools.Call(env.ctx, "collab_inbox", "frontend-feedback-inbox", []byte(`{}`)); result.Error != "" {
		t.Fatalf("collab_inbox: %+v", result)
	}
	if result := tools.Call(env.ctx, "collab_ack", "frontend-feedback-ack", mustJSON(t, map[string]string{"message_id": message.ID})); result.Error != "" {
		t.Fatalf("collab_ack: %+v", result)
	}
	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation := tools.Call(env.ctx, "workspace_replace", "frontend-feedback-replace", mustJSON(t, map[string]any{"expected_digest": workspace.Digest, "content": fixture.PeerFrontendV2}))
	if mutation.Error != "" || mutation.Receipt == nil {
		t.Fatalf("workspace_replace: %+v", mutation)
	}
	check := tools.Call(env.ctx, "workspace_check", "frontend-feedback-check", []byte(`{}`))
	if check.Receipt == nil {
		t.Fatalf("workspace_check: %+v", check)
	}
	progress := tools.Call(env.ctx, "work_checkpoint", "frontend-feedback-progress", mustJSON(t, Checkpoint{Kind: CheckpointProgress, Summary: "preserve progress", Facts: []string{"workspace changed"}, Decisions: []string{"continue"}, Rejected: []string{"finalize early"}, EvidenceRefs: []string{mutation.Receipt.ID, check.Receipt.ID}, NextAction: "continue"}))
	if progress.Error != "" || progress.Receipt == nil {
		t.Fatalf("progress checkpoint: %+v", progress)
	}

	result := tools.Call(env.ctx, "collab_apply", "frontend-feedback-apply", mustJSON(t, map[string]any{
		"obligation_id": message.ObligationID, "contract_revision_id": revision.ID, "workspace_revision": workspace.Revision + 1,
		"evidence_refs": []string{mutation.Receipt.ID, check.Receipt.ID, progress.Receipt.ID},
	}))
	if result.Error != string(core.Denied) {
		t.Fatalf("unsupported checkpoint receipt was not denied: %+v", result)
	}
	var rejection PeerToolRejection
	must(t, json.Unmarshal(mustJSON(t, result.Data), &rejection))
	if rejection.ReasonCode != "evidence_ref_invalid" || len(rejection.InvalidEvidenceRefs) != 1 || rejection.InvalidEvidenceRefs[0].Ref != progress.Receipt.ID || rejection.InvalidEvidenceRefs[0].DetectedType != "work.checkpoint" {
		t.Fatalf("unsupported checkpoint feedback was not precise: %+v", rejection)
	}
	if len(rejection.AcceptedEvidenceTypes) != 3 || rejection.AcceptedEvidenceTypes[0] != "workspace.replace" || rejection.AcceptedEvidenceTypes[1] != "workspace.check" || rejection.AcceptedEvidenceTypes[2] != "collab.apply" {
		t.Fatalf("accepted evidence types were not public and stable: %+v", rejection)
	}
}

func TestFrontendInteractionRejectsStaleAndAmbiguousReferences(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "frontend-apply-negative")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply the accepted revision", Actionable: true}, "frontend-apply-negative-message")
	must(t, err)
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	if result := tools.Call(env.ctx, "collab_inbox", "frontend-negative-inbox", []byte(`{}`)); result.Error != "" {
		t.Fatalf("collab_inbox: %+v", result)
	}
	ackArgs, err := json.Marshal(map[string]string{"message_id": message.ID})
	must(t, err)
	if result := tools.Call(env.ctx, "collab_ack", "frontend-negative-ack", ackArgs); result.Error != "" {
		t.Fatalf("collab_ack: %+v", result)
	}

	noChangeArgs, err := json.Marshal(map[string]any{"obligation_id": message.ObligationID, "contract_revision_id": revision.ID, "workspace_revision": 1, "evidence_refs": []string{"fake-receipt"}})
	must(t, err)
	noChange := tools.Call(env.ctx, "collab_apply", "frontend-no-change", noChangeArgs)
	if noChange.Error != string(core.Denied) || rejectionReason(t, noChange) != "workspace_not_changed_since_observation" {
		t.Fatalf("unchanged workspace was not rejected specifically: %+v", noChange)
	}

	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation := tools.Call(env.ctx, "workspace_replace", "frontend-negative-replace", mustJSON(t, map[string]any{"expected_digest": workspace.Digest, "content": fixture.PeerFrontendV2}))
	if mutation.Error != "" || mutation.Receipt == nil {
		t.Fatalf("workspace_replace: %+v", mutation)
	}
	fakeArgs, err := json.Marshal(map[string]any{"obligation_id": message.ObligationID, "contract_revision_id": revision.ID, "workspace_revision": workspace.Revision + 1, "evidence_refs": []string{"fake-receipt"}})
	must(t, err)
	fake := tools.Call(env.ctx, "collab_apply", "frontend-fake-receipt", fakeArgs)
	if fake.Error != string(core.Denied) || rejectionReason(t, fake) != "evidence_ref_invalid" {
		t.Fatalf("fake receipt was not rejected specifically: %+v", fake)
	}
	staleRevision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":"", "total":0}`, "frontend-apply-stale")
	if staleRevision.ID == revision.ID {
		t.Fatal("stale-contract setup did not create a new revision")
	}
	staleArgs, err := json.Marshal(map[string]any{"obligation_id": message.ObligationID, "contract_revision_id": revision.ID, "workspace_revision": workspace.Revision + 1, "evidence_refs": []string{mutation.Receipt.ID}})
	must(t, err)
	stale := tools.Call(env.ctx, "collab_apply", "frontend-stale-contract", staleArgs)
	if stale.Error != string(core.Denied) || rejectionReason(t, stale) != "obligation_not_current" {
		t.Fatalf("stale obligation/contract was not rejected specifically: %+v", stale)
	}

	legacyCheckpoint := tools.Call(env.ctx, "work_checkpoint", "frontend-natural-language-evidence", []byte(`{"kind":"progress","summary":"checkpoint","facts":["fact"],"decisions":["decision"],"rejected":["rejected"],"evidence":["workspace check passed in prose"],"next_action":"continue"}`))
	if legacyCheckpoint.Error != string(core.Malformed) || rejectionReason(t, legacyCheckpoint) != "evidence_refs_required" {
		t.Fatalf("natural-language checkpoint evidence was not rejected specifically: %+v", legacyCheckpoint)
	}
}

func rejectionReason(t *testing.T, result ToolResult) string {
	t.Helper()
	raw, err := json.Marshal(result.Data)
	must(t, err)
	var rejection map[string]any
	must(t, json.Unmarshal(raw, &rejection))
	reason, _ := rejection["reason_code"].(string)
	return reason
}
