// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/fixture"
	"strings"
)

type PeerContractRevision struct {
	ID           string `json:"id"`
	Mission      string `json:"mission"`
	Endpoint     string `json:"endpoint"`
	Schema       string `json:"schema"`
	Digest       string `json:"digest"`
	State        string `json:"state"`
	Revision     int64  `json:"revision"`
	BaseRevision int64  `json:"base_revision"`
}

type PeerFixture struct {
	Mission    string
	Backend    Task
	Frontend   Task
	ContractV1 PeerContractRevision
}

type PeerContractProposal struct {
	Endpoint string
	Schema   string
}

type PeerSendInput struct {
	FromTask           string
	ToEmployeeID       string
	ToTaskID           string
	ToTask             string
	ContractRevisionID string
	Body               string
	Actionable         bool
}

type PeerMessage struct {
	ID                 string `json:"id"`
	TaskID             string `json:"task_id"`
	ObligationID       string `json:"obligation_id"`
	ContractRevisionID string `json:"contract_revision_id"`
	DeliveryState      string `json:"delivery_state"`
}

type PeerIntegrationInput struct {
	Mission, BackendArtifactID, FrontendArtifactID     string
	ContractRevisionID, BaseRevision, VerifierRevision string
}

func (k *Kernel) PeerContractAt(ctx context.Context, s Scope, revisionID string) (PeerContractRevision, error) {
	var out PeerContractRevision
	e := k.pool.QueryRow(ctx, "SELECT id,mission_id,endpoint,schema::text,digest,state,revision,COALESCE(base_revision,0) FROM contract_revisions WHERE company_id=$1 AND id=$2", s.company, revisionID).Scan(&out.ID, &out.Mission, &out.Endpoint, &out.Schema, &out.Digest, &out.State, &out.Revision, &out.BaseRevision)
	return out, e
}

func (k *Kernel) PeerMessageAt(ctx context.Context, s Scope, messageID string) (PeerMessage, string, string, error) {
	var out PeerMessage
	var body, obligationState string
	e := k.pool.QueryRow(ctx, "SELECT m.id,m.task_id,m.contract_revision_id,m.delivery_state,m.body,COALESCE(o.state,''),COALESCE(o.id,'') FROM messages m LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id WHERE m.company_id=$1 AND m.id=$2", s.company, messageID).Scan(&out.ID, &out.TaskID, &out.ContractRevisionID, &out.DeliveryState, &body, &obligationState, &out.ObligationID)
	return out, body, obligationState, e
}

func (k *Kernel) PeerWorkspaceAt(ctx context.Context, s Scope, taskID string) (Workspace, error) {
	var out Workspace
	e := k.pool.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", s.company, taskID).Scan(&out.Digest, &out.Revision)
	if e != nil {
		return out, e
	}
	content, e := readBlob(k.root, s.company, out.Digest)
	if e != nil {
		return out, e
	}
	out.Content = string(content)
	return out, nil
}

func (k *Kernel) PeerPlannerPathCount(ctx context.Context, s Scope) (int64, error) {
	var count int64
	e := k.pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE company_id=$1 AND (sender='emp-planning' OR recipient='emp-planning')", s.company).Scan(&count)
	return count, e
}

// PeerFrontendSessionCount is a read-only starting-state guard for a new
// Frontend business session.
func (k *Kernel) PeerFrontendSessionCount(ctx context.Context, s Scope, taskID string) (int64, error) {
	var count int64
	err := k.pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND employee_id='emp-frontend'", s.company, taskID).Scan(&count)
	return count, err
}

func (k *Kernel) PeerArtifactAt(ctx context.Context, s Scope, artifactID string) (Artifact, error) {
	var out Artifact
	e := k.pool.QueryRow(ctx, "SELECT id,digest,state,verdict FROM artifacts WHERE company_id=$1 AND id=$2", s.company, artifactID).Scan(&out.ID, &out.Digest, &out.State, &out.Verdict)
	return out, e
}

type ArtifactQualification struct {
	ArtifactID         string `json:"artifact_id"`
	CheckpointID       string `json:"checkpoint_id"`
	WorkspaceRevision  int64  `json:"workspace_revision"`
	WorkspaceDigest    string `json:"workspace_digest"`
	ContractRevisionID string `json:"contract_revision_id"`
}

func (k *Kernel) PeerArtifactQualificationAt(ctx context.Context, s Scope, artifactID string) (ArtifactQualification, error) {
	var out ArtifactQualification
	err := k.pool.QueryRow(ctx, `SELECT artifact_id,checkpoint_id,workspace_revision,workspace_digest,contract_revision_id FROM artifact_qualifications WHERE company_id=$1 AND artifact_id=$2`, s.company, artifactID).Scan(&out.ArtifactID, &out.CheckpointID, &out.WorkspaceRevision, &out.WorkspaceDigest, &out.ContractRevisionID)
	return out, err
}

func (k *Kernel) PeerCheckpointAt(ctx context.Context, s Scope, checkpointID string) (Checkpoint, error) {
	var raw []byte
	if err := k.pool.QueryRow(ctx, "SELECT data FROM worker_checkpoints WHERE company_id=$1 AND id=$2", s.company, checkpointID).Scan(&raw); err != nil {
		return Checkpoint{}, err
	}
	var out Checkpoint
	if err := json.Unmarshal(raw, &out); err != nil {
		return Checkpoint{}, err
	}
	return out, nil
}

func (k *Kernel) PeerCASAt(ctx context.Context, companyID string) ([]PeerHandoverCASBlob, error) {
	return peerHandoverCAS(k.root, companyID)
}

type PeerVerificationReport struct {
	Passed                     bool                               `json:"passed"`
	Reason                     string                             `json:"reason"`
	LifecycleBinding           string                             `json:"lifecycle_binding"`
	CollaborationCausality     string                             `json:"collaboration_causality"`
	PaginationRuntimeBehavior  string                             `json:"pagination_runtime_behavior"`
	ResponseShapeCompatibility string                             `json:"response_shape_compatibility"`
	VerifierRevision           string                             `json:"verifier_revision"`
	PaginationReport           *fixture.PaginationBehaviorReport  `json:"pagination_report,omitempty"`
	FrontendConsumptionReport  *fixture.FrontendConsumptionReport `json:"frontend_consumption_report,omitempty"`
}

type PeerHandoverBundle struct {
	EmployeeID, TaskID, WorkspaceDigest, ContractRevisionID string
	MessageID, ObligationID, MessageState, ObligationState  string
	WorkspaceRevision                                       int64
	Checkpoints                                             []Checkpoint
	ToolBudget                                              ToolCallBudget
	MemoryContext                                           []MemoryTaskDependencyContext `json:"memory_context,omitempty"`
	MemoryStatus                                            *MemoryTaskStatus             `json:"memory_status,omitempty"`
}

type PeerHandoverCASBlob struct {
	CompanyID     string `json:"company_id"`
	ContentSHA256 string `json:"content_sha256"`
	Size          int64  `json:"size"`
}

// PeerHandoverBoundarySnapshot is a read-only, DB-derived recovery cut taken
// after the initial worker is stopped and before a successor is created.
// It is evidence for a future successor-only run, not a replacement for the
// authoritative database transaction itself.
type PeerHandoverBoundarySnapshot struct {
	SchemaVersion            string                `json:"schema_version"`
	RuntimeIncarnation       string                `json:"runtime_incarnation"`
	SourceRuntimeIncarnation string                `json:"source_runtime_incarnation"`
	EmployeeID               string                `json:"employee_id"`
	Epoch                    int64                 `json:"epoch"`
	SessionID                string                `json:"session_id"`
	RecoveryAnchors          PeerRecoveryAnchors   `json:"recovery_anchors"`
	Handover                 PeerHandoverBundle    `json:"handover"`
	MessageBody              string                `json:"message_body"`
	Contract                 PeerContractRevision  `json:"contract"`
	Workspace                Workspace             `json:"workspace"`
	State                    Snapshot              `json:"state"`
	CAS                      []PeerHandoverCASBlob `json:"cas"`
	CASManifestDigest        string                `json:"cas_manifest_digest"`
}

const (
	PeerFrontendHandoverBoundarySnapshotSchemaVersion = "r0.3a-frontend-handover-boundary-v1"
	PeerBackendTerminalSnapshotSchemaVersion          = "r0.3a-backend-terminal-authoritative-snapshot-v1"
)

// RecoveryAnchorError identifies a missing persisted fact needed by an
// authoritative recovery cut. It deliberately distinguishes recovery
// eligibility from Frontend handover progress.
type RecoveryAnchorError struct {
	ReasonCode    string
	AnchorType    string
	ExpectedKey   string
	TerminalState string
	Err           error
}

func (e *RecoveryAnchorError) Error() string {
	return fmt.Sprintf("%s: anchor_type=%s expected_key=%s terminal_state=%s: %v", e.ReasonCode, e.AnchorType, e.ExpectedKey, e.TerminalState, e.Err)
}

func (e *RecoveryAnchorError) Unwrap() error { return e.Err }

func recoveryAnchorFailure(anchorType, expectedKey, terminalState string, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return &RecoveryAnchorError{
		ReasonCode:    "recovery_anchor_missing",
		AnchorType:    anchorType,
		ExpectedKey:   expectedKey,
		TerminalState: terminalState,
		Err:           err,
	}
}

func (k *Kernel) PeerHandover(ctx context.Context, b Binding) (PeerHandoverBundle, error) {
	var out PeerHandoverBundle
	h, e := k.Handover(ctx, b)
	if e != nil {
		return out, e
	}
	out.EmployeeID, out.TaskID = h.EmployeeID, h.Task.ID
	out.WorkspaceDigest, out.WorkspaceRevision = h.Workspace.Digest, h.Workspace.Revision
	out.Checkpoints = h.Checkpoints
	out.ToolBudget = h.ToolBudget
	out.MemoryContext = h.MemoryContext
	if h.MemoryStatus.State != "clear" {
		out.MemoryStatus = &h.MemoryStatus
	}
	if e := k.pool.QueryRow(ctx, `SELECT m.id,m.delivery_state,m.contract_revision_id,COALESCE(o.id,''),COALESCE(o.state,'')
FROM messages m
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
LEFT JOIN events e ON e.company_id=m.company_id AND e.kind='collab.send' AND e.payload->>'id'=m.id
WHERE m.company_id=$1 AND m.task_id=$2 AND m.contract_revision_id IS NOT NULL
  AND ((o.id IS NOT NULL AND o.state IN ('pending','observed','acknowledged','applied'))
    OR (o.id IS NULL AND m.kind='fyi' AND m.delivery_state IN ('persisted','delivered','observed')))
ORDER BY CASE WHEN o.id IS NOT NULL THEN 0 ELSE 1 END,COALESCE(e.company_seq,0),m.id
LIMIT 1`, b.scope.company, out.TaskID).Scan(&out.MessageID, &out.MessageState, &out.ContractRevisionID, &out.ObligationID, &out.ObligationState); errors.Is(e, pgx.ErrNoRows) {
		return out, nil
	} else if e != nil {
		return out, e
	}
	return out, nil
}

func (k *Kernel) PeerHandoverBoundarySnapshot(ctx context.Context, b Binding, handover PeerHandoverBundle) (PeerHandoverBoundarySnapshot, error) {
	var out PeerHandoverBoundarySnapshot
	anchors, err := k.PeerRecoveryAnchors(ctx, b.scope.company)
	if err != nil {
		return out, err
	}
	backendBoundary := b.employee == "emp-backend"
	messageID, contractID, taskID := handover.MessageID, handover.ContractRevisionID, handover.TaskID
	if backendBoundary {
		// The employee handover bundle is task-local and may not contain an
		// outbound Backend->Frontend message. Recovery must derive that
		// message from persisted anchors; it must not require Frontend
		// observation.
		messageID, contractID, taskID = anchors.MessageID, anchors.FinalContractRevisionID, anchors.BackendTaskID
	}
	message, body, obligationState, err := k.PeerMessageAt(ctx, b.scope, messageID)
	if err != nil {
		return out, recoveryAnchorFailure("pending_peer_obligation", messageID, "Message persisted and Obligation pending", err)
	}
	contract, err := k.PeerContractAt(ctx, b.scope, contractID)
	if err != nil {
		return out, recoveryAnchorFailure("effective_contract", contractID, "accepted effective ContractRevision", err)
	}
	workspace, err := k.PeerWorkspaceAt(ctx, b.scope, taskID)
	if err != nil {
		return out, recoveryAnchorFailure("backend_workspace", taskID, "worker workspace at terminal revision", err)
	}
	state, err := k.Snapshot(ctx, b.scope, anchors.MissionID)
	if err != nil {
		return out, err
	}
	cas, err := peerHandoverCAS(k.root, b.scope.company)
	if err != nil {
		return out, err
	}
	derivedHandover := handover
	if backendBoundary {
		derivedHandover.EmployeeID = b.employee
		derivedHandover.TaskID = anchors.BackendTaskID
		derivedHandover.WorkspaceDigest = workspace.Digest
		derivedHandover.WorkspaceRevision = workspace.Revision
		derivedHandover.ContractRevisionID = anchors.FinalContractRevisionID
		derivedHandover.MessageID = message.ID
		derivedHandover.ObligationID = anchors.ObligationID
		derivedHandover.MessageState = message.DeliveryState
		derivedHandover.ObligationState = obligationState
	}
	if !backendBoundary && handover.WorkspaceRevision != 0 && handover.WorkspaceRevision != workspace.Revision {
		return out, &RecoveryAnchorError{
			ReasonCode:    "recovery_anchor_mismatch",
			AnchorType:    "backend_workspace_revision",
			ExpectedKey:   fmt.Sprintf("workspace_revision=%d", workspace.Revision),
			TerminalState: fmt.Sprintf("handover_workspace_revision=%d", handover.WorkspaceRevision),
			Err:           core.Conflict,
		}
	}
	schemaVersion := PeerFrontendHandoverBoundarySnapshotSchemaVersion
	if b.employee == "emp-backend" {
		schemaVersion = PeerBackendTerminalSnapshotSchemaVersion
	}
	out = PeerHandoverBoundarySnapshot{
		SchemaVersion:            schemaVersion,
		RuntimeIncarnation:       k.Incarnation(),
		SourceRuntimeIncarnation: k.SourceIncarnation(),
		EmployeeID:               b.employee,
		Epoch:                    b.epoch,
		SessionID:                b.session,
		RecoveryAnchors:          anchors,
		Handover:                 derivedHandover,
		MessageBody:              body,
		Contract:                 contract,
		Workspace:                workspace,
		State:                    state,
		CAS:                      cas,
		CASManifestDigest:        fingerprint(cas),
	}
	if (backendBoundary && (message.ID != anchors.MessageID || message.ContractRevisionID != anchors.FinalContractRevisionID || workspace.Revision != anchors.BackendWorkspaceRevision || anchors.ObligationState != "pending")) || (!backendBoundary && (message.ID != handover.MessageID || message.ContractRevisionID != handover.ContractRevisionID || workspace.Revision != handover.WorkspaceRevision)) {
		return PeerHandoverBoundarySnapshot{}, &RecoveryAnchorError{
			ReasonCode:    "recovery_anchor_mismatch",
			AnchorType:    "backend_terminal_state",
			ExpectedKey:   fmt.Sprintf("message=%s contract=%s workspace_revision=%d obligation_state=pending", anchors.MessageID, anchors.FinalContractRevisionID, anchors.BackendWorkspaceRevision),
			TerminalState: fmt.Sprintf("actual_message=%s actual_contract=%s actual_workspace_revision=%d actual_obligation_state=%s", message.ID, message.ContractRevisionID, workspace.Revision, anchors.ObligationState),
			Err:           core.Conflict,
		}
	}
	return out, nil
}

func peerHandoverCAS(root, company string) ([]PeerHandoverCASBlob, error) {
	entries, err := os.ReadDir(filepath.Join(root, company))
	if err != nil {
		return nil, err
	}
	cas := make([]PeerHandoverCASBlob, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return nil, core.Integrity
		}
		content, err := readBlob(root, company, entry.Name())
		if err != nil {
			return nil, err
		}
		cas = append(cas, PeerHandoverCASBlob{CompanyID: company, ContentSHA256: entry.Name(), Size: int64(len(content))})
	}
	return cas, nil
}

type PeerWorkView struct {
	EmployeeID         string               `json:"employee_id"`
	Role               string               `json:"role"`
	Mission            string               `json:"mission"`
	TaskID             string               `json:"task_id"`
	PeerEmployeeID     string               `json:"peer_employee_id"`
	PeerTaskID         string               `json:"peer_task_id"`
	WorkspaceDigest    string               `json:"workspace_digest"`
	WorkspaceRevision  int64                `json:"workspace_revision"`
	InitialContract    PeerContractRevision `json:"initial_contract"`
	ObligationID       string               `json:"obligation_id,omitempty"`
	ToolCallLimit      int64                `json:"tool_call_limit"`
	ToolCallsUsed      int64                `json:"tool_calls_used"`
	ToolCallsRemaining int64                `json:"tool_calls_remaining"`
}

type PeerInbox struct {
	Message  PeerMessage          `json:"message"`
	Sender   string               `json:"sender"`
	Body     string               `json:"body"`
	Contract PeerContractRevision `json:"contract"`
}

type PeerRecoveryAnchors struct {
	CompanyID                 string `json:"company_id"`
	MissionID                 string `json:"mission_id"`
	BackendTaskID             string `json:"backend_task_id"`
	FrontendTaskID            string `json:"frontend_task_id"`
	BackendSessionID          string `json:"backend_session_id"`
	BackendSessionIncarnation string `json:"backend_session_incarnation"`
	BackendSessionEpoch       int64  `json:"backend_session_epoch"`
	BackendSessionState       string `json:"backend_session_state"`
	FinalContractRevisionID   string `json:"final_contract_revision_id"`
	FinalContractRevision     int64  `json:"final_contract_revision"`
	MessageID                 string `json:"message_id"`
	MessageContractRevisionID string `json:"message_contract_revision_id"`
	MessageState              string `json:"message_state"`
	ObligationID              string `json:"obligation_id"`
	ObligationOwner           string `json:"obligation_owner"`
	ObligationState           string `json:"obligation_state"`
	BackendCheckpointID       string `json:"backend_checkpoint_id"`
	BackendArtifactID         string `json:"backend_artifact_id"`
	BackendArtifactDigest     string `json:"backend_artifact_digest"`
	BackendWorkspaceDigest    string `json:"backend_workspace_digest"`
	BackendWorkspaceRevision  int64  `json:"backend_workspace_revision"`
	CompanySequence           int64  `json:"company_sequence"`
}

type peerRecoveryQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ReadOnlyPeerRecoveryAnchors resolves the same persisted Backend/Frontend
// anchors as PeerRecoveryAnchors without acquiring runtime ownership or
// changing recovery control state. It is safe to call before allowance or
// worker creation.
func ReadOnlyPeerRecoveryAnchors(ctx context.Context, dsn, companyID string) (PeerRecoveryAnchors, error) {
	var out PeerRecoveryAnchors
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return out, err
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "polis_r0_") {
		return out, core.Denied
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return out, err
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out, err = peerRecoveryAnchors(ctx, tx, companyID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (k *Kernel) PeerRecoveryAnchors(ctx context.Context, companyID string) (PeerRecoveryAnchors, error) {
	return peerRecoveryAnchors(ctx, k.pool, companyID)
}

func peerRecoveryAnchors(ctx context.Context, q peerRecoveryQueryer, companyID string) (PeerRecoveryAnchors, error) {
	var out PeerRecoveryAnchors
	if companyID == "" {
		if err := q.QueryRow(ctx, `SELECT c.id FROM companies c JOIN missions m ON m.company_id=c.id AND m.state='active' WHERE EXISTS (SELECT 1 FROM tasks t WHERE t.company_id=c.id AND t.mission_id=m.id AND t.kind='peer_frontend') ORDER BY c.id LIMIT 1`).Scan(&companyID); err != nil {
			return out, recoveryAnchorFailure("company_mission", "active mission with peer_frontend task", "Backend terminal recovery", err)
		}
	}
	out.CompanyID = companyID
	if err := q.QueryRow(ctx, "SELECT m.id FROM missions m WHERE m.company_id=$1 AND m.state='active' ORDER BY m.id LIMIT 1", companyID).Scan(&out.MissionID); err != nil {
		return out, recoveryAnchorFailure("active_mission", companyID, "active mission", err)
	}
	if err := q.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND owner='emp-backend' AND kind='peer_backend' ORDER BY id LIMIT 1", companyID, out.MissionID).Scan(&out.BackendTaskID); err != nil {
		return out, recoveryAnchorFailure("backend_task", fmt.Sprintf("company=%s mission=%s owner=emp-backend kind=peer_backend", companyID, out.MissionID), "Backend task exists", err)
	}
	if err := q.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND owner='emp-frontend' AND kind='peer_frontend' ORDER BY id LIMIT 1", companyID, out.MissionID).Scan(&out.FrontendTaskID); err != nil {
		return out, recoveryAnchorFailure("frontend_task", fmt.Sprintf("company=%s mission=%s owner=emp-frontend kind=peer_frontend", companyID, out.MissionID), "Frontend task exists; Frontend worker may remain unstarted", err)
	}
	if err := q.QueryRow(ctx, `SELECT id,epoch,incarnation,state FROM worker_sessions WHERE company_id=$1 AND employee_id='emp-backend' AND task_id=$2 ORDER BY generation DESC,id DESC LIMIT 1`, companyID, out.BackendTaskID).Scan(&out.BackendSessionID, &out.BackendSessionEpoch, &out.BackendSessionIncarnation, &out.BackendSessionState); err != nil {
		return out, recoveryAnchorFailure("backend_session", fmt.Sprintf("company=%s task=%s employee=emp-backend", companyID, out.BackendTaskID), "Backend worker stopped after terminal turn", err)
	}
	if err := q.QueryRow(ctx, "SELECT id,revision FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND state='accepted' ORDER BY revision DESC LIMIT 1", companyID, out.MissionID).Scan(&out.FinalContractRevisionID, &out.FinalContractRevision); err != nil {
		return out, recoveryAnchorFailure("effective_contract", fmt.Sprintf("company=%s mission=%s state=accepted", companyID, out.MissionID), "accepted effective ContractRevision", err)
	}
	if err := q.QueryRow(ctx, `SELECT m.id,m.contract_revision_id,m.delivery_state,COALESCE(o.id,''),COALESCE(o.owner,''),COALESCE(o.state,'') FROM messages m LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id WHERE m.company_id=$1 AND m.mission_id=$2 AND m.recipient='emp-frontend' AND m.contract_revision_id=$3 ORDER BY m.id DESC LIMIT 1`, companyID, out.MissionID, out.FinalContractRevisionID).Scan(&out.MessageID, &out.MessageContractRevisionID, &out.MessageState, &out.ObligationID, &out.ObligationOwner, &out.ObligationState); err != nil {
		return out, recoveryAnchorFailure("pending_peer_obligation", fmt.Sprintf("company=%s mission=%s recipient=emp-frontend contract=%s", companyID, out.MissionID, out.FinalContractRevisionID), "Message persisted and Obligation pending", err)
	}
	if err := q.QueryRow(ctx, `SELECT a.id,a.digest FROM artifacts a WHERE a.company_id=$1 AND a.task_id=$2 AND a.author='emp-backend' AND a.verdict='candidate' ORDER BY a.id DESC LIMIT 1`, companyID, out.BackendTaskID).Scan(&out.BackendArtifactID, &out.BackendArtifactDigest); err != nil {
		return out, recoveryAnchorFailure("backend_artifact", fmt.Sprintf("company=%s task=%s author=emp-backend verdict=candidate", companyID, out.BackendTaskID), "Backend artifact finalized as candidate", err)
	}
	if err := q.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", companyID, out.BackendTaskID).Scan(&out.BackendWorkspaceDigest, &out.BackendWorkspaceRevision); err != nil {
		return out, recoveryAnchorFailure("backend_workspace", fmt.Sprintf("company=%s task=%s", companyID, out.BackendTaskID), "Backend workspace at terminal revision", err)
	}
	if err := q.QueryRow(ctx, `SELECT cp.id FROM worker_checkpoints cp JOIN worker_sessions s ON s.company_id=cp.company_id AND s.id=cp.session_id WHERE cp.company_id=$1 AND s.task_id=$2 AND cp.data->>'kind'='qualified' AND cp.data->>'contract_revision_id'=$3 ORDER BY cp.id DESC LIMIT 1`, companyID, out.BackendTaskID, out.FinalContractRevisionID).Scan(&out.BackendCheckpointID); err != nil {
		return out, recoveryAnchorFailure("qualified_checkpoint", fmt.Sprintf("company=%s task=%s contract=%s kind=qualified", companyID, out.BackendTaskID, out.FinalContractRevisionID), "current qualified Backend checkpoint", err)
	}
	if err := q.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", companyID).Scan(&out.CompanySequence); err != nil {
		return out, recoveryAnchorFailure("company_sequence", companyID, "event ordering anchor", err)
	}
	return out, nil
}

func (k *Kernel) PeerMessageLifecycle(ctx context.Context, s Scope, messageID string) ([]string, error) {
	rows, err := k.pool.Query(ctx, `SELECT kind FROM events WHERE company_id=$1 AND payload->>'id'=$2 AND kind IN ('collab.delivered','collab.observed','collab.acknowledged','collab.applied','collab.resolved') ORDER BY company_seq`, s.company, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []string
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			return nil, err
		}
		states = append(states, kind)
	}
	return states, rows.Err()
}

func (k *Kernel) PeerWorkCurrent(ctx context.Context, b Binding) (PeerWorkView, error) {
	var out PeerWorkView
	h, e := k.Handover(ctx, b)
	if e != nil {
		return out, e
	}
	out.EmployeeID, out.TaskID = b.employee, h.Task.ID
	out.Role, out.Mission = string(h.Task.Kind), h.Task.Mission
	out.WorkspaceDigest, out.WorkspaceRevision = h.Workspace.Digest, h.Workspace.Revision
	out.ToolCallLimit, out.ToolCallsUsed, out.ToolCallsRemaining = h.ToolBudget.Limit, h.ToolBudget.Used, h.ToolBudget.Remaining
	if h.Obligations != nil {
		for _, obligation := range h.Obligations {
			if obligation.State == "pending" || obligation.State == "observed" || obligation.State == "applied" {
				out.ObligationID = obligation.ID
				break
			}
		}
	}
	if e = k.pool.QueryRow(ctx, "SELECT id,owner FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind=$3 AND id<>$4", b.scope.company, h.Task.Mission, map[string]string{"peer_backend": "peer_frontend", "peer_frontend": "peer_backend"}[string(h.Task.Kind)], h.Task.ID).Scan(&out.PeerTaskID, &out.PeerEmployeeID); e != nil {
		return out, e
	}
	if e = k.pool.QueryRow(ctx, "SELECT id,mission_id,endpoint,schema::text,digest,state,revision,COALESCE(base_revision,0) FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND revision=1", b.scope.company, h.Task.Mission).Scan(&out.InitialContract.ID, &out.InitialContract.Mission, &out.InitialContract.Endpoint, &out.InitialContract.Schema, &out.InitialContract.Digest, &out.InitialContract.State, &out.InitialContract.Revision, &out.InitialContract.BaseRevision); e != nil {
		return out, e
	}
	return out, nil
}

func (k *Kernel) PeerContractRead(ctx context.Context, b Binding, revisionID string) (PeerContractRevision, error) {
	var out PeerContractRevision
	h, e := k.Handover(ctx, b)
	if e != nil {
		return out, e
	}
	e = k.pool.QueryRow(ctx, "SELECT id,mission_id,endpoint,schema::text,digest,state,revision,COALESCE(base_revision,0) FROM contract_revisions WHERE company_id=$1 AND id=$2 AND mission_id=$3", b.scope.company, revisionID, h.Task.Mission).Scan(&out.ID, &out.Mission, &out.Endpoint, &out.Schema, &out.Digest, &out.State, &out.Revision, &out.BaseRevision)
	return out, e
}

func (k *Kernel) PeerInbox(ctx context.Context, b Binding) (PeerInbox, error) {
	var out PeerInbox
	h, e := k.PeerHandover(ctx, b)
	if e != nil {
		return out, e
	}
	if h.MessageID == "" || h.ContractRevisionID == "" {
		return out, core.OutOfScope
	}
	var messageKind string
	if e = k.pool.QueryRow(ctx, "SELECT kind FROM messages WHERE company_id=$1 AND id=$2 AND recipient=$3", b.scope.company, h.MessageID, b.employee).Scan(&messageKind); e != nil {
		return out, e
	}
	if h.ObligationID == "" {
		if messageKind != "fyi" {
			return out, core.OutOfScope
		}
	} else if h.ObligationState != "pending" && h.ObligationState != "observed" && h.ObligationState != "acknowledged" && h.ObligationState != "applied" {
		return out, core.OutOfScope
	}
	if h.MessageState == "persisted" {
		if e = k.TXPeerDeliver(ctx, b, h.MessageID, "peer-deliver-"+h.MessageID); e != nil {
			return out, e
		}
		h, e = k.PeerHandover(ctx, b)
		if e != nil {
			return out, e
		}
	}
	if h.MessageState == "delivered" {
		if e = k.TXPeerObserve(ctx, b, h.MessageID, "peer-observe-"+h.MessageID); e != nil {
			return out, e
		}
		h, e = k.PeerHandover(ctx, b)
		if e != nil {
			return out, e
		}
	}
	if e = k.pool.QueryRow(ctx, "SELECT sender,body,contract_revision_id FROM messages WHERE company_id=$1 AND id=$2 AND recipient=$3", b.scope.company, h.MessageID, b.employee).Scan(&out.Sender, &out.Body, &out.Message.ContractRevisionID); e != nil {
		return out, e
	}
	out.Message = PeerMessage{ID: h.MessageID, TaskID: h.TaskID, ObligationID: h.ObligationID, ContractRevisionID: h.ContractRevisionID, DeliveryState: h.MessageState}
	out.Contract, e = k.PeerContractRead(ctx, b, h.ContractRevisionID)
	return out, e
}

func (k *Kernel) PeerReviewIntegration(ctx context.Context, s Scope, integrationID string) (PeerVerificationReport, error) {
	var state string
	if e := k.pool.QueryRow(ctx, "SELECT state FROM integration_candidates WHERE company_id=$1 AND id=$2", s.company, integrationID).Scan(&state); e != nil {
		return PeerVerificationReport{}, e
	}
	if state != "passed" {
		return PeerVerificationReport{Passed: false, Reason: "integration candidate has no passed deterministic verifier record"}, nil
	}
	return PeerVerificationReport{Passed: true}, nil
}

func digestPeerContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func (k *Kernel) TXCreatePeerFixture(ctx context.Context, s Scope, mission string) (PeerFixture, error) {
	var out PeerFixture
	if !core.ValidID(mission) {
		return out, core.Malformed
	}
	backendDigest, frontendDigest := digestPeerContent([]byte(fixture.PeerBackendV1)), digestPeerContent([]byte(fixture.PeerFrontendV1))
	if _, e := putBlob(k.root, s.company, []byte(fixture.PeerBackendV1)); e != nil {
		return out, e
	}
	if _, e := putBlob(k.root, s.company, []byte(fixture.PeerFrontendV1)); e != nil {
		return out, e
	}
	v1Schema := []byte(`{"items":["id","name"]}`)
	v1Digest := digestPeerContent(v1Schema)
	_, e := k.TXWrite(ctx, s, nil, "peer-fixture-"+mission, "peer.fixture.create", mission, func(tx pgx.Tx) (Receipt, error) {
		if _, e := tx.Exec(ctx, "INSERT INTO missions(company_id,id,state,activation_id,contract) VALUES($1,$2,'active',$3,'r03-api@1')", s.company, mission, newID()); e != nil {
			return Receipt{}, e
		}
		backendID, frontendID := newID(), newID()
		if _, e := tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state,plan) VALUES($1,$2,$3,'emp-backend','peer_backend','ready',$4),($1,$5,$3,'emp-frontend','peer_frontend','ready',$6)", s.company, backendID, mission, []byte(`{"role":"backend","contract":"api-pagination@1"}`), frontendID, []byte(`{"role":"frontend","contract":"api-pagination@1"}`)); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3),($1,$4,$5)", s.company, backendID, backendDigest, frontendID, frontendDigest); e != nil {
			return Receipt{}, e
		}
		contractID := newID()
		if _, e := tx.Exec(ctx, "INSERT INTO contract_revisions(company_id,id,mission_id,revision,base_revision,endpoint,schema,digest,state,proposer,accepter) VALUES($1,$2,$3,1,NULL,'GET /items',$4,$5,'accepted','emp-backend','emp-backend')", s.company, contractID, mission, v1Schema, v1Digest); e != nil {
			return Receipt{}, e
		}
		out.Mission = mission
		out.Backend = Task{ID: backendID, Mission: mission, Owner: "emp-backend", Kind: "peer_backend", State: "ready"}
		out.Frontend = Task{ID: frontendID, Mission: mission, Owner: "emp-frontend", Kind: "peer_frontend", State: "ready"}
		out.ContractV1 = PeerContractRevision{ID: contractID, Mission: mission, Endpoint: "GET /items", Schema: string(v1Schema), Digest: v1Digest, State: "accepted", Revision: 1}
		return Receipt{ID: mission, Status: "ready"}, nil
	})
	return out, e
}

func (k *Kernel) TXProposePeerContract(ctx context.Context, b Binding, task Task, proposal PeerContractProposal, key string) (PeerContractRevision, error) {
	var out PeerContractRevision
	if proposal.Endpoint == "" || proposal.Schema == "" {
		return out, core.Malformed
	}
	_, e := k.TXWrite(ctx, b.scope, &b, key, "contract.propose", proposal, func(tx pgx.Tx) (Receipt, error) {
		t, e := checkWork(ctx, tx, b, task)
		if e != nil || t.Kind != "peer_backend" || t.State != "working" || b.employee != "emp-backend" {
			if e != nil {
				return Receipt{}, e
			}
			return Receipt{}, core.Denied
		}
		var base int64
		if e = tx.QueryRow(ctx, "SELECT COALESCE(max(revision),0) FROM contract_revisions WHERE company_id=$1 AND mission_id=$2", b.scope.company, t.Mission).Scan(&base); e != nil {
			return Receipt{}, e
		}
		id := newID()
		digest := digestPeerContent([]byte(proposal.Schema))
		if _, e = tx.Exec(ctx, "INSERT INTO contract_revisions(company_id,id,mission_id,revision,base_revision,endpoint,schema,digest,state,proposer) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'proposed',$9)", b.scope.company, id, t.Mission, base+1, base, proposal.Endpoint, []byte(proposal.Schema), digest, b.employee); e != nil {
			return Receipt{}, e
		}
		out = PeerContractRevision{ID: id, Mission: t.Mission, Endpoint: proposal.Endpoint, Schema: proposal.Schema, Digest: digest, State: "proposed", Revision: base + 1, BaseRevision: base}
		return Receipt{ID: id, Status: "proposed"}, nil
	})
	return out, e
}

func (k *Kernel) TXAcceptPeerContract(ctx context.Context, b Binding, revisionID, key string) error {
	_, e := k.TXWrite(ctx, b.scope, &b, key, "contract.accept", revisionID, func(tx pgx.Tx) (Receipt, error) {
		var mission, proposer, state string
		var targetRevision int64
		if e := tx.QueryRow(ctx, "SELECT mission_id,proposer,state,revision FROM contract_revisions WHERE company_id=$1 AND id=$2", b.scope.company, revisionID).Scan(&mission, &proposer, &state, &targetRevision); e != nil {
			return Receipt{}, e
		}
		if proposer != b.employee || b.employee != "emp-backend" || state != "proposed" {
			return Receipt{}, core.Denied
		}
		if _, e := tx.Exec(ctx, "UPDATE contract_revisions SET state='superseded' WHERE company_id=$1 AND mission_id=$2 AND state='accepted'", b.scope.company, mission); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, "UPDATE contract_revisions SET state='accepted',accepter=$3 WHERE company_id=$1 AND id=$2", b.scope.company, revisionID, b.employee); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, `UPDATE worker_checkpoints cp SET data=jsonb_set(cp.data,'{finalization_state}','"superseded_for_finalization"') FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE cp.company_id=$1 AND s.company_id=cp.company_id AND s.id=cp.session_id AND t.mission_id=$2 AND cp.data->>'kind'='qualified' AND cp.data->>'contract_revision_id' IS DISTINCT FROM $3`, b.scope.company, mission, revisionID); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, `UPDATE obligations o SET state='superseded'
FROM messages m JOIN contract_revisions c ON c.company_id=m.company_id AND c.id=m.contract_revision_id
WHERE o.company_id=$1 AND o.id=m.id AND m.mission_id=$2 AND c.revision<$3
  AND o.state IN ('pending','observed','applied')`, b.scope.company, mission, targetRevision); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, `UPDATE messages m SET delivery_state='superseded'
FROM contract_revisions c
WHERE m.company_id=$1 AND m.mission_id=$2 AND c.company_id=m.company_id AND c.id=m.contract_revision_id
  AND c.revision<$3 AND m.delivery_state IN ('persisted','delivered','observed','acknowledged','applied')`, b.scope.company, mission, targetRevision); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, `UPDATE peer_work_signals s SET state='superseded'
FROM messages m JOIN contract_revisions c ON c.company_id=m.company_id AND c.id=m.contract_revision_id
WHERE s.company_id=$1 AND s.message_id=m.id AND m.mission_id=$2 AND c.revision<$3
  AND s.state IN ('pending','observed','acknowledged','applied')`, b.scope.company, mission, targetRevision); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: revisionID, Status: "accepted"}, nil
	})
	return e
}

func (k *Kernel) TXPeerSend(ctx context.Context, b Binding, input PeerSendInput, key string) (PeerMessage, error) {
	var out PeerMessage
	if input.FromTask == "" || input.FromTask != b.task {
		return out, core.Denied
	}
	if input.Body == "" || len(input.Body) > core.MaxContent {
		return out, core.Malformed
	}
	targetTask := input.ToTaskID
	if targetTask == "" {
		targetTask = input.ToTask
	}
	targetEmployee := input.ToEmployeeID
	if targetEmployee == "" {
		targetEmployee = "emp-frontend"
	}
	r, e := k.TXWrite(ctx, b.scope, &b, key, "collab.send", input, func(tx pgx.Tx) (Receipt, error) {
		var fromOwner, fromKind, fromMission, toOwner, toKind, toMission, state, targetState string
		if e := tx.QueryRow(ctx, "SELECT owner,kind,mission_id,state FROM tasks WHERE company_id=$1 AND id=$2", b.scope.company, input.FromTask).Scan(&fromOwner, &fromKind, &fromMission, &state); e != nil {
			return Receipt{}, e
		}
		if e := tx.QueryRow(ctx, "SELECT owner,kind,mission_id,state FROM tasks WHERE company_id=$1 AND id=$2", b.scope.company, targetTask).Scan(&toOwner, &toKind, &toMission, &targetState); errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, peerSendDenied("target_task_not_found", "to_task_id must be the current peer task ID returned by work_current.", targetEmployee, targetTask, state, input.ContractRevisionID)
		} else if e != nil {
			return Receipt{}, e
		}
		if toMission != fromMission {
			return Receipt{}, peerSendDenied("target_task_outside_mission", "the target peer task must belong to the sender's current Mission.", targetEmployee, targetTask, state, input.ContractRevisionID)
		}
		if input.FromTask != b.task || fromOwner != b.employee ||
			!((fromKind == "peer_backend" && toKind == "peer_frontend") || (fromKind == "peer_frontend" && toKind == "peer_backend")) ||
			toOwner != targetEmployee {
			return Receipt{}, peerSendDenied("target_task_not_owned_by_employee", "to_task_id must belong to the supplied employee and be the other current peer task in this company.", targetEmployee, targetTask, state, input.ContractRevisionID)
		}
		if targetState != "ready" && targetState != "working" {
			return Receipt{}, peerSendDenied("target_task_not_current", "the target peer task must be ready or working.", targetEmployee, targetTask, targetState, input.ContractRevisionID)
		}
		if state != "working" {
			reason := "peer_send_not_allowed_in_task_state"
			if state == "candidate" {
				reason = "peer_send_not_allowed_in_candidate_state"
			}
			return Receipt{}, peerSendDenied(reason, "collaboration must be persisted while the sender task is working, before candidate publication.", targetEmployee, targetTask, state, input.ContractRevisionID)
		}
		var contractState string
		if e := tx.QueryRow(ctx, "SELECT state FROM contract_revisions WHERE company_id=$1 AND id=$2 AND mission_id=$3", b.scope.company, input.ContractRevisionID, fromMission).Scan(&contractState); errors.Is(e, pgx.ErrNoRows) {
			return Receipt{}, peerSendDenied("stale_contract_revision", "use the current accepted contract_revision_id for this mission.", targetEmployee, targetTask, state, input.ContractRevisionID)
		} else if e != nil {
			return Receipt{}, e
		}
		if contractState != "accepted" {
			return Receipt{}, peerSendDenied("stale_contract_revision", "use the current accepted contract_revision_id for this mission.", targetEmployee, targetTask, state, input.ContractRevisionID)
		}
		mission, e := missionState(ctx, tx, b.scope, fromMission)
		if e != nil {
			return Receipt{}, e
		}
		if mission != "active" {
			return Receipt{}, core.Denied
		}
		messageID := newID()
		kind := "fyi"
		if input.Actionable {
			kind = "request"
		}
		var obligationID string
		if input.Actionable {
			obligationID = messageID
		}
		if _, e := tx.Exec(ctx, "INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body,contract_revision_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", b.scope.company, messageID, fromMission, targetTask, b.employee, targetEmployee, kind, input.Body, input.ContractRevisionID); e != nil {
			return Receipt{}, e
		}
		if input.Actionable {
			if _, e := tx.Exec(ctx, "INSERT INTO obligations(company_id,id,task_id,owner,state) VALUES($1,$2,$3,$4,'pending')", b.scope.company, obligationID, targetTask, targetEmployee); e != nil {
				return Receipt{}, e
			}
			if _, e := tx.Exec(ctx, "INSERT INTO peer_work_signals(company_id,id,message_id,obligation_id,recipient,state) VALUES($1,$2,$3,$4,$5,'pending')", b.scope.company, newID(), messageID, obligationID, targetEmployee); e != nil {
				return Receipt{}, e
			}
			if e := signalEmployeeScheduleTX(ctx, tx, b.scope, targetEmployee); e != nil {
				return Receipt{}, e
			}
		}
		out = PeerMessage{ID: messageID, TaskID: targetTask, ObligationID: obligationID, ContractRevisionID: input.ContractRevisionID, DeliveryState: "persisted"}
		return Receipt{ID: messageID, Status: "persisted"}, nil
	})
	if e == nil && out.ID == "" {
		out.ID, out.DeliveryState = r.ID, r.Status
		out.TaskID, out.ContractRevisionID = targetTask, input.ContractRevisionID
		_ = k.pool.QueryRow(ctx, "SELECT COALESCE(o.id,'') FROM messages m LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id WHERE m.company_id=$1 AND m.id=$2", b.scope.company, r.ID).Scan(&out.ObligationID)
	}
	return out, e
}

func peerSendDenied(reason, summary, targetEmployee, targetTask, taskState, contractID string) error {
	return peerToolError{Code: core.Denied, Rejection: PeerToolRejection{
		ReasonCode:              reason,
		ActionableSummary:       summary,
		ExpectedReferenceTypes:  []string{"to_employee_id", "to_task_id", "contract_revision_id"},
		TargetEmployeeID:        targetEmployee,
		TargetTaskID:            targetTask,
		CurrentTaskState:        taskState,
		CurrentContractRevision: contractID,
		TransactionOutcome:      "rolled_back_before_message_insert",
	}}
}

func (k *Kernel) TXPeerObserve(ctx context.Context, b Binding, messageID, key string) error {
	return k.peerMessageState(ctx, b, messageID, key, "observed", "observed", "observed")
}
func (k *Kernel) TXPeerAck(ctx context.Context, b Binding, messageID, key string) error {
	return k.peerMessageState(ctx, b, messageID, key, "acknowledged", "observed", "acknowledged")
}

func (k *Kernel) TXPeerDeliver(ctx context.Context, b Binding, messageID, key string) error {
	return k.peerMessageState(ctx, b, messageID, key, "delivered", "pending", "pending")
}

func (k *Kernel) peerMessageState(ctx context.Context, b Binding, messageID, key, messageState, obligationState, signalState string) error {
	_, e := k.TXWrite(ctx, b.scope, &b, key, "collab."+messageState, messageID, func(tx pgx.Tx) (Receipt, error) {
		var recipient, state, obligationID string
		if e := tx.QueryRow(ctx, "SELECT recipient,delivery_state,COALESCE((SELECT id FROM obligations WHERE company_id=messages.company_id AND id=messages.id),'') FROM messages WHERE company_id=$1 AND id=$2", b.scope.company, messageID).Scan(&recipient, &state, &obligationID); e != nil {
			return Receipt{}, e
		}
		if recipient != b.employee || (messageState == "delivered" && state != "persisted") || (messageState == "observed" && state != "delivered") || (messageState == "acknowledged" && state != "observed") {
			return Receipt{}, core.Denied
		}
		if _, e := tx.Exec(ctx, "UPDATE messages SET delivery_state=$3 WHERE company_id=$1 AND id=$2", b.scope.company, messageID, messageState); e != nil {
			return Receipt{}, e
		}
		if messageState == "observed" {
			var workspaceRevision int64
			if e := tx.QueryRow(ctx, "SELECT w.revision FROM worker_workspaces w JOIN messages m ON m.company_id=w.company_id AND m.task_id=w.task_id WHERE m.company_id=$1 AND m.id=$2", b.scope.company, messageID).Scan(&workspaceRevision); e != nil {
				return Receipt{}, e
			}
			if _, e := tx.Exec(ctx, "UPDATE messages SET task_revision=$3 WHERE company_id=$1 AND id=$2", b.scope.company, messageID, workspaceRevision); e != nil {
				return Receipt{}, e
			}
		}
		if obligationID != "" {
			if _, e := tx.Exec(ctx, "UPDATE obligations SET state=$3 WHERE company_id=$1 AND id=$2 AND state IN ('pending','observed')", b.scope.company, obligationID, obligationState); e != nil {
				return Receipt{}, e
			}
			if _, e := tx.Exec(ctx, "UPDATE peer_work_signals SET state=$3 WHERE company_id=$1 AND message_id=$2", b.scope.company, messageID, signalState); e != nil {
				return Receipt{}, e
			}
		}
		return Receipt{ID: messageID, Status: messageState}, nil
	})
	return e
}

func (k *Kernel) TXPeerApply(ctx context.Context, b Binding, request PeerApplyRequest, key string) (Receipt, error) {
	if rejection, code := validatePeerApplyRequest(request); code != "" {
		return Receipt{}, peerToolError{Code: code, Rejection: rejection}
	}
	evidenceRaw, err := json.Marshal(request.EvidenceRefs)
	if err != nil {
		return Receipt{}, err
	}
	return k.TXWrite(ctx, b.scope, &b, key, "collab.apply", request, func(tx pgx.Tx) (Receipt, error) {
		var task, mission, owner, messageState, obligationState, messageContractID string
		if err := tx.QueryRow(ctx, `SELECT o.task_id,t.mission_id,o.owner,m.delivery_state,o.state,m.contract_revision_id
FROM obligations o
JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
JOIN worker_sessions ws ON ws.company_id=o.company_id AND ws.id=$3 AND ws.task_id=o.task_id
WHERE o.company_id=$1 AND o.id=$2
FOR UPDATE OF o,m`, b.scope.company, request.ObligationID, b.session).Scan(&task, &mission, &owner, &messageState, &obligationState, &messageContractID); err != nil {
			return Receipt{}, peerApplyDenied("obligation_not_current", "the supplied obligation is not the current peer responsibility; read collab_inbox or work_current again.", request, "", 0)
		}
		if b.employee != owner || (messageState != "observed" && messageState != "acknowledged") || (obligationState != "observed" && obligationState != "acknowledged") {
			return Receipt{}, peerApplyDenied("obligation_not_current", "the obligation must be observed or acknowledged and must not be stale, superseded or resolved.", request, request.ContractRevisionID, 0)
		}
		var currentContractID string
		if err := tx.QueryRow(ctx, "SELECT id FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND state='accepted' ORDER BY revision DESC LIMIT 1", b.scope.company, mission).Scan(&currentContractID); err != nil {
			return Receipt{}, peerApplyDenied("contract_revision_not_current", "the mission has no current accepted contract revision.", request, "", 0)
		}
		if messageContractID != request.ContractRevisionID || currentContractID != request.ContractRevisionID {
			return Receipt{}, peerApplyDenied("contract_revision_not_current", "use the current accepted contract_revision_id referenced by the peer message.", request, currentContractID, 0)
		}
		var currentWorkspaceRevision int64
		var currentWorkspaceDigest string
		if err := tx.QueryRow(ctx, "SELECT revision,digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", b.scope.company, task).Scan(&currentWorkspaceRevision, &currentWorkspaceDigest); err != nil {
			return Receipt{}, err
		}
		if request.WorkspaceRevision != currentWorkspaceRevision {
			return Receipt{}, peerApplyDenied("workspace_revision_mismatch", "workspace_revision must equal the current recipient workspace revision.", request, currentContractID, currentWorkspaceRevision)
		}
		var observedWorkspaceRevision int64
		if err := tx.QueryRow(ctx, "SELECT task_revision FROM messages WHERE company_id=$1 AND id=$2", b.scope.company, request.ObligationID).Scan(&observedWorkspaceRevision); err != nil {
			return Receipt{}, err
		}
		if observedWorkspaceRevision <= 0 || currentWorkspaceRevision <= observedWorkspaceRevision {
			return Receipt{}, peerApplyDenied("workspace_not_changed_since_observation", "write the recipient workspace after observing the obligation, then submit its new workspace revision.", request, currentContractID, currentWorkspaceRevision)
		}
		mutationEvidence := false
		invalidEvidence := make([]InvalidEvidenceRef, 0)
		for _, ref := range request.EvidenceRefs {
			kind, valid, err := peerEvidenceRefKind(ctx, tx, b, ref, currentWorkspaceDigest)
			if err != nil {
				return Receipt{}, err
			}
			if !valid {
				detectedType, detectErr := peerEvidenceRefType(ctx, tx, b, ref)
				if detectErr != nil {
					return Receipt{}, detectErr
				}
				invalidEvidence = append(invalidEvidence, InvalidEvidenceRef{Ref: ref, DetectedType: detectedType, Reason: evidenceRefReason(detectedType)})
				continue
			}
			if kind == "workspace.replace" {
				mutationEvidence = true
			}
		}
		if len(invalidEvidence) != 0 {
			return Receipt{}, peerApplyEvidenceDenied(request, currentContractID, currentWorkspaceRevision, invalidEvidence)
		}
		if !mutationEvidence {
			return Receipt{}, peerApplyDenied("workspace_change_evidence_required", "include the workspace_replace receipt proving the current workspace changed after observation.", request, currentContractID, currentWorkspaceRevision)
		}
		if _, err := tx.Exec(ctx, "UPDATE messages SET delivery_state='applied' WHERE company_id=$1 AND id=$2", b.scope.company, request.ObligationID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE obligations SET state='applied',evidence_ref=$3 WHERE company_id=$1 AND id=$2", b.scope.company, request.ObligationID, string(evidenceRaw)); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE peer_work_signals SET state='applied' WHERE company_id=$1 AND obligation_id=$2", b.scope.company, request.ObligationID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: request.ObligationID, Status: "applied"}, nil
	})
}

func peerApplyDenied(reason, summary string, request PeerApplyRequest, currentContract string, currentWorkspace int64) error {
	rejection := newPeerToolRejection(reason, summary)
	rejection.CurrentObligationID = request.ObligationID
	rejection.CurrentContractRevision = currentContract
	rejection.CurrentWorkspaceRevision = currentWorkspace
	return peerToolError{Code: core.Denied, Rejection: rejection}
}

func peerApplyEvidenceDenied(request PeerApplyRequest, currentContract string, currentWorkspace int64, invalid []InvalidEvidenceRef) error {
	rejection := newPeerToolRejection("evidence_ref_invalid", "remove the listed references and submit only current workspace.replace, workspace.check or successful collab.apply receipt IDs; include a workspace.replace receipt.")
	rejection.InvalidEvidenceRefs = invalid
	rejection.CurrentObligationID = request.ObligationID
	rejection.CurrentContractRevision = currentContract
	rejection.CurrentWorkspaceRevision = currentWorkspace
	return peerToolError{Code: core.Denied, Rejection: rejection}
}

func evidenceRefReason(detectedType string) string {
	if detectedType == "unknown" {
		return "no persisted receipt of this worker was found for the supplied ref"
	}
	return "receipt type is not accepted by collab_apply"
}

func peerEvidenceRefKind(ctx context.Context, tx pgx.Tx, b Binding, ref, workspaceDigest string) (string, bool, error) {
	var kind string
	err := tx.QueryRow(ctx, `SELECT CASE
WHEN EXISTS (SELECT 1 FROM worker_checks WHERE company_id=$1 AND id=$3 AND session_id=$2 AND digest=$4) THEN 'workspace.check'
WHEN $3=$4 AND EXISTS (SELECT 1 FROM receipts r JOIN events e ON e.company_id=r.company_id AND e.payload->>'id'=r.result->>'id' WHERE r.company_id=$1 AND r.actor=$5 AND r.result->>'id'=$3 AND e.kind='workspace.replace' AND e.payload->>'id'=$3) THEN 'workspace.replace'
WHEN EXISTS (SELECT 1 FROM receipts r JOIN events e ON e.company_id=r.company_id AND e.payload->>'id'=r.result->>'id' WHERE r.company_id=$1 AND r.actor=$5 AND r.result->>'id'=$3 AND e.kind='collab.apply' AND e.payload->>'id'=$3) THEN 'collab.apply'
ELSE '' END`, b.scope.company, b.session, ref, workspaceDigest, b.employee).Scan(&kind)
	return kind, err == nil && kind != "", err
}

func peerEvidenceRefType(ctx context.Context, tx pgx.Tx, b Binding, ref string) (string, error) {
	var kind string
	err := tx.QueryRow(ctx, `SELECT CASE
WHEN EXISTS (SELECT 1 FROM worker_checks WHERE company_id=$1 AND id=$3 AND session_id=$2) THEN 'workspace.check'
WHEN EXISTS (SELECT 1 FROM receipts r JOIN events e ON e.company_id=r.company_id AND e.payload->>'id'=r.result->>'id' WHERE r.company_id=$1 AND r.actor=$4 AND r.result->>'id'=$3 AND e.kind='workspace.replace' AND e.payload->>'id'=$3) THEN 'workspace.replace'
WHEN EXISTS (SELECT 1 FROM receipts r JOIN events e ON e.company_id=r.company_id AND e.payload->>'id'=r.result->>'id' WHERE r.company_id=$1 AND r.actor=$4 AND r.result->>'id'=$3 AND e.kind='collab.apply' AND e.payload->>'id'=$3) THEN 'collab.apply'
WHEN EXISTS (SELECT 1 FROM receipts r JOIN events e ON e.company_id=r.company_id AND e.payload->>'id'=r.result->>'id' WHERE r.company_id=$1 AND r.actor=$4 AND r.result->>'id'=$3 AND e.kind='work.checkpoint' AND e.payload->>'id'=$3) THEN 'work.checkpoint'
WHEN EXISTS (SELECT 1 FROM receipts r WHERE r.company_id=$1 AND r.actor=$4 AND r.result->>'id'=$3) THEN 'other.receipt'
ELSE 'unknown' END`, b.scope.company, b.session, ref, b.employee).Scan(&kind)
	return kind, err
}

func (k *Kernel) TXPeerResolve(ctx context.Context, b Binding, obligationID, artifactID, key string) error {
	_, e := k.TXWrite(ctx, b.scope, &b, key, "obligation.resolve", []string{obligationID, artifactID}, func(tx pgx.Tx) (Receipt, error) {
		var task, owner, state string
		if e := tx.QueryRow(ctx, "SELECT task_id,owner,state FROM obligations WHERE company_id=$1 AND id=$2", b.scope.company, obligationID).Scan(&task, &owner, &state); e != nil {
			return Receipt{}, e
		}
		if task != b.task || b.employee != owner || state != "applied" {
			return Receipt{}, core.Denied
		}
		var digest string
		if e := tx.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND task_id=$3 AND author=$4 AND state='ready' AND verdict='candidate' AND contract='r03-api@1'", b.scope.company, artifactID, task, b.employee).Scan(&digest); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, "UPDATE obligations SET state='fulfilled',evidence_id=$3,evidence_ref=$3 WHERE company_id=$1 AND id=$2", b.scope.company, obligationID, artifactID); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, "UPDATE messages SET delivery_state='resolved' WHERE company_id=$1 AND id=$2", b.scope.company, obligationID); e != nil {
			return Receipt{}, e
		}
		if _, e := tx.Exec(ctx, "UPDATE peer_work_signals SET state='resolved' WHERE company_id=$1 AND obligation_id=$2", b.scope.company, obligationID); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: artifactID, Status: "fulfilled"}, nil
	})
	return e
}

func (k *Kernel) TXFreezePeerIntegration(ctx context.Context, s Scope, input PeerIntegrationInput, key string) (Receipt, error) {
	return k.TXWrite(ctx, s, nil, key, "integration.freeze", input, func(tx pgx.Tx) (Receipt, error) {
		var state string
		if e := tx.QueryRow(ctx, "SELECT state FROM contract_revisions WHERE company_id=$1 AND id=$2 AND mission_id=$3", s.company, input.ContractRevisionID, input.Mission).Scan(&state); e != nil {
			return Receipt{}, e
		}
		if state != "accepted" {
			return Receipt{}, core.Denied
		}
		id := newID()
		if _, e := tx.Exec(ctx, "INSERT INTO integration_candidates(company_id,id,mission_id,backend_artifact_id,frontend_artifact_id,contract_revision_id,base_revision,verifier_revision,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'candidate')", s.company, id, input.Mission, input.BackendArtifactID, input.FrontendArtifactID, input.ContractRevisionID, input.BaseRevision, input.VerifierRevision); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: id, Status: "candidate"}, nil
	})
}

func (k *Kernel) VerifyPeerIntegration(ctx context.Context, s Scope, integrationID string) (PeerVerificationReport, error) {
	var report PeerVerificationReport
	var mission, backendID, frontendID, contractID, verifierRevision string
	if e := k.pool.QueryRow(ctx, "SELECT mission_id,backend_artifact_id,frontend_artifact_id,contract_revision_id,verifier_revision FROM integration_candidates WHERE company_id=$1 AND id=$2", s.company, integrationID).Scan(&mission, &backendID, &frontendID, &contractID, &verifierRevision); e != nil {
		return report, e
	}
	report.VerifierRevision = verifierRevision
	var contractState, contractEndpoint, contractSchema string
	if e := k.pool.QueryRow(ctx, "SELECT state,endpoint,schema::text FROM contract_revisions WHERE company_id=$1 AND id=$2 AND state='accepted'", s.company, contractID).Scan(&contractState, &contractEndpoint, &contractSchema); errors.Is(e, pgx.ErrNoRows) {
		report.Reason = "required accepted contract revision is absent"
		report.LifecycleBinding = "failed"
		return report, nil
	} else if e != nil {
		return report, e
	}
	var backendDigest, frontendDigest string
	if e := k.pool.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND verdict='candidate' AND contract='r03-api@1'", s.company, backendID).Scan(&backendDigest); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			report.Reason = "required backend peer candidate artifact is absent"
			report.LifecycleBinding = "failed"
			return report, nil
		}
		return report, e
	}
	if e := k.pool.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND verdict='candidate' AND contract='r03-api@1'", s.company, frontendID).Scan(&frontendDigest); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			report.Reason = "required frontend peer candidate artifact is absent"
			report.LifecycleBinding = "failed"
			return report, nil
		}
		return report, e
	}
	backend, e := readBlob(k.root, s.company, backendDigest)
	if e != nil {
		return report, e
	}
	frontend, e := readBlob(k.root, s.company, frontendDigest)
	if e != nil {
		return report, e
	}
	var obligationState string
	if e := k.pool.QueryRow(ctx, "SELECT o.state FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id WHERE o.company_id=$1 AND t.mission_id=$2 AND o.owner='emp-frontend'", s.company, mission).Scan(&obligationState); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			report.Reason = "required frontend peer obligation is absent"
			report.LifecycleBinding = "failed"
			return report, nil
		}
		return report, e
	}
	report.LifecycleBinding = "passed"
	report.CollaborationCausality = peerVerificationStatus(obligationState == "fulfilled")
	backendCriteria := fixture.CheckPeerBackendCandidate(string(backend), fixture.PeerContractSpec{Endpoint: contractEndpoint, Schema: contractSchema})
	frontendCriteria := fixture.CheckPeerFrontendCandidate(string(frontend), fixture.PeerContractSpec{Endpoint: contractEndpoint, Schema: contractSchema})
	valid := contractState == "accepted" && obligationState == "fulfilled"
	report.PaginationRuntimeBehavior = "not_evaluated"
	report.ResponseShapeCompatibility = "static_only"
	if paginationContract, parseErr := fixture.ParsePaginationBehaviorContract(contractSchema); parseErr == nil {
		backendReport, runErr := VerifyPaginationBackendCandidateSourceV2(ctx, filepath.Join(k.root, ".pagination-runtime"), string(backend), paginationContract)
		if runErr != nil {
			return report, runErr
		}
		frontendReport, runErr := VerifyFrontendConsumptionCandidateSource(ctx, filepath.Join(k.root, ".pagination-runtime"), string(frontend))
		if runErr != nil {
			return report, runErr
		}
		report.PaginationReport = &backendReport
		report.FrontendConsumptionReport = &frontendReport
		report.PaginationRuntimeBehavior = peerVerificationStatus(backendReport.Passed && frontendReport.Passed)
		report.ResponseShapeCompatibility = peerVerificationStatus(frontendReport.Passed)
		valid = valid && backendReport.Passed && frontendReport.Passed
	} else {
		valid = valid && fixture.CandidateCriteriaPassed(backendCriteria) && fixture.CandidateCriteriaPassed(frontendCriteria)
	}
	report.Passed = valid
	if !valid {
		report.Reason = "peer integration contract or causal application evidence missing"
	}
	_, _ = k.TXWrite(ctx, s, nil, "verify-integration-"+integrationID, "integration.verify", report, func(tx pgx.Tx) (Receipt, error) {
		state := "failed"
		if report.Passed {
			state = "passed"
		}
		_, e := tx.Exec(ctx, "UPDATE integration_candidates SET state=$3 WHERE company_id=$1 AND id=$2", s.company, integrationID, state)
		return Receipt{ID: integrationID, Status: state}, e
	})
	return report, nil
}

func peerVerificationStatus(passed bool) string {
	if passed {
		return "passed"
	}
	return "failed"
}
