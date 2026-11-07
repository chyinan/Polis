// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/fixture"
)

// PeerEmployeeTools is the narrow mediated tool surface for the R0.3A
// employees. It contains no hidden verifier, no Planner relay, and no
// controller-owned acceptance operation.
type PeerEmployeeTools struct {
	Kernel  *Kernel
	Binding Binding
	Role    string
	Initial bool
}

type peerWorkspaceReplaceArgs struct {
	ExpectedDigest string `json:"expected_digest"`
	Content        string `json:"content"`
}

func (t PeerEmployeeTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	key := "peer-tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
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

func (t PeerEmployeeTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	k, b := t.Kernel, t.Binding
	switch name {
	case "work_current", "context_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		view, e := k.PeerWorkCurrent(ctx, b)
		return ToolResult{Data: view}, e
	case "workspace_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		w, e := k.Workspace(ctx, b)
		return ToolResult{Data: w}, e
	case "workspace_replace":
		var args peerWorkspaceReplaceArgs
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		r, e := k.TXReplace(ctx, b, key, args.ExpectedDigest, args.Content)
		return ToolResult{Receipt: &r}, e
	case "contract_propose":
		if t.Role != "peer_backend" {
			return ToolResult{}, core.Denied
		}
		var args PeerContractProposal
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		revision, e := k.TXProposePeerContract(ctx, b, h.Task, args, key)
		return ToolResult{Data: revision}, e
	case "contract_accept":
		if t.Role != "peer_backend" {
			return ToolResult{}, core.Denied
		}
		var args struct {
			RevisionID string `json:"revision_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXAcceptPeerContract(ctx, b, args.RevisionID, key)
	case "contract_read":
		var args struct {
			RevisionID string `json:"revision_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		revision, e := k.PeerContractRead(ctx, b, args.RevisionID)
		return ToolResult{Data: revision}, e
	case "collab_send":
		if t.Role != "peer_backend" {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ToEmployeeID       string `json:"to_employee_id"`
			ToTaskID           string `json:"to_task_id"`
			ContractRevisionID string `json:"contract_revision_id"`
			Body               string `json:"body"`
			Actionable         bool   `json:"actionable"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{Data: PeerToolRejection{ReasonCode: "target_fields_required", ActionableSummary: "submit to_employee_id and to_task_id returned by work_current; to_task is not accepted.", ExpectedReferenceTypes: []string{"to_employee_id", "to_task_id", "contract_revision_id"}, TransactionOutcome: "not_started"}, Error: string(core.Malformed)}, nil
		}
		if args.ToEmployeeID == "" || args.ToTaskID == "" || args.ContractRevisionID == "" {
			return ToolResult{Data: PeerToolRejection{ReasonCode: "target_fields_required", ActionableSummary: "submit non-empty to_employee_id, to_task_id and contract_revision_id from the current peer context.", ExpectedReferenceTypes: []string{"to_employee_id", "to_task_id", "contract_revision_id"}, TransactionOutcome: "not_started"}, Error: string(core.Malformed)}, nil
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		message, e := k.TXPeerSend(ctx, b, PeerSendInput{FromTask: h.Task.ID, ToEmployeeID: args.ToEmployeeID, ToTaskID: args.ToTaskID, ContractRevisionID: args.ContractRevisionID, Body: args.Body, Actionable: args.Actionable}, key)
		if e != nil {
			var policyErr peerToolError
			if errors.As(e, &policyErr) {
				return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}, nil
			}
		}
		return ToolResult{Data: message}, e
	case "collab_inbox":
		if t.Role != "peer_frontend" {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		inbox, e := k.PeerInbox(ctx, b)
		return ToolResult{Data: inbox}, e
	case "collab_ack":
		if t.Role != "peer_frontend" {
			return ToolResult{}, core.Denied
		}
		var args struct {
			MessageID string `json:"message_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXPeerAck(ctx, b, args.MessageID, key)
	case "collab_apply":
		if t.Role != "peer_frontend" {
			return ToolResult{}, core.Denied
		}
		args, rejection, code := parsePeerApplyRequest(raw)
		if code != "" {
			return ToolResult{Data: k.enrichPeerToolRejection(ctx, b, rejection), Error: code.Error()}, nil
		}
		r, e := k.TXPeerApply(ctx, b, args, key)
		if e != nil {
			var policyErr peerToolError
			if errors.As(e, &policyErr) {
				return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}, nil
			}
			return ToolResult{}, e
		}
		return ToolResult{Receipt: &r}, nil
	case "workspace_check":
		report, e := k.TXPeerCheck(ctx, b, key)
		return ToolResult{Data: report, Receipt: report.Receipt}, e
	case "work_checkpoint":
		checkpoint, rejection, code := parsePeerCheckpointRequest(raw)
		if code != "" {
			return ToolResult{Data: k.enrichPeerToolRejection(ctx, b, rejection), Error: code.Error()}, nil
		}
		if checkpoint.Summary == "" || len(checkpoint.Facts) == 0 || len(checkpoint.Decisions) == 0 || len(checkpoint.Rejected) == 0 || len(checkpoint.EvidenceRefs) == 0 {
			return ToolResult{}, core.Malformed
		}
		r, e := k.TXCheckpoint(ctx, b, key, checkpoint)
		if e != nil {
			var policyErr peerToolError
			if errors.As(e, &policyErr) {
				return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}, nil
			}
		}
		return ToolResult{Receipt: &r}, e
	case "artifact_submit":
		if t.Initial {
			return ToolResult{}, core.Denied
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		if !HasQualifiedCheckpoint(h.Checkpoints, h.Workspace) {
			return ToolResult{}, core.Denied
		}
		r, e := k.TXSubmit(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		return ToolResult{Receipt: &r}, e
	case "obligation_resolve":
		if t.Role != "peer_frontend" {
			return ToolResult{}, core.Denied
		}
		if t.Initial {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ObligationID string `json:"obligation_id"`
			ArtifactID   string `json:"artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXPeerResolve(ctx, b, args.ObligationID, args.ArtifactID, key)
	default:
		return ToolResult{}, core.Denied
	}
}

// enrichPeerToolRejection only exposes current IDs/revisions that the worker
// is already authorized to observe. It never exposes checker internals.
func (k *Kernel) enrichPeerToolRejection(ctx context.Context, b Binding, rejection PeerToolRejection) PeerToolRejection {
	h, err := k.PeerHandover(ctx, b)
	if err != nil {
		return rejection
	}
	if h.ObligationState == "pending" || h.ObligationState == "observed" || h.ObligationState == "applied" {
		rejection.CurrentObligationID = h.ObligationID
	}
	rejection.CurrentWorkspaceRevision = h.WorkspaceRevision
	if h.TaskID != "" {
		_ = k.pool.QueryRow(ctx, "SELECT id FROM contract_revisions WHERE company_id=$1 AND mission_id=(SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2) AND state='accepted' ORDER BY revision DESC LIMIT 1", b.scope.company, h.TaskID).Scan(&rejection.CurrentContractRevision)
	}
	return rejection
}

// PeerCheckReceipt exposes only public contract criteria; hidden verifier
// implementation and private test details never cross this boundary.
type PeerCheckReceipt struct {
	AcceptanceCheckerRevision string               `json:"acceptance_checker_revision"`
	Passed                    bool                 `json:"passed"`
	BindingCheckApplied       bool                 `json:"binding_check_applied"`
	PublicBindingPassed       bool                 `json:"public_binding_passed"`
	PaginationBehaviorChecked bool                 `json:"pagination_behavior_checked"`
	PaginationBehaviorPassed  bool                 `json:"pagination_behavior_passed"`
	BindingContractRevision   string               `json:"binding_contract_revision,omitempty"`
	BehaviorVerifierRevision  string               `json:"behavior_verifier_revision,omitempty"`
	Criteria                  []PeerCheckCriterion `json:"criteria"`
	CheckReceiptID            string               `json:"check_receipt_id"`
	Digest                    string               `json:"digest"`
	Phase                     string               `json:"phase"`
	RelevantContractRevision  string               `json:"relevant_contract_revision"`
	RelevantWorkspaceRevision int64                `json:"relevant_workspace_revision"`
	Receipt                   *Receipt             `json:"-"`
}

type PeerCheckCriterion struct {
	CriterionID               string                          `json:"criterion_id"`
	Passed                    bool                            `json:"passed"`
	PublicReasonCode          string                          `json:"public_reason_code"`
	ActionableSummary         string                          `json:"actionable_summary"`
	ParserDiagnostics         []fixture.ParserDiagnostic      `json:"parser_diagnostics,omitempty"`
	ExpectedPublicShape       map[string]string               `json:"expected_public_shape,omitempty"`
	ActualShapeSummary        string                          `json:"actual_shape_summary,omitempty"`
	MissingRequiredFields     []string                        `json:"missing_required_fields,omitempty"`
	TypeMismatches            []fixture.PublicTypeMismatch    `json:"type_mismatches,omitempty"`
	UnexpectedFields          []string                        `json:"unexpected_fields,omitempty"`
	BindingContractRevision   string                          `json:"binding_contract_revision,omitempty"`
	BehaviorVerifierRevision  string                          `json:"behavior_verifier_revision,omitempty"`
	ExpectedRepresentation    string                          `json:"expected_representation,omitempty"`
	ExpectedPublicBehavior    string                          `json:"expected_public_behavior,omitempty"`
	ActualObservedBehavior    string                          `json:"actual_observed_behavior,omitempty"`
	FailingStep               string                          `json:"failing_step,omitempty"`
	ExpectedPublicBinding     *fixture.BackendBindingContract `json:"expected_public_binding,omitempty"`
	ActualPublicBinding       *fixture.BackendBindingActual   `json:"actual_public_binding,omitempty"`
	ContractRevisionID        string                          `json:"contract_revision_id"`
	CheckReceiptID            string                          `json:"check_receipt_id"`
	RelevantContractRevision  string                          `json:"relevant_contract_revision"`
	RelevantWorkspaceRevision int64                           `json:"relevant_workspace_revision"`
}

func (k *Kernel) TXPeerCheck(ctx context.Context, b Binding, key string) (PeerCheckReceipt, error) {
	var report PeerCheckReceipt
	w, e := k.Workspace(ctx, b)
	if e != nil {
		return report, e
	}
	var kind, mission, contractID, contractEndpoint, contractSchema string
	if e = k.pool.QueryRow(ctx, "SELECT t.kind,t.mission_id FROM tasks t JOIN worker_sessions s ON s.company_id=t.company_id AND s.task_id=t.id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&kind, &mission); e != nil {
		return report, e
	}
	if e = k.pool.QueryRow(ctx, "SELECT id,endpoint,schema::text FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND state='accepted' ORDER BY revision DESC LIMIT 1", b.scope.company, mission).Scan(&contractID, &contractEndpoint, &contractSchema); e != nil {
		return report, e
	}
	report.Digest = w.Digest
	report.Phase = "full"
	if paginationContract, parseErr := fixture.ParsePaginationBehaviorContract(contractSchema); parseErr == nil {
		report.Phase = "full"
		if kind == "peer_backend" {
			bindingReport := fixture.CheckBackendPublicBindingV2(w.Content)
			report.BindingCheckApplied = true
			report.PublicBindingPassed = bindingReport.Passed
			report.PaginationBehaviorChecked = true
			report.BindingContractRevision = bindingReport.ContractRevision
			report.BehaviorVerifierRevision = fixture.PeerPaginationBehaviorVerifierRevisionV2
			report.Criteria = append(report.Criteria, bindingPeerCheckCriterion(bindingReport, contractID, w.Revision, ""))
			if !bindingReport.Passed {
				report.PaginationBehaviorPassed = false
			} else {
				var behaviorReport fixture.PaginationBehaviorReport
				behaviorReport, e = VerifyPaginationBackendCandidateSourceV2(ctx, filepath.Join(k.root, ".pagination-runtime"), w.Content, paginationContract)
				if e != nil {
					return report, e
				}
				report.PaginationBehaviorPassed = behaviorReport.Passed
				report.BehaviorVerifierRevision = behaviorReport.VerifierRevision
				report.Criteria = append(report.Criteria, paginationPeerCheckCriteria(behaviorReport.Criteria, contractID, w.Revision, "")...)
			}
		} else if kind == "peer_frontend" {
			consumptionReport, verifyErr := VerifyFrontendConsumptionCandidateSource(ctx, filepath.Join(k.root, ".pagination-runtime"), w.Content)
			if verifyErr != nil {
				return report, verifyErr
			}
			report.BindingCheckApplied = true
			binding := fixture.CheckFrontendPublicBindingV1(w.Content)
			report.PublicBindingPassed = binding.Passed
			report.PaginationBehaviorChecked = true
			report.PaginationBehaviorPassed = consumptionReport.Passed
			report.BehaviorVerifierRevision = fixture.FrontendPaginationBehaviorVerifierRevision
			report.BindingContractRevision = fixture.FrontendBindingContractRevision
			report.Criteria = frontendConsumptionPeerCheckCriteria(consumptionReport, contractID, w.Revision, "")
		}
	} else if kind == "peer_backend" {
		criteria := fixture.CheckPeerBackendCandidate(w.Content, fixture.PeerContractSpec{Endpoint: contractEndpoint, Schema: contractSchema})
		report.Criteria = peerCheckCriteria(criteria, contractID, w.Revision, "")
	} else if kind == "peer_frontend" {
		criteria := fixture.CheckPeerFrontendCandidate(w.Content, fixture.PeerContractSpec{Endpoint: contractEndpoint, Schema: contractSchema})
		report.Criteria = peerCheckCriteria(criteria, contractID, w.Revision, "")
	}
	report.Passed = peerCriteriaPassed(report.Criteria)
	report.AcceptanceCheckerRevision = core.PeerAcceptanceCheckerRevision
	report.RelevantContractRevision, report.RelevantWorkspaceRevision = contractID, w.Revision
	r, e := k.TXWrite(ctx, b.scope, &b, key, "workspace.check", w.Digest, func(tx pgx.Tx) (Receipt, error) {
		var current bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_workspaces w JOIN worker_sessions s ON s.company_id=w.company_id AND s.task_id=w.task_id JOIN contract_revisions c ON c.company_id=w.company_id AND c.id=$5 AND c.state='accepted' WHERE s.company_id=$1 AND s.id=$2 AND w.digest=$3 AND w.revision=$4)`, b.scope.company, b.session, w.Digest, w.Revision, contractID).Scan(&current); err != nil {
			return Receipt{}, err
		}
		if !current {
			return Receipt{}, core.Denied
		}
		id := newID()
		report.CheckReceiptID = id
		for i := range report.Criteria {
			report.Criteria[i].CheckReceiptID = id
		}
		body, e := json.Marshal(report)
		if e != nil {
			return Receipt{}, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO worker_checks VALUES($1,$2,$3,$4,$5,$6,$7)", b.scope.company, id, b.session, w.Digest, report.Phase, report.Passed, body)
		return Receipt{ID: id, Status: "persisted"}, e
	})
	if e == nil {
		// Idempotent replay returns the original evidence, never newly computed
		// criteria under the same receipt identity.
		var stored []byte
		e = k.pool.QueryRow(ctx, "SELECT report FROM worker_checks WHERE company_id=$1 AND id=$2", b.scope.company, r.ID).Scan(&stored)
		if e != nil {
			return report, e
		}
		if e = json.Unmarshal(stored, &report); e != nil {
			return report, e
		}
		report.Receipt = &r
	}
	return report, e
}

func frontendConsumptionPeerCheckCriteria(report fixture.FrontendConsumptionReport, contractID string, workspaceRevision int64, receiptID string) []PeerCheckCriterion {
	criteria := make([]PeerCheckCriterion, 0, len(report.Criteria))
	for _, criterion := range report.Criteria {
		criteria = append(criteria, PeerCheckCriterion{
			CriterionID:               criterion.CriterionID,
			Passed:                    criterion.Passed,
			PublicReasonCode:          criterion.ReasonCode,
			ActionableSummary:         criterion.Expected + "; actual=" + criterion.Actual + "; failing_step=" + criterion.FailingStep,
			ExpectedPublicBehavior:    criterion.Expected,
			ActualObservedBehavior:    criterion.Actual,
			FailingStep:               criterion.FailingStep,
			BehaviorVerifierRevision:  report.VerifierRevision,
			ContractRevisionID:        contractID,
			CheckReceiptID:            receiptID,
			RelevantContractRevision:  contractID,
			RelevantWorkspaceRevision: workspaceRevision,
		})
	}
	return criteria
}

func peerCheckCriteria(criteria []fixture.PeerCandidateCriterion, contractID string, workspaceRevision int64, receiptID string) []PeerCheckCriterion {
	out := make([]PeerCheckCriterion, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, PeerCheckCriterion{
			CriterionID: criterion.CriterionID, Passed: criterion.Passed, PublicReasonCode: criterion.PublicReasonCode,
			ActionableSummary: criterion.ActionableSummary, ParserDiagnostics: criterion.ParserDiagnostics,
			ExpectedPublicShape: criterion.ExpectedPublicShape, ActualShapeSummary: criterion.ActualShapeSummary,
			MissingRequiredFields: criterion.MissingRequiredFields, TypeMismatches: criterion.TypeMismatches,
			UnexpectedFields: criterion.UnexpectedFields, ContractRevisionID: contractID, CheckReceiptID: receiptID,
			RelevantContractRevision: contractID, RelevantWorkspaceRevision: workspaceRevision,
		})
	}
	return out
}

func paginationPeerCheckCriteria(criteria []fixture.PaginationBehaviorCriterion, contractID string, workspaceRevision int64, receiptID string) []PeerCheckCriterion {
	out := make([]PeerCheckCriterion, 0, len(criteria))
	for _, criterion := range criteria {
		reasonCode := criterion.ReasonCode
		if reasonCode == "" {
			reasonCode = "pagination_behavior_matches_public_contract"
			if !criterion.Passed {
				reasonCode = "pagination_behavior_mismatch"
			}
		}
		out = append(out, PeerCheckCriterion{
			CriterionID: criterion.CriterionID, Passed: criterion.Passed, PublicReasonCode: reasonCode,
			ActionableSummary: criterion.Reason, ContractRevisionID: contractID, CheckReceiptID: receiptID,
			BehaviorVerifierRevision: criterion.VerifierRevision,
			ExpectedRepresentation:   criterion.ExpectedRepresentation,
			RelevantContractRevision: contractID, RelevantWorkspaceRevision: workspaceRevision,
		})
	}
	return out
}

func bindingPeerCheckCriterion(report fixture.BackendBindingReport, contractID string, workspaceRevision int64, receiptID string) PeerCheckCriterion {
	return PeerCheckCriterion{
		CriterionID:               "public_binding_check",
		Passed:                    report.Passed,
		PublicReasonCode:          report.ReasonCode,
		ActionableSummary:         report.ActionableSummary,
		ParserDiagnostics:         report.ParserDiagnostics,
		BindingContractRevision:   report.ContractRevision,
		ExpectedPublicBinding:     &report.ExpectedBinding,
		ActualPublicBinding:       &report.ActualBinding,
		ContractRevisionID:        contractID,
		CheckReceiptID:            receiptID,
		RelevantContractRevision:  contractID,
		RelevantWorkspaceRevision: workspaceRevision,
	}
}

func peerCriteriaPassed(criteria []PeerCheckCriterion) bool {
	if len(criteria) == 0 {
		return false
	}
	for _, criterion := range criteria {
		if !criterion.Passed {
			return false
		}
	}
	return true
}
