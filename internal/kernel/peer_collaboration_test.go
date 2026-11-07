// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/runner"
	"testing"
)

func TestPeerHandoverBoundarySnapshotReadsStoppedInitialSession(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "boundary-snapshot")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	backendWorkspace, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "boundary-backend-replace", backendWorkspace.Digest, fixture.PeerBackendV2)
	must(t, err)
	backendCheck, err := env.k.TXPeerCheck(env.ctx, env.backend, "boundary-backend-check")
	must(t, err)
	_, err = env.k.TXCheckpoint(env.ctx, env.backend, "boundary-backend-qualified", Checkpoint{Kind: CheckpointQualified, Summary: "backend qualified", Facts: []string{"current contract checked"}, Decisions: []string{"deliver"}, Rejected: []string{"unchecked"}, EvidenceRefs: []string{backendCheck.Receipt.ID}, NextAction: "deliver"})
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply current revision", Actionable: true}, "boundary-message")
	must(t, err)
	// Peer delivery is part of the working-task phase. Artifact publication
	// intentionally follows collab.send and transitions the task to candidate.
	_, err = env.k.TXSubmit(env.ctx, env.backend, backendHandover.Task, "boundary-backend-artifact", []byte(fixture.PeerBackendV2))
	must(t, err)
	_, err = env.k.PeerInbox(env.ctx, env.frontend)
	must(t, err)
	must(t, env.k.TXPeerAck(env.ctx, env.frontend, message.ID, "boundary-ack"))
	workspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation, err := env.k.TXReplace(env.ctx, env.frontend, "boundary-replace", workspace.Digest, fixture.PeerFrontendV2)
	must(t, err)
	applied, err := env.k.TXPeerApply(env.ctx, env.frontend, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: workspace.Revision + 1, EvidenceRefs: []string{mutation.ID}}, "boundary-apply")
	must(t, err)
	checked, err := env.k.TXPeerCheck(env.ctx, env.frontend, "boundary-check")
	must(t, err)
	checkpoint, err := env.k.TXCheckpoint(env.ctx, env.frontend, "boundary-progress", Checkpoint{Kind: CheckpointProgress, Summary: "frontend progress", Facts: []string{"workspace changed"}, Decisions: []string{"continue"}, Rejected: []string{"finalize early"}, EvidenceRefs: []string{mutation.ID, applied.ID, checked.Receipt.ID}, NextAction: "continue"})
	must(t, err)
	handover, err := env.k.PeerHandover(env.ctx, env.frontend)
	must(t, err)
	if checkpoint.ID == "" || handover.WorkspaceRevision != workspace.Revision+1 {
		t.Fatalf("boundary state was not persisted before stop: checkpoint=%+v handover=%+v", checkpoint, handover)
	}
	must(t, env.k.TXBeginStop(env.ctx, env.frontend))
	proof, err := env.frontendProcess.Stop()
	must(t, err)
	must(t, env.k.TXConfirmStopped(env.ctx, env.frontend, proof))
	snapshot, err := env.k.PeerHandoverBoundarySnapshot(env.ctx, env.frontend, handover)
	must(t, err)
	if snapshot.SchemaVersion == "" || snapshot.EmployeeID != "emp-frontend" || snapshot.SessionID != env.frontend.SessionID() || snapshot.Handover.WorkspaceRevision != handover.WorkspaceRevision || snapshot.Workspace.Revision != handover.WorkspaceRevision || snapshot.Contract.ID != revision.ID || len(snapshot.CAS) == 0 || snapshot.CASManifestDigest == "" {
		t.Fatalf("stopped initial boundary snapshot is incomplete: %+v", snapshot)
	}
}

func TestPeerSendDeniesFinalizedCandidateTask(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "send-after-artifact")
	handover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	workspace, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "send-after-artifact-replace", workspace.Digest, fixture.PeerBackendV2)
	must(t, err)
	checked, err := env.k.TXPeerCheck(env.ctx, env.backend, "send-after-artifact-check")
	must(t, err)
	_, err = env.k.TXCheckpoint(env.ctx, env.backend, "send-after-artifact-checkpoint", Checkpoint{Kind: CheckpointQualified, Summary: "candidate qualified", Facts: []string{"current contract checked"}, Decisions: []string{"publish"}, Rejected: []string{"unchecked"}, EvidenceRefs: []string{checked.Receipt.ID}, NextAction: "publish"})
	must(t, err)
	_, err = env.k.TXSubmit(env.ctx, env.backend, handover.Task, "send-after-artifact-submit", []byte(fixture.PeerBackendV2))
	must(t, err)
	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: handover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "late delivery", Actionable: true}, "send-after-artifact")
	wantCode(t, err, core.Denied)
}

func TestPeerIntegrationUsesRuntimePaginationVerifierAndSeparateCoverage(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items", fixture.PeerPaginationContractV3, "pagination-integration")
	backendHandover, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	backendWorkspace, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "pagination-backend-replace", backendWorkspace.Digest, fixture.PeerBackendPaginationReference)
	must(t, err)
	backendCheck, err := env.k.TXPeerCheck(env.ctx, env.backend, "pagination-backend-check")
	must(t, err)
	if !backendCheck.Passed {
		t.Fatalf("reference backend was not behaviorally qualified: %+v", backendCheck)
	}
	_, err = env.k.TXCheckpoint(env.ctx, env.backend, "pagination-backend-checkpoint", Checkpoint{Kind: CheckpointQualified, Summary: "pagination backend qualified", Facts: []string{"runtime pagination behavior passed"}, Decisions: []string{"send and submit"}, Rejected: []string{"unchecked"}, EvidenceRefs: []string{backendCheck.Receipt.ID}, NextAction: "send"})
	must(t, err)
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{FromTask: backendHandover.Task.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply pagination contract", Actionable: true}, "pagination-send")
	must(t, err)
	backendArtifact, err := env.k.TXSubmit(env.ctx, env.backend, backendHandover.Task, "pagination-backend-submit", []byte(fixture.PeerBackendPaginationReference))
	must(t, err)
	_, err = env.k.PeerInbox(env.ctx, env.frontend)
	must(t, err)
	must(t, env.k.TXPeerAck(env.ctx, env.frontend, message.ID, "pagination-ack"))
	frontendWorkspace, err := env.k.Workspace(env.ctx, env.frontend)
	must(t, err)
	mutation, err := env.k.TXReplace(env.ctx, env.frontend, "pagination-frontend-replace", frontendWorkspace.Digest, fixture.FrontendConsumptionReferenceSource)
	must(t, err)
	_, err = env.k.TXPeerApply(env.ctx, env.frontend, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: frontendWorkspace.Revision + 1, EvidenceRefs: []string{mutation.ID}}, "pagination-apply")
	must(t, err)
	frontendCheck, err := env.k.TXPeerCheck(env.ctx, env.frontend, "pagination-frontend-check")
	must(t, err)
	if !frontendCheck.Passed {
		t.Fatalf("reference frontend was not behaviorally qualified: %+v", frontendCheck)
	}
	_, err = env.k.TXCheckpoint(env.ctx, env.frontend, "pagination-frontend-checkpoint", Checkpoint{Kind: CheckpointQualified, Summary: "pagination candidate qualified", Facts: []string{"runtime pagination behavior passed"}, Decisions: []string{"submit"}, Rejected: []string{"unchecked"}, EvidenceRefs: []string{frontendCheck.Receipt.ID}, NextAction: "submit"})
	must(t, err)
	frontendHandover, err := env.k.Handover(env.ctx, env.frontend)
	must(t, err)
	frontendArtifact, err := env.k.TXSubmit(env.ctx, env.frontend, frontendHandover.Task, "pagination-frontend-submit", []byte(fixture.FrontendConsumptionReferenceSource))
	must(t, err)
	must(t, env.k.TXPeerResolve(env.ctx, env.frontend, message.ObligationID, frontendArtifact.ID, "pagination-resolve"))
	integration, err := env.k.TXFreezePeerIntegration(env.ctx, env.scope, PeerIntegrationInput{Mission: env.fixture.Mission, BackendArtifactID: backendArtifact.ID, FrontendArtifactID: frontendArtifact.ID, ContractRevisionID: revision.ID, BaseRevision: "api-v1", VerifierRevision: fixture.FrontendPaginationBehaviorVerifierRevision}, "pagination-freeze")
	must(t, err)
	report, err := env.k.VerifyPeerIntegration(env.ctx, env.scope, integration.ID)
	must(t, err)
	if !report.Passed || report.LifecycleBinding != "passed" || report.CollaborationCausality != "passed" || report.PaginationRuntimeBehavior != "passed" || report.ResponseShapeCompatibility != "passed" {
		t.Fatalf("runtime pagination integration did not pass by separate dimensions: %+v", report)
	}
}

func TestPeerInboxMakesMessageObservableBeforeAck(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-inbox-observe-"+newID())
	must(t, err)
	fx, err := k.TXCreatePeerFixture(ctx, scope, "r03a-inbox-observe")
	must(t, err)
	backend, err := k.BindFake(ctx, scope, "emp-backend")
	must(t, err)
	backendTask, err := k.TXClaim(ctx, backend)
	must(t, err)
	revision, err := k.TXProposePeerContract(ctx, backend, backendTask, PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}, "inbox-propose")
	must(t, err)
	must(t, k.TXAcceptPeerContract(ctx, backend, revision.ID, "inbox-accept"))
	frontend, err := k.TXNewWorker(ctx, scope, fx.Frontend.ID, "fake/frontend")
	must(t, err)
	p, err := runner.Start(frontend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, frontend, p))
	must(t, k.TXValidateWorker(ctx, frontend))
	must(t, k.TXActivateWorker(ctx, frontend, "frontend-inbox-observe"))
	message, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: fx.Frontend.ID, ContractRevisionID: revision.ID, Body: "observe and apply", Actionable: true}, "inbox-send")
	must(t, err)

	tools := PeerEmployeeTools{Kernel: k, Binding: frontend, Role: "peer_frontend", Initial: true}
	inboxRaw, err := json.Marshal(struct{}{})
	must(t, err)
	inbox := tools.Call(ctx, "collab_inbox", "inbox-read", inboxRaw)
	if inbox.Error != "" {
		t.Fatalf("collab_inbox returned %s: %s", inbox.Error, inbox.Detail)
	}
	var received PeerInbox
	rawReceived, err := json.Marshal(inbox.Data)
	must(t, err)
	must(t, json.Unmarshal(rawReceived, &received))
	if received.Message.ID != message.ID || received.Message.DeliveryState != "observed" || received.Contract.ID != revision.ID {
		t.Fatalf("inbox did not expose observed current message: %+v", received)
	}
	ackArgs, err := json.Marshal(map[string]string{"message_id": message.ID})
	must(t, err)
	ack := tools.Call(ctx, "collab_ack", "inbox-ack", ackArgs)
	if ack.Error != "" {
		t.Fatalf("collab_ack after inbox observation returned %s: %s", ack.Error, ack.Detail)
	}
	wantPeerStates(t, k, scope, message.ID, "acknowledged", "observed", "acknowledged")
	must(t, k.TXBeginStop(ctx, frontend))
	proof, err := p.Stop()
	must(t, err)
	must(t, k.TXConfirmStopped(ctx, frontend, proof))
}

func peerStates(t *testing.T, k *Kernel, scope Scope, messageID string) (string, string, string) {
	t.Helper()
	var message, obligation, signal string
	must(t, k.pool.QueryRow(context.Background(), `SELECT m.delivery_state,COALESCE(o.state,''),COALESCE(s.state,'')
FROM messages m
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
LEFT JOIN peer_work_signals s ON s.company_id=m.company_id AND s.message_id=m.id
WHERE m.company_id=$1 AND m.id=$2`, scope.company, messageID).Scan(&message, &obligation, &signal))
	return message, obligation, signal
}

func wantPeerStates(t *testing.T, k *Kernel, scope Scope, messageID, wantMessage, wantObligation, wantSignal string) {
	t.Helper()
	message, obligation, signal := peerStates(t, k, scope, messageID)
	if message != wantMessage || obligation != wantObligation || signal != wantSignal {
		t.Fatalf("peer state = (%q,%q,%q), want (%q,%q,%q)", message, obligation, signal, wantMessage, wantObligation, wantSignal)
	}
}

func TestR03APeerSendCommitVisibleToIndependentSession(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-durable-"+newID())
	must(t, err)
	fixtureState, err := k.TXCreatePeerFixture(ctx, scope, "r03a-durable")
	must(t, err)
	backend, err := k.BindFake(ctx, scope, "emp-backend")
	must(t, err)
	frontend, err := k.BindFake(ctx, scope, "emp-frontend")
	must(t, err)
	backendTask, err := k.TXClaim(ctx, backend)
	must(t, err)
	_, err = k.TXClaim(ctx, frontend)
	must(t, err)
	revision, err := k.TXProposePeerContract(ctx, backend, backendTask, PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}, "durable-propose")
	must(t, err)
	must(t, k.TXAcceptPeerContract(ctx, backend, revision.ID, "durable-accept"))
	message, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: fixtureState.Frontend.ID, ContractRevisionID: revision.ID, Body: "durable v2", Actionable: true}, "durable-send")
	must(t, err)

	other, err := pgxpool.New(ctx, dsn)
	must(t, err)
	defer other.Close()
	var state, obligation, signal string
	must(t, other.QueryRow(ctx, `SELECT m.delivery_state,COALESCE(o.state,''),COALESCE(s.state,'')
FROM messages m
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
LEFT JOIN peer_work_signals s ON s.company_id=m.company_id AND s.message_id=m.id
WHERE m.company_id=$1 AND m.id=$2`, scope.company, message.ID).Scan(&state, &obligation, &signal))
	if state != "persisted" || obligation != "pending" || signal != "pending" {
		t.Fatalf("independent session read (%q,%q,%q), want persisted/pending/pending", state, obligation, signal)
	}
}

func TestR03APeerCollaborationPositiveAndNegative(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-positive-company")
	must(t, err)
	fixtureState, err := k.TXCreatePeerFixture(ctx, scope, "r03a-positive")
	must(t, err)
	backend, err := k.BindFake(ctx, scope, "emp-backend")
	must(t, err)
	frontendOwner, err := k.BindFake(ctx, scope, "emp-frontend")
	must(t, err)
	backendTask, err := k.TXClaim(ctx, backend)
	must(t, err)
	frontendTask, err := k.TXClaim(ctx, frontendOwner)
	must(t, err)
	frontend, err := k.TXNewWorker(ctx, scope, frontendTask.ID, "fake/frontend")
	must(t, err)
	frontendProcess, err := runner.Start(frontend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer frontendProcess.Stop()
	must(t, k.TXAttachWorker(ctx, frontend, frontendProcess))
	must(t, k.TXValidateWorker(ctx, frontend))
	must(t, k.TXActivateWorker(ctx, frontend, "frontend-positive"))
	frontendHandover, err := k.Handover(ctx, frontend)
	must(t, err)
	frontendTask = frontendHandover.Task
	revision, err := k.TXProposePeerContract(ctx, backend, backendTask, PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}, "propose-v2")
	must(t, err)
	must(t, k.TXAcceptPeerContract(ctx, backend, revision.ID, "accept-v2"))
	_, err = k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: frontendTask.ID, ContractRevisionID: fixtureState.ContractV1.ID, Body: "late v1", Actionable: true}, "send-late-v1")
	wantCode(t, err, core.Denied)
	message, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: frontendTask.ID, ContractRevisionID: revision.ID, Body: "apply contract revision 2", Actionable: true}, "send-v2")
	must(t, err)
	wantPeerStates(t, k, scope, message.ID, "persisted", "pending", "pending")
	duplicate, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: frontendTask.ID, ContractRevisionID: revision.ID, Body: "apply contract revision 2", Actionable: true}, "send-v2")
	must(t, err)
	if duplicate.ID != message.ID || duplicate.ObligationID != message.ObligationID {
		t.Fatal("duplicate collab.send created a second obligation")
	}
	_, err = k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: frontendTask.ID, ContractRevisionID: revision.ID, Body: "changed body", Actionable: true}, "send-v2")
	wantCode(t, err, core.Conflict)
	fyi, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: frontendTask.ID, ContractRevisionID: revision.ID, Body: "FYI only", Actionable: false}, "send-fyi")
	must(t, err)
	if fyi.ObligationID != "" {
		t.Fatal("FYI created an actionable obligation")
	}
	if _, cross := k.TXPeerApply(ctx, backend, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: 2, EvidenceRefs: []string{"cross-write"}}, "backend-cross-write"); cross == nil {
		t.Fatal("backend wrote frontend workspace")
	}
	must(t, k.TXPeerDeliver(ctx, frontend, message.ID, "deliver-v2"))
	wantPeerStates(t, k, scope, message.ID, "delivered", "pending", "pending")
	must(t, k.TXPeerObserve(ctx, frontend, message.ID, "observe-v2"))
	wantPeerStates(t, k, scope, message.ID, "observed", "observed", "observed")
	must(t, k.TXPeerAck(ctx, frontend, message.ID, "ack-v2"))
	wantPeerStates(t, k, scope, message.ID, "acknowledged", "observed", "acknowledged")
	frontendWorkspace, err := k.Workspace(ctx, frontend)
	must(t, err)
	frontendMutation, err := k.TXReplace(ctx, frontend, "frontend-materialize-v2", frontendWorkspace.Digest, fixture.PeerFrontendV2)
	must(t, err)
	_, err = k.TXPeerApply(ctx, frontend, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: frontendWorkspace.Revision + 1, EvidenceRefs: []string{frontendMutation.ID}}, "apply-v2")
	must(t, err)
	wantPeerStates(t, k, scope, message.ID, "applied", "applied", "applied")
	frontendCheck, err := k.TXPeerCheck(ctx, frontend, "frontend-v2-check")
	must(t, err)
	if !frontendCheck.Passed || frontendCheck.Receipt == nil {
		t.Fatalf("frontend candidate did not pass current check: %+v", frontendCheck)
	}
	_, err = k.TXCheckpoint(ctx, frontend, "frontend-v2-qualified", Checkpoint{Kind: CheckpointQualified, Summary: "frontend candidate checked", Facts: []string{"response shape matches current contract"}, Decisions: []string{"submit current candidate"}, Rejected: []string{"unchecked artifact"}, EvidenceRefs: []string{frontendCheck.Receipt.ID}, NextAction: "submit"})
	must(t, err)
	frontendArtifact, err := k.TXSubmit(ctx, frontend, frontendTask, "frontend-v2", []byte(fixture.PeerFrontendV2))
	must(t, err)
	backendArtifact, err := k.TXSubmit(ctx, backend, backendTask, "backend-v2", []byte(fixture.PeerBackendV2))
	must(t, err)
	must(t, k.TXPeerResolve(ctx, frontend, message.ObligationID, frontendArtifact.ID, "resolve-v2"))
	wantPeerStates(t, k, scope, message.ID, "resolved", "fulfilled", "resolved")
	integration, err := k.TXFreezePeerIntegration(ctx, scope, PeerIntegrationInput{Mission: fixtureState.Mission, BackendArtifactID: backendArtifact.ID, FrontendArtifactID: frontendArtifact.ID, ContractRevisionID: revision.ID, BaseRevision: "api-v1", VerifierRevision: "r03a-verifier@1"}, "freeze-positive")
	must(t, err)
	report, err := k.VerifyPeerIntegration(ctx, scope, integration.ID)
	must(t, err)
	if !report.Passed {
		t.Fatalf("positive peer integration failed: %+v", report)
	}
	review, err := k.PeerReviewIntegration(ctx, scope, integration.ID)
	must(t, err)
	if !review.Passed {
		t.Fatalf("positive fake independent review failed: %+v", review)
	}
	var plannerPath int
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE company_id=$1 AND (sender='emp-planning' OR recipient='emp-planning')", scope.company).Scan(&plannerPath))
	if plannerPath != 0 {
		t.Fatalf("Planner appeared in peer forwarding path: %d", plannerPath)
	}

	negativeScope, err := k.TXCreateCompany(ctx, "r03a-negative-company")
	must(t, err)
	negative, err := k.TXCreatePeerFixture(ctx, negativeScope, "r03a-negative")
	must(t, err)
	negativeBackend, err := k.BindFake(ctx, negativeScope, "emp-backend")
	must(t, err)
	negativeFrontend, err := k.BindFake(ctx, negativeScope, "emp-frontend")
	must(t, err)
	negativeBackendTask, err := k.TXClaim(ctx, negativeBackend)
	must(t, err)
	negativeFrontendTask, err := k.TXClaim(ctx, negativeFrontend)
	must(t, err)
	negativeBackendArtifact, err := k.TXSubmit(ctx, negativeBackend, negativeBackendTask, "backend-v1", []byte(fixture.PeerBackendV1))
	must(t, err)
	negativeFrontendArtifact, err := k.TXSubmit(ctx, negativeFrontend, negativeFrontendTask, "frontend-v1", []byte(fixture.PeerFrontendV1))
	must(t, err)
	negativeIntegration, err := k.TXFreezePeerIntegration(ctx, negativeScope, PeerIntegrationInput{Mission: negative.Mission, BackendArtifactID: negativeBackendArtifact.ID, FrontendArtifactID: negativeFrontendArtifact.ID, ContractRevisionID: negative.ContractV1.ID, BaseRevision: "api-v1", VerifierRevision: "r03a-verifier@1"}, "freeze-negative")
	must(t, err)
	negativeReport, err := k.VerifyPeerIntegration(ctx, negativeScope, negativeIntegration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if negativeReport.Passed {
		t.Fatal("collaboration-suppressed negative control passed")
	}
	negativeReview, err := k.PeerReviewIntegration(ctx, negativeScope, negativeIntegration.ID)
	must(t, err)
	if negativeReview.Passed {
		t.Fatal("negative candidate passed fake independent review")
	}
}

func TestR03AFrontendHandoverPreservesPeerObligation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-handover-"+newID())
	must(t, err)
	mission := "r03a-handover-" + newID()
	fx, err := k.TXCreatePeerFixture(ctx, scope, mission)
	must(t, err)
	backend, err := k.BindFake(ctx, scope, "emp-backend")
	must(t, err)
	backendTask, err := k.TXClaim(ctx, backend)
	must(t, err)
	revision, err := k.TXProposePeerContract(ctx, backend, backendTask, PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}, "handover-propose")
	must(t, err)
	must(t, k.TXAcceptPeerContract(ctx, backend, revision.ID, "handover-accept"))
	frontend, err := k.TXNewWorker(ctx, scope, fx.Frontend.ID, "fake/frontend")
	must(t, err)
	p, err := runner.Start(frontend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, frontend, p))
	must(t, k.TXValidateWorker(ctx, frontend))
	must(t, k.TXActivateWorker(ctx, frontend, "frontend-handover"))
	message, err := k.TXPeerSend(ctx, backend, PeerSendInput{FromTask: backendTask.ID, ToTask: fx.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply v2", Actionable: true}, "handover-send")
	must(t, err)
	must(t, k.TXPeerDeliver(ctx, frontend, message.ID, "handover-deliver"))
	must(t, k.TXPeerObserve(ctx, frontend, message.ID, "handover-observe"))
	must(t, k.TXPeerAck(ctx, frontend, message.ID, "handover-ack"))
	bundle, err := k.PeerHandover(ctx, frontend)
	must(t, err)
	if bundle.ObligationState != "observed" || bundle.MessageState != "acknowledged" || bundle.ContractRevisionID != revision.ID {
		t.Fatalf("handover lost peer state: %+v", bundle)
	}
	must(t, k.TXBeginStop(ctx, frontend))
	proof, err := p.Stop()
	must(t, err)
	must(t, k.TXConfirmStopped(ctx, frontend, proof))
	successor, err := k.TXNewWorker(ctx, scope, fx.Frontend.ID, "fake/frontend")
	must(t, err)
	p2, err := runner.Start(successor.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p2.Stop()
	must(t, k.TXAttachWorker(ctx, successor, p2))
	must(t, k.TXValidateWorker(ctx, successor))
	must(t, k.TXActivateWorker(ctx, successor, "frontend-successor"))
	_, staleApplyErr := k.TXPeerApply(ctx, frontend, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: 2, EvidenceRefs: []string{"stale-apply"}}, "stale-apply")
	wantCode(t, staleApplyErr, core.StaleEpoch)
	workspace, err := k.Workspace(ctx, successor)
	must(t, err)
	mutation, err := k.TXReplace(ctx, successor, "successor-materialize", workspace.Digest, fixture.PeerFrontendV2)
	must(t, err)
	_, err = k.TXPeerApply(ctx, successor, PeerApplyRequest{ObligationID: message.ObligationID, ContractRevisionID: revision.ID, WorkspaceRevision: workspace.Revision + 1, EvidenceRefs: []string{mutation.ID}}, "successor-apply")
	must(t, err)
	// The successor must now qualify its own current workspace before final
	// submission; a collaboration application alone is not acceptance evidence.
	checked, err := k.TXPeerCheck(ctx, successor, "successor-check")
	must(t, err)
	if !checked.Passed {
		t.Fatal("successor current workspace did not qualify")
	}
	_, err = k.TXCheckpoint(ctx, successor, "successor-qualified", Checkpoint{Kind: CheckpointQualified, Summary: "successor checked", Facts: []string{"current contract"}, Decisions: []string{"finalize"}, Rejected: []string{"unchecked submit"}, EvidenceRefs: []string{checked.Receipt.ID}})
	must(t, err)
	artifact, err := k.TXSubmit(ctx, successor, Task{ID: fx.Frontend.ID, Mission: mission, Owner: "emp-frontend", Kind: "peer_frontend", State: "working", Generation: 2}, "successor-artifact", []byte(fixture.PeerFrontendV2))
	must(t, err)
	must(t, k.TXPeerResolve(ctx, successor, message.ObligationID, artifact.ID, "successor-resolve"))
	bundle, err = k.PeerHandover(ctx, successor)
	must(t, err)
	if bundle.ObligationState != "fulfilled" || bundle.MessageState != "resolved" {
		t.Fatalf("successor did not close peer obligation: %+v", bundle)
	}
}
