// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"polis/internal/core"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

type ToolResult struct {
	Receipt *Receipt `json:"receipt,omitempty"`
	Data    any      `json:"data,omitempty"`
	Error   string   `json:"error,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}
type CheckRunner interface {
	Check(string, string) (runner.Report, error)
}

// EmployeeTools is constructed by the trusted adapter. No payload can select
// company, employee, attempt, epoch or task. callID comes from the native bridge.
type EmployeeTools struct {
	Kernel                    *Kernel
	Binding                   Binding
	Checker                   CheckRunner
	Phase                     string
	ReadOnly                  bool
	ProductSurface            bool
	DirectMessagingSurface    bool
	SkillLoadSurface          bool
	GuidanceSurface           bool
	ControlledMCPSurface      bool
	ControlledStdioMCPEnabled bool
	StreamableHTTPMCPEnabled  bool
}

func strictArgs(raw []byte, v any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return core.Malformed
	}
	if len(raw) > 8192 {
		return core.TooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return core.Malformed
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return core.Malformed
	}
	return nil
}
func (t EmployeeTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	key := "tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
	budget, budgetErr := t.Kernel.TXConsumeToolCall(ctx, t.Binding, key+"-budget", name)
	if budgetErr != nil {
		if errors.Is(budgetErr, core.StaleEpoch) || errors.Is(budgetErr, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, budgetErr.Error())
		}
		var known core.Code
		if errors.As(budgetErr, &known) {
			return ToolResult{Data: budget, Error: known.Error()}
		}
		return ToolResult{Data: budget, Error: "OUTCOME_UNKNOWN", Detail: budgetErr.Error()}
	}
	result, e := t.call(ctx, name, key, raw)
	if e != nil {
		if errors.Is(e, core.StaleEpoch) || errors.Is(e, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, e.Error())
		}
		var policyErr peerToolError
		if errors.As(e, &policyErr) {
			return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}
		}
		var known core.Code
		if errors.As(e, &known) {
			return ToolResult{Error: known.Error()}
		}
		var pathErr *PaginationRuntimePathError
		if errors.As(e, &pathErr) {
			return ToolResult{Error: "INFRASTRUCTURE_FAILED", Detail: e.Error()}
		}
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: e.Error()}
	}
	return result
}
func (t EmployeeTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	k, b := t.Kernel, t.Binding
	switch name {
	case "work_current", "context_read", "workspace_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if e == nil && name == "work_current" && t.ProductSurface && t.DirectMessagingSurface {
			h.DirectMessageTargets, h.DirectMessageTargetsTruncated, e = k.ProductDirectMessageTargets(ctx, b)
		}
		if !t.SkillLoadSurface {
			h.SkillCatalog = nil
			h.SkillCatalogTruncated = false
			h.SkillLoads = nil
			h.SkillLoadsTruncated = false
		}
		if t.ControlledMCPSurface && name != "workspace_read" {
			h.MCPToolSets, e = k.BoundMCPToolSets(ctx, b, t.ControlledStdioMCPEnabled, t.StreamableHTTPMCPEnabled)
			if e != nil {
				return ToolResult{}, e
			}
		}
		if name == "workspace_read" {
			return ToolResult{Data: h.Workspace}, e
		}
		return ToolResult{Data: h}, e
	case "collab_send":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args ProductDirectMessageInput
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		message, e := k.TXProductDirectMessage(ctx, b, args, key)
		return ToolResult{Data: message}, e
	case "collab_inbox":
		if !t.ProductSurface || !t.DirectMessagingSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		inbox, e := k.ProductDirectInbox(ctx, b)
		return ToolResult{Data: inbox}, e
	case "collab_ack":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			MessageID string `json:"message_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXProductDirectMessageAck(ctx, b, args.MessageID, key)
	case "collab_apply":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args ProductDirectApplyRequest
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.TXProductDirectApply(ctx, b, args, key)
		return ToolResult{Receipt: &receipt}, e
	case "obligation_resolve":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ObligationID string `json:"obligation_id"`
			ArtifactID   string `json:"artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXProductDirectResolve(ctx, b, args.ObligationID, args.ArtifactID, key)
	case "guidance_read":
		if !t.ProductSurface || !t.GuidanceSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		inbox, e := k.ReadPendingOperatorInstructions(ctx, b)
		return ToolResult{Data: inbox}, e
	case "guidance_respond":
		if !t.ProductSurface || !t.GuidanceSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			InstructionID string                     `json:"instruction_id"`
			Outcome       OperatorInstructionOutcome `json:"outcome"`
			Summary       string                     `json:"summary"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.TXRespondToOperatorInstruction(ctx, b, OperatorInstructionResponseInput{InstructionID: args.InstructionID, Outcome: args.Outcome, Summary: args.Summary}, key)
		return ToolResult{Receipt: &receipt, Data: receipt}, e
	case "skills_load":
		if !t.ProductSurface || !t.SkillLoadSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			SkillID      string `json:"skill_id"`
			RelativePath string `json:"relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, document, e := k.TXLoadBoundReadOnlySkill(ctx, b, key, SkillLoadRequest{SkillID: args.SkillID, RelativePath: args.RelativePath})
		return ToolResult{Receipt: &receipt, Data: document}, e
	case "workspace_replace":
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ExpectedDigest   string `json:"expected_digest"`
			ExpectedRevision int64  `json:"expected_revision"`
			Content          string `json:"content"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		var r Receipt
		var e error
		if t.ProductSurface {
			r, e = k.TXReplaceAtRevision(ctx, b, key, args.ExpectedDigest, args.ExpectedRevision, args.Content)
		} else {
			r, e = k.TXReplace(ctx, b, key, args.ExpectedDigest, args.Content)
		}
		return ToolResult{Receipt: &r}, e
	case "workspace_check":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if t.ProductSurface {
			r, report, e := k.TXProductWorkspaceCheck(ctx, b, key)
			return ToolResult{Receipt: &r, Data: report}, e
		}
		w, e := k.Workspace(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		report, e := t.Checker.Check(w.Content, t.Phase)
		if e != nil {
			return ToolResult{}, e
		}
		r, e := k.TXWrite(ctx, b.scope, &b, key, "workspace.check", w.Digest, func(tx pgx.Tx) (Receipt, error) {
			id := newID()
			body, e := json.Marshal(report)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "INSERT INTO worker_checks VALUES($1,$2,$3,$4,$5,$6,$7)", b.scope.company, id, b.session, w.Digest, t.Phase, report.Passed, body)
			return Receipt{ID: id, Status: "persisted"}, e
		})
		return ToolResult{Receipt: &r, Data: report}, e
	case "work_checkpoint":
		if t.ProductSurface {
			checkpoint, rejection, code := parseProductCheckpointRequest(raw)
			if code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
			r, e := k.TXProductCheckpoint(ctx, b, key, checkpoint)
			if e != nil {
				var policyErr peerToolError
				if errors.As(e, &policyErr) {
					return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}, nil
				}
				if rejection, code := productCheckpointCodeRejection(e); code != "" {
					return ToolResult{Data: rejection, Error: code.Error()}, nil
				}
			}
			return ToolResult{Receipt: &r}, e
		}
		var args Checkpoint
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if args.Summary == "" || len(args.Facts) == 0 || len(args.Decisions) == 0 || len(args.Rejected) == 0 || len(args.EvidenceRefs) == 0 {
			return ToolResult{}, core.Malformed
		}
		var r Receipt
		var e error
		if t.ProductSurface {
			r, e = k.TXProductCheckpoint(ctx, b, key, args)
		} else {
			r, e = k.TXCheckpoint(ctx, b, key, args)
		}
		return ToolResult{Receipt: &r}, e
	case "task_submit":
		if !t.ProductSurface {
			return ToolResult{}, core.Denied
		}
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		if rejection, code := ParseProductTaskDeliveryRequest(raw); code != "" {
			return ToolResult{Data: rejection, Error: code.Error()}, nil
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		r, delivery, e := k.TXSubmitTaskDelivery(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		if e != nil {
			if rejection, code := productDeliveryCodeRejection(e); code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
		}
		return ToolResult{Receipt: &r, Data: delivery}, e
	case "artifact_submit":
		if t.ProductSurface {
			if t.ReadOnly {
				return ToolResult{}, core.Denied
			}
			if rejection, code := ParseProductTaskDeliveryRequest(raw); code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
			h, e := k.Handover(ctx, b)
			if e != nil {
				return ToolResult{}, e
			}
			r, delivery, e := k.TXSubmitTaskDelivery(ctx, b, h.Task, key, []byte(h.Workspace.Content))
			if e != nil {
				if rejection, code := productDeliveryCodeRejection(e); code != "" {
					return ToolResult{Data: rejection, Error: code.Error()}, nil
				}
			}
			return ToolResult{Receipt: &r, Data: delivery}, e
		}
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		qualified := HasQualifiedCheckpoint(h.Checkpoints, h.Workspace)
		if t.ProductSurface {
			bindingDigest := ""
			if h.Task.ValidationBinding != nil {
				bindingDigest = h.Task.ValidationBinding.ConfigurationDigest
			}
			qualified = HasQualifiedCheckpointForBinding(h.Checkpoints, h.Workspace, bindingDigest)
		}
		if !qualified {
			return ToolResult{}, core.Denied
		}
		var r Receipt
		if t.ProductSurface {
			r, e = k.TXSubmitProduct(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		} else {
			r, e = k.TXSubmit(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		}
		return ToolResult{Receipt: &r}, e
	default:
		return ToolResult{}, core.Denied
	}
}

func productCheckpointCodeRejection(err error) (PeerToolRejection, core.Code) {
	rejection := newCheckpointRejection("checkpoint_not_allowed_in_task_state", "the current authorized Task/session cannot create this checkpoint.")
	switch {
	case errors.Is(err, core.StaleEpoch):
		rejection.ReasonCode = "writer_fenced"
		rejection.ActionableSummary = "the worker session is fenced; use the current authorized writer session."
		rejection.FailingField = "session"
		rejection.ExpectedPublicShape = "active current worker session"
		rejection.ActualCategory = "stale_or_fenced_writer"
	case errors.Is(err, core.Conflict):
		rejection.ReasonCode = "stale_workspace_revision"
		rejection.ActionableSummary = "re-read the current workspace and validation receipt before checkpointing."
		rejection.FailingField = "workspace_revision"
		rejection.ExpectedPublicShape = "current workspace revision and digest"
		rejection.ActualCategory = "stale_workspace"
	case errors.Is(err, core.Denied):
		rejection.FailingField = "task_state"
		rejection.ExpectedPublicShape = "authorized compat Task in working state with an active session"
		rejection.ActualCategory = "checkpoint_not_allowed"
	default:
		return PeerToolRejection{}, ""
	}
	return rejection, core.Denied
}

func (k *Kernel) TXCheckpoint(ctx context.Context, b Binding, key string, c Checkpoint) (Receipt, error) {
	return k.txCheckpoint(ctx, b, key, c, false)
}

func (k *Kernel) txCheckpoint(ctx context.Context, b Binding, key string, c Checkpoint, product bool) (Receipt, error) {
	if c.Kind == "" {
		c.Kind = CheckpointQualified
	}
	if c.Kind != CheckpointProgress && c.Kind != CheckpointQualified {
		return Receipt{}, core.Malformed
	}
	if c.Kind == CheckpointProgress && c.NextAction == "" {
		return Receipt{}, core.Malformed
	}
	if rejection, code := validateEvidenceRefs(c.EvidenceRefs); code != "" {
		return Receipt{}, peerToolError{Code: code, Rejection: rejection}
	}
	return k.TXWrite(ctx, b.scope, &b, key, "work.checkpoint", c, func(tx pgx.Tx) (Receipt, error) {
		var digest string
		var workspaceRevision, epoch int64
		var task, mission string
		var taskKind core.TaskKind
		e := tx.QueryRow(ctx, "SELECT s.task_id,t.mission_id,t.kind,w.digest,w.revision,s.epoch FROM worker_workspaces w JOIN worker_sessions s ON w.company_id=s.company_id AND w.task_id=s.task_id JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&task, &mission, &taskKind, &digest, &workspaceRevision, &epoch)
		if e != nil {
			return Receipt{}, e
		}
		bindingDigest, runnerRevision := "", ""
		if product {
			if taskKind != core.TaskKindCompat {
				return Receipt{}, core.Denied
			}
			currentTaskID, stateErr := k.requireProductTaskWorking(ctx, tx, b)
			if stateErr != nil {
				return Receipt{}, stateErr
			}
			if currentTaskID != task {
				return Receipt{}, core.Denied
			}
			e = tx.QueryRow(ctx, "SELECT configuration_digest,runner_revision FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2", b.scope.company, task).Scan(&bindingDigest, &runnerRevision)
			if errors.Is(e, pgx.ErrNoRows) {
				if c.Kind == CheckpointQualified {
					return Receipt{}, peerCheckpointDenied("validation_not_configured", "a Task without TaskValidationBinding may be explored but cannot be qualified.", "", workspaceRevision)
				}
				bindingDigest, runnerRevision = "", ""
			} else if e != nil {
				return Receipt{}, e
			}
		}
		contractID := ""
		var hasContracts bool
		if e = tx.QueryRow(ctx, "SELECT to_regclass('public.contract_revisions') IS NOT NULL").Scan(&hasContracts); e != nil {
			return Receipt{}, e
		}
		if hasContracts {
			e = tx.QueryRow(ctx, "SELECT id FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND state='accepted' ORDER BY revision DESC LIMIT 1", b.scope.company, mission).Scan(&contractID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return Receipt{}, e
			}
		}
		if c.ContractRevisionID != "" && c.ContractRevisionID != contractID {
			return Receipt{}, core.Conflict
		}
		for _, id := range c.EvidenceRefs {
			var passed bool
			var checkRaw []byte
			e = tx.QueryRow(ctx, "SELECT passed,report FROM worker_checks WHERE company_id=$1 AND id=$2 AND session_id=$3 AND digest=$4", b.scope.company, id, b.session, digest).Scan(&passed, &checkRaw)
			if errors.Is(e, pgx.ErrNoRows) {
				if c.Kind == CheckpointQualified {
					return Receipt{}, peerCheckpointDenied("qualified_checkpoint_requires_check_receipt", "a qualified checkpoint must reference a current workspace_check receipt.", contractID, workspaceRevision)
				}
				_, valid, refErr := peerEvidenceRefKind(ctx, tx, b, id, digest)
				if refErr != nil {
					return Receipt{}, refErr
				}
				if !valid {
					return Receipt{}, peerCheckpointDenied("evidence_ref_invalid", "each evidence_refs[] value must be a persisted receipt for this worker and workspace.", contractID, workspaceRevision)
				}
				continue
			}
			if e != nil {
				return Receipt{}, e
			}
			if c.Kind == CheckpointQualified && !passed {
				return Receipt{}, peerCheckpointDenied("qualified_check_not_passed", "a qualified checkpoint requires a passed workspace_check receipt.", contractID, workspaceRevision)
			}
			if product && c.Kind == CheckpointQualified {
				var check taskvalidation.Result
				if json.Unmarshal(checkRaw, &check) != nil || check.Status != taskvalidation.StatusPass || check.TaskID != task || check.MissionID != mission || check.ConfigurationDigest != bindingDigest || check.WorkspaceDigest != digest || check.WorkspaceRevision != workspaceRevision || check.SessionID != b.session || check.Epoch != epoch {
					return Receipt{}, peerCheckpointDenied("qualified_check_stale", "a qualified product checkpoint requires a passing receipt for the current TaskValidationBinding, session and workspace revision.", contractID, workspaceRevision)
				}
			}
			if c.Kind == CheckpointQualified && contractID != "" {
				var checked PeerCheckReceipt
				if json.Unmarshal(checkRaw, &checked) != nil || checked.RelevantContractRevision != contractID || checked.RelevantWorkspaceRevision != workspaceRevision || checked.AcceptanceCheckerRevision != core.PeerAcceptanceCheckerRevision {
					return Receipt{}, peerCheckpointDenied("qualified_check_stale", "the qualified checkpoint evidence must reference the current contract, workspace revision and checker revision.", contractID, workspaceRevision)
				}
			}
		}
		if c.Kind == CheckpointQualified && len(c.EvidenceRefs) == 0 {
			return Receipt{}, peerCheckpointDenied("missing_evidence_refs", "a qualified checkpoint requires at least one workspace_check receipt.", contractID, workspaceRevision)
		}
		c.WorkspaceDigest = digest
		c.WorkspaceRevision = workspaceRevision
		c.ContractRevisionID = contractID
		if product {
			c.TaskValidationBindingDigest = bindingDigest
			if c.Kind == CheckpointQualified {
				c.AcceptanceCheckerRevision = runnerRevision
				c.ValidationStatus = string(taskvalidation.StatusPass)
			}
		} else {
			c.AcceptanceCheckerRevision = core.PeerAcceptanceCheckerRevision
		}
		c.CheckpointPolicyRevision = core.CheckpointPolicyRevision
		c.ArtifactEligibilityPolicyRevision = core.ArtifactEligibilityPolicyRevision
		c.ContractSupersessionPolicyRevision = core.PeerContractSupersessionPolicyRevision
		c.FinalizationState = "not_qualified"
		if c.Kind == CheckpointQualified {
			c.FinalizationState = "current"
		}
		c.SessionID = b.session
		c.Epoch = epoch
		if hasContracts {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(m.id,''),COALESCE(m.delivery_state,''),o.id,o.state
FROM obligations o
LEFT JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
WHERE o.company_id=$1 AND o.state IN ('pending','observed','applied')
  AND (o.task_id=$2 OR (m.sender=$3 AND m.mission_id=$4))
			ORDER BY o.id LIMIT 1`, b.scope.company, task, b.employee, mission).Scan(&c.PendingMessageID, &c.PendingMessageState, &c.PendingObligationID, &c.PendingObligationState)
		} else {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(m.id,''),o.id,o.state
FROM obligations o
LEFT JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
WHERE o.company_id=$1 AND o.state IN ('pending','observed','applied')
  AND (o.task_id=$2 OR (m.sender=$3 AND m.mission_id=$4))
ORDER BY o.id LIMIT 1`, b.scope.company, task, b.employee, mission).Scan(&c.PendingMessageID, &c.PendingObligationID, &c.PendingObligationState)
		}
		raw, e := json.Marshal(c)
		if e != nil {
			return Receipt{}, e
		}
		if len(raw) > 4096 {
			return Receipt{}, core.TooLarge
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_checkpoints VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, digest, raw)
		return Receipt{ID: id, Status: "persisted"}, e
	})
}
func (k *Kernel) RecordHistorical(ctx context.Context, b Binding, name string, raw []byte, reason string) error {
	if len(raw) > 8192 {
		return core.TooLarge
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.historical", b.session, func(tx pgx.Tx) (Receipt, error) {
		data, e := json.Marshal(struct {
			Name      string
			Arguments json.RawMessage
		}{name, raw})
		if e != nil {
			return Receipt{}, e
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_observations VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, reason, data)
		return Receipt{ID: id, Status: "historical_only"}, e
	})
	return e
}

// VerifyProbe is a trusted control-side, non-author acceptance entry, never a
// dynamic tool. The supplied checker is frozen by the executable, not the model.
func (k *Kernel) VerifyProbe(ctx context.Context, s Scope, artifact, phase string, v CheckRunner) (runner.Report, error) {
	var digest, author, task string
	e := k.pool.QueryRow(ctx, "SELECT digest,author,task_id FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND contract='signed-zero@1'", s.company, artifact).Scan(&digest, &author, &task)
	if e != nil {
		return runner.Report{}, e
	}
	if author == "emp-review" {
		return runner.Report{}, core.Denied
	}
	content, e := readBlob(k.root, s.company, digest)
	if e != nil {
		return runner.Report{}, e
	}
	report, e := v.Check(string(content), phase)
	if e != nil {
		return report, e
	}
	if report.Digest != digest {
		return report, core.Integrity
	}
	_, e = k.TXWrite(ctx, s, nil, newID(), "probe.independent_review", []string{artifact, digest, phase}, func(tx pgx.Tx) (Receipt, error) {
		verdict := "failed"
		if report.Passed {
			verdict = "passed"
		}
		_, e := tx.Exec(ctx, "UPDATE artifacts SET verifier='emp-review',verdict=$3 WHERE company_id=$1 AND id=$2 AND digest=$4", s.company, artifact, verdict, digest)
		if e != nil {
			return Receipt{}, e
		}
		if report.Passed {
			_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", s.company, task)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "UPDATE obligations SET state='fulfilled',evidence_id=$3 WHERE company_id=$1 AND task_id=$2", s.company, task, artifact)
			if e != nil {
				return Receipt{}, e
			}
		}
		return Receipt{ID: artifact, Status: verdict}, nil
	})
	if e != nil {
		return report, e
	}
	if !report.Passed {
		return report, fmt.Errorf("independent acceptance failed")
	}
	return report, nil
}
