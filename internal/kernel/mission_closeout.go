// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const missionCancellationRationale = "Cancelled by the installation owner"

// TXBeginMissionCancellation commits the admission fence before Control stops
// host processes. Replaying the same request returns the durable closing receipt.
func (k *Kernel) TXBeginMissionCancellation(ctx context.Context, scope Scope, missionID, requestID string) (Receipt, error) {
	return k.beginMissionCloseout(ctx, scope, missionID, "cancelled", missionCancellationRationale, nil, requestID, true)
}

// TXBeginMissionCloseout records an explicit terminal intent. Success requires
// exact independently passed Artifact references and an owner rationale; Task
// states alone are never interpreted as Mission acceptance.
func (k *Kernel) TXBeginMissionCloseout(ctx context.Context, scope Scope, missionID, outcome, rationale string, acceptanceArtifactIDs []string, requestID string) (Receipt, error) {
	if !core.ValidID(missionID) || !core.ValidID(requestID) || strings.TrimSpace(rationale) == "" || len(rationale) > 4096 {
		return Receipt{}, core.Malformed
	}
	if outcome != "succeeded" && outcome != "ended_not_met" && outcome != "cancelled" {
		return Receipt{}, core.Malformed
	}
	if (outcome == "succeeded") != (len(acceptanceArtifactIDs) > 0) {
		return Receipt{}, core.Malformed
	}
	seen := make(map[string]struct{}, len(acceptanceArtifactIDs))
	for _, artifactID := range acceptanceArtifactIDs {
		if !core.ValidID(artifactID) {
			return Receipt{}, core.Malformed
		}
		if _, exists := seen[artifactID]; exists {
			return Receipt{}, core.Malformed
		}
		seen[artifactID] = struct{}{}
	}
	return k.beginMissionCloseout(ctx, scope, missionID, outcome, strings.TrimSpace(rationale), acceptanceArtifactIDs, requestID, false)
}

func (k *Kernel) beginMissionCloseout(ctx context.Context, scope Scope, missionID, outcome, rationale string, artifactIDs []string, requestID string, cancellation bool) (Receipt, error) {
	if !core.ValidID(missionID) {
		return Receipt{}, core.Malformed
	}
	if artifactIDs == nil {
		artifactIDs = []string{}
	}
	operation := "mission.closeout.begin"
	var input any = struct {
		MissionID   string
		Outcome     string
		Rationale   string
		ArtifactIDs []string
	}{missionID, outcome, rationale, artifactIDs}
	if cancellation {
		operation = "mission.cancel"
		input = missionID
	}
	return k.TXWrite(ctx, scope, nil, requestID, operation, input, func(tx pgx.Tx) (Receipt, error) {
		state, err := missionState(ctx, tx, scope, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "active" && state != "paused" && !(state == "draft" && outcome != "succeeded") {
			return Receipt{}, core.ConflictError{Reason: "Mission cannot enter closeout from " + state, CurrentState: state}
		}
		if outcome == "succeeded" {
			var matched int
			var unsettledTasks, unresolvedObligations int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM artifacts a
JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND t.mission_id=$2 AND a.id=ANY($3::text[])
 AND a.state='ready' AND a.verdict='passed'`, scope.company, missionID, artifactIDs).Scan(&matched); err != nil {
				return Receipt{}, err
			}
			if matched != len(artifactIDs) {
				return Receipt{}, core.ConflictError{Reason: "Mission success evidence must reference exact ready Artifacts with an independent passed review", CurrentState: "acceptance_evidence_required"}
			}
			acceptedDeliveries, err := missionAcceptedDeliveryArtifactCount(ctx, tx, scope, missionID, artifactIDs)
			if err != nil {
				return Receipt{}, err
			}
			if err = validateMissionSuccessDeliveryAcceptance(matched == len(artifactIDs), acceptedDeliveries == len(artifactIDs)); err != nil {
				return Receipt{}, err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND state NOT IN ('completed','cancelled')`, scope.company, missionID).Scan(&unsettledTasks); err != nil {
				return Receipt{}, err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
WHERE o.company_id=$1 AND t.mission_id=$2 AND o.state NOT IN ('fulfilled','declined','superseded')`, scope.company, missionID).Scan(&unresolvedObligations); err != nil {
				return Receipt{}, err
			}
			if unsettledTasks != 0 || unresolvedObligations != 0 {
				return Receipt{}, core.ConflictError{Reason: "Mission success can be requested only after all Tasks and Obligations are settled", CurrentState: "acceptance_unresolved"}
			}
		}
		if err = beginMissionCloseoutTX(ctx, tx, scope, missionID, outcome, rationale, artifactIDs, requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: missionID, Status: "closing"}, nil
	})
}

func beginMissionCloseoutTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, outcome, rationale string, artifactIDs []string, requestID string) error {
	if artifactIDs == nil {
		artifactIDs = []string{}
	}
	tag, err := tx.Exec(ctx, `UPDATE missions SET state='closing' WHERE company_id=$1 AND id=$2 AND state IN ('draft','active','paused')`, scope.company, missionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return core.ConflictError{Reason: "Mission could not enter closing", CurrentState: "lifecycle_changed"}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mission_closeouts(company_id,mission_id,closeout_id,requested_outcome,rationale,acceptance_artifact_ids,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.company, missionID, newID(), outcome, rationale, artifactIDs, requestID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE routines SET scheduling_active=false,next_due_at=NULL,updated_at=now()
WHERE company_id=$1 AND mission_id=$2`, scope.company, missionID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE employee_schedules s
SET state='quiescing',work_generation=work_generation+1,checked_generation=work_generation+1,
 pause_reason='mission_closing',next_due_at=NULL,updated_at=now()
WHERE s.company_id=$1 AND s.employee_id IN (
 SELECT t.owner FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
 UNION SELECT r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
)`, scope.company, missionID)
	return err
}

func finishMissionCloseoutTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, outcome string, reportJSON []byte) error {
	if _, err := tx.Exec(ctx, "UPDATE missions SET state=$3 WHERE company_id=$1 AND id=$2 AND state='closing'", scope.company, missionID, outcome); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE mission_closeouts SET terminal_outcome=$3,closeout_report=$4,finished_at=now()
WHERE company_id=$1 AND mission_id=$2 AND terminal_outcome IS NULL`, scope.company, missionID, outcome, reportJSON)
	return err
}

func missionCloseoutFinalizeKey(missionID string) string {
	return "mission-closeout-final-" + fingerprint(missionID)[:32]
}

// TXFinalizeMissionCloseout settles obligations and routine occurrences only
// after all exact WorkerSessions, JobRuns and service leases are stopped.
func (k *Kernel) TXFinalizeMissionCloseout(ctx context.Context, scope Scope, missionID string) (Receipt, error) {
	if !core.ValidID(missionID) {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, scope, nil, missionCloseoutFinalizeKey(missionID), "mission.closeout.finalize", missionID, func(tx pgx.Tx) (Receipt, error) {
		var requestedOutcome, rationale string
		var artifactIDs []string
		if err := tx.QueryRow(ctx, `SELECT requested_outcome,rationale,acceptance_artifact_ids
FROM mission_closeouts WHERE company_id=$1 AND mission_id=$2`, scope.company, missionID).Scan(&requestedOutcome, &rationale, &artifactIDs); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.ConflictError{Reason: "Mission has no durable closeout intent", CurrentState: "not_closing"}
		} else if err != nil {
			return Receipt{}, err
		}
		state, err := missionState(ctx, tx, scope, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "closing" {
			return Receipt{}, core.ConflictError{Reason: "Mission closeout is not awaiting finalization", CurrentState: state}
		}
		outstanding, err := missionHasOutstandingJobWork(ctx, tx, scope.company, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if outstanding {
			return Receipt{}, core.ConflictError{Reason: "Mission closeout is waiting for WorkerSessions, JobRuns or service leases to stop and reconcile", CurrentState: "reconcile_required"}
		}
		if requestedOutcome == "succeeded" {
			var unsettledTasks, unresolvedObligations, matched int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND state NOT IN ('completed','cancelled')`, scope.company, missionID).Scan(&unsettledTasks); err != nil {
				return Receipt{}, err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
WHERE o.company_id=$1 AND t.mission_id=$2 AND o.state NOT IN ('fulfilled','declined','superseded')`, scope.company, missionID).Scan(&unresolvedObligations); err != nil {
				return Receipt{}, err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND t.mission_id=$2 AND a.id=ANY($3::text[]) AND a.state='ready' AND a.verdict='passed'`, scope.company, missionID, artifactIDs).Scan(&matched); err != nil {
				return Receipt{}, err
			}
			acceptedDeliveries, err := missionAcceptedDeliveryArtifactCount(ctx, tx, scope, missionID, artifactIDs)
			if err != nil {
				return Receipt{}, err
			}
			if err = validateMissionSuccessDeliveryAcceptance(matched == len(artifactIDs), acceptedDeliveries == len(artifactIDs)); err != nil {
				return Receipt{}, err
			}
			if unsettledTasks != 0 || unresolvedObligations != 0 {
				return Receipt{}, core.ConflictError{Reason: "Mission success requires all Tasks and Obligations settled and the recorded acceptance Artifacts still independently passed", CurrentState: "acceptance_unresolved"}
			}
		} else {
			if _, err = tx.Exec(ctx, `UPDATE obligations o SET state='declined'
FROM tasks t WHERE o.company_id=t.company_id AND o.task_id=t.id AND t.company_id=$1 AND t.mission_id=$2
 AND o.state NOT IN ('fulfilled','declined','superseded')`, scope.company, missionID); err != nil {
				return Receipt{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE tasks SET state='cancelled'
WHERE company_id=$1 AND mission_id=$2 AND state NOT IN ('completed','cancelled')`, scope.company, missionID); err != nil {
				return Receipt{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE routine_occurrences o SET state='cancelled'
FROM routines r WHERE o.company_id=r.company_id AND o.routine_id=r.id
 AND r.company_id=$1 AND r.mission_id=$2
 AND (o.state IN ('pending','needs_instruction') OR (o.state='delivered' AND EXISTS(
  SELECT 1 FROM tasks t WHERE t.company_id=o.company_id AND t.id=o.task_id AND t.state='cancelled')))`, scope.company, missionID); err != nil {
			return Receipt{}, err
		}
		var report struct {
			Outcome                 string   `json:"outcome"`
			Rationale               string   `json:"rationale"`
			AcceptanceArtifactIDs   []string `json:"acceptanceArtifactIds"`
			CancelledTaskTotal      int64    `json:"cancelledTaskTotal"`
			DeclinedObligationTotal int64    `json:"declinedObligationTotal"`
			CancelledRoutineTotal   int64    `json:"cancelledRoutineTotal"`
		}
		report.Outcome = requestedOutcome
		report.Rationale = rationale
		report.AcceptanceArtifactIDs = artifactIDs
		if requestedOutcome != "succeeded" {
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND state='cancelled'", scope.company, missionID).Scan(&report.CancelledTaskTotal); err != nil {
				return Receipt{}, err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
WHERE o.company_id=$1 AND t.mission_id=$2 AND o.state='declined'`, scope.company, missionID).Scan(&report.DeclinedObligationTotal); err != nil {
				return Receipt{}, err
			}
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences o JOIN routines r ON r.company_id=o.company_id AND r.id=o.routine_id
WHERE r.company_id=$1 AND r.mission_id=$2 AND o.state='cancelled'`, scope.company, missionID).Scan(&report.CancelledRoutineTotal); err != nil {
			return Receipt{}, err
		}
		reportJSON, err := json.Marshal(report)
		if err != nil {
			return Receipt{}, err
		}
		if err = finishMissionCloseoutTX(ctx, tx, scope, missionID, requestedOutcome, reportJSON); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE employee_schedules s SET state='sleeping',checked_generation=work_generation,
 pause_reason=NULL,next_due_at=NULL,updated_at=now()
WHERE s.company_id=$1 AND s.employee_id IN (
 SELECT t.owner FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
 UNION SELECT r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
)`, scope.company, missionID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: missionID, Status: requestedOutcome}, nil
	})
}

func missionAcceptedDeliveryArtifactCount(ctx context.Context, tx pgx.Tx, scope Scope, missionID string, artifactIDs []string) (int, error) {
	if len(artifactIDs) == 0 {
		return 0, nil
	}
	var tableAvailable bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass('public.delivery_manifest_revisions') IS NOT NULL").Scan(&tableAvailable); err != nil {
		return 0, err
	}
	if !tableAvailable {
		return 0, nil
	}
	var accepted int
	err := tx.QueryRow(ctx, `SELECT count(*)::int
FROM unnest($3::text[]) AS requested(artifact_id)
WHERE EXISTS (
 SELECT 1
 FROM delivery_manifest_revisions m
 WHERE m.company_id=$1 AND m.mission_id=$2 AND m.delivery_id=requested.artifact_id AND m.artifact_id=requested.artifact_id AND m.state='ready'
   AND m.revision=(SELECT max(latest.revision) FROM delivery_manifest_revisions latest WHERE latest.company_id=m.company_id AND latest.delivery_id=m.delivery_id AND latest.artifact_id=m.artifact_id)
   AND EXISTS (
    SELECT 1 FROM delivery_user_dispositions d
    WHERE d.company_id=m.company_id AND d.delivery_id=m.delivery_id AND d.manifest_revision=m.revision AND d.state='accepted'
      AND d.revision=(SELECT max(latestDisposition.revision) FROM delivery_user_dispositions latestDisposition WHERE latestDisposition.company_id=d.company_id AND latestDisposition.delivery_id=d.delivery_id AND latestDisposition.manifest_revision=d.manifest_revision)
   )
)`, scope.company, missionID, artifactIDs).Scan(&accepted)
	return accepted, err
}
