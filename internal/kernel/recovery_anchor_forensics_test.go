// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"polis/internal/fixture"
	"polis/internal/runner"
	"testing"
)

func TestBackendTerminalRecoveryAnchorForensicsReproducesErrNoRows(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	scope, err := k.TXCreateCompany(ctx, "r03a-recovery-anchor-forensics-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	fx, err := k.TXCreatePeerFixture(ctx, scope, "r03a-recovery-anchor-forensics-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	backend, err := k.TXNewWorkerWithToolBudget(ctx, scope, fx.Backend.ID, "gpt-5.6-luna/medium", 48)
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(backend.SessionID(), []string{"/bin/sleep", "60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, backend, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, backend); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, backend, "recovery-anchor-forensics"); err != nil {
		t.Fatal(err)
	}

	handover, err := k.Handover(ctx, backend)
	if err != nil {
		t.Fatal(err)
	}
	staleHandover := handover
	revision, err := k.TXProposePeerContract(ctx, backend, handover.Task, PeerContractProposal{
		Endpoint: "GET /items",
		Schema:   fixture.PeerPaginationContractV3,
	}, "recovery-anchor-propose")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXAcceptPeerContract(ctx, backend, revision.ID, "recovery-anchor-accept"); err != nil {
		t.Fatal(err)
	}
	workspace, err := k.Workspace(ctx, backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXReplace(ctx, backend, "recovery-anchor-replace", workspace.Digest, fixture.PeerBackendPaginationReference); err != nil {
		t.Fatal(err)
	}
	check, err := k.TXPeerCheck(ctx, backend, "recovery-anchor-check")
	if err != nil || !check.Passed || check.Receipt.ID == "" {
		t.Fatalf("backend check did not pass: receipt=%+v err=%v", check, err)
	}
	if _, err = k.TXCheckpoint(ctx, backend, "recovery-anchor-qualified", Checkpoint{
		Kind:         CheckpointQualified,
		Summary:      "backend terminal candidate qualified",
		Facts:        []string{"frontend has not started", "peer obligation remains pending"},
		Decisions:    []string{"snapshot before frontend"},
		Rejected:     []string{"synthetic frontend state"},
		EvidenceRefs: []string{check.Receipt.ID},
		NextAction:   "authoritative snapshot",
	}); err != nil {
		t.Fatal(err)
	}
	message, err := k.TXPeerSend(ctx, backend, PeerSendInput{
		FromTask:           handover.Task.ID,
		ToEmployeeID:       "emp-frontend",
		ToTaskID:           fx.Frontend.ID,
		ContractRevisionID: revision.ID,
		Body:               "apply the accepted pagination contract",
		Actionable:         true,
	}, "recovery-anchor-send")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXSubmit(ctx, backend, handover.Task, "recovery-anchor-artifact", []byte(fixture.PeerBackendPaginationReference)); err != nil {
		t.Fatal(err)
	}
	currentHandover, err := k.Handover(ctx, backend)
	if err != nil {
		t.Fatal(err)
	}

	// The worker handover is intentionally an incoming/task-local view. It
	// does not contain the Backend's outbound peer message.
	peerHandover := PeerHandoverBundle{
		EmployeeID:        "emp-backend",
		TaskID:            staleHandover.Task.ID,
		WorkspaceDigest:   staleHandover.Workspace.Digest,
		WorkspaceRevision: staleHandover.Workspace.Revision,
		Checkpoints:       staleHandover.Checkpoints,
		ToolBudget:        staleHandover.ToolBudget,
	}
	if err = k.TXBeginStop(ctx, backend); err != nil {
		t.Fatal(err)
	}
	proof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, backend, proof); err != nil {
		t.Fatal(err)
	}

	snapshot, err := k.PeerHandoverBoundarySnapshot(ctx, backend, peerHandover)
	if err != nil {
		t.Fatalf("Backend terminal snapshot must not require Frontend handover state: %v", err)
	}
	if snapshot.RecoveryAnchors.MessageID != message.ID || snapshot.RecoveryAnchors.ObligationState != "pending" || snapshot.Handover.MessageID != message.ID || snapshot.Handover.ObligationState != "pending" || snapshot.Contract.ID != revision.ID || snapshot.Workspace.Revision != currentHandover.Workspace.Revision {
		t.Fatalf("snapshot did not preserve the real pending peer responsibility: %+v", snapshot)
	}
	readOnlyAnchors, err := ReadOnlyPeerRecoveryAnchors(ctx, dsn, scope.company)
	if err != nil || readOnlyAnchors.MessageID != message.ID || readOnlyAnchors.BackendArtifactID != snapshot.RecoveryAnchors.BackendArtifactID {
		t.Fatalf("read-only recovery anchor probe failed or diverged: anchors=%+v err=%v", readOnlyAnchors, err)
	}
	_, err = k.PeerRecoveryAnchors(ctx, "missing-recovery-company")
	var missing *RecoveryAnchorError
	if !errors.As(err, &missing) || missing.AnchorType != "active_mission" || missing.ReasonCode != "recovery_anchor_missing" {
		t.Fatalf("missing required recovery anchor did not produce a structured recovery error: %v", err)
	}
}
