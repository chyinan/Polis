// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	maxMissionChangeAssessmentBytes     = 32768
	maxMissionChangeAssessmentText      = 4096
	maxMissionChangeAssessmentList      = 512
	maxMissionChangeAssessmentRevisions = 20
)

type missionChangeTaskAnalysisBasis struct {
	TaskID            string  `json:"taskId"`
	OwnerEmployeeID   string  `json:"ownerEmployeeId"`
	Kind              string  `json:"kind"`
	WorkspaceDigest   *string `json:"workspaceDigest"`
	WorkspaceRevision *int64  `json:"workspaceRevision"`
}

type missionChangePlanningAnalysisBasis struct {
	ChangeRequestID        string                                `json:"changeRequestId"`
	MissionID              string                                `json:"missionId"`
	BaseRequirementsSHA256 string                                `json:"baseRequirementsSha256"`
	ChangeSummary          string                                `json:"changeSummary"`
	ProposedTitle          string                                `json:"proposedTitle"`
	ProposedGoal           string                                `json:"proposedGoal"`
	ProposedAcceptance     any                                   `json:"proposedAcceptanceContract"`
	BlockPreviousResults   bool                                  `json:"blockPreviousResults"`
	InputRevisions         []MissionChangeInputRevision          `json:"inputRevisions"`
	Tasks                  []missionChangeTaskAnalysisBasis      `json:"tasks"`
	Artifacts              []MissionChangeArtifactImpact         `json:"artifacts"`
	ReturnedSnapshots      []MissionChangeTakeoverSnapshotImpact `json:"returnedHumanTakeoverSnapshots"`
}

type missionChangePlanningAssessmentEvidence struct {
	SchemaVersion       string   `json:"schemaVersion"`
	ChangeRequestID     string   `json:"changeRequestId"`
	AnalysisBasisSHA256 string   `json:"analysisBasisSha256"`
	RiskLevel           string   `json:"riskLevel"`
	Summary             string   `json:"summary"`
	AffectedTaskIDs     []string `json:"affectedTaskIds"`
	UnaffectedTaskIDs   []string `json:"unaffectedTaskIds"`
	UncertainTaskIDs    []string `json:"uncertainTaskIds"`
	Questions           []string `json:"questions"`
	RecommendedControls []string `json:"recommendedControls"`
}

func missionChangePlanningAnalysisBasisDigest(request MissionChangeRequest, impact MissionChangeImpact) (string, error) {
	inputs := append([]MissionChangeInputRevision(nil), impact.InputRevisions...)
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].InputID != inputs[j].InputID {
			return inputs[i].InputID < inputs[j].InputID
		}
		return inputs[i].Revision < inputs[j].Revision
	})
	tasks := make([]missionChangeTaskAnalysisBasis, 0, len(impact.Tasks))
	for _, task := range impact.Tasks {
		tasks = append(tasks, missionChangeTaskAnalysisBasis{
			TaskID: task.TaskID, OwnerEmployeeID: task.OwnerEmployeeID, Kind: task.Kind,
			WorkspaceDigest: task.WorkspaceDigest, WorkspaceRevision: task.WorkspaceRevision,
		})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].TaskID < tasks[j].TaskID })
	artifacts := append([]MissionChangeArtifactImpact(nil), impact.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].ArtifactID != artifacts[j].ArtifactID {
			return artifacts[i].ArtifactID < artifacts[j].ArtifactID
		}
		return artifacts[i].Digest < artifacts[j].Digest
	})
	snapshots := append([]MissionChangeTakeoverSnapshotImpact(nil), impact.ReturnedHumanTakeoverSnapshots...)
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].InputID != snapshots[j].InputID {
			return snapshots[i].InputID < snapshots[j].InputID
		}
		if snapshots[i].InputRevision != snapshots[j].InputRevision {
			return snapshots[i].InputRevision < snapshots[j].InputRevision
		}
		return snapshots[i].LeaseID < snapshots[j].LeaseID
	})
	basis := missionChangePlanningAnalysisBasis{
		ChangeRequestID: request.ID, MissionID: request.MissionID, BaseRequirementsSHA256: request.BaseRequirementsSHA256,
		ChangeSummary: request.ChangeSummary, ProposedTitle: request.ProposedTitle, ProposedGoal: request.ProposedGoal,
		ProposedAcceptance: request.ProposedAcceptanceContract, BlockPreviousResults: request.BlockPreviousResults,
		InputRevisions: inputs, Tasks: tasks, Artifacts: artifacts, ReturnedSnapshots: snapshots,
	}
	encoded, err := json.Marshal(basis)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func normalizeMissionChangePlanningAssessment(input MissionChangePlanningAssessmentInput, impact MissionChangeImpact) (MissionChangePlanningAssessmentInput, error) {
	input.Summary = strings.TrimSpace(input.Summary)
	if !core.ValidID(input.ChangeRequestID) || !validTaskInputDigest(input.AnalysisBasisSHA256) ||
		(input.RiskLevel != "low" && input.RiskLevel != "high" && input.RiskLevel != "uncertain") ||
		!utf8.ValidString(input.Summary) || input.Summary == "" || len(input.Summary) > maxMissionChangeAssessmentText {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	var err error
	input.AffectedTaskIDs, err = normalizeAssessmentIDs(input.AffectedTaskIDs)
	if err != nil {
		return MissionChangePlanningAssessmentInput{}, err
	}
	input.UnaffectedTaskIDs, err = normalizeAssessmentIDs(input.UnaffectedTaskIDs)
	if err != nil {
		return MissionChangePlanningAssessmentInput{}, err
	}
	input.UncertainTaskIDs, err = normalizeAssessmentIDs(input.UncertainTaskIDs)
	if err != nil {
		return MissionChangePlanningAssessmentInput{}, err
	}
	if input.AffectedTaskIDs == nil || input.UnaffectedTaskIDs == nil || input.UncertainTaskIDs == nil {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	var errStrings error
	input.Questions, errStrings = normalizeAssessmentTextList(input.Questions, 12, 512)
	if errStrings != nil {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	input.RecommendedControls, errStrings = normalizeAssessmentTextList(input.RecommendedControls, 12, 1024)
	if errStrings != nil {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	if input.RiskLevel == "low" && (len(input.UncertainTaskIDs) != 0 || len(input.Questions) != 0) {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	if input.RiskLevel == "uncertain" && len(input.Questions) == 0 {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	if input.RiskLevel != "low" && len(input.RecommendedControls) == 0 {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	known := make(map[string]struct{}, len(impact.Tasks))
	for _, task := range impact.Tasks {
		known[task.TaskID] = struct{}{}
	}
	classified := make(map[string]struct{}, len(known))
	for _, group := range [][]string{input.AffectedTaskIDs, input.UnaffectedTaskIDs, input.UncertainTaskIDs} {
		for _, taskID := range group {
			if _, exists := known[taskID]; !exists {
				return MissionChangePlanningAssessmentInput{}, core.Malformed
			}
			if _, duplicate := classified[taskID]; duplicate {
				return MissionChangePlanningAssessmentInput{}, core.Malformed
			}
			classified[taskID] = struct{}{}
		}
	}
	if len(classified) != len(known) {
		return MissionChangePlanningAssessmentInput{}, core.Malformed
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > maxMissionChangeAssessmentBytes {
		return MissionChangePlanningAssessmentInput{}, core.TooLarge
	}
	return input, nil
}

func normalizeAssessmentIDs(values []string) ([]string, error) {
	if values == nil || len(values) > maxMissionChangeAssessmentList {
		return nil, core.Malformed
	}
	out := append([]string(nil), values...)
	for _, value := range out {
		if !core.ValidID(value) {
			return nil, core.Malformed
		}
	}
	sort.Strings(out)
	for i := 1; i < len(out); i++ {
		if out[i] == out[i-1] {
			return nil, core.Malformed
		}
	}
	return out, nil
}

func normalizeAssessmentTextList(values []string, maxCount, maxBytes int) ([]string, error) {
	if values == nil || len(values) > maxCount {
		return nil, core.Malformed
	}
	out := make([]string, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if !utf8.ValidString(value) || value == "" || len(value) > maxBytes {
			return nil, core.Malformed
		}
		out[i] = value
	}
	return out, nil
}

func missionChangePlanningAssessmentDigest(input MissionChangePlanningAssessmentInput) (string, error) {
	evidence := missionChangePlanningAssessmentEvidence{
		SchemaVersion: MissionChangePlanningAssessmentSchema, ChangeRequestID: input.ChangeRequestID,
		AnalysisBasisSHA256: input.AnalysisBasisSHA256, RiskLevel: input.RiskLevel, Summary: input.Summary,
		AffectedTaskIDs: input.AffectedTaskIDs, UnaffectedTaskIDs: input.UnaffectedTaskIDs,
		UncertainTaskIDs: input.UncertainTaskIDs, Questions: input.Questions, RecommendedControls: input.RecommendedControls,
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// MissionChangePlanningContext exposes at most the one currently open change
// request to the fixed Planning employee's database-bound active session.
func (k *Kernel) MissionChangePlanningContext(ctx context.Context, binding Binding) (MissionChangePlanningContext, error) {
	if binding.employee != core.EmployeePlanningID || binding.session == "" {
		return MissionChangePlanningContext{}, core.Denied
	}
	handover, err := k.Handover(ctx, binding)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	requests, err := k.MissionChangeRequests(ctx, binding.scope, handover.Task.Mission)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	var selected *MissionChangeRequest
	for i := range requests {
		if requests[i].State != "received" && requests[i].State != "queued" && requests[i].State != "considered" {
			continue
		}
		if selected != nil {
			return MissionChangePlanningContext{}, core.Integrity
		}
		selected = &requests[i]
	}
	if selected == nil {
		return MissionChangePlanningContext{}, core.OutOfScope
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	defer tx.Rollback(ctx)
	basis, err := missionChangeBasisTx(ctx, tx, binding.scope, handover.Task.Mission, false)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	currentRequirementsSHA256, err := missionChangeRequirementsDigest(handover.Task.Mission, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	currentImpact, currentImpactSHA256, err := missionChangeImpactTx(ctx, tx, binding.scope, handover.Task.Mission, currentRequirementsSHA256, basis.InputRevisions)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	selected.BaseRequirementsSHA256 = currentRequirementsSHA256
	selected.Impact = currentImpact
	selected.ImpactSHA256 = currentImpactSHA256
	basisDigest, err := missionChangePlanningAnalysisBasisDigest(*selected, currentImpact)
	if err != nil {
		return MissionChangePlanningContext{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MissionChangePlanningContext{}, err
	}
	return MissionChangePlanningContext{ChangeRequest: *selected, AnalysisBasisSHA256: basisDigest}, nil
}

// TXAssessMissionChangeRequest accepts an immutable bounded analysis only
// from the active Planning WorkerSession bound to the same Mission.
func (k *Kernel) TXAssessMissionChangeRequest(ctx context.Context, binding Binding, input MissionChangePlanningAssessmentInput, key string) (Receipt, error) {
	if binding.employee != core.EmployeePlanningID || binding.session == "" || !core.ValidID(key) || !core.ValidID(input.ChangeRequestID) || !validTaskInputDigest(input.AnalysisBasisSHA256) {
		return Receipt{}, core.Denied
	}
	receipt, err := k.TXWrite(ctx, binding.scope, &binding, key, "mission.change_request.planning_assessed", input, func(tx pgx.Tx) (Receipt, error) {
		var missionID, taskOwner, taskState string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id,t.owner,t.state FROM tasks t WHERE t.company_id=$1 AND t.id=$2`, binding.scope.company, binding.task).Scan(&missionID, &taskOwner, &taskState); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			}
			return Receipt{}, err
		}
		if taskOwner != core.EmployeePlanningID || taskState != "working" {
			return Receipt{}, core.Denied
		}
		request, state, err := missionChangeRequestForUpdate(ctx, tx, binding.scope, missionID, input.ChangeRequestID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "received" && state != "queued" && state != "considered" {
			return Receipt{}, core.ConflictError{Reason: "formal change request is no longer open for Planning analysis", CurrentState: state}
		}
		basis, err := missionChangeBasisTx(ctx, tx, binding.scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		baseDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if baseDigest != request.BaseRequirementsSHA256 {
			return Receipt{}, core.ConflictError{Reason: "requirements or inputs changed after the change request was received", CurrentState: "base_requirements_changed"}
		}
		impact, _, err := missionChangeImpactTx(ctx, tx, binding.scope, missionID, baseDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		currentBasisDigest, err := missionChangePlanningAnalysisBasisDigest(request, impact)
		if err != nil {
			return Receipt{}, err
		}
		if input.AnalysisBasisSHA256 != currentBasisDigest {
			return Receipt{}, core.ConflictError{Reason: "the change request basis changed; read it again before assessing", CurrentState: "planning_basis_changed"}
		}
		normalized, err := normalizeMissionChangePlanningAssessment(input, impact)
		if err != nil {
			return Receipt{}, err
		}
		assessmentSHA256, err := missionChangePlanningAssessmentDigest(normalized)
		if err != nil {
			return Receipt{}, err
		}
		encoded, err := json.Marshal(normalized)
		if err != nil || len(encoded) > maxMissionChangeAssessmentBytes {
			return Receipt{}, core.TooLarge
		}
		var revision int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM mission_change_request_planning_assessments WHERE company_id=$1 AND change_request_id=$2`, binding.scope.company, input.ChangeRequestID).Scan(&revision); err != nil {
			return Receipt{}, err
		}
		if revision > maxMissionChangeAssessmentRevisions {
			return Receipt{}, core.TooLarge
		}
		assessmentID := newID()
		var createdAt time.Time
		if err = tx.QueryRow(ctx, `INSERT INTO mission_change_request_planning_assessments(company_id,change_request_id,assessment_id,revision,analysis_basis_sha256,assessment_sha256,assessment,worker_session_id,worker_task_id,worker_epoch)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at`, binding.scope.company, input.ChangeRequestID, assessmentID, revision, currentBasisDigest, assessmentSHA256, encoded, binding.session, binding.task, binding.epoch).Scan(&createdAt); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, binding.scope, "mission.change_request.planning_assessed", map[string]any{
			"mission_id": missionID, "change_request_id": input.ChangeRequestID, "assessment_id": assessmentID,
			"assessment_revision": revision, "analysis_basis_sha256": currentBasisDigest, "assessment_sha256": assessmentSHA256,
			"risk_level": normalized.RiskLevel, "worker_session_id": binding.session, "worker_task_id": binding.task,
			"worker_epoch": binding.epoch, "created_at": createdAt.UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: assessmentID, Status: "planning_assessed", Revision: revision}, nil
	})
	return receipt, err
}

type missionChangeAssessmentQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func missionChangePlanningAssessmentForTX(ctx context.Context, tx missionChangeAssessmentQueryer, scope Scope, request MissionChangeRequest, impact MissionChangeImpact) (*MissionChangePlanningAssessment, error) {
	var assessmentID, analysisBasisSHA256, assessmentSHA256, workerSessionID, workerTaskID string
	var revision, workerEpoch int64
	var raw []byte
	var createdAt time.Time
	err := tx.QueryRow(ctx, `SELECT assessment_id,revision,analysis_basis_sha256,assessment_sha256,assessment,worker_session_id,worker_task_id,worker_epoch,created_at
FROM mission_change_request_planning_assessments WHERE company_id=$1 AND change_request_id=$2 ORDER BY revision DESC LIMIT 1`, scope.company, request.ID).Scan(
		&assessmentID, &revision, &analysisBasisSHA256, &assessmentSHA256, &raw, &workerSessionID, &workerTaskID, &workerEpoch, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var input MissionChangePlanningAssessmentInput
	if json.Unmarshal(raw, &input) != nil || input.ChangeRequestID != request.ID || input.AnalysisBasisSHA256 != analysisBasisSHA256 {
		return nil, core.Integrity
	}
	basisSHA256, err := missionChangePlanningAnalysisBasisDigest(request, impact)
	if err != nil {
		return nil, err
	}
	status := "current"
	validationImpact := impact
	if basisSHA256 != analysisBasisSHA256 {
		status = "stale"
		validationImpact = MissionChangeImpact{Tasks: []MissionChangeTaskImpact{}}
		for _, taskID := range append(append(append([]string{}, input.AffectedTaskIDs...), input.UnaffectedTaskIDs...), input.UncertainTaskIDs...) {
			validationImpact.Tasks = append(validationImpact.Tasks, MissionChangeTaskImpact{TaskID: taskID})
		}
	}
	normalized, err := normalizeMissionChangePlanningAssessment(input, validationImpact)
	if err != nil {
		return nil, core.Integrity
	}
	calculatedSHA256, err := missionChangePlanningAssessmentDigest(normalized)
	if err != nil || calculatedSHA256 != assessmentSHA256 {
		return nil, core.Integrity
	}
	return &MissionChangePlanningAssessment{
		SchemaVersion: MissionChangePlanningAssessmentSchema, AssessmentID: assessmentID, Revision: revision, Status: status,
		AnalysisBasisSHA256: analysisBasisSHA256, AssessmentSHA256: assessmentSHA256, RiskLevel: normalized.RiskLevel,
		Summary: normalized.Summary, AffectedTaskIDs: normalized.AffectedTaskIDs, UnaffectedTaskIDs: normalized.UnaffectedTaskIDs,
		UncertainTaskIDs: normalized.UncertainTaskIDs, Questions: normalized.Questions, RecommendedControls: normalized.RecommendedControls,
		WorkerSessionID: workerSessionID, WorkerTaskID: workerTaskID, WorkerEpoch: workerEpoch, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}, nil
}
