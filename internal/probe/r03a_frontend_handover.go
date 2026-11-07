// pattern: Imperative Shell
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/kernel"
)

const r03aFrontendHandoverPurpose = "real_frontend_handover_from_recovered_state"

type R03AFrontendConfig struct {
	Config
	ProblemKey                   string
	RunPurpose                   string
	AuthorizationBindingPath     string
	CurrentL1EvidencePath        string
	CurrentL1Fingerprint         string
	ExecutionFingerprint         string
	HandoverBoundaryEvidencePath string
	HandoverBoundarySnapshotPath string
	InitialOnly                  bool
}

func RunR03AFrontendHandover(cfg R03AFrontendConfig) (map[string]any, error) {
	if cfg.Model != "gpt-5.6-luna" || (cfg.MediumLimit != 2 && !(cfg.InitialOnly && cfg.MediumLimit == 1)) || cfg.HighLimit != 0 || cfg.ToolCallLimit < 1 || cfg.ProblemKey != R03AProblemKey || cfg.RunPurpose != r03aFrontendHandoverPurpose {
		return nil, errors.New("Frontend handover requires the authorized Medium allowance, zero High and the fixed ProblemKey/purpose")
	}
	missing := []string{}
	if cfg.Evidence == "" {
		missing = append(missing, "evidence")
	}
	if cfg.Root == "" {
		missing = append(missing, "runtime_root")
	}
	if cfg.DSN == "" {
		missing = append(missing, "dsn")
	}
	if cfg.AuthorizationBindingPath == "" {
		missing = append(missing, "authorization_binding")
	}
	if cfg.CurrentL1EvidencePath == "" {
		missing = append(missing, "current_l1_evidence")
	}
	if cfg.ExecutionFingerprint == "" {
		missing = append(missing, "execution_fingerprint")
	}
	if cfg.CheckerFeedbackQualificationPath == "" {
		missing = append(missing, "checker_feedback_qualification")
	}
	if cfg.HandoverBoundaryEvidencePath == "" {
		missing = append(missing, "handover_boundary_evidence")
	}
	if cfg.HandoverBoundarySnapshotPath == "" {
		missing = append(missing, "handover_boundary_snapshot")
	}
	if cfg.PostgresDumpPath == "" || cfg.PostgresSnapshotDSN == "" {
		missing = append(missing, "postgres_snapshot_command")
	}
	if len(missing) != 0 {
		return nil, fmt.Errorf("Frontend handover configuration is incomplete: %s", strings.Join(missing, ","))
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("Frontend handover evidence path is not fresh; refusing retry/reset")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}

	l2Config := R03AT2Config{Config: cfg.Config}
	l2Config.CurrentL1EvidencePath = cfg.CurrentL1EvidencePath
	l2Config.ExecutionFingerprint = cfg.ExecutionFingerprint
	l2Config.CheckerFeedbackQualificationPath = cfg.CheckerFeedbackQualificationPath
	qualification, err := requireCurrentBinaryRevisedFrontendL2Qualification(l2Config)
	if err != nil {
		return nil, err
	}
	blobQualification, err := RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if err != nil {
		return nil, err
	}
	binding, err := LoadBusinessAuthorization(cfg.AuthorizationBindingPath, codex.BusinessExecutionContext{
		ExecutionFingerprint: cfg.ExecutionFingerprint,
		EmployeeID:           "emp-frontend",
		ProblemKey:           cfg.ProblemKey,
		Purpose:              cfg.RunPurpose,
		Model:                cfg.Model,
		Profile:              cfg.Model + "/medium",
		Effort:               "medium",
		Limits:               codex.AllowanceLimits{Medium: cfg.MediumLimit, High: cfg.HighLimit, Concurrency: 1, ToolCallLimit: cfg.ToolCallLimit},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	k, err := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if err != nil {
		return nil, err
	}
	defer k.Close()
	anchors, err := k.PeerRecoveryAnchors(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("read restored peer anchors: %w", err)
	}
	scope := k.LocalScope(anchors.CompanyID)
	initialWorkspace, err := k.PeerWorkspaceAt(ctx, scope, anchors.FrontendTaskID)
	if err != nil {
		return nil, fmt.Errorf("read restored Frontend workspace: %w", err)
	}
	if anchors.MessageState != "persisted" || anchors.ObligationState != "pending" || anchors.ObligationOwner != "emp-frontend" || anchors.MessageContractRevisionID != anchors.FinalContractRevisionID || anchors.BackendSessionState != "stopped" {
		return nil, errors.New("restored peer anchors are not the expected pending frontend responsibility")
	}
	if err := k.VerifyHistoricalWorkerFence(ctx, anchors.CompanyID, anchors.BackendSessionID, "emp-backend", anchors.BackendSessionIncarnation, anchors.BackendSessionEpoch); err != nil {
		return nil, fmt.Errorf("old Backend writer was not fenced: %w", err)
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"qualification": qualification.Qualification, "status": "frontend_recovery_preflight_passed", "passed": true,
		"problem_key": cfg.ProblemKey, "purpose": cfg.RunPurpose, "execution_fingerprint": qualification.ExecutionFingerprint,
		"tool_manifest_digest": qualification.ToolManifestDigest, "tool_count": qualification.ToolCount,
		"model": cfg.Model, "effort": "medium", "tool_call_limit": cfg.ToolCallLimit,
		"checker_feedback_qualification":  cfg.CheckerFeedbackQualificationPath,
		"handover_boundary_snapshot_path": cfg.HandoverBoundarySnapshotPath,
		"blob_durability":                 blobQualification, "recovery_anchors": anchors, "old_backend_writer_fenced": true,
		"medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "authorization-binding.json"), binding); err != nil {
		return nil, err
	}

	budget, err := codex.NewBusinessBudget(filepath.Join(cfg.Evidence, "allowance.json"), cfg.MediumLimit, cfg.HighLimit, cfg.ToolCallLimit)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"qualification": "R0.3A-REAL-FRONTEND-HANDOVER", "status": "inconclusive", "problem_key": cfg.ProblemKey, "purpose": cfg.RunPurpose,
		"execution_fingerprint": qualification.ExecutionFingerprint, "l1_fingerprint": cfg.CurrentL1Fingerprint,
		"revised_l2_fingerprint": qualification.ExecutionFingerprint, "medium_authorized": cfg.MediumLimit, "high_authorized": cfg.HighLimit,
		"tool_call_limit": cfg.ToolCallLimit, "medium_started": 0, "high_started": 0, "provider_egress": 0,
		"source_runtime_incarnation": k.SourceIncarnation(), "runtime_incarnation": k.Incarnation(),
		"frontend_initial_incarnation": "", "frontend_successor_incarnation": "", "frontend_initial_epoch": int64(0), "frontend_successor_epoch": int64(0),
		"transport_state":            "NOT_STARTED",
		"handover_boundary_snapshot": "NOT_STARTED", "eligible_for_successor_only_revised_run": false,
		"checker_feedback_qualification": cfg.CheckerFeedbackQualificationPath,
		"recovery_anchors":               anchors, "frontend_initial": "not_started", "frontend_successor": "not_started",
		"successor_started": false, "successor_business_completion": "NOT_RUN", "artifact_state": "NOT_CREATED", "business_result": "NOT_RUN",
		"real_frontend_handover": "inconclusive", "real_peer_collaboration": "inconclusive", "session_records": []R03ATurn{},
	}
	writeResult := func() { _ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result) }
	sessionTurns := make([]R03ATurn, 0, 2)
	syncSessionEvidence := func() {
		rollup := rollupFrontendSessions(sessionTurns)
		result["session_records"] = rollup.Sessions
		result["medium_started"] = rollup.MediumStarted
		result["provider_egress"] = rollup.ProviderEgress
		result["successor_started"] = rollup.SuccessorStarted
		result["tool_calls_used"] = rollup.ToolCallsUsed
		result["tool_calls_remaining"] = rollup.ToolCallsRemaining
		result["artifact_state"] = rollup.ArtifactState
		result["business_result"] = rollup.BusinessResult
		result["transport_state"] = rollup.TransportState
		if len(rollup.Sessions) > 0 {
			result["initial_session"] = rollup.Sessions[0]
			result["frontend_initial_incarnation"] = rollup.Sessions[0].Incarnation
			result["frontend_initial_epoch"] = rollup.Sessions[0].Epoch
		}
		if len(rollup.Sessions) > 1 {
			result["successor_session"] = rollup.Sessions[1]
			result["frontend_successor_incarnation"] = rollup.Sessions[1].Incarnation
			result["frontend_successor_epoch"] = rollup.Sessions[1].Epoch
		}
	}

	frontendInitial, err := k.TXNewWorkerWithToolBudget(ctx, scope, anchors.FrontendTaskID, cfg.Model+"/medium", int64(cfg.ToolCallLimit))
	if err != nil {
		writeResult()
		return result, err
	}
	if err := budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	initial, err := runPeerSession(ctx, R03AConfig{Config: cfg.Config, ProblemKey: cfg.ProblemKey}, k, frontendInitial, "peer_frontend", frontendInitialContinuation7Prompt(), true, true, 1, "frontend initial employee: observe and apply recovered peer responsibility")
	sessionTurns = append(sessionTurns, initial.turn)
	syncSessionEvidence()
	if err != nil {
		result["frontend_initial"] = "failed"
		result["error"] = err.Error()
		writeResult()
		return result, err
	}
	initialBudget, err := k.WorkerToolCallBudget(ctx, initial.binding)
	if err != nil {
		writeResult()
		return result, err
	}
	result["initial_tool_budget"] = initialBudget
	_ = writeJSON(filepath.Join(cfg.Evidence, "initial-handover.json"), initial.handover)
	initialCheckpoint := lastCheckpointOfKind(initial.handover.Checkpoints, kernel.CheckpointProgress)
	lifecycle, err := k.PeerMessageLifecycle(ctx, scope, anchors.MessageID)
	if err != nil {
		writeResult()
		return result, err
	}
	initialMessage, _, initialObligationState, err := k.PeerMessageAt(ctx, scope, anchors.MessageID)
	if err != nil {
		writeResult()
		return result, err
	}
	initialPass := turnHas(initial.turn, "collab_inbox") && turnHas(initial.turn, "collab_ack") && turnHas(initial.turn, "collab_apply") && turnHas(initial.turn, "workspace_check") && initialCheckpoint != nil && initialCheckpoint.Kind == kernel.CheckpointProgress && initial.turn.StopConfirmed && initialMessage.ID == anchors.MessageID && initialObligationState == "applied" && !turnHas(initial.turn, "artifact_submit") && !turnHas(initial.turn, "obligation_resolve") && containsString(lifecycle, "collab.observed")
	if !initialPass {
		result["frontend_initial"] = "failed"
		result["initial_tool_events"] = initial.turn.ToolEvents
		result["initial_message_lifecycle"] = lifecycle
		result["error"] = "Frontend initial did not complete the observed/ack/applied progress handover boundary"
		writeResult()
		return result, errors.New("Frontend initial handover boundary failed")
	}
	result["frontend_initial"] = "passed"
	if cfg.InitialOnly {
		result["frontend_initial_handover_boundary"] = "PASSED"
		result["eligible_for_frontend_successor"] = true
		result["initial_session"] = initial.turn
		result["initial_handover"] = initial.handover
		result["message_id"] = initialMessage.ID
		result["obligation_id"] = anchors.ObligationID
		result["obligation_state"] = initialObligationState
		result["contract_revision_id"] = anchors.FinalContractRevisionID
		result["starting_workspace_revision"] = initialWorkspace.Revision
		result["final_workspace_revision"] = initial.handover.WorkspaceRevision
		result["progress_checkpoint"] = initialCheckpoint
		writeResult()
		return result, nil
	}
	snapshot, snapshotErr := k.PeerHandoverBoundarySnapshot(ctx, initial.binding, initial.handover)
	var snapshotDigest string
	var snapshotSize int64
	if snapshotErr == nil {
		snapshotDigest, snapshotSize, snapshotErr = dumpHandoverBoundaryDatabase(ctx, cfg.PostgresDumpPath, cfg.PostgresSnapshotDSN, cfg.HandoverBoundarySnapshotPath)
	}
	if snapshotErr != nil {
		result["handover_boundary_snapshot"] = "FAILED"
		result["handover_boundary_snapshot_error"] = snapshotErr.Error()
		result["eligible_for_successor_only_revised_run"] = false
	} else {
		snapshotDir := cfg.HandoverBoundaryEvidencePath
		if err := os.MkdirAll(snapshotDir, 0700); err != nil {
			result["handover_boundary_snapshot"] = "FAILED"
			result["handover_boundary_snapshot_error"] = err.Error()
			result["eligible_for_successor_only_revised_run"] = false
		} else if err := writeJSON(filepath.Join(snapshotDir, "handover-boundary-snapshot.json"), snapshot); err != nil {
			result["handover_boundary_snapshot"] = "FAILED"
			result["handover_boundary_snapshot_error"] = err.Error()
			result["eligible_for_successor_only_revised_run"] = false
		} else if err := writeJSON(filepath.Join(snapshotDir, "handover-boundary-cas-manifest.json"), map[string]any{
			"schema_version": "r0.3a-frontend-handover-boundary-cas-v1", "runtime_incarnation": snapshot.RuntimeIncarnation,
			"source_runtime_incarnation": snapshot.SourceRuntimeIncarnation, "entries": snapshot.CAS,
			"manifest_digest": snapshot.CASManifestDigest, "historical_evidence_modified": false,
		}); err != nil {
			result["handover_boundary_snapshot"] = "FAILED"
			result["handover_boundary_snapshot_error"] = err.Error()
			result["eligible_for_successor_only_revised_run"] = false
		} else {
			result["handover_boundary_snapshot"] = "PASSED"
			result["handover_boundary_snapshot_path"] = filepath.Join(snapshotDir, "handover-boundary-snapshot.json")
			result["handover_boundary_cas_manifest_path"] = filepath.Join(snapshotDir, "handover-boundary-cas-manifest.json")
			result["handover_boundary_cas_manifest_digest"] = snapshot.CASManifestDigest
			result["handover_boundary_database_snapshot_path"] = cfg.HandoverBoundarySnapshotPath
			result["handover_boundary_database_snapshot_sha256"] = snapshotDigest
			result["handover_boundary_database_snapshot_size"] = snapshotSize
			result["eligible_for_successor_only_revised_run"] = true
		}
	}

	frontendSuccessor, err := k.TXNewWorkerWithToolBudget(ctx, scope, anchors.FrontendTaskID, cfg.Model+"/medium", int64(cfg.ToolCallLimit))
	if err != nil {
		writeResult()
		return result, err
	}
	if err := budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	successor, err := runPeerSession(ctx, R03AConfig{Config: cfg.Config, ProblemKey: cfg.ProblemKey}, k, frontendSuccessor, "peer_frontend", frontendSuccessorContinuation7Prompt(initial.handover), false, false, 2, "frontend successor: restore and fulfill the inherited peer responsibility")
	sessionTurns = append(sessionTurns, successor.turn)
	syncSessionEvidence()
	if err != nil {
		if successor.turn.TurnStarted || successor.turn.ProviderEgress {
			result["frontend_successor"] = "started"
		} else {
			result["frontend_successor"] = "failed"
		}
		result["successor_business_completion"] = "FAILED"
		result["error"] = err.Error()
		writeResult()
		return result, err
	}
	result["frontend_successor"] = "started"
	result["successor_business_completion"] = "IN_PROGRESS"
	successorBudget, err := k.WorkerToolCallBudget(ctx, successor.binding)
	if err != nil {
		writeResult()
		return result, err
	}
	result["successor_tool_budget"] = successorBudget
	_ = writeJSON(filepath.Join(cfg.Evidence, "successor-handover.json"), successor.handover)

	_, oldApply := k.TXPeerApply(ctx, initial.binding, kernel.PeerApplyRequest{ObligationID: anchors.ObligationID, ContractRevisionID: anchors.FinalContractRevisionID, WorkspaceRevision: 2, EvidenceRefs: []string{"frontend-old-epoch-apply"}}, "frontend-old-epoch-apply")
	_, oldCheckpointErr := k.TXCheckpoint(ctx, initial.binding, "frontend-old-epoch-checkpoint", kernel.Checkpoint{Kind: kernel.CheckpointProgress, Summary: "late old writer", Facts: []string{"stale"}, Decisions: []string{"deny"}, Rejected: []string{"current"}, EvidenceRefs: []string{"stale"}, NextAction: "deny"})
	oldResolve := k.TXPeerResolve(ctx, initial.binding, anchors.ObligationID, successor.turn.ArtifactID, "frontend-old-epoch-resolve")
	oldWriterRejected := errors.Is(oldApply, core.StaleEpoch) && errors.Is(oldCheckpointErr, core.StaleEpoch) && errors.Is(oldResolve, core.StaleEpoch)
	_ = writeJSON(filepath.Join(cfg.Evidence, "old-writer-rejection.json"), map[string]any{"workspace_apply": fmt.Sprint(oldApply), "checkpoint": fmt.Sprint(oldCheckpointErr), "obligation_resolve": fmt.Sprint(oldResolve), "all_stale_epoch": oldWriterRejected})
	if successor.turn.ArtifactID == "" {
		result["successor_business_completion"] = "FAILED"
		result["artifact_state"] = "NOT_CREATED"
		result["business_result"] = "FAILED"
		result["real_frontend_handover"] = "failed"
		result["real_peer_collaboration"] = "failed"
		result["eligible_for_high_review"] = false
		result["finalizer"] = "artifact_id_empty_skip_db_lookup"
		result["error"] = "successor business completion failed: artifact was not created"
		syncSessionEvidence()
		writeResult()
		return result, errors.New("successor business completion failed: artifact was not created")
	}

	finalAnchors, err := k.PeerRecoveryAnchors(ctx, anchors.CompanyID)
	if err != nil {
		writeResult()
		return result, err
	}
	frontendArtifact, err := k.PeerArtifactAt(ctx, scope, successor.turn.ArtifactID)
	if err != nil {
		writeResult()
		return result, err
	}
	frontendWorkspace, err := k.PeerWorkspaceAt(ctx, scope, anchors.FrontendTaskID)
	if err != nil {
		writeResult()
		return result, err
	}
	finalContract, err := k.PeerContractAt(ctx, scope, anchors.FinalContractRevisionID)
	if err != nil {
		writeResult()
		return result, err
	}
	finalLifecycle, err := k.PeerMessageLifecycle(ctx, scope, anchors.MessageID)
	if err != nil {
		writeResult()
		return result, err
	}
	positive, err := k.TXFreezePeerIntegration(ctx, scope, kernel.PeerIntegrationInput{Mission: anchors.MissionID, BackendArtifactID: anchors.BackendArtifactID, FrontendArtifactID: successor.turn.ArtifactID, ContractRevisionID: anchors.FinalContractRevisionID, BaseRevision: "api-v1", VerifierRevision: "r03a-verifier@1"}, "frontend-positive-integration")
	if err != nil {
		writeResult()
		return result, err
	}
	positiveReport, err := k.VerifyPeerIntegration(ctx, scope, positive.ID)
	if err != nil {
		writeResult()
		return result, err
	}
	_ = writeJSON(filepath.Join(cfg.Evidence, "integration-verifier.json"), positiveReport)
	negativeReport, err := runPeerNegativeControl(ctx, k)
	if err != nil {
		writeResult()
		return result, err
	}
	_ = writeJSON(filepath.Join(cfg.Evidence, "negative-control.json"), negativeReport)

	success := oldWriterRejected && finalAnchors.MessageID == anchors.MessageID && finalAnchors.ObligationID == anchors.ObligationID && finalAnchors.ObligationState == "fulfilled" && finalAnchors.MessageState == "resolved" && finalAnchors.MessageContractRevisionID == anchors.FinalContractRevisionID && finalContract.State == "accepted" && finalContract.Revision == anchors.FinalContractRevision && frontendArtifact.State == "ready" && frontendArtifact.Verdict == "candidate" && frontendWorkspace.Revision > initial.handover.WorkspaceRevision && turnHas(successor.turn, "workspace_check") && turnHas(successor.turn, "work_checkpoint") && turnHas(successor.turn, "artifact_submit") && turnHas(successor.turn, "obligation_resolve") && !turnHas(successor.turn, "contract_propose") && positiveReport.Passed && !negativeReport.Passed
	result["frontend_successor"] = statusIf(success, "passed")
	result["successor_business_completion"] = statusIf(success, "PASSED")
	result["real_frontend_handover"] = statusIf(success, "passed")
	result["real_peer_collaboration"] = statusIf(success, "passed")
	result["status"] = statusIf(success, "passed")
	result["high_started"] = budget.High
	syncSessionEvidence()
	result["initial_session"] = initial.turn
	result["successor_session"] = successor.turn
	result["message_id"] = anchors.MessageID
	result["obligation_id"] = anchors.ObligationID
	result["final_contract_revision_id"] = finalContract.ID
	result["final_contract_revision"] = finalContract.Revision
	result["frontend_artifact_id"] = successor.turn.ArtifactID
	result["frontend_artifact_digest"] = frontendArtifact.Digest
	result["frontend_workspace_revision"] = frontendWorkspace.Revision
	result["frontend_workspace_digest"] = frontendWorkspace.Digest
	result["message_lifecycle"] = finalLifecycle
	result["old_writer_rejection"] = oldWriterRejected
	result["integration_positive"] = positiveReport.Passed
	result["negative_control_passed_as_failure"] = !negativeReport.Passed
	result["eligible_for_high_review"] = success
	if !success {
		result["error"] = "Frontend handover or collaboration acceptance conditions failed"
	}
	writeResult()
	if !success {
		return result, errors.New("Frontend handover acceptance conditions failed")
	}
	return result, nil
}

func dumpHandoverBoundaryDatabase(ctx context.Context, dumpPath, dsn, outputPath string) (string, int64, error) {
	if dumpPath == "" || dsn == "" || outputPath == "" {
		return "", 0, errors.New("handover boundary database snapshot configuration is incomplete")
	}
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", 0, err
	}
	wslOutput, err := windowsPathToWSL(outputPath)
	if err != nil {
		return "", 0, err
	}
	command := shellQuote(dumpPath) + " --format=custom --no-owner --no-acl --exit-on-error --file=" + shellQuote(wslOutput) + " --dbname=" + shellQuote(dsn)
	process := exec.CommandContext(ctx, "bash", "-lc", command)
	var stderr bytes.Buffer
	process.Stderr = &stderr
	if err := process.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if len([]rune(message)) > 512 {
			message = string([]rune(message)[:512])
		}
		return "", 0, fmt.Errorf("pg_dump failed: %s: %w", message, err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return "", 0, err
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return "", 0, err
	}
	return digest(raw), info.Size(), nil
}

func windowsPathToWSL(path string) (string, error) {
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') {
		return "", fmt.Errorf("expected absolute Windows path: %s", path)
	}
	return "/mnt/" + strings.ToLower(string(path[0])) + strings.ReplaceAll(path[2:], "\\", "/"), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func frontendInitialContinuation7Prompt() string {
	return `You are the initial Frontend Employee in a recovered disposable Polis peer-collaboration task. Use only the registered Polis peer tools. Do not use shell, external tools, Planner, delegation or natural-language workarounds. Discover the current work and pending peer responsibility yourself: call work_current, read the pending peer message with collab_inbox, acknowledge it with collab_ack, and read the exact contract referenced by the message with contract_read. Use workspace_replace to make the real frontend workspace change with its current digest. Then call collab_apply with obligation_id, contract_revision_id, workspace_revision and receipt IDs in evidence_refs[] to declare that persisted workspace change applied the current responsibility; collab_apply does not accept source content. Run workspace_check and use its public semantic feedback. Persist a progress checkpoint with kind progress using receipt IDs in evidence_refs[]. Do not submit an artifact and do not resolve the obligation: this session ends at the real handover boundary while the responsibility remains unfulfilled.`
}

func frontendSuccessorContinuation7Prompt(bundle kernel.PeerHandoverBundle) string {
	raw, _ := json.Marshal(bundle)
	return `You are the successor Frontend Employee with the same EmployeeId after a neutral Polis handover. Use only the registered Polis peer tools. Do not use shell, external tools, Planner, delegation or guessed Backend answers. Restore the inherited responsibility from the persisted handover and your own current work state. Discover the current accepted contract with contract_read or the current peer inbox, keep the original obligation identity, ensure your workspace is compatible with that contract, run workspace_check, persist a qualified checkpoint, submit the frontend candidate, and resolve the original obligation with the persisted artifact id. Do not propose a new contract unless the persisted state itself requires supersession.

Neutral persisted handover facts:
` + string(raw)
}

func lastCheckpointOfKind(checkpoints []kernel.Checkpoint, kind string) *kernel.Checkpoint {
	for index := len(checkpoints) - 1; index >= 0; index-- {
		if checkpoints[index].Kind == kind {
			checkpoint := checkpoints[index]
			return &checkpoint
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
