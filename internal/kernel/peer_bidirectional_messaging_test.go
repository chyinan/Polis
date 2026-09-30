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

func TestProductWorkerDirectMessageToolSurfaceCompletesAnActionableRequest(t *testing.T) {
	env := newPeerHardeningEnv(t, 32)
	productTools := EmployeeTools{Kernel: env.k, Binding: env.backend, ProductSurface: true, DirectMessagingSurface: true}
	workCurrent := productTools.Call(env.ctx, "work_current", "direct-work-current", []byte(`{}`))
	if workCurrent.Error != "" {
		t.Fatalf("work_current returned %s: %s", workCurrent.Error, workCurrent.Detail)
	}
	handover, ok := workCurrent.Data.(HandoverBundle)
	if !ok || len(handover.DirectMessageTargets) == 0 {
		t.Fatalf("product Worker handover omitted direct-message targets: %#v", workCurrent.Data)
	}
	foundFrontend := false
	for _, target := range handover.DirectMessageTargets {
		if target.EmployeeID == "emp-frontend" && target.TaskID == env.fixture.Frontend.ID && (target.State == "ready" || target.State == "working") {
			foundFrontend = true
		}
	}
	if !foundFrontend {
		t.Fatalf("product Worker handover did not expose the current frontend Task: %+v", handover.DirectMessageTargets)
	}

	sendArgs, err := json.Marshal(map[string]any{
		"to_employee_id": "emp-frontend", "to_task_id": env.fixture.Frontend.ID,
		"body": "Please update the accepted response and verify the current pagination contract.", "actionable": true,
	})
	must(t, err)
	sent := productTools.Call(env.ctx, "collab_send", "product-send-request", sendArgs)
	if sent.Error != "" {
		t.Fatalf("product collab_send returned %s: %s", sent.Error, sent.Detail)
	}
	message, ok := sent.Data.(ProductDirectMessage)
	if !ok || message.ID == "" || message.ObligationID == "" || message.TaskID != env.fixture.Frontend.ID {
		t.Fatalf("product direct send result=%+v, want a persisted frontend obligation", sent.Data)
	}

	recipientTools := EmployeeTools{Kernel: env.k, Binding: env.frontend, ProductSurface: true, DirectMessagingSurface: true}
	inboxResult := recipientTools.Call(env.ctx, "collab_inbox", "product-inbox", []byte(`{}`))
	if inboxResult.Error != "" {
		t.Fatalf("product collab_inbox returned %s: %s", inboxResult.Error, inboxResult.Detail)
	}
	inbox, ok := inboxResult.Data.(ProductDirectInbox)
	if !ok || inbox.Message == nil || inbox.Message.ID != message.ID || inbox.Message.SenderID != "emp-backend" || inbox.Message.Body != "Please update the accepted response and verify the current pagination contract." {
		t.Fatalf("product inbox=%+v, want the sender's exact message", inboxResult.Data)
	}
	ackArgs, err := json.Marshal(map[string]any{"message_id": message.ID})
	must(t, err)
	if ack := recipientTools.Call(env.ctx, "collab_ack", "product-ack", ackArgs); ack.Error != "" {
		t.Fatalf("product collab_ack returned %s: %s", ack.Error, ack.Detail)
	}

	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation, err := env.k.TXReplaceAtRevision(env.ctx, env.frontend, "product-request-change", workspace.Digest, workspace.Revision, fixture.PeerFrontendV2)
	must(t, err)
	preApplyCheck, err := env.k.TXPeerCheck(env.ctx, env.frontend, "product-direct-preapply-check")
	must(t, err)
	applyArgs, err := json.Marshal(map[string]any{
		"obligation_id": message.ObligationID, "workspace_revision": workspace.Revision + 1,
		"evidence_refs": []string{mutation.ID, preApplyCheck.Receipt.ID},
	})
	must(t, err)
	if applied := recipientTools.Call(env.ctx, "collab_apply", "product-apply", applyArgs); applied.Error != "" {
		t.Fatalf("product collab_apply returned %s: %s", applied.Error, applied.Detail)
	}
	checked, err := env.k.TXPeerCheck(env.ctx, env.frontend, "product-direct-check")
	must(t, err)
	_, err = env.k.TXCheckpoint(env.ctx, env.frontend, "product-direct-checkpoint", Checkpoint{
		Kind: CheckpointQualified, Summary: "handled product direct request",
		Facts: []string{"the requested frontend response was checked"}, Decisions: []string{"publish the current candidate"},
		Rejected: []string{"publish unchecked changes"}, EvidenceRefs: []string{checked.Receipt.ID},
	})
	must(t, err)
	frontendHandover, err := env.k.Handover(env.ctx, env.frontend)
	must(t, err)
	artifact, err := env.k.TXSubmit(env.ctx, env.frontend, frontendHandover.Task, "product-direct-candidate", []byte(fixture.PeerFrontendV2))
	must(t, err)
	resolveArgs, err := json.Marshal(map[string]any{"obligation_id": message.ObligationID, "artifact_id": artifact.ID})
	must(t, err)
	if resolved := recipientTools.Call(env.ctx, "obligation_resolve", "product-resolve", resolveArgs); resolved.Error != "" {
		t.Fatalf("product obligation_resolve returned %s: %s", resolved.Error, resolved.Detail)
	}
	var messageState, obligationState string
	if err = env.k.pool.QueryRow(env.ctx, `SELECT m.delivery_state,o.state FROM messages m JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id WHERE m.company_id=$1 AND m.id=$2`, env.scope.company, message.ID).Scan(&messageState, &obligationState); err != nil {
		t.Fatal(err)
	}
	if messageState != "resolved" || obligationState != "fulfilled" {
		t.Fatalf("product direct lifecycle=(%s,%s), want (resolved,fulfilled)", messageState, obligationState)
	}
}

func TestProductDirectFYIsHaveNoObligationAndAdvanceInEventOrder(t *testing.T) {
	env := newPeerHardeningEnv(t, 24)
	scheduleBefore, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	must(t, err)
	sender := EmployeeTools{Kernel: env.k, Binding: env.backend, ProductSurface: true, DirectMessagingSurface: true}
	recipient := EmployeeTools{Kernel: env.k, Binding: env.frontend, ProductSurface: true, DirectMessagingSurface: true}
	for index, body := range []string{"First FYI", "Second FYI"} {
		args, err := json.Marshal(ProductDirectMessageInput{ToEmployeeID: "emp-frontend", ToTaskID: env.fixture.Frontend.ID, Body: body})
		must(t, err)
		key := []string{"one", "two"}[index]
		result := sender.Call(env.ctx, "collab_send", "product-fyi-send-"+key, args)
		if result.Error != "" {
			t.Fatalf("FYI collab_send returned %s: %s", result.Error, result.Detail)
		}
		message, ok := result.Data.(ProductDirectMessage)
		if !ok || message.ID == "" || message.Kind != "fyi" || message.ObligationID != "" {
			t.Fatalf("FYI send result=%+v, want a persisted message without an obligation", result.Data)
		}
	}
	scheduleAfterSend, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	must(t, err)
	if scheduleAfterSend.WorkGeneration != scheduleBefore.WorkGeneration || scheduleAfterSend.State != scheduleBefore.State {
		t.Fatalf("FYI send changed recipient schedule from %+v to %+v; FYIs must not wake employees", scheduleBefore, scheduleAfterSend)
	}
	firstResult := recipient.Call(env.ctx, "collab_inbox", "product-fyi-inbox-first", []byte(`{}`))
	if firstResult.Error != "" {
		t.Fatalf("first FYI inbox returned %s: %s", firstResult.Error, firstResult.Detail)
	}
	first, ok := firstResult.Data.(ProductDirectInbox)
	if !ok || first.Message == nil || first.Message.Body != "First FYI" {
		t.Fatalf("first FYI inbox=%+v, want the first sent FYI", firstResult.Data)
	}
	ack, err := json.Marshal(map[string]string{"message_id": first.Message.ID})
	must(t, err)
	if ackResult := recipient.Call(env.ctx, "collab_ack", "product-fyi-ack-first", ack); ackResult.Error != "" {
		t.Fatalf("first FYI acknowledgement returned %s: %s", ackResult.Error, ackResult.Detail)
	}
	secondResult := recipient.Call(env.ctx, "collab_inbox", "product-fyi-inbox-second", []byte(`{}`))
	if secondResult.Error != "" {
		t.Fatalf("second FYI inbox returned %s: %s", secondResult.Error, secondResult.Detail)
	}
	second, ok := secondResult.Data.(ProductDirectInbox)
	if !ok || second.Message == nil || second.Message.Body != "Second FYI" || second.Message.ID == first.Message.ID {
		t.Fatalf("second FYI inbox=%+v, want the next sent FYI", secondResult.Data)
	}
	var obligations, signals int
	if err = env.k.pool.QueryRow(env.ctx, `SELECT count(*) FROM obligations WHERE company_id=$1`, env.scope.company).Scan(&obligations); err != nil {
		t.Fatal(err)
	}
	if err = env.k.pool.QueryRow(env.ctx, `SELECT count(*) FROM peer_work_signals WHERE company_id=$1`, env.scope.company).Scan(&signals); err != nil {
		t.Fatal(err)
	}
	if obligations != 0 || signals != 0 {
		t.Fatalf("FYIs created obligations/signals=(%d,%d), want (0,0)", obligations, signals)
	}
}

func TestProductDirectMessageSurfaceIsExplicitAndReadOnlyCannotWrite(t *testing.T) {
	env := newPeerHardeningEnv(t, 16)
	args := []byte(`{"to_employee_id":"emp-frontend","to_task_id":"` + env.fixture.Frontend.ID + `","body":"FYI","actionable":false}`)
	withoutSurface := (EmployeeTools{Kernel: env.k, Binding: env.backend, ProductSurface: true}).Call(env.ctx, "collab_send", "direct-disabled", args)
	if withoutSurface.Error != string(core.Denied) {
		t.Fatalf("collab_send without the explicit surface error=%q, want %q", withoutSurface.Error, core.Denied)
	}
	readOnly := (EmployeeTools{Kernel: env.k, Binding: env.backend, ProductSurface: true, DirectMessagingSurface: true, ReadOnly: true}).Call(env.ctx, "collab_send", "direct-read-only", args)
	if readOnly.Error != string(core.Denied) {
		t.Fatalf("read-only collab_send error=%q, want %q", readOnly.Error, core.Denied)
	}
}

func TestProductDirectMessageRejectsTargetsOutsideTheCurrentMission(t *testing.T) {
	env := newPeerHardeningEnv(t, 16)
	otherMission, err := env.k.TXCreateMissionGoal(env.ctx, env.scope, "Other mission", "A target from another Mission must not be addressable", "product-direct-other-mission-create")
	must(t, err)
	if _, err = env.k.TXStartMissionCommand(env.ctx, env.scope, otherMission.ID, "product-direct-other-mission-start"); err != nil {
		t.Fatal(err)
	}
	otherTaskID := newID()
	if _, err = env.k.pool.Exec(env.ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
VALUES($1,$2,$3,'emp-review','review','ready')`, env.scope.company, otherTaskID, otherMission.ID); err != nil {
		t.Fatal(err)
	}
	tools := EmployeeTools{Kernel: env.k, Binding: env.backend, ProductSurface: true, DirectMessagingSurface: true}
	current := tools.Call(env.ctx, "work_current", "product-direct-target-scope-current", []byte(`{}`))
	if current.Error != "" {
		t.Fatalf("work_current returned %s: %s", current.Error, current.Detail)
	}
	handover := current.Data.(HandoverBundle)
	for _, target := range handover.DirectMessageTargets {
		if target.TaskID == otherTaskID {
			t.Fatalf("work_current exposed cross-Mission Task %+v", target)
		}
	}
	args, err := json.Marshal(ProductDirectMessageInput{ToEmployeeID: "emp-review", ToTaskID: otherTaskID, Body: "wrong mission", Actionable: true})
	must(t, err)
	result := tools.Call(env.ctx, "collab_send", "product-direct-cross-mission", args)
	if result.Error != string(core.Denied) {
		t.Fatalf("cross-Mission collab_send error=%q, want %q", result.Error, core.Denied)
	}
	var count int
	if err = env.k.pool.QueryRow(env.ctx, `SELECT count(*) FROM messages WHERE company_id=$1 AND body='wrong mission'`, env.scope.company).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cross-Mission message was persisted %d times", count)
	}
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
