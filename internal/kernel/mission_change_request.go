// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
	"polis/internal/taskvalidation"
)

type missionChangeBasis struct {
	State              string
	Title              string
	Goal               string
	AcceptanceContract *taskvalidation.AcceptanceContract
	InputRevisions     []MissionChangeInputRevision
}

type missionChangeInputClone struct {
	InputID        string
	Revision       int64
	TakeoverTaskID string
	SourceKind     string
	DisplayName    string
	MediaType      string
	ByteSize       int64
	ContentDigest  string
	State          string
	ImageWidth     int
	ImageHeight    int
}

func (k *Kernel) TXCreateMissionChangeRequest(ctx context.Context, scope Scope, missionID string, input MissionChangeRequestInput, key string) (MissionChangeRequest, error) {
	if !core.ValidID(missionID) || !core.ValidID(key) || !validChangeRequestText(input.ChangeSummary) || !validChangeRequestText(input.ProposedTitle) || !validChangeRequestText(input.ProposedGoal) {
		return MissionChangeRequest{}, core.Malformed
	}
	requestFingerprint := fingerprint(struct {
		MissionID string
		Input     MissionChangeRequestInput
	}{missionID, input})
	receipt, err := k.TXWrite(ctx, scope, nil, key, "mission.change_request.created", requestFingerprint, func(tx pgx.Tx) (Receipt, error) {
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "active" && basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "formal changes require an active or paused Mission", CurrentState: basis.State}
		}
		input, err = normalizeMissionChangeRequestInput(input, basis.Title, basis.Goal)
		if err != nil {
			return Receipt{}, err
		}
		if input.ProposedAcceptanceContract == nil {
			input.ProposedAcceptanceContract = basis.AcceptanceContract
		}
		if input.ProposedAcceptanceContract == nil {
			return Receipt{}, core.Malformed
		}
		baseDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		openRequest, err := missionHasOpenChangeRequestTx(ctx, tx, scope, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if openRequest {
			return Receipt{}, core.ConflictError{Reason: "this Mission already has a pending formal change request", CurrentState: "change_request_open"}
		}
		impact, impactDigest, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if len(impact.ActiveTaskTakeoverLeases) > 0 {
			return Receipt{}, core.ConflictError{Reason: "return or release the active Task takeover lease before creating a formal change request", CurrentState: "takeover_lease_active"}
		}
		changeRequestID := newID()
		var proposedAcceptanceJSON []byte
		if input.ProposedAcceptanceContract != nil {
			proposedAcceptanceJSON, err = json.Marshal(input.ProposedAcceptanceContract)
			if err != nil {
				return Receipt{}, err
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO mission_change_requests(company_id,change_request_id,mission_id,client_request_id,base_requirements_sha256,change_summary,proposed_title,proposed_goal,proposed_acceptance_contract,block_previous_results)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, scope.company, changeRequestID, missionID, key, baseDigest, input.ChangeSummary, input.ProposedTitle, input.ProposedGoal, proposedAcceptanceJSON, input.BlockPreviousResults); err != nil {
			return Receipt{}, err
		}
		if err = insertMissionChangeImpact(ctx, tx, scope, changeRequestID, 1, impactDigest, impact); err != nil {
			return Receipt{}, err
		}
		if input.BlockPreviousResults {
			if err = insertMissionChangeOutputBlocks(ctx, tx, scope, changeRequestID, impact); err != nil {
				return Receipt{}, err
			}
		}
		state := "received"
		if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, state, int64Pointer(1), nil, "request_received", map[string]any{}); err != nil {
			return Receipt{}, err
		}
		if basis.State == "active" {
			state = "queued"
			if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, state, int64Pointer(1), nil, "awaiting_safe_boundary", map[string]any{}); err != nil {
				return Receipt{}, err
			}
		}
		if err = appendEvent(ctx, tx, scope, "mission.change_request.created", map[string]any{
			"mission_id": missionID, "change_request_id": changeRequestID, "base_requirements_sha256": baseDigest,
			"impact_sha256": impactDigest, "block_previous_results": input.BlockPreviousResults,
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: changeRequestID, Status: state}, nil
	})
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return k.MissionChangeRequest(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TXConsiderMissionChangeRequest(ctx context.Context, scope Scope, missionID, changeRequestID, key string) (MissionChangeRequest, error) {
	return k.txConsiderMissionChangeRequest(ctx, scope, missionID, changeRequestID, "", key, false)
}

// TXConsiderMissionChangeRequestWithAssessment binds operator review to the
// exact Planning assessment the UI displayed.
func (k *Kernel) TXConsiderMissionChangeRequestWithAssessment(ctx context.Context, scope Scope, missionID, changeRequestID, expectedAssessmentSHA256, key string) (MissionChangeRequest, error) {
	if !validTaskInputDigest(expectedAssessmentSHA256) {
		return MissionChangeRequest{}, core.Malformed
	}
	return k.txConsiderMissionChangeRequest(ctx, scope, missionID, changeRequestID, expectedAssessmentSHA256, key, true)
}

func (k *Kernel) txConsiderMissionChangeRequest(ctx context.Context, scope Scope, missionID, changeRequestID, expectedAssessmentSHA256, key string, requireExpectedAssessment bool) (MissionChangeRequest, error) {
	if !core.ValidID(missionID) || !core.ValidID(changeRequestID) || !core.ValidID(key) {
		return MissionChangeRequest{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, key, "mission.change_request.considered", struct{ MissionID, ChangeRequestID, ExpectedAssessmentSHA256 string }{missionID, changeRequestID, expectedAssessmentSHA256}, func(tx pgx.Tx) (Receipt, error) {
		request, state, err := missionChangeRequestForUpdate(ctx, tx, scope, missionID, changeRequestID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "received" && state != "queued" && state != "considered" {
			return Receipt{}, core.ConflictError{Reason: "formal change request is no longer open", CurrentState: state}
		}
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "pause the Mission before considering a formal change", CurrentState: basis.State}
		}
		baseDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if baseDigest != request.BaseRequirementsSHA256 {
			return Receipt{}, core.ConflictError{Reason: "requirements or inputs changed after the request was received", CurrentState: "base_requirements_changed"}
		}
		impact, impactDigest, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if !missionChangeSafeBoundary(impact) {
			return Receipt{}, core.ConflictError{Reason: "a WorkerSession, JobRun, service lease or incomplete input still needs reconciliation", CurrentState: "writers_not_stopped"}
		}
		planningAssessment, err := missionChangePlanningAssessmentForTX(ctx, tx, scope, request, impact)
		if err != nil {
			return Receipt{}, err
		}
		if planningAssessment == nil {
			return Receipt{}, core.ConflictError{Reason: "the fixed Planning role must assess natural-language change impacts before operator review", CurrentState: "planning_assessment_required"}
		}
		if planningAssessment.Status != "current" {
			return Receipt{}, core.ConflictError{Reason: "the Planning assessment no longer matches the current change basis", CurrentState: "planning_assessment_stale"}
		}
		if requireExpectedAssessment && planningAssessment.AssessmentSHA256 != expectedAssessmentSHA256 {
			return Receipt{}, core.ConflictError{Reason: "the Planning assessment changed after operator review loaded it", CurrentState: "planning_assessment_changed"}
		}
		if planningAssessment.RiskLevel != "low" && !request.BlockPreviousResults {
			return Receipt{}, core.ConflictError{Reason: "high-risk or uncertain change scope requires blocking previous results", CurrentState: "previous_results_must_be_blocked"}
		}
		var nextRevision int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM mission_change_request_impacts WHERE company_id=$1 AND change_request_id=$2`, scope.company, changeRequestID).Scan(&nextRevision); err != nil {
			return Receipt{}, err
		}
		if err = insertMissionChangeImpact(ctx, tx, scope, changeRequestID, nextRevision, impactDigest, impact); err != nil {
			return Receipt{}, err
		}
		if request.BlockPreviousResults {
			if err = insertMissionChangeOutputBlocks(ctx, tx, scope, changeRequestID, impact); err != nil {
				return Receipt{}, err
			}
		}
		if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, "considered", int64Pointer(nextRevision), nil, "impact_review_ready", map[string]any{
			"planning_assessment_id":     planningAssessment.AssessmentID,
			"planning_assessment_sha256": planningAssessment.AssessmentSHA256,
			"planning_risk_level":        planningAssessment.RiskLevel,
		}); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "mission.change_request.considered", map[string]any{
			"mission_id": missionID, "change_request_id": changeRequestID, "impact_revision": nextRevision, "impact_sha256": impactDigest,
			"planning_assessment_id":     planningAssessment.AssessmentID,
			"planning_assessment_sha256": planningAssessment.AssessmentSHA256,
			"planning_risk_level":        planningAssessment.RiskLevel,
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: request.ID, Status: "considered"}, nil
	})
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return k.MissionChangeRequest(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TXDeclineMissionChangeRequest(ctx context.Context, scope Scope, missionID, changeRequestID, key string) (MissionChangeRequest, error) {
	if !core.ValidID(missionID) || !core.ValidID(changeRequestID) || !core.ValidID(key) {
		return MissionChangeRequest{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, key, "mission.change_request.declined", struct{ MissionID, ChangeRequestID string }{missionID, changeRequestID}, func(tx pgx.Tx) (Receipt, error) {
		request, state, err := missionChangeRequestForUpdate(ctx, tx, scope, missionID, changeRequestID)
		if err != nil {
			return Receipt{}, err
		}
		if !validMissionChangeRequestTransition(state, "declined") {
			return Receipt{}, core.ConflictError{Reason: "formal change request is no longer open", CurrentState: state}
		}
		if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, "declined", nil, nil, "operator_declined", map[string]any{}); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "mission.change_request.declined", map[string]any{"mission_id": missionID, "change_request_id": changeRequestID}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: request.ID, Status: "declined"}, nil
	})
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return k.MissionChangeRequest(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TXApplyMissionChangeRequest(ctx context.Context, scope Scope, missionID, changeRequestID, expectedImpactSHA256, key string) (MissionChangeRequest, error) {
	if !core.ValidID(missionID) || !core.ValidID(changeRequestID) || !core.ValidID(key) || !validTaskInputDigest(expectedImpactSHA256) {
		return MissionChangeRequest{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, key, "mission.change_request.applied", struct {
		MissionID, ChangeRequestID, ExpectedImpactSHA256 string
	}{missionID, changeRequestID, expectedImpactSHA256}, func(tx pgx.Tx) (Receipt, error) {
		request, state, err := missionChangeRequestForUpdate(ctx, tx, scope, missionID, changeRequestID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "considered" {
			return Receipt{}, core.ConflictError{Reason: "consider the exact impact before applying the change", CurrentState: state}
		}
		var revision int64
		var storedImpactDigest string
		if err = tx.QueryRow(ctx, `SELECT revision,impact_sha256 FROM mission_change_request_impacts WHERE company_id=$1 AND change_request_id=$2 ORDER BY revision DESC LIMIT 1`, scope.company, changeRequestID).Scan(&revision, &storedImpactDigest); err != nil {
			return Receipt{}, err
		}
		if expectedImpactSHA256 != storedImpactDigest {
			return Receipt{}, core.ConflictError{Reason: "the considered impact snapshot changed; review it again", CurrentState: "impact_changed"}
		}
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "pause the Mission before applying a formal change", CurrentState: basis.State}
		}
		baseDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if baseDigest != request.BaseRequirementsSHA256 {
			return Receipt{}, core.ConflictError{Reason: "requirements or inputs changed after impact review", CurrentState: "base_requirements_changed"}
		}
		currentImpact, currentImpactDigest, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if currentImpactDigest != storedImpactDigest || !missionChangeSafeBoundary(currentImpact) {
			return Receipt{}, core.ConflictError{Reason: "the Mission changed after impact review; reconsider before applying", CurrentState: "impact_changed"}
		}
		planningAssessment, err := missionChangePlanningAssessmentForTX(ctx, tx, scope, request, currentImpact)
		if err != nil {
			return Receipt{}, err
		}
		if planningAssessment == nil {
			return Receipt{}, core.ConflictError{Reason: "the fixed Planning role must assess natural-language change impacts before application", CurrentState: "planning_assessment_required"}
		}
		if planningAssessment.Status != "current" {
			return Receipt{}, core.ConflictError{Reason: "the Planning assessment no longer matches the current change basis", CurrentState: "planning_assessment_stale"}
		}
		var consideredAssessmentSHA256 string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(details->>'planning_assessment_sha256','')
FROM mission_change_request_events WHERE company_id=$1 AND change_request_id=$2
ORDER BY event_seq DESC LIMIT 1`, scope.company, changeRequestID).Scan(&consideredAssessmentSHA256); err != nil {
			return Receipt{}, err
		}
		if !validTaskInputDigest(consideredAssessmentSHA256) || planningAssessment.AssessmentSHA256 != consideredAssessmentSHA256 {
			return Receipt{}, core.ConflictError{Reason: "the Planning assessment changed after operator review; reconsider before applying", CurrentState: "planning_assessment_changed"}
		}
		if planningAssessment.RiskLevel != "low" && !request.BlockPreviousResults {
			return Receipt{}, core.ConflictError{Reason: "high-risk or uncertain change scope requires blocking previous results", CurrentState: "previous_results_must_be_blocked"}
		}
		successorMissionID, revisionMap, err := k.applyMissionChangeSuccessor(ctx, tx, scope, missionID, changeRequestID, request.ProposedTitle, request.ProposedGoal, request.ProposedAcceptanceContract, currentImpact)
		if err != nil {
			return Receipt{}, err
		}
		details := map[string]any{
			"input_revision_map": revisionMap, "base_requirements_sha256": baseDigest, "impact_sha256": storedImpactDigest,
			"planning_assessment_id": planningAssessment.AssessmentID, "planning_assessment_sha256": planningAssessment.AssessmentSHA256,
			"planning_risk_level": planningAssessment.RiskLevel,
		}
		if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, "applied", int64Pointer(revision), &successorMissionID, "successor_mission_created", details); err != nil {
			return Receipt{}, err
		}
		if err = appendDeliveryRevisionRouteSuccessorTX(ctx, tx, scope, changeRequestID, successorMissionID); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "mission.change_request.applied", map[string]any{
			"mission_id": missionID, "change_request_id": changeRequestID, "successor_mission_id": successorMissionID,
			"base_requirements_sha256": baseDigest, "impact_sha256": storedImpactDigest,
			"planning_assessment_id": planningAssessment.AssessmentID, "planning_assessment_sha256": planningAssessment.AssessmentSHA256,
			"planning_risk_level": planningAssessment.RiskLevel,
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: request.ID, Status: "applied"}, nil
	})
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return k.MissionChangeRequest(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) MissionChangeRequest(ctx context.Context, scope Scope, missionID, changeRequestID string) (MissionChangeRequest, error) {
	if !core.ValidID(missionID) || !core.ValidID(changeRequestID) {
		return MissionChangeRequest{}, core.Malformed
	}
	var out MissionChangeRequest
	var acceptanceJSON, impactJSON, latestDetails []byte
	var impactRevision pgtype.Int8
	var successorMissionID *string
	var createdAt time.Time
	err := k.pool.QueryRow(ctx, `SELECT r.change_request_id,r.mission_id,r.client_request_id,r.base_requirements_sha256,r.change_summary,r.proposed_title,r.proposed_goal,r.proposed_acceptance_contract,r.block_previous_results,r.created_at,
latest.state,latest.successor_mission_id,latest.details,impact.revision,impact.impact_sha256,impact.impact
FROM mission_change_requests r
JOIN LATERAL (SELECT state,successor_mission_id,details FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
JOIN LATERAL (SELECT revision,impact_sha256,impact FROM mission_change_request_impacts i WHERE i.company_id=r.company_id AND i.change_request_id=r.change_request_id ORDER BY revision DESC LIMIT 1) impact ON true
WHERE r.company_id=$1 AND r.mission_id=$2 AND r.change_request_id=$3`, scope.company, missionID, changeRequestID).Scan(
		&out.ID, &out.MissionID, &out.ClientRequestID, &out.BaseRequirementsSHA256, &out.ChangeSummary, &out.ProposedTitle, &out.ProposedGoal,
		&acceptanceJSON, &out.BlockPreviousResults, &createdAt, &out.State, &successorMissionID, &latestDetails, &impactRevision, &out.ImpactSHA256, &impactJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionChangeRequest{}, core.OutOfScope
	}
	if err != nil {
		return MissionChangeRequest{}, err
	}
	if len(acceptanceJSON) > 0 {
		var acceptance taskvalidation.AcceptanceContract
		if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
			return MissionChangeRequest{}, core.Integrity
		}
		out.ProposedAcceptanceContract = &acceptance
	}
	if !impactRevision.Valid || json.Unmarshal(impactJSON, &out.Impact) != nil {
		return MissionChangeRequest{}, core.Integrity
	}
	calculatedImpactDigest, err := missionChangeImpactDigest(out.Impact)
	if err != nil || calculatedImpactDigest != out.ImpactSHA256 {
		return MissionChangeRequest{}, core.Integrity
	}
	out.ImpactRevision = impactRevision.Int64
	out.SuccessorMissionID = successorMissionID
	out.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	assessmentTx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MissionChangeRequest{}, err
	}
	defer assessmentTx.Rollback(ctx)
	assessmentImpact := out.Impact
	assessmentRequest := out
	if out.State == "received" || out.State == "queued" || out.State == "considered" {
		basis, basisErr := missionChangeBasisTx(ctx, assessmentTx, scope, missionID, false)
		if basisErr != nil {
			return MissionChangeRequest{}, basisErr
		}
		currentRequirementsSHA256, digestErr := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if digestErr != nil {
			return MissionChangeRequest{}, digestErr
		}
		currentImpact, _, impactErr := missionChangeImpactTx(ctx, assessmentTx, scope, missionID, currentRequirementsSHA256, basis.InputRevisions)
		if impactErr != nil {
			return MissionChangeRequest{}, impactErr
		}
		assessmentRequest.BaseRequirementsSHA256 = currentRequirementsSHA256
		assessmentImpact = currentImpact
	}
	out.PlanningAssessment, err = missionChangePlanningAssessmentForTX(ctx, assessmentTx, scope, assessmentRequest, assessmentImpact)
	if err != nil {
		return MissionChangeRequest{}, err
	}
	if err = assessmentTx.Commit(ctx); err != nil {
		return MissionChangeRequest{}, err
	}
	var latestDetail struct {
		InputRevisionMap []MissionChangeInputRevisionMap `json:"input_revision_map"`
	}
	out.InputRevisionMap = []MissionChangeInputRevisionMap{}
	if len(latestDetails) > 0 && json.Unmarshal(latestDetails, &latestDetail) != nil {
		return MissionChangeRequest{}, core.Integrity
	}
	if latestDetail.InputRevisionMap != nil {
		out.InputRevisionMap = latestDetail.InputRevisionMap
	}
	out.Events, err = k.missionChangeRequestEvents(ctx, scope, changeRequestID)
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return out, nil
}

func (k *Kernel) MissionChangeRequests(ctx context.Context, scope Scope, missionID string) ([]MissionChangeRequest, error) {
	if !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT change_request_id FROM mission_change_requests WHERE company_id=$1 AND mission_id=$2 ORDER BY created_at DESC,change_request_id DESC LIMIT 50`, scope.company, missionID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]MissionChangeRequest, 0, len(ids))
	for _, id := range ids {
		item, readErr := k.MissionChangeRequest(ctx, scope, missionID, id)
		if readErr != nil {
			return nil, readErr
		}
		out = append(out, item)
	}
	return out, nil
}

type missionChangeRequestEventQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (k *Kernel) missionChangeRequestEvents(ctx context.Context, scope Scope, changeRequestID string) ([]MissionChangeRequestEvent, error) {
	return missionChangeRequestEventsFrom(ctx, k.pool, scope, changeRequestID)
}

func missionChangeRequestEventsFrom(ctx context.Context, queryer missionChangeRequestEventQueryer, scope Scope, changeRequestID string) ([]MissionChangeRequestEvent, error) {
	rows, err := queryer.Query(ctx, `SELECT event_id,state,impact_revision,successor_mission_id,reason_code,created_at,details
FROM mission_change_request_events WHERE company_id=$1 AND change_request_id=$2 ORDER BY event_seq`, scope.company, changeRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]MissionChangeRequestEvent, 0, 4)
	for rows.Next() {
		var event MissionChangeRequestEvent
		event.InputRevisionMap = []MissionChangeInputRevisionMap{}
		var impactRevision pgtype.Int8
		var successorMissionID *string
		var details []byte
		var createdAt time.Time
		if err = rows.Scan(&event.EventID, &event.State, &impactRevision, &successorMissionID, &event.ReasonCode, &createdAt, &details); err != nil {
			return nil, err
		}
		if impactRevision.Valid {
			event.ImpactRevision = &impactRevision.Int64
		}
		event.SuccessorMissionID = successorMissionID
		event.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		var data struct {
			InputRevisionMap         []MissionChangeInputRevisionMap `json:"input_revision_map"`
			PlanningAssessmentID     string                          `json:"planning_assessment_id"`
			PlanningAssessmentSHA256 string                          `json:"planning_assessment_sha256"`
			PlanningRiskLevel        string                          `json:"planning_risk_level"`
		}
		if len(details) > 0 && json.Unmarshal(details, &data) != nil {
			return nil, core.Integrity
		}
		if data.PlanningAssessmentID != "" || data.PlanningAssessmentSHA256 != "" || data.PlanningRiskLevel != "" {
			if !core.ValidID(data.PlanningAssessmentID) || !validTaskInputDigest(data.PlanningAssessmentSHA256) ||
				(data.PlanningRiskLevel != "low" && data.PlanningRiskLevel != "high" && data.PlanningRiskLevel != "uncertain") {
				return nil, core.Integrity
			}
			event.PlanningAssessmentID = data.PlanningAssessmentID
			event.PlanningAssessmentSHA256 = data.PlanningAssessmentSHA256
			event.PlanningRiskLevel = data.PlanningRiskLevel
		}
		if data.InputRevisionMap != nil {
			event.InputRevisionMap = data.InputRevisionMap
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func missionChangeOpenRequestForTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID string) (MissionChangeRequest, error) {
	rows, err := tx.Query(ctx, `SELECT r.change_request_id,r.mission_id,r.client_request_id,r.base_requirements_sha256,r.change_summary,r.proposed_title,r.proposed_goal,r.proposed_acceptance_contract,r.block_previous_results,r.created_at,
latest.state,latest.successor_mission_id,latest.details,impact.revision,impact.impact_sha256,impact.impact
FROM mission_change_requests r
JOIN LATERAL (SELECT state,successor_mission_id,details FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
JOIN LATERAL (SELECT revision,impact_sha256,impact FROM mission_change_request_impacts i WHERE i.company_id=r.company_id AND i.change_request_id=r.change_request_id ORDER BY revision DESC LIMIT 1) impact ON true
WHERE r.company_id=$1 AND r.mission_id=$2 AND latest.state IN ('received','queued','considered')
ORDER BY r.created_at DESC,r.change_request_id DESC LIMIT 2`, scope.company, missionID)
	if err != nil {
		return MissionChangeRequest{}, err
	}
	defer rows.Close()
	var selected *MissionChangeRequest
	for rows.Next() {
		if selected != nil {
			return MissionChangeRequest{}, core.Integrity
		}
		var request MissionChangeRequest
		var acceptanceJSON, impactJSON, latestDetails []byte
		var impactRevision pgtype.Int8
		var createdAt time.Time
		if err = rows.Scan(&request.ID, &request.MissionID, &request.ClientRequestID, &request.BaseRequirementsSHA256, &request.ChangeSummary,
			&request.ProposedTitle, &request.ProposedGoal, &acceptanceJSON, &request.BlockPreviousResults, &createdAt, &request.State,
			&request.SuccessorMissionID, &latestDetails, &impactRevision, &request.ImpactSHA256, &impactJSON); err != nil {
			return MissionChangeRequest{}, err
		}
		if len(acceptanceJSON) > 0 {
			var acceptance taskvalidation.AcceptanceContract
			if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
				return MissionChangeRequest{}, core.Integrity
			}
			request.ProposedAcceptanceContract = &acceptance
		}
		if !impactRevision.Valid || json.Unmarshal(impactJSON, &request.Impact) != nil {
			return MissionChangeRequest{}, core.Integrity
		}
		calculatedImpactDigest, digestErr := missionChangeImpactDigest(request.Impact)
		if digestErr != nil || calculatedImpactDigest != request.ImpactSHA256 {
			return MissionChangeRequest{}, core.Integrity
		}
		request.ImpactRevision = impactRevision.Int64
		request.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		request.InputRevisionMap = []MissionChangeInputRevisionMap{}
		var latestDetail struct {
			InputRevisionMap []MissionChangeInputRevisionMap `json:"input_revision_map"`
		}
		if len(latestDetails) > 0 && json.Unmarshal(latestDetails, &latestDetail) != nil {
			return MissionChangeRequest{}, core.Integrity
		}
		if latestDetail.InputRevisionMap != nil {
			request.InputRevisionMap = latestDetail.InputRevisionMap
		}
		selected = &request
	}
	if err = rows.Err(); err != nil {
		return MissionChangeRequest{}, err
	}
	if selected == nil {
		return MissionChangeRequest{}, core.OutOfScope
	}
	selected.Events, err = missionChangeRequestEventsFrom(ctx, tx, scope, selected.ID)
	if err != nil {
		return MissionChangeRequest{}, err
	}
	return *selected, nil
}

func missionChangeBasisTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID string, lock bool) (missionChangeBasis, error) {
	var basis missionChangeBasis
	query := `SELECT state,title,goal,acceptance_contract FROM missions WHERE company_id=$1 AND id=$2`
	if lock {
		query += " FOR UPDATE"
	}
	var acceptanceJSON []byte
	err := tx.QueryRow(ctx, query, scope.company, missionID).Scan(&basis.State, &basis.Title, &basis.Goal, &acceptanceJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return missionChangeBasis{}, core.OutOfScope
	}
	if err != nil {
		return missionChangeBasis{}, err
	}
	if len(acceptanceJSON) > 0 {
		var acceptance taskvalidation.AcceptanceContract
		if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
			return missionChangeBasis{}, core.Integrity
		}
		basis.AcceptanceContract = &acceptance
	}
	basis.InputRevisions, err = missionChangeInputRevisionsTx(ctx, tx, scope, missionID)
	return basis, err
}

func missionChangeInputRevisionsTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID string) ([]MissionChangeInputRevision, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON (input_id) input_id,revision,content_digest,state
FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 ORDER BY input_id,revision DESC LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inputs := make([]MissionChangeInputRevision, 0, 8)
	for rows.Next() {
		var input MissionChangeInputRevision
		if err = rows.Scan(&input.InputID, &input.Revision, &input.ContentDigest, &input.State); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
		if len(inputs) > maxMissionChangeImpactItems {
			rows.Close()
			return nil, core.TooLarge
		}
	}
	return inputs, rows.Err()
}

func missionChangeImpactTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID, baseRequirementsDigest string, inputs []MissionChangeInputRevision) (MissionChangeImpact, string, error) {
	impact := MissionChangeImpact{
		SchemaVersion: MissionChangeImpactSchema, MissionID: missionID, BaseRequirementsSHA256: baseRequirementsDigest,
		InputRevisions: append([]MissionChangeInputRevision{}, inputs...), Tasks: []MissionChangeTaskImpact{}, Artifacts: []MissionChangeArtifactImpact{},
		ActiveWorkerSessions: []MissionChangeWorkerImpact{}, NonterminalJobRuns: []MissionChangeJobImpact{}, ActiveServiceEndpoints: []MissionChangeServiceEndpointImpact{},
		ActiveTaskTakeoverLeases: []MissionChangeTakeoverLeaseImpact{}, ReturnedHumanTakeoverSnapshots: []MissionChangeTakeoverSnapshotImpact{},
		NaturalLanguageImpactStatus: "not_assessed",
	}
	rows, err := tx.Query(ctx, `SELECT t.id,t.owner,t.kind,t.state,t.generation,w.digest,w.revision
FROM tasks t LEFT JOIN worker_workspaces w ON w.company_id=t.company_id AND w.task_id=t.id
WHERE t.company_id=$1 AND t.mission_id=$2 ORDER BY t.id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var task MissionChangeTaskImpact
		var workspaceDigest pgtype.Text
		var workspaceRevision pgtype.Int8
		if err = rows.Scan(&task.TaskID, &task.OwnerEmployeeID, &task.Kind, &task.State, &task.Generation, &workspaceDigest, &workspaceRevision); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		if workspaceDigest.Valid {
			task.WorkspaceDigest = &workspaceDigest.String
		}
		if workspaceRevision.Valid {
			task.WorkspaceRevision = &workspaceRevision.Int64
		}
		impact.Tasks = append(impact.Tasks, task)
		if len(impact.Tasks) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT a.id,a.task_id,a.digest,a.verdict
FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND t.mission_id=$2 AND a.artifact_kind='deliverable' ORDER BY a.id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var artifact MissionChangeArtifactImpact
		if err = rows.Scan(&artifact.ArtifactID, &artifact.TaskID, &artifact.Digest, &artifact.Verdict); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		impact.Artifacts = append(impact.Artifacts, artifact)
		if len(impact.Artifacts) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT s.id,s.task_id,s.employee_id,s.state
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND t.mission_id=$2 AND s.state!='stopped' ORDER BY s.id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var worker MissionChangeWorkerImpact
		if err = rows.Scan(&worker.SessionID, &worker.TaskID, &worker.EmployeeID, &worker.State); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		impact.ActiveWorkerSessions = append(impact.ActiveWorkerSessions, worker)
		if len(impact.ActiveWorkerSessions) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT jr.job_id,jr.task_id,COALESCE(latest.state,'missing'),COALESCE(latest.readiness,'not_ready')
FROM job_runs jr JOIN tasks t ON t.company_id=jr.company_id AND t.id=jr.task_id
LEFT JOIN LATERAL (SELECT state,readiness FROM job_run_events e WHERE e.company_id=jr.company_id AND e.job_id=jr.job_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE jr.company_id=$1 AND t.mission_id=$2 AND (latest.state IS NULL OR latest.state NOT IN ('exited','failed','cancelled'))
ORDER BY jr.job_id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var job MissionChangeJobImpact
		if err = rows.Scan(&job.JobID, &job.TaskID, &job.State, &job.Readiness); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		impact.NonterminalJobRuns = append(impact.NonterminalJobRuns, job)
		if len(impact.NonterminalJobRuns) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT latest.job_id,latest.generation,latest.readiness
FROM (
 SELECT DISTINCT ON (e.job_id) e.job_id,e.generation,e.readiness
 FROM service_endpoint_events e JOIN job_runs jr ON jr.company_id=e.company_id AND jr.job_id=e.job_id
 JOIN tasks t ON t.company_id=jr.company_id AND t.id=jr.task_id
 WHERE e.company_id=$1 AND t.mission_id=$2
 ORDER BY e.job_id,e.generation DESC,e.event_seq DESC
) latest WHERE latest.readiness!='revoked' ORDER BY latest.job_id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var endpoint MissionChangeServiceEndpointImpact
		if err = rows.Scan(&endpoint.JobID, &endpoint.Generation, &endpoint.Readiness); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		impact.ActiveServiceEndpoints = append(impact.ActiveServiceEndpoints, endpoint)
		if len(impact.ActiveServiceEndpoints) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT l.lease_id,l.task_id,l.base_requirements_sha256,l.base_workspace_digest,l.base_workspace_revision
FROM task_takeover_active_slots slot
JOIN task_takeover_leases l ON l.company_id=slot.company_id AND l.lease_id=slot.lease_id
WHERE slot.company_id=$1 AND slot.mission_id=$2 ORDER BY l.lease_id`, scope.company, missionID)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var lease MissionChangeTakeoverLeaseImpact
		if err = rows.Scan(&lease.LeaseID, &lease.TaskID, &lease.BaseRequirementsSHA256, &lease.BaseWorkspaceDigest, &lease.BaseWorkspaceRevision); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		impact.ActiveTaskTakeoverLeases = append(impact.ActiveTaskTakeoverLeases, lease)
		if len(impact.ActiveTaskTakeoverLeases) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT l.lease_id,l.task_id,e.snapshot_input_id,e.snapshot_revision,e.snapshot_digest,e.snapshot_bytes,e.human_effort_seconds
FROM task_takeover_leases l
JOIN LATERAL (SELECT state,snapshot_input_id,snapshot_revision,snapshot_digest,snapshot_bytes,human_effort_seconds
 FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND l.mission_id=$2 AND e.state='returned' ORDER BY l.lease_id LIMIT $3`, scope.company, missionID, maxMissionChangeImpactItems+1)
	if err != nil {
		return MissionChangeImpact{}, "", err
	}
	for rows.Next() {
		var snapshot MissionChangeTakeoverSnapshotImpact
		var humanEffortSeconds pgtype.Int4
		if err = rows.Scan(&snapshot.LeaseID, &snapshot.TaskID, &snapshot.InputID, &snapshot.InputRevision, &snapshot.ContentDigest, &snapshot.ByteSize, &humanEffortSeconds); err != nil {
			rows.Close()
			return MissionChangeImpact{}, "", err
		}
		if humanEffortSeconds.Valid {
			seconds := int64(humanEffortSeconds.Int32)
			snapshot.HumanEffortSeconds = &seconds
		}
		impact.ReturnedHumanTakeoverSnapshots = append(impact.ReturnedHumanTakeoverSnapshots, snapshot)
		if len(impact.ReturnedHumanTakeoverSnapshots) > maxMissionChangeImpactItems {
			rows.Close()
			return MissionChangeImpact{}, "", core.TooLarge
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return MissionChangeImpact{}, "", err
	}
	rows.Close()
	digest, err := missionChangeImpactDigest(impact)
	return impact, digest, err
}

func insertMissionChangeImpact(ctx context.Context, tx pgx.Tx, scope Scope, changeRequestID string, revision int64, digest string, impact MissionChangeImpact) error {
	encoded, err := json.Marshal(impact)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO mission_change_request_impacts(company_id,change_request_id,revision,impact_sha256,impact,created_by)
VALUES($1,$2,$3,$4,$5,'operator')`, scope.company, changeRequestID, revision, digest, encoded)
	return err
}

func insertMissionChangeOutputBlocks(ctx context.Context, tx pgx.Tx, scope Scope, changeRequestID string, impact MissionChangeImpact) error {
	for _, artifact := range impact.Artifacts {
		if _, err := tx.Exec(ctx, `INSERT INTO mission_change_request_output_blocks(company_id,change_request_id,artifact_id)
VALUES($1,$2,$3) ON CONFLICT(company_id,change_request_id,artifact_id) DO NOTHING`, scope.company, changeRequestID, artifact.ArtifactID); err != nil {
			return err
		}
	}
	return nil
}

func missionChangeRequestForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, missionID, changeRequestID string) (MissionChangeRequest, string, error) {
	var request MissionChangeRequest
	var acceptanceJSON []byte
	err := tx.QueryRow(ctx, `SELECT r.change_request_id,r.mission_id,r.client_request_id,r.base_requirements_sha256,r.change_summary,r.proposed_title,r.proposed_goal,r.proposed_acceptance_contract,r.block_previous_results,
latest.state
FROM mission_change_requests r
JOIN LATERAL (SELECT state FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE r.company_id=$1 AND r.mission_id=$2 AND r.change_request_id=$3 FOR UPDATE OF r`, scope.company, missionID, changeRequestID).Scan(
		&request.ID, &request.MissionID, &request.ClientRequestID, &request.BaseRequirementsSHA256, &request.ChangeSummary,
		&request.ProposedTitle, &request.ProposedGoal, &acceptanceJSON, &request.BlockPreviousResults, &request.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionChangeRequest{}, "", core.OutOfScope
	}
	if err != nil {
		return MissionChangeRequest{}, "", err
	}
	if len(acceptanceJSON) > 0 {
		var acceptance taskvalidation.AcceptanceContract
		if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
			return MissionChangeRequest{}, "", core.Integrity
		}
		request.ProposedAcceptanceContract = &acceptance
	}
	return request, request.State, nil
}

func appendMissionChangeState(ctx context.Context, tx pgx.Tx, scope Scope, changeRequestID, state string, impactRevision *int64, successorMissionID *string, reasonCode string, details any) error {
	var current string
	err := tx.QueryRow(ctx, `SELECT state FROM mission_change_request_events WHERE company_id=$1 AND change_request_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, scope.company, changeRequestID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		current = ""
	} else if err != nil {
		return err
	}
	if !validMissionChangeRequestTransition(current, state) {
		return core.ConflictError{Reason: "formal change request transition is not valid", CurrentState: current}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if len(encoded) == 0 {
		encoded = []byte(`{}`)
	}
	var impactValue any
	if impactRevision != nil {
		impactValue = *impactRevision
	}
	_, err = tx.Exec(ctx, `INSERT INTO mission_change_request_events(company_id,event_id,change_request_id,state,impact_revision,successor_mission_id,details,reason_code,actor,command_request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'operator',$9)`, scope.company, newID(), changeRequestID, state, impactValue, successorMissionID, encoded, reasonCode, newID())
	return err
}

func missionChangeSafeBoundary(impact MissionChangeImpact) bool {
	return missionChangeWritersStopped(impact) && len(impact.ActiveTaskTakeoverLeases) == 0
}

func missionChangeWritersStopped(impact MissionChangeImpact) bool {
	if len(impact.ActiveWorkerSessions) > 0 || len(impact.NonterminalJobRuns) > 0 || len(impact.ActiveServiceEndpoints) > 0 {
		return false
	}
	for _, input := range impact.InputRevisions {
		if input.State == "uploading" || input.State == "stored" {
			return false
		}
	}
	return true
}

func int64Pointer(value int64) *int64 { return &value }

func (k *Kernel) applyMissionChangeSuccessor(ctx context.Context, tx pgx.Tx, scope Scope, missionID, changeRequestID, title, goal string, acceptance *taskvalidation.AcceptanceContract, impact MissionChangeImpact) (string, []MissionChangeInputRevisionMap, error) {
	var contractJSON []byte
	if acceptance != nil {
		var err error
		contractJSON, err = json.Marshal(acceptance)
		if err != nil {
			return "", nil, err
		}
	}
	successorMissionID := newID()
	if _, err := tx.Exec(ctx, `INSERT INTO missions(company_id,id,title,goal,state,contract,acceptance_contract)
VALUES($1,$2,$3,$4,'draft',$5,$6)`, scope.company, successorMissionID, title, goal, core.Contract, contractJSON); err != nil {
		return "", nil, err
	}
	inputs, err := missionInputClonesTx(ctx, tx, scope, missionID)
	if err != nil {
		return "", nil, err
	}
	mapping := make([]MissionChangeInputRevisionMap, 0, len(inputs))
	for _, input := range inputs {
		successorInputID, successorRequestID := newID(), newID()
		if _, err = tx.Exec(ctx, `INSERT INTO mission_inputs(company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height)
VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, scope.company, successorInputID, successorMissionID, successorRequestID,
			input.SourceKind, input.DisplayName, input.MediaType, input.ByteSize, input.ContentDigest, input.State, input.ImageWidth, input.ImageHeight); err != nil {
			return "", nil, err
		}
		origin := "mission_input"
		if input.TakeoverTaskID != "" {
			origin = "human_takeover"
		}
		mapping = append(mapping, MissionChangeInputRevisionMap{Origin: origin, SourceTaskID: input.TakeoverTaskID, PreviousInputID: input.InputID, PreviousRevision: input.Revision, SuccessorInputID: successorInputID, SuccessorRevision: 1, ContentDigest: input.ContentDigest})
	}
	var workspaceCarryoverBytes int
	for _, task := range impact.Tasks {
		if task.State == "completed" || task.State == "cancelled" || task.WorkspaceDigest == nil || task.WorkspaceRevision == nil {
			continue
		}
		content, readErr := readBlob(k.root, scope.company, *task.WorkspaceDigest)
		if readErr != nil || len(content) == 0 {
			return "", nil, core.Integrity
		}
		workspaceCarryoverBytes += len(content)
		if workspaceCarryoverBytes > maxMissionChangeWorkspaceBytes {
			return "", nil, core.TooLarge
		}
		successorInputID, successorRequestID := newID(), newID()
		displayName := "handover-" + task.TaskID + ".md"
		if _, err = tx.Exec(ctx, `INSERT INTO mission_inputs(company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height)
VALUES($1,$2,$3,1,$4,'upload',$5,'text/markdown',$6,$7,'usable',0,0)`, scope.company, successorInputID, successorMissionID, successorRequestID, displayName, len(content), *task.WorkspaceDigest); err != nil {
			return "", nil, err
		}
		mapping = append(mapping, MissionChangeInputRevisionMap{Origin: "task_workspace", SourceTaskID: task.TaskID, PreviousInputID: "workspace:" + task.TaskID, PreviousRevision: *task.WorkspaceRevision, SuccessorInputID: successorInputID, SuccessorRevision: 1, ContentDigest: *task.WorkspaceDigest})
	}
	rationale := "Replaced through approved change request " + changeRequestID + "; responsibility carryover is recorded in successor Mission " + successorMissionID
	if err = beginMissionCloseoutTX(ctx, tx, scope, missionID, "cancelled", rationale, []string{}, changeRequestID); err != nil {
		return "", nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE obligations o SET state='superseded'
FROM tasks t WHERE o.company_id=t.company_id AND o.task_id=t.id AND t.company_id=$1 AND t.mission_id=$2
	 AND o.state NOT IN ('fulfilled','declined','superseded')`, scope.company, missionID); err != nil {
		return "", nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE tasks SET state='cancelled' WHERE company_id=$1 AND mission_id=$2 AND state NOT IN ('completed','cancelled')`, scope.company, missionID); err != nil {
		return "", nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE routine_occurrences o SET state='cancelled'
FROM routines r WHERE o.company_id=r.company_id AND o.routine_id=r.id
	 AND r.company_id=$1 AND r.mission_id=$2 AND o.state IN ('pending','needs_instruction','delivered')`, scope.company, missionID); err != nil {
		return "", nil, err
	}
	var cancelledTaskTotal, supersededObligationTotal, cancelledRoutineTotal int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND state='cancelled'`, scope.company, missionID).Scan(&cancelledTaskTotal); err != nil {
		return "", nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
WHERE o.company_id=$1 AND t.mission_id=$2 AND o.state='superseded'`, scope.company, missionID).Scan(&supersededObligationTotal); err != nil {
		return "", nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences o JOIN routines r ON r.company_id=o.company_id AND r.id=o.routine_id
WHERE r.company_id=$1 AND r.mission_id=$2 AND o.state='cancelled'`, scope.company, missionID).Scan(&cancelledRoutineTotal); err != nil {
		return "", nil, err
	}
	closeoutReport, err := json.Marshal(map[string]any{
		"outcome":                   "cancelled",
		"rationale":                 rationale,
		"successorMissionId":        successorMissionID,
		"changeRequestId":           changeRequestID,
		"cancelledTaskTotal":        cancelledTaskTotal,
		"supersededObligationTotal": supersededObligationTotal,
		"cancelledRoutineTotal":     cancelledRoutineTotal,
	})
	if err != nil {
		return "", nil, err
	}
	if err = finishMissionCloseoutTX(ctx, tx, scope, missionID, "cancelled", closeoutReport); err != nil {
		return "", nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE employee_schedules s SET state='sleeping',checked_generation=work_generation,
 pause_reason=NULL,next_due_at=NULL,updated_at=now()
WHERE s.company_id=$1 AND s.employee_id IN (
 SELECT t.owner FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
 UNION SELECT r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
)`, scope.company, missionID); err != nil {
		return "", nil, err
	}
	if err = appendEvent(ctx, tx, scope, "mission.cancelled", map[string]any{"mission_id": missionID, "change_request_id": changeRequestID}); err != nil {
		return "", nil, err
	}
	if err = appendEvent(ctx, tx, scope, "mission.created", map[string]any{"mission_id": successorMissionID, "successor_of": missionID, "change_request_id": changeRequestID}); err != nil {
		return "", nil, err
	}
	return successorMissionID, mapping, nil
}

func missionInputClonesTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID string) ([]missionChangeInputClone, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(i.input_id) i.input_id,i.revision,COALESCE(takeover.task_id,''),i.source_kind,i.display_name,i.media_type,i.byte_size,i.content_digest,i.state,i.image_width,i.image_height
FROM mission_inputs i
LEFT JOIN LATERAL (
 SELECT l.task_id FROM task_takeover_leases l
 JOIN LATERAL (SELECT state,snapshot_input_id,snapshot_revision FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) latest ON latest.state='returned'
 WHERE l.company_id=i.company_id AND l.mission_id=i.mission_id AND latest.snapshot_input_id=i.input_id AND latest.snapshot_revision=i.revision
 ORDER BY l.lease_id LIMIT 1
) takeover ON true
WHERE i.company_id=$1 AND i.mission_id=$2 AND i.state NOT IN ('uploading','stored')
ORDER BY i.input_id,i.revision DESC`, scope.company, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inputs := make([]missionChangeInputClone, 0, 8)
	for rows.Next() {
		var input missionChangeInputClone
		if err = rows.Scan(&input.InputID, &input.Revision, &input.TakeoverTaskID, &input.SourceKind, &input.DisplayName, &input.MediaType, &input.ByteSize, &input.ContentDigest, &input.State, &input.ImageWidth, &input.ImageHeight); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, rows.Err()
}

func validChangeRequestText(value string) bool {
	return len(strings.TrimSpace(value)) <= core.MaxContent
}

func missionHasOpenChangeRequestTx(ctx context.Context, tx pgx.Tx, scope Scope, missionID string) (bool, error) {
	var open bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM mission_change_requests r
 JOIN LATERAL (SELECT state FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) current_state ON true
 WHERE r.company_id=$1 AND r.mission_id=$2 AND current_state.state IN ('received','queued','considered')
)`, scope.company, missionID).Scan(&open)
	return open, err
}
