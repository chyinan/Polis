// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/fixture"
)

func TestTXPeerSendRejectsSourceTaskOutsideSessionBinding(t *testing.T) {
	k := &Kernel{}
	binding := Binding{employee: "emp-backend", task: "bound-task"}
	_, err := k.TXPeerSend(context.Background(), binding, PeerSendInput{FromTask: "other-task"}, "unbound-source")
	wantCode(t, err, core.Denied)
}

func TestPeerDirectMessageCanBeSentAndResolvedInBothDirections(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "bidirectional-message")

	frontendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	sendArgs, err := json.Marshal(map[string]any{
		"to_employee_id":       "emp-backend",
		"to_task_id":           env.fixture.Backend.ID,
		"contract_revision_id": revision.ID,
		"body":                 "Please update the current response contract and verify pagination.",
		"actionable":           true,
	})
	must(t, err)
	sent := frontendTools.Call(env.ctx, "collab_send", "frontend-to-backend-send", sendArgs)
	if sent.Error != "" {
		t.Fatalf("frontend collab_send returned %s: %s", sent.Error, sent.Detail)
	}
	message, ok := sent.Data.(PeerMessage)
	if !ok || message.ID == "" || message.ObligationID == "" || message.TaskID != env.fixture.Backend.ID {
		t.Fatalf("frontend direct message receipt=%+v, want a persisted backend obligation", sent.Data)
	}

	backendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	inbox := backendTools.Call(env.ctx, "collab_inbox", "backend-direct-inbox", []byte(`{}`))
	if inbox.Error != "" {
		t.Fatalf("backend collab_inbox returned %s: %s", inbox.Error, inbox.Detail)
	}
	received, ok := inbox.Data.(PeerInbox)
	if !ok || received.Message.ID != message.ID || received.Sender != "emp-frontend" || received.Body != "Please update the current response contract and verify pagination." {
		t.Fatalf("backend inbox=%+v, want the frontend's exact message", inbox.Data)
	}
	ackArgs, err := json.Marshal(map[string]any{"message_id": message.ID})
	must(t, err)
	if ack := backendTools.Call(env.ctx, "collab_ack", "backend-direct-ack", ackArgs); ack.Error != "" {
		t.Fatalf("backend collab_ack returned %s: %s", ack.Error, ack.Detail)
	}

	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	workspace, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	mutation, err := env.k.TXReplace(env.ctx, env.backend, "backend-direct-response", workspace.Digest, fixture.PeerBackendV2)
	must(t, err)
	_, err = env.k.TXPeerApply(env.ctx, env.backend, PeerApplyRequest{
		ObligationID: message.ObligationID, ContractRevisionID: revision.ID,
		WorkspaceRevision: workspace.Revision + 1, EvidenceRefs: []string{mutation.ID},
	}, "backend-direct-apply")
	must(t, err)
	checked, err := env.k.TXPeerCheck(env.ctx, env.backend, "backend-direct-check")
	must(t, err)
	_, err = env.k.TXCheckpoint(env.ctx, env.backend, "backend-direct-qualified", Checkpoint{
		Kind: CheckpointQualified, Summary: "backend handled direct peer request",
		Facts:     []string{"the current accepted contract was implemented"},
		Decisions: []string{"submit the checked response"}, Rejected: []string{"submit unchecked content"},
		EvidenceRefs: []string{checked.Receipt.ID}, NextAction: "publish the response candidate",
	})
	must(t, err)
	artifact, err := env.k.TXSubmit(env.ctx, env.backend, backendHandover.Task, "backend-direct-candidate", []byte(fixture.PeerBackendV2))
	must(t, err)
	if err := env.k.TXPeerResolve(env.ctx, env.backend, message.ObligationID, artifact.ID, "backend-direct-resolve"); err != nil {
		t.Fatalf("backend could not resolve the direct message with its own candidate: %v", err)
	}
}

func TestPeerFYIMessageCanBeReadAndAcknowledgedByEitherPeer(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "bidirectional-fyi")
	frontendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	args, err := json.Marshal(map[string]any{
		"to_employee_id":       "emp-backend",
		"to_task_id":           env.fixture.Backend.ID,
		"contract_revision_id": revision.ID,
		"body":                 "FYI: the contract has a new example response.",
		"actionable":           false,
	})
	must(t, err)
	sent := frontendTools.Call(env.ctx, "collab_send", "frontend-fyi-send", args)
	if sent.Error != "" {
		t.Fatalf("frontend FYI send returned %s: %s", sent.Error, sent.Detail)
	}
	message, ok := sent.Data.(PeerMessage)
	if !ok || message.ID == "" || message.ObligationID != "" {
		t.Fatalf("FYI receipt=%+v, want a persisted non-actionable message", sent.Data)
	}
	backendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	inbox := backendTools.Call(env.ctx, "collab_inbox", "backend-fyi-inbox", []byte(`{}`))
	if inbox.Error != "" {
		t.Fatalf("backend could not read the FYI: %s %s", inbox.Error, inbox.Detail)
	}
	received, ok := inbox.Data.(PeerInbox)
	if !ok || received.Message.ID != message.ID || received.Sender != "emp-frontend" || received.Body != "FYI: the contract has a new example response." {
		t.Fatalf("backend FYI inbox=%+v", inbox.Data)
	}
	ackArgs, err := json.Marshal(map[string]any{"message_id": message.ID})
	must(t, err)
	if ack := backendTools.Call(env.ctx, "collab_ack", "backend-fyi-ack", ackArgs); ack.Error != "" {
		t.Fatalf("backend could not acknowledge the FYI: %s %s", ack.Error, ack.Detail)
	}
}

func TestPeerFYIInboxAdvancesAfterAcknowledgement(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "fyi-inbox-advance")
	frontendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.frontend, Role: "peer_frontend"}
	for i, body := range []string{"first FYI", "second FYI"} {
		label := []string{"first", "second"}[i]
		args, err := json.Marshal(map[string]any{
			"to_employee_id": "emp-backend", "to_task_id": env.fixture.Backend.ID,
			"contract_revision_id": revision.ID, "body": body, "actionable": false,
		})
		must(t, err)
		if sent := frontendTools.Call(env.ctx, "collab_send", "frontend-fyi-order-"+label, args); sent.Error != "" {
			t.Fatalf("FYI send returned %s: %s", sent.Error, sent.Detail)
		}
	}
	backendTools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	var firstID string
	for i, wantBody := range []string{"first FYI", "second FYI"} {
		label := []string{"first", "second"}[i]
		inbox := backendTools.Call(env.ctx, "collab_inbox", "backend-fyi-page-"+label, []byte(`{}`))
		if inbox.Error != "" {
			t.Fatalf("FYI read %d returned %s: %s", i+1, inbox.Error, inbox.Detail)
		}
		received, ok := inbox.Data.(PeerInbox)
		if !ok || received.Body != wantBody || received.Message.ID == firstID {
			t.Fatalf("FYI read %d=%+v, want next unacknowledged body %q", i+1, inbox.Data, wantBody)
		}
		firstID = received.Message.ID
		ackArgs, err := json.Marshal(map[string]any{"message_id": received.Message.ID})
		must(t, err)
		if ack := backendTools.Call(env.ctx, "collab_ack", "backend-fyi-page-ack-"+label, ackArgs); ack.Error != "" {
			t.Fatalf("FYI ack %d returned %s: %s", i+1, ack.Error, ack.Detail)
		}
	}
	if inbox := backendTools.Call(env.ctx, "collab_inbox", "backend-fyi-empty", []byte(`{}`)); inbox.Error != string(core.OutOfScope) {
		t.Fatalf("empty FYI inbox returned %+v, want OUT_OF_SCOPE", inbox)
	}
}

func TestPeerDirectMessageCannotCrossMissionWithinCompany(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "cross-mission-message")
	otherMissionID, otherTaskID := newID(), newID()
	_, err := env.k.pool.Exec(env.ctx, "INSERT INTO missions(company_id,id,state,contract) VALUES($1,$2,'draft','r03-api@1')", env.scope.company, otherMissionID)
	must(t, err)
	_, err = env.k.pool.Exec(env.ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,'emp-frontend','peer_frontend')", env.scope.company, otherTaskID, otherMissionID)
	must(t, err)
	handover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)

	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{
		FromTask: handover.Task.ID, ToEmployeeID: "emp-frontend", ToTaskID: otherTaskID,
		ContractRevisionID: revision.ID, Body: "this message must remain in the current Mission", Actionable: false,
	}, "cross-mission-peer-message")
	wantCode(t, err, core.Denied)
}

func TestPeerDirectMessageCannotTargetFinalizedTask(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "finalized-recipient-message")
	if _, err := env.k.pool.Exec(env.ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2", env.scope.company, env.fixture.Frontend.ID); err != nil {
		t.Fatal(err)
	}
	handover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{
		FromTask: handover.Task.ID, ToEmployeeID: "emp-frontend", ToTaskID: env.fixture.Frontend.ID,
		ContractRevisionID: revision.ID, Body: "a finalized peer task is not current work", Actionable: false,
	}, "finalized-peer-message")
	wantCode(t, err, core.Denied)
}

func TestPeerResolveRequiresObligationTaskBoundToSession(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	missionID, taskID, messageID, artifactID := newID(), newID(), newID(), newID()
	_, err := env.k.pool.Exec(env.ctx, "INSERT INTO missions(company_id,id,state,contract) VALUES($1,$2,'draft','r03-api@1')", env.scope.company, missionID)
	must(t, err)
	_, err = env.k.pool.Exec(env.ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,'emp-backend','peer_backend')", env.scope.company, taskID, missionID)
	must(t, err)
	_, err = env.k.pool.Exec(env.ctx, "INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body,delivery_state) VALUES($1,$2,$3,$4,'emp-frontend','emp-backend','request','foreign task obligation','applied')", env.scope.company, messageID, missionID, taskID)
	must(t, err)
	_, err = env.k.pool.Exec(env.ctx, "INSERT INTO obligations(company_id,id,task_id,owner,state) VALUES($1,$2,$3,'emp-backend','applied')", env.scope.company, messageID, taskID)
	must(t, err)
	_, err = env.k.pool.Exec(env.ctx, "INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract) VALUES($1,$2,$3,'emp-backend',$4,1,'ready','candidate','r03-api@1')", env.scope.company, artifactID, taskID, strings.Repeat("a", 64))
	must(t, err)

	err = env.k.TXPeerResolve(env.ctx, env.backend, messageID, artifactID, "wrong-task-resolve")
	wantCode(t, err, core.Denied)
	var obligationState, messageState string
	if err = env.k.pool.QueryRow(env.ctx, "SELECT o.state,m.delivery_state FROM obligations o JOIN messages m ON m.company_id=o.company_id AND m.id=o.id WHERE o.company_id=$1 AND o.id=$2", env.scope.company, messageID).Scan(&obligationState, &messageState); err != nil {
		t.Fatal(err)
	}
	if obligationState != "applied" || messageState != "applied" {
		t.Fatalf("mismatched session changed other Task's message: obligation=%s message=%s", obligationState, messageState)
	}
}
