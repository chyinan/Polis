// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
	"polis/internal/qqnotify"
)

type HumanInterventionInput struct {
	MissionID        string
	TaskID           string
	ProblemKey       string
	Severity         string
	ReasonCode       string
	AffectedScope    string
	ProtectionAction string
	RequiredAction   string
	EvidenceRefs     []string
	ExpiresAt        time.Time
}

type HumanIntervention struct {
	CompanyID        string
	ID               string
	IncidentID       string
	MissionID        string
	TaskID           string
	ProblemKey       string
	Occurrence       int64
	Severity         string
	ReasonCode       string
	AffectedScope    string
	ProtectionAction string
	RequiredAction   string
	EvidenceRefs     []string
	State            string
	StateRevision    int64
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type HumanInterventionDeliveryPlan struct {
	CompanyID      string
	InstallationID string
	SenderEpoch    string
	IntentID       string
	DeliveryID     string
	PermitID       string
	CredentialRef  string
	Route          qqnotify.Route
	Intervention   qqnotify.Intervention
	Grant          qqnotify.DeliveryGrant
	MessageDigest  string
	AttemptCount   int
}

func (k *Kernel) TXCreateHumanIntervention(ctx context.Context, scope Scope, input HumanInterventionInput, requestID string) (HumanIntervention, error) {
	if !core.ValidID(requestID) || !validHumanInterventionInput(input, time.Now()) {
		return HumanIntervention{}, core.Malformed
	}
	if input.EvidenceRefs == nil {
		input.EvidenceRefs = []string{}
	}
	receipt, err := k.TXWrite(ctx, scope, nil, requestID, "human_intervention.open", input, func(tx pgx.Tx) (Receipt, error) {
		expiresAt := input.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = time.Now().Add(24 * time.Hour).UTC()
		}
		if input.MissionID != "" {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND id=$2)", scope.company, input.MissionID).Scan(&exists); err != nil {
				return Receipt{}, err
			} else if !exists {
				return Receipt{}, core.OutOfScope
			}
		}
		if input.TaskID != "" {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE company_id=$1 AND id=$2 AND mission_id=$3)", scope.company, input.TaskID, input.MissionID).Scan(&exists); err != nil {
				return Receipt{}, err
			} else if !exists {
				return Receipt{}, core.OutOfScope
			}
		}
		var priorID, priorState string
		var priorOccurrence int64
		var priorExpiry time.Time
		err := tx.QueryRow(ctx, `SELECT id,occurrence,state,expires_at FROM human_interventions
WHERE company_id=$1 AND problem_key=$2 ORDER BY occurrence DESC LIMIT 1 FOR UPDATE`, scope.company, input.ProblemKey).Scan(&priorID, &priorOccurrence, &priorState, &priorExpiry)
		occurrence := int64(1)
		if err == nil {
			if (priorState == "open" || priorState == "acknowledged") && time.Now().Before(priorExpiry) {
				return Receipt{ID: priorID, Status: priorState, Revision: priorOccurrence}, nil
			}
			occurrence = priorOccurrence + 1
			if priorState == "open" || priorState == "acknowledged" {
				if _, err = tx.Exec(ctx, `UPDATE human_interventions SET state='obsolete',state_revision=state_revision+1,updated_at=clock_timestamp()
WHERE company_id=$1 AND id=$2`, scope.company, priorID); err != nil {
					return Receipt{}, err
				}
				if _, err = tx.Exec(ctx, `UPDATE notification_intents SET state='superseded'
WHERE company_id=$1 AND intervention_id=$2 AND state IN ('pending','retry_wait')`, scope.company, priorID); err != nil {
					return Receipt{}, err
				}
				if _, err = tx.Exec(ctx, `UPDATE notification_deliveries SET state='superseded'
WHERE company_id=$1 AND intent_id IN (SELECT id FROM notification_intents WHERE company_id=$1 AND intervention_id=$2)
AND state IN ('pending','retry_wait')`, scope.company, priorID); err != nil {
					return Receipt{}, err
				}
				if err = appendEvent(ctx, tx, scope, "human_intervention.obsolete", map[string]any{"id": priorID, "reason": "expired"}); err != nil {
					return Receipt{}, err
				}
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}

		id := newID()
		incidentID := "INC-" + strings.ToUpper(id[:8])
		refs, err := json.Marshal(input.EvidenceRefs)
		if err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO human_interventions(company_id,id,mission_id,task_id,problem_key,occurrence,severity,reason_code,affected_scope,protection_action,required_action,evidence_refs,state,expires_at)
		VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11,$12,'open',$13)`, scope.company, id, input.MissionID, input.TaskID, input.ProblemKey, occurrence, input.Severity, input.ReasonCode, input.AffectedScope, input.ProtectionAction, input.RequiredAction, refs, expiresAt); err != nil {
			return Receipt{}, err
		}
		safePayload, err := json.Marshal(struct {
			InterventionID string   `json:"intervention_id"`
			IncidentID     string   `json:"incident_id"`
			Severity       string   `json:"severity"`
			ReasonCode     string   `json:"reason_code"`
			AffectedScope  string   `json:"affected_scope"`
			Protection     string   `json:"protection_action"`
			RequiredAction string   `json:"required_action"`
			EvidenceRefs   []string `json:"evidence_refs"`
		}{id, incidentID, input.Severity, input.ReasonCode, input.AffectedScope, input.ProtectionAction, input.RequiredAction, input.EvidenceRefs})
		if err != nil {
			return Receipt{}, err
		}
		intentID := newID()
		dedupeKey := fmt.Sprintf("%s-%d", input.ProblemKey, occurrence)
		if _, err = tx.Exec(ctx, `INSERT INTO notification_intents(company_id,id,event_kind,subject_id,payload,state,intervention_id,dedupe_key,expires_at)
		VALUES($1,$2,'human_intervention.open',$3,$4,'pending',$3,$5,$6)`, scope.company, intentID, id, safePayload, dedupeKey, expiresAt); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: "open", Revision: occurrence}, nil
	})
	if err != nil {
		return HumanIntervention{}, err
	}
	return k.humanInterventionByID(ctx, scope, receipt.ID)
}

func (k *Kernel) TXSetHumanInterventionState(ctx context.Context, scope Scope, id, state, requestID string) (HumanIntervention, error) {
	if !core.ValidID(id) || !core.ValidID(requestID) || (state != "acknowledged" && state != "resolved" && state != "obsolete") {
		return HumanIntervention{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, requestID, "human_intervention."+state, struct{ ID, State string }{id, state}, func(tx pgx.Tx) (Receipt, error) {
		var currentState string
		var currentRevision, occurrence int64
		if err := tx.QueryRow(ctx, `SELECT state,state_revision,occurrence FROM human_interventions WHERE company_id=$1 AND id=$2 FOR UPDATE`, scope.company, id).Scan(&currentState, &currentRevision, &occurrence); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentState == state {
			return Receipt{ID: id, Status: state, Revision: currentRevision}, nil
		}
		if currentState != "open" && currentState != "acknowledged" {
			return Receipt{}, core.ConflictError{Reason: "human intervention is already terminal", CurrentState: currentState}
		}
		if state == "acknowledged" && currentState != "open" {
			return Receipt{}, core.ConflictError{Reason: "human intervention cannot be re-acknowledged", CurrentState: currentState}
		}
		if _, err := tx.Exec(ctx, `UPDATE human_interventions SET state=$3,state_revision=state_revision+1,updated_at=clock_timestamp()
WHERE company_id=$1 AND id=$2`, scope.company, id, state); err != nil {
			return Receipt{}, err
		}
		if state == "acknowledged" || state == "resolved" || state == "obsolete" {
			if _, err := tx.Exec(ctx, `UPDATE notification_intents SET state='superseded'
WHERE company_id=$1 AND intervention_id=$2 AND state IN ('pending','retry_wait')`, scope.company, id); err != nil {
				return Receipt{}, err
			}
			if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET state='superseded'
WHERE company_id=$1 AND intent_id IN (SELECT id FROM notification_intents WHERE company_id=$1 AND intervention_id=$2)
AND state IN ('pending','retry_wait')`, scope.company, id); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: id, Status: state, Revision: currentRevision + 1}, nil
	})
	if err != nil {
		return HumanIntervention{}, err
	}
	return k.humanInterventionByID(ctx, scope, receipt.ID)
}

func (k *Kernel) humanInterventionByID(ctx context.Context, scope Scope, id string) (HumanIntervention, error) {
	var intervention HumanIntervention
	var missionID, taskID *string
	var refs []byte
	err := k.pool.QueryRow(ctx, `SELECT id,mission_id,task_id,problem_key,occurrence,severity,reason_code,affected_scope,protection_action,required_action,evidence_refs,state,state_revision,expires_at,created_at,updated_at
FROM human_interventions WHERE company_id=$1 AND id=$2`, scope.company, id).Scan(&intervention.ID, &missionID, &taskID, &intervention.ProblemKey, &intervention.Occurrence, &intervention.Severity, &intervention.ReasonCode, &intervention.AffectedScope, &intervention.ProtectionAction, &intervention.RequiredAction, &refs, &intervention.State, &intervention.StateRevision, &intervention.ExpiresAt, &intervention.CreatedAt, &intervention.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return HumanIntervention{}, core.OutOfScope
	}
	if err != nil {
		return HumanIntervention{}, err
	}
	if err = json.Unmarshal(refs, &intervention.EvidenceRefs); err != nil {
		return HumanIntervention{}, core.Integrity
	}
	intervention.CompanyID = scope.company
	if missionID != nil {
		intervention.MissionID = *missionID
	}
	if taskID != nil {
		intervention.TaskID = *taskID
	}
	intervention.IncidentID = "INC-" + strings.ToUpper(intervention.ID[:8])
	return intervention, nil
}

func (k *Kernel) TXPrepareHumanInterventionDelivery(ctx context.Context, scope Scope, interventionID string, senderSession qqnotify.SenderSessionInfo) (HumanInterventionDeliveryPlan, error) {
	if !core.ValidID(interventionID) || !core.ValidID(senderSession.InstallationID) || !core.ValidID(senderSession.SenderEpoch) {
		return HumanInterventionDeliveryPlan{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	var item HumanIntervention
	var missionID, taskID *string
	var evidenceJSON []byte
	err = tx.QueryRow(ctx, `SELECT id,mission_id,task_id,problem_key,occurrence,severity,reason_code,affected_scope,protection_action,required_action,evidence_refs,state,state_revision,expires_at,created_at,updated_at
FROM human_interventions WHERE company_id=$1 AND id=$2 FOR UPDATE`, scope.company, interventionID).Scan(&item.ID, &missionID, &taskID, &item.ProblemKey, &item.Occurrence, &item.Severity, &item.ReasonCode, &item.AffectedScope, &item.ProtectionAction, &item.RequiredAction, &evidenceJSON, &item.State, &item.StateRevision, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return HumanInterventionDeliveryPlan{}, core.OutOfScope
	}
	if err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if item.State != "open" || !now.Before(item.ExpiresAt) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "intervention is no longer eligible for notification", CurrentState: item.State}
	}
	if missionID != nil {
		item.MissionID = *missionID
	}
	if taskID != nil {
		item.TaskID = *taskID
	}
	if err = json.Unmarshal(evidenceJSON, &item.EvidenceRefs); err != nil {
		return HumanInterventionDeliveryPlan{}, core.Integrity
	}
	item.CompanyID = scope.company
	item.IncidentID = "INC-" + strings.ToUpper(item.ID[:8])

	var intentID string
	var intentState string
	var intentExpiry time.Time
	err = tx.QueryRow(ctx, `SELECT id,state,COALESCE(expires_at,$3) FROM notification_intents
WHERE company_id=$1 AND intervention_id=$2 AND event_kind='human_intervention.open' FOR UPDATE`, scope.company, interventionID, item.ExpiresAt).Scan(&intentID, &intentState, &intentExpiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return HumanInterventionDeliveryPlan{}, core.OutOfScope
	}
	if err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if intentState != "pending" || !now.Before(intentExpiry) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "notification intent is no longer eligible", CurrentState: intentState}
	}

	var routeID, target, alias, credentialRef, routeStatus, qualificationStatus string
	var enabled bool
	var routeRevision int64
	var qualifiedUntil pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT id,destination,safety_alias,credential_ref,status,enabled,route_revision,qualification_status,qualified_until
FROM notification_routes WHERE company_id=$1 AND adapter='qq_official' FOR UPDATE`, scope.company).Scan(&routeID, &target, &alias, &credentialRef, &routeStatus, &enabled, &routeRevision, &qualificationStatus, &qualifiedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "QQ route is not configured", CurrentState: "unverified"}
	}
	if err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if !enabled || routeStatus != "ready" || qualificationStatus != "qualified" || !qualifiedUntil.Valid || !now.Before(qualifiedUntil.Time) || !qqnotify.ValidTargetOpenID(target) || !qqnotify.ValidSafetyAlias(alias) || !qqnotify.ValidCredentialRef(credentialRef) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "QQ route qualification is not current", CurrentState: qualificationStatus}
	}
	var qualified bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM qq_channel_qualifications WHERE company_id=$1 AND route_id=$2 AND route_revision=$3 AND target_openid=$4 AND qualification_status='qualified' AND qualified_until>clock_timestamp())`, scope.company, routeID, routeRevision, target).Scan(&qualified); err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if !qualified {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "QQ route has no matching current qualification", CurrentState: "unverified"}
	}

	var attemptCount int
	var previousDeliveryID, previousState string
	var retryAt pgtype.Timestamptz
	var previousExpiry pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT id,state,attempt_count,retry_at,expires_at FROM notification_deliveries
WHERE company_id=$1 AND intent_id=$2 ORDER BY attempt_count DESC,created_at DESC LIMIT 1 FOR UPDATE`, scope.company, intentID).Scan(&previousDeliveryID, &previousState, &attemptCount, &retryAt, &previousExpiry)
	if err == nil {
		// A sender process may have stopped after acquiring a single-use permit.
		// Once that permit expires, the provider outcome cannot be safely retried.
		if previousState == "sending" {
			if previousExpiry.Valid && now.Before(previousExpiry.Time) {
				return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "notification delivery is still inside its sender permit window", CurrentState: previousState}
			}
			if _, err = tx.Exec(ctx, `UPDATE notification_deliveries SET state='outcome_unknown',error_code='sender_interrupted_after_permit_expiry',retry_at=NULL
WHERE company_id=$1 AND id=$2 AND state='sending'`, scope.company, previousDeliveryID); err != nil {
				return HumanInterventionDeliveryPlan{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE notification_intents SET state='outcome_unknown' WHERE company_id=$1 AND id=$2 AND state='pending'`, scope.company, intentID); err != nil {
				return HumanInterventionDeliveryPlan{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return HumanInterventionDeliveryPlan{}, err
			}
			return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "expired sender attempt is outcome unknown and cannot be retried", CurrentState: "outcome_unknown"}
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		attemptCount = 0
	} else if err != nil {
		return HumanInterventionDeliveryPlan{}, err
	} else if previousState != "retry_wait" || !retryAt.Valid || now.Before(retryAt.Time) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "notification delivery is not eligible for another attempt", CurrentState: previousState}
	}
	if attemptCount >= qqnotify.MaxNotificationAttempts {
		if _, err = tx.Exec(ctx, `UPDATE notification_intents SET state='exhausted' WHERE company_id=$1 AND id=$2 AND state='pending'`, scope.company, intentID); err != nil {
			return HumanInterventionDeliveryPlan{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return HumanInterventionDeliveryPlan{}, err
		}
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "notification retry budget is exhausted", CurrentState: "exhausted"}
	}
	attemptCount++
	message, err := qqnotify.BuildInterventionMessage(qqnotify.Intervention{
		CompanyAlias: alias, IncidentID: item.IncidentID, Severity: item.Severity, ReasonCode: item.ReasonCode,
		ProtectionAction: item.ProtectionAction, RequiredAction: item.RequiredAction, ObservedAt: item.CreatedAt,
	})
	if err != nil {
		return HumanInterventionDeliveryPlan{}, core.Integrity
	}
	messageDigest := qqnotify.MessageDigest(message)
	deliveryID, permitID := newID(), newID()
	expiresAt := now.Add(qqnotify.MaxPermitLifetime)
	if expiresAt.After(item.ExpiresAt) {
		expiresAt = item.ExpiresAt
	}
	if expiresAt.After(qualifiedUntil.Time) {
		expiresAt = qualifiedUntil.Time
	}
	if !expiresAt.After(now) {
		return HumanInterventionDeliveryPlan{}, core.ConflictError{Reason: "notification permit would already be expired", CurrentState: "expired"}
	}
	messageSequence := int64(attemptCount)
	if _, err = tx.Exec(ctx, `INSERT INTO notification_deliveries(company_id,id,intent_id,adapter,state,route_revision,attempt_count,expires_at,message_digest,sender_epoch,permit_id)
VALUES($1,$2,$3,'qq_official','sending',$4,$5,$6,$7,$8,$9)`, scope.company, deliveryID, intentID, routeRevision, attemptCount, expiresAt, messageDigest, senderSession.SenderEpoch, permitID); err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_intents SET route_id=$3,route_revision=$4,message_digest=$5 WHERE company_id=$1 AND id=$2`, scope.company, intentID, routeID, routeRevision, messageDigest); err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return HumanInterventionDeliveryPlan{}, err
	}
	return HumanInterventionDeliveryPlan{
		CompanyID: scope.company, InstallationID: senderSession.InstallationID, SenderEpoch: senderSession.SenderEpoch,
		IntentID: intentID, DeliveryID: deliveryID, PermitID: permitID,
		CredentialRef: credentialRef,
		Route:         qqnotify.Route{Adapter: qqnotify.AdapterName, Enabled: enabled, Status: routeStatus, TargetOpenID: target, CredentialRef: credentialRef, Revision: routeRevision, QualifiedUntil: qualifiedUntil.Time, MaxAttempts: qqnotify.MaxNotificationAttempts},
		Intervention:  qqnotify.Intervention{CompanyAlias: alias, IncidentID: item.IncidentID, Severity: item.Severity, ReasonCode: item.ReasonCode, ProtectionAction: item.ProtectionAction, RequiredAction: item.RequiredAction, ObservedAt: item.CreatedAt},
		Grant:         qqnotify.DeliveryGrant{PermitID: permitID, DeliveryID: deliveryID, MessageSequence: messageSequence, ExpiresAt: expiresAt},
		MessageDigest: messageDigest, AttemptCount: attemptCount,
	}, nil
}

func (k *Kernel) TXCompleteHumanInterventionDelivery(ctx context.Context, scope Scope, plan HumanInterventionDeliveryPlan, result qqnotify.SendResult) (Receipt, error) {
	if !core.ValidID(plan.DeliveryID) || (result.Outcome != qqnotify.OutcomeProviderAccepted && result.Outcome != qqnotify.OutcomeRetryWait && result.Outcome != qqnotify.OutcomeRejected && result.Outcome != qqnotify.OutcomeUnknown) {
		return Receipt{}, core.Malformed
	}
	requestID := "notify-result-" + plan.DeliveryID
	if len(requestID) > 80 {
		requestID = "notify-result-" + plan.DeliveryID[:40]
	}
	return k.TXWrite(ctx, scope, nil, requestID, "notification.delivery.result", struct {
		DeliveryID string
		Outcome    string
		Status     int
		RemoteID   string
		ErrorCode  string
	}{plan.DeliveryID, string(result.Outcome), result.HTTPStatus, result.RemoteMessageID, result.ErrorCode}, func(tx pgx.Tx) (Receipt, error) {
		var intentID, currentState string
		var attempts int
		if err := tx.QueryRow(ctx, `SELECT intent_id,state,attempt_count FROM notification_deliveries WHERE company_id=$1 AND id=$2 FOR UPDATE`, scope.company, plan.DeliveryID).Scan(&intentID, &currentState, &attempts); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		} else if currentState != "sending" {
			return Receipt{}, core.ConflictError{Reason: "notification delivery no longer owns the send attempt", CurrentState: currentState}
		}
		deliveryState, intentState := string(result.Outcome), "pending"
		var retryAt any
		switch result.Outcome {
		case qqnotify.OutcomeProviderAccepted:
			intentState = "delivered"
		case qqnotify.OutcomeRejected:
			intentState = "failed"
		case qqnotify.OutcomeUnknown:
			intentState = "outcome_unknown"
		case qqnotify.OutcomeRetryWait:
			if attempts >= qqnotify.MaxNotificationAttempts {
				deliveryState, intentState = "exhausted", "exhausted"
			} else {
				delay := result.RetryAfterSeconds
				if delay < 1 || delay > qqnotify.MaxRetryAfterSeconds {
					delay = 60
				}
				retryAt = time.Now().UTC().Add(time.Duration(delay) * time.Second)
			}
		}
		var remoteID any
		if result.RemoteMessageID != "" {
			remoteID = result.RemoteMessageID
		}
		var httpStatus any
		if result.HTTPStatus >= 100 && result.HTTPStatus <= 599 {
			httpStatus = result.HTTPStatus
		}
		var errorCode any
		if result.ErrorCode != "" {
			errorCode = result.ErrorCode
		}
		if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET state=$3,error_code=$4,remote_message_id=$5,provider_http_status=$6,retry_at=$7
WHERE company_id=$1 AND id=$2`, scope.company, plan.DeliveryID, deliveryState, errorCode, remoteID, httpStatus, retryAt); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE notification_intents SET state=$3 WHERE company_id=$1 AND id=$2", scope.company, intentID, intentState); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: plan.DeliveryID, Status: deliveryState, Revision: int64(attempts)}, nil
	})
}

func validHumanInterventionInput(input HumanInterventionInput, now time.Time) bool {
	if (input.MissionID != "" && !core.ValidID(input.MissionID)) || (input.TaskID != "" && (!core.ValidID(input.TaskID) || input.MissionID == "")) || !core.ValidID(input.ProblemKey) || (input.Severity != "high" && input.Severity != "critical") || (input.AffectedScope != "company" && input.AffectedScope != "mission" && input.AffectedScope != "task") || len(input.EvidenceRefs) > 8 {
		return false
	}
	if (input.AffectedScope == "mission" && input.MissionID == "") || (input.AffectedScope == "task" && input.TaskID == "") {
		return false
	}
	for _, ref := range input.EvidenceRefs {
		if !core.ValidID(ref) {
			return false
		}
	}
	expiresAt := input.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(24 * time.Hour)
	}
	if !expiresAt.After(now) || expiresAt.After(now.Add(7*24*time.Hour)) {
		return false
	}
	_, err := qqnotify.BuildInterventionMessage(qqnotify.Intervention{CompanyAlias: "Polis", IncidentID: "INC-00000001", Severity: input.Severity, ReasonCode: input.ReasonCode, ProtectionAction: input.ProtectionAction, RequiredAction: input.RequiredAction, ObservedAt: now})
	return err == nil
}
