// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// ProductDeliveryDispositionCommand is an optimistic-concurrency command for
// appending the installation owner's disposition of one ready delivery.
type ProductDeliveryDispositionCommand struct {
	ArtifactID                  string `json:"artifactId"`
	ExpectedManifestRevision    int64  `json:"expectedManifestRevision"`
	ExpectedDispositionRevision int64  `json:"expectedDispositionRevision"`
	State                       string `json:"state"`
	Reason                      string `json:"reason"`
	RequestID                   string `json:"requestId"`
}

// ProductDeliveryDispositionReceipt is the durable receipt returned after the
// disposition row and its TXWrite receipt have committed together.
type ProductDeliveryDispositionReceipt struct {
	RequestID           string `json:"requestId"`
	CompanyID           string `json:"companyId"`
	DeliveryID          string `json:"deliveryId"`
	ManifestRevision    string `json:"manifestRevision"`
	DispositionRevision string `json:"dispositionRevision"`
	State               string `json:"state"`
	Actor               string `json:"actor"`
	Reason              string `json:"reason"`
	CreatedAt           string `json:"createdAt"`
}

const productDeliveryDispositionActor = "installation-owner"

// RecordProductDeliveryDisposition appends one owner disposition under the
// company lifecycle guard. It has no worker or external-service side effects.
func (k *Kernel) RecordProductDeliveryDisposition(ctx context.Context, companyID string, command ProductDeliveryDispositionCommand) (ProductDeliveryDispositionReceipt, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(command.ArtifactID) || !core.ValidID(command.RequestID) ||
		command.ExpectedManifestRevision <= 0 || command.ExpectedDispositionRevision <= 0 ||
		(command.State != "accepted" && command.State != "changes_requested") {
		return ProductDeliveryDispositionReceipt{}, core.Malformed
	}
	reason := strings.TrimSpace(command.Reason)
	if !utf8.ValidString(reason) || strings.ContainsRune(reason, '\x00') || len(reason) == 0 || len(reason) > 4096 {
		return ProductDeliveryDispositionReceipt{}, core.Malformed
	}
	command.Reason = reason
	if command.ExpectedManifestRevision == math.MaxInt64 || command.ExpectedDispositionRevision == math.MaxInt64 {
		return ProductDeliveryDispositionReceipt{}, core.Conflict
	}

	const operation = "delivery.user_disposition.record"
	writeReceipt, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, command.RequestID, operation, command, func(tx pgx.Tx) (Receipt, error) {
		var manifest durableProductDeliveryManifest
		var manifestRevision int64
		var storedMissionID, storedTaskID, storedArtifactID, storedState, storedSHA256, manifestJSON string
		err := tx.QueryRow(ctx, `SELECT r.revision,r.mission_id,r.task_id,r.artifact_id,r.state,r.manifest::text,r.manifest_sha256
FROM delivery_manifest_revisions r
WHERE r.company_id=$1 AND r.delivery_id=$2 AND r.artifact_id=$2
ORDER BY r.revision DESC LIMIT 1 FOR UPDATE OF r`, companyID, command.ArtifactID).Scan(
			&manifestRevision, &storedMissionID, &storedTaskID, &storedArtifactID, &storedState, &manifestJSON, &storedSHA256,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if storedArtifactID != command.ArtifactID || manifestRevision != command.ExpectedManifestRevision {
			return Receipt{}, core.Conflict
		}
		if storedState != "ready" {
			return Receipt{}, core.ConflictError{Reason: "delivery manifest is not ready", CurrentState: storedState}
		}
		if err := validateReadyProductDeliveryManifest(manifestJSON, storedSHA256, companyID, command.ArtifactID, manifestRevision, storedMissionID, storedTaskID, storedState, &manifest); err != nil {
			return Receipt{}, err
		}

		var artifactTaskID, artifactMissionID, artifactDigest, artifactState, artifactVerdict, artifactKind string
		var artifactBytes int64
		err = tx.QueryRow(ctx, `SELECT a.task_id,t.mission_id,a.digest,a.bytes,a.state,a.verdict,a.artifact_kind
FROM artifacts a
JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND a.id=$2
FOR UPDATE OF a,t`, companyID, command.ArtifactID).Scan(
			&artifactTaskID, &artifactMissionID, &artifactDigest, &artifactBytes, &artifactState, &artifactVerdict, &artifactKind,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if artifactTaskID != storedTaskID || artifactMissionID != storedMissionID || artifactKind != "deliverable" {
			return Receipt{}, core.Integrity
		}
		if artifactState != "ready" || artifactVerdict != "passed" {
			return Receipt{}, core.ConflictError{Reason: "delivery Artifact has not passed independent verification", CurrentState: artifactState + "/" + artifactVerdict}
		}
		manifestBytes, byteErr := strconv.ParseInt(manifest.Artifact.ByteSize, 10, 64)
		if byteErr != nil || manifestBytes != artifactBytes || manifest.Artifact.SHA256 != artifactDigest {
			return Receipt{}, core.Integrity
		}
		if err = requireProductDeliveryQualification(ctx, tx, companyID, storedMissionID, storedTaskID, command.ArtifactID, artifactDigest); err != nil {
			return Receipt{}, err
		}

		var deliveryBlocked bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM mission_change_requests r
 JOIN LATERAL (
  SELECT state FROM mission_change_request_events e
  WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id
  ORDER BY e.event_seq DESC LIMIT 1
 ) latest ON true
 WHERE r.company_id=$1 AND r.mission_id=$2 AND r.block_previous_results
   AND latest.state NOT IN ('declined','superseded')
)`, companyID, storedMissionID).Scan(&deliveryBlocked); err != nil {
			return Receipt{}, err
		}
		if deliveryBlocked {
			return Receipt{}, core.ConflictError{Reason: "artifact delivery is blocked by an unresolved formal requirement change", CurrentState: "change_review"}
		}

		var latestDispositionRevision int64
		var latestDispositionState string
		var latestDispositionDeadline *time.Time
		err = tx.QueryRow(ctx, `SELECT revision,state,feedback_deadline
FROM delivery_user_dispositions
WHERE company_id=$1 AND delivery_id=$2 AND manifest_revision=$3
ORDER BY revision DESC LIMIT 1 FOR UPDATE`, companyID, command.ArtifactID, manifestRevision).Scan(&latestDispositionRevision, &latestDispositionState, &latestDispositionDeadline)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		}
		if err != nil {
			return Receipt{}, err
		}
		if latestDispositionRevision != command.ExpectedDispositionRevision {
			return Receipt{}, core.Conflict
		}
		if latestDispositionState != "not_requested" && latestDispositionState != "awaiting_feedback" && latestDispositionState != "accepted" && latestDispositionState != "changes_requested" {
			return Receipt{}, core.ConflictError{Reason: "delivery disposition is not writable in its current state", CurrentState: latestDispositionState}
		}
		var databaseNow time.Time
		if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&databaseNow); err != nil {
			return Receipt{}, err
		}
		if productDeliveryFeedbackExpired(latestDispositionState, latestDispositionDeadline, databaseNow.UTC()) {
			return Receipt{}, core.ConflictError{Reason: "delivery feedback window has expired", CurrentState: "feedback_expired"}
		}

		nextDispositionRevision := latestDispositionRevision + 1
		_, err = tx.Exec(ctx, `INSERT INTO delivery_user_dispositions(
 company_id,delivery_id,revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULL,clock_timestamp())`,
			companyID, command.ArtifactID, nextDispositionRevision, manifestRevision, command.State, productDeliveryDispositionActor, command.Reason, command.RequestID)
		if err != nil {
			return Receipt{}, err
		}
		var missionState string
		if err = tx.QueryRow(ctx, `SELECT state FROM missions WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, storedMissionID).Scan(&missionState); err != nil {
			return Receipt{}, err
		}
		switch productDeliveryDispositionRoute(missionState, command.State) {
		case "mission_change_request":
			if err = appendDeliveryMissionChangeRequestTX(ctx, tx, Scope{company: companyID}, storedMissionID, command.ArtifactID, manifestRevision, nextDispositionRevision, command.Reason, command.RequestID); err != nil {
				return Receipt{}, err
			}
		case "company_backlog":
			backlogRequestID := "delivery-feedback-" + fingerprint(command.RequestID)[:48]
			if _, err = tx.Exec(ctx, `INSERT INTO delivery_feedback_backlog_events(
company_id, event_id, delivery_id, manifest_revision, disposition_revision, mission_id, task_id, artifact_id, status, reason, actor, request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$3,'open',$8,'system',$9)`, companyID, newID(), command.ArtifactID, manifestRevision, nextDispositionRevision, storedMissionID, storedTaskID, command.Reason, backlogRequestID); err != nil {
				return Receipt{}, err
			}
		default:
			return Receipt{}, core.ConflictError{Reason: "delivery changes cannot be routed while the Mission is not active, paused, or terminal", CurrentState: missionState}
		}
		return Receipt{ID: command.ArtifactID, Status: command.State, Revision: nextDispositionRevision}, nil
	})
	if err != nil {
		return ProductDeliveryDispositionReceipt{}, err
	}
	if writeReceipt.ID != command.ArtifactID || writeReceipt.Status != command.State || writeReceipt.Revision <= 1 {
		return ProductDeliveryDispositionReceipt{}, core.Integrity
	}

	var result ProductDeliveryDispositionReceipt
	var createdAt time.Time
	err = k.pool.QueryRow(ctx, `SELECT request_id,company_id,delivery_id,manifest_revision::text,revision::text,state,actor,reason,created_at
FROM delivery_user_dispositions
WHERE company_id=$1 AND request_id=$2`, companyID, command.RequestID).Scan(
		&result.RequestID, &result.CompanyID, &result.DeliveryID, &result.ManifestRevision, &result.DispositionRevision,
		&result.State, &result.Actor, &result.Reason, &createdAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductDeliveryDispositionReceipt{}, core.Integrity
	}
	if err != nil {
		return ProductDeliveryDispositionReceipt{}, err
	}
	if result.RequestID != command.RequestID || result.CompanyID != companyID || result.DeliveryID != command.ArtifactID ||
		result.ManifestRevision != strconv.FormatInt(command.ExpectedManifestRevision, 10) || result.State != command.State ||
		result.Actor != productDeliveryDispositionActor || result.Reason != command.Reason ||
		result.DispositionRevision != strconv.FormatInt(writeReceipt.Revision, 10) {
		return ProductDeliveryDispositionReceipt{}, core.Integrity
	}
	result.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return result, nil
}

func appendDeliveryMissionChangeRequestTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, artifactID string, manifestRevision, dispositionRevision int64, reason, dispositionRequestID string) error {
	basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
	if err != nil {
		return err
	}
	if basis.State != "active" && basis.State != "paused" {
		return core.ConflictError{Reason: "formal changes require an active or paused Mission", CurrentState: basis.State}
	}
	if basis.AcceptanceContract == nil {
		return core.ConflictError{Reason: "delivery change routing requires the Mission acceptance contract", CurrentState: "acceptance_contract_missing"}
	}
	openRequest, err := missionHasOpenChangeRequestTx(ctx, tx, scope, missionID)
	if err != nil {
		return err
	}
	if openRequest {
		return core.ConflictError{Reason: "this Mission already has a pending formal change request", CurrentState: "change_request_open"}
	}
	baseDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
	if err != nil {
		return err
	}
	impact, impactDigest, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseDigest, basis.InputRevisions)
	if err != nil {
		return err
	}
	changeRequestID := newID()
	clientRequestID := "delivery-change-" + fingerprint(dispositionRequestID)[:48]
	const changeSummaryPrefix = "Delivery changes requested: "
	changeSummaryRunes := []rune(reason)
	for len(changeSummaryPrefix)+len(string(changeSummaryRunes)) > core.MaxContent {
		changeSummaryRunes = changeSummaryRunes[:len(changeSummaryRunes)-1]
	}
	changeSummary := changeSummaryPrefix + string(changeSummaryRunes)
	proposedAcceptanceJSON, err := json.Marshal(basis.AcceptanceContract)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mission_change_requests(company_id,change_request_id,mission_id,client_request_id,base_requirements_sha256,change_summary,proposed_title,proposed_goal,proposed_acceptance_contract,block_previous_results)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true)`, scope.company, changeRequestID, missionID, clientRequestID, baseDigest, changeSummary, basis.Title, basis.Goal, proposedAcceptanceJSON); err != nil {
		return err
	}
	if err = insertMissionChangeImpact(ctx, tx, scope, changeRequestID, 1, impactDigest, impact); err != nil {
		return err
	}
	if err = insertMissionChangeOutputBlocks(ctx, tx, scope, changeRequestID, impact); err != nil {
		return err
	}
	if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, "received", int64Pointer(1), nil, "delivery_changes_requested", map[string]any{
		"delivery_id": artifactID, "disposition_revision": dispositionRevision,
	}); err != nil {
		return err
	}
	if basis.State == "active" {
		if err = appendMissionChangeState(ctx, tx, scope, changeRequestID, "queued", int64Pointer(1), nil, "awaiting_safe_boundary", map[string]any{
			"delivery_id": artifactID, "disposition_revision": dispositionRevision,
		}); err != nil {
			return err
		}
	}
	routeID := stableCapabilityID("delivery-revision-route", scope.company, dispositionRequestID)
	if _, err = tx.Exec(ctx, `INSERT INTO delivery_revision_routes(
company_id,route_id,delivery_id,manifest_revision,disposition_revision,mission_id,change_request_id)
VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.company, routeID, artifactID, manifestRevision, dispositionRevision, missionID, changeRequestID); err != nil {
		return err
	}
	routeEventID := stableCapabilityID("delivery-revision-route-event", scope.company, routeID, "pending")
	routeRequestID := "delivery-route-pending-" + fingerprint(dispositionRequestID)[:48]
	if _, err = tx.Exec(ctx, `INSERT INTO delivery_revision_route_events(company_id,event_id,route_id,state,reason_code,request_id)
VALUES($1,$2,$3,'change_request_pending','delivery_changes_requested',$4)`, scope.company, routeEventID, routeID, routeRequestID); err != nil {
		return err
	}
	return appendEvent(ctx, tx, scope, "mission.change_request.created", map[string]any{
		"mission_id": missionID, "change_request_id": changeRequestID, "base_requirements_sha256": baseDigest,
		"impact_sha256": impactDigest, "block_previous_results": true, "delivery_id": artifactID,
		"disposition_revision": dispositionRevision,
	})
}

func appendDeliveryRevisionRouteSuccessorTX(ctx context.Context, tx pgx.Tx, scope Scope, changeRequestID, successorMissionID string) error {
	var routeID, state string
	err := tx.QueryRow(ctx, `SELECT r.route_id,latest.state
FROM delivery_revision_routes r
JOIN LATERAL (SELECT state FROM delivery_revision_route_events e WHERE e.company_id=r.company_id AND e.route_id=r.route_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE r.company_id=$1 AND r.change_request_id=$2 FOR UPDATE OF r`, scope.company, changeRequestID).Scan(&routeID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state == "successor_mission_created" || state == "revision_task_ready" {
		return nil
	}
	if state != "change_request_pending" {
		return core.ConflictError{Reason: "delivery revision route is not awaiting successor Mission", CurrentState: state}
	}
	eventID := stableCapabilityID("delivery-revision-route-event", scope.company, routeID, "successor", successorMissionID)
	requestID := "delivery-route-successor-" + fingerprint([]string{changeRequestID, successorMissionID})[:48]
	_, err = tx.Exec(ctx, `INSERT INTO delivery_revision_route_events(company_id,event_id,route_id,state,successor_mission_id,reason_code,request_id)
VALUES($1,$2,$3,'successor_mission_created',$4,'successor_mission_created',$5)`, scope.company, eventID, routeID, successorMissionID, requestID)
	return err
}

func appendDeliveryRevisionRouteTaskTX(ctx context.Context, tx pgx.Tx, scope Scope, successorMissionID, taskID string) error {
	var routeID, state string
	err := tx.QueryRow(ctx, `SELECT r.route_id,latest.state
FROM delivery_revision_routes r
JOIN LATERAL (SELECT state,successor_mission_id FROM delivery_revision_route_events e WHERE e.company_id=r.company_id AND e.route_id=r.route_id ORDER BY event_seq DESC LIMIT 1) latest ON latest.successor_mission_id=$3
WHERE r.company_id=$1 AND latest.state IN ('successor_mission_created','revision_task_ready')
ORDER BY r.created_at DESC LIMIT 1 FOR UPDATE OF r`, scope.company, successorMissionID, successorMissionID).Scan(&routeID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state == "revision_task_ready" {
		return nil
	}
	eventID := stableCapabilityID("delivery-revision-route-event", scope.company, routeID, "task", taskID)
	requestID := "delivery-route-task-" + fingerprint([]string{routeID, taskID})[:48]
	_, err = tx.Exec(ctx, `INSERT INTO delivery_revision_route_events(company_id,event_id,route_id,state,successor_mission_id,task_id,reason_code,request_id)
VALUES($1,$2,$3,'revision_task_ready',$4,$5,'revision_task_ready',$6)`, scope.company, eventID, routeID, successorMissionID, taskID, requestID)
	return err
}

func validateReadyProductDeliveryManifest(rawJSON, storedSHA256, companyID, artifactID string, revision int64, missionID, taskID, state string, out *durableProductDeliveryManifest) error {
	decoder := json.NewDecoder(bytes.NewBufferString(rawJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return core.Integrity
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return core.Integrity
	}
	if out.SchemaVersion != durableProductDeliveryManifestSchema || out.DeliveryID != artifactID || out.ArtifactID != artifactID ||
		out.Revision != strconv.FormatInt(revision, 10) || out.CompanyID != companyID || out.MissionID != missionID ||
		out.TaskID != taskID || out.State != state || out.Artifact.FileName != "artifact.bin" || !validSHA256(out.Artifact.SHA256) {
		return core.Integrity
	}
	byteSize, err := strconv.ParseInt(out.Artifact.ByteSize, 10, 64)
	if err != nil || byteSize <= 0 || byteSize > 64*1024*1024 || strconv.FormatInt(byteSize, 10) != out.Artifact.ByteSize {
		return core.Integrity
	}
	if !validReadyProductDeliverySections(out.Sections) {
		return core.Integrity
	}
	if _, err = time.Parse(time.RFC3339Nano, out.CreatedAt); err != nil {
		return core.Integrity
	}
	canonical, err := marshalDurableProductDeliveryManifest(*out)
	if err != nil {
		return core.Integrity
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != storedSHA256 {
		return core.Integrity
	}
	return nil
}

func validReadyProductDeliverySections(sections []durableProductDeliveryManifestSection) bool {
	required := [...]string{"source_inputs", "environment_build", "file_inventory", "run_instructions", "verification", "limitations", "license_source", "feedback"}
	if len(sections) != len(required) {
		return false
	}
	for index, section := range sections {
		if section.Key != required[index] || section.Detail == "" || len(section.Detail) > 512 {
			return false
		}
		switch section.State {
		case "available", "unavailable", "missing", "not_requested":
		default:
			return false
		}
		if index != len(required)-1 && section.State != "available" {
			return false
		}
	}
	return sections[2].State == "available" && sections[4].State == "available"
}

func requireProductDeliveryQualification(ctx context.Context, tx pgx.Tx, companyID, missionID, taskID, artifactID, artifactDigest string) error {
	var checkpointRaw []byte
	var checkpointID, checkID, sessionID, bindingDigest, workspaceDigest, runnerRevision string
	var workspaceRevision int64
	err := tx.QueryRow(ctx, `SELECT q.checkpoint_id,q.check_id,q.session_id,q.validation_binding_digest,q.workspace_digest,q.workspace_revision,q.runner_revision,cp.data
FROM task_validation_artifact_qualifications q
JOIN task_validation_bindings b ON b.company_id=q.company_id AND b.task_id=q.task_id
JOIN worker_sessions s ON s.company_id=q.company_id AND s.id=q.session_id AND s.task_id=q.task_id
JOIN worker_checkpoints cp ON cp.company_id=q.company_id AND cp.id=q.checkpoint_id AND cp.session_id=q.session_id AND cp.digest=q.workspace_digest
JOIN worker_checks wc ON wc.company_id=q.company_id AND wc.id=q.check_id AND wc.session_id=q.session_id AND wc.digest=q.workspace_digest AND wc.phase='product' AND wc.passed
WHERE q.company_id=$1 AND q.task_id=$2 AND q.artifact_id=$3 AND b.mission_id=$4
  AND b.configuration_digest=q.validation_binding_digest AND b.runner_revision=q.runner_revision
  AND q.workspace_digest=$5`, companyID, taskID, artifactID, missionID, artifactDigest).Scan(
		&checkpointID, &checkID, &sessionID, &bindingDigest, &workspaceDigest, &workspaceRevision, &runnerRevision, &checkpointRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Integrity
	}
	if err != nil {
		return err
	}
	var checkpoint Checkpoint
	if err = json.Unmarshal(checkpointRaw, &checkpoint); err != nil {
		return core.Integrity
	}
	if checkpoint.Kind != CheckpointQualified || checkpoint.FinalizationState != "current" || checkpoint.ValidationStatus != "PASS" ||
		checkpoint.TaskValidationBindingDigest != bindingDigest || checkpoint.WorkspaceDigest != workspaceDigest ||
		checkpoint.WorkspaceRevision != workspaceRevision || checkpoint.SessionID != sessionID || checkpoint.Epoch <= 0 ||
		len(checkpoint.EvidenceRefs) == 0 || checkpointID == "" || checkID == "" || runnerRevision == "" {
		return fmt.Errorf("%w: delivery qualification checkpoint mismatch", core.Integrity)
	}
	return nil
}
