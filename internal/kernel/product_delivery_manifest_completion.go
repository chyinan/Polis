// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ProductDeliveryManifestCompletionReceipt struct {
	RequestID           string `json:"requestId"`
	CompanyID           string `json:"companyId"`
	DeliveryID          string `json:"deliveryId"`
	ManifestRevision    string `json:"manifestRevision"`
	DispositionRevision string `json:"dispositionRevision"`
	State               string `json:"state"`
	Actor               string `json:"actor"`
	CreatedAt           string `json:"createdAt"`
}

const productDeliveryManifestCompletionActor = "system"

func (k *Kernel) CompleteProductDeliveryManifest(ctx context.Context, companyID string, command ProductDeliveryManifestCompletionCommand) (ProductDeliveryManifestCompletionReceipt, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || validateProductDeliveryManifestCompletionCommand(command) != nil {
		return ProductDeliveryManifestCompletionReceipt{}, core.Malformed
	}
	writeReceipt, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, command.RequestID, "delivery.manifest.complete", command, func(tx pgx.Tx) (Receipt, error) {
		var storedRevision int64
		var missionID, taskID, artifactID, state, storedSHA256, manifestJSON string
		err := tx.QueryRow(ctx, `SELECT revision,mission_id,task_id,artifact_id,state,manifest::text,manifest_sha256
FROM delivery_manifest_revisions
WHERE company_id=$1 AND delivery_id=$2 AND artifact_id=$2
ORDER BY revision DESC LIMIT 1 FOR UPDATE`, companyID, command.ArtifactID).Scan(
			&storedRevision, &missionID, &taskID, &artifactID, &state, &manifestJSON, &storedSHA256,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if storedRevision != command.ExpectedManifestRevision || artifactID != command.ArtifactID {
			return Receipt{}, core.Conflict
		}
		if state != "assembling" {
			return Receipt{}, core.ConflictError{Reason: "delivery manifest is not assembling", CurrentState: state}
		}
		if _, err = validateStoredProductDeliveryManifest([]byte(manifestJSON), storedSHA256, companyID, missionID, taskID, artifactID, storedRevision, state); err != nil {
			return Receipt{}, core.Integrity
		}

		var artifactTaskID, artifactMissionID, artifactDigest, artifactState, artifactVerdict, artifactKind string
		var artifactBytes int
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
		if artifactTaskID != taskID || artifactMissionID != missionID || artifactKind != "deliverable" {
			return Receipt{}, core.Integrity
		}
		if artifactState != "ready" || artifactVerdict != "passed" {
			return Receipt{}, core.ConflictError{Reason: "delivery Artifact has not passed independent verification", CurrentState: artifactState + "/" + artifactVerdict}
		}
		if err = requireProductDeliveryQualification(ctx, tx, companyID, missionID, taskID, command.ArtifactID, artifactDigest); err != nil {
			return Receipt{}, err
		}

		var sourceManifestDigest string
		if err = tx.QueryRow(ctx, `SELECT manifest_digest FROM task_input_manifests WHERE company_id=$1 AND task_id=$2 AND mission_id=$3`, companyID, taskID, missionID).Scan(&sourceManifestDigest); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.Integrity
			}
			return Receipt{}, err
		}
		if command.Evidence.SourceInputs.Reference != "task_input_manifests/"+taskID || command.Evidence.SourceInputs.Digest != sourceManifestDigest {
			return Receipt{}, core.Integrity
		}

		var bindingDigest, runnerRevision string
		var bindingContract []byte
		if err = tx.QueryRow(ctx, `SELECT configuration_digest,runner_revision,contract FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2 AND mission_id=$3`, companyID, taskID, missionID).Scan(&bindingDigest, &runnerRevision, &bindingContract); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.Integrity
			}
			return Receipt{}, err
		}
		if command.Evidence.EnvironmentBuild.Reference != "task_validation_bindings/"+taskID || command.Evidence.EnvironmentBuild.Digest != bindingDigest {
			return Receipt{}, core.Integrity
		}
		contractDigest := sha256.Sum256(bindingContract)
		if command.Evidence.RunInstructions.Reference != "task_validation_contract/"+taskID || command.Evidence.RunInstructions.Digest != hex.EncodeToString(contractDigest[:]) {
			return Receipt{}, core.Integrity
		}
		if !strings.HasPrefix(command.Evidence.Limitations.Reference, "polis.delivery.limitations@") || !strings.HasPrefix(command.Evidence.LicenseSource.Reference, "polis.delivery.license-source@") {
			return Receipt{}, core.Integrity
		}

		var checkpointID, checkID, qualificationBindingDigest, workspaceDigest, qualificationRunnerRevision string
		var workspaceRevision int64
		if err = tx.QueryRow(ctx, `SELECT checkpoint_id,check_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision
FROM task_validation_artifact_qualifications
WHERE company_id=$1 AND task_id=$2 AND artifact_id=$3`, companyID, taskID, command.ArtifactID).Scan(
			&checkpointID, &checkID, &qualificationBindingDigest, &workspaceDigest, &workspaceRevision, &qualificationRunnerRevision,
		); err != nil {
			return Receipt{}, err
		}
		if qualificationBindingDigest != bindingDigest || qualificationRunnerRevision != runnerRevision {
			return Receipt{}, core.Integrity
		}
		var createdAt time.Time
		if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&createdAt); err != nil {
			return Receipt{}, err
		}
		newRevision := storedRevision + 1
		verificationDetail := fmt.Sprintf("checkpoint=%s; validation=%s; taskValidationBindingDigest=%s; workspaceDigest=%s; workspaceRevision=%d; runnerRevision=%s", checkpointID, checkID, bindingDigest, workspaceDigest, workspaceRevision, runnerRevision)
		_, manifestBytes, manifestSHA256, err := buildReadyProductDeliveryManifest(readyProductDeliveryManifestInput{
			CompanyID: companyID, MissionID: missionID, TaskID: taskID, ArtifactID: artifactID, Revision: newRevision,
			ArtifactBytes: artifactBytes, ArtifactSHA256: artifactDigest, VerificationDetail: verificationDetail,
			CreatedAt: createdAt, Evidence: command.Evidence,
		})
		if err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO delivery_manifest_revisions(
company_id,delivery_id,revision,mission_id,task_id,artifact_id,state,manifest,manifest_sha256,created_at)
VALUES($1,$2,$3,$4,$5,$6,'ready',$7,$8,$9)`, companyID, artifactID, newRevision, missionID, taskID, artifactID, manifestBytes, manifestSHA256, createdAt); err != nil {
			return Receipt{}, err
		}
		dispositionRequestID := productDeliveryManifestDispositionRequestID(command.RequestID)
		if _, err = tx.Exec(ctx, `INSERT INTO delivery_user_dispositions(
company_id,delivery_id,revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at)
VALUES($1,$2,1,$3,'not_requested',$4,$5,$6,NULL,$7)`, companyID, artifactID, newRevision, productDeliveryManifestCompletionActor,
			"Manifest was completed from verified immutable evidence; user feedback is not requested.", dispositionRequestID, createdAt); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: artifactID, Status: "ready", Revision: newRevision}, nil
	})
	if err != nil {
		return ProductDeliveryManifestCompletionReceipt{}, err
	}
	if writeReceipt.ID != command.ArtifactID || writeReceipt.Status != "ready" || writeReceipt.Revision <= command.ExpectedManifestRevision {
		return ProductDeliveryManifestCompletionReceipt{}, core.Integrity
	}

	var result ProductDeliveryManifestCompletionReceipt
	var createdAt time.Time
	dispositionRequestID := productDeliveryManifestDispositionRequestID(command.RequestID)
	err = k.pool.QueryRow(ctx, `SELECT request_id,company_id,delivery_id,manifest_revision::text,revision::text,state,actor,created_at
FROM delivery_user_dispositions WHERE company_id=$1 AND request_id=$2`, companyID, dispositionRequestID).Scan(
		&result.RequestID, &result.CompanyID, &result.DeliveryID, &result.ManifestRevision, &result.DispositionRevision, &result.State, &result.Actor, &createdAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductDeliveryManifestCompletionReceipt{}, core.Integrity
	}
	if err != nil {
		return ProductDeliveryManifestCompletionReceipt{}, err
	}
	if result.CompanyID != companyID || result.DeliveryID != command.ArtifactID || result.ManifestRevision != strconv.FormatInt(writeReceipt.Revision, 10) || result.State != "not_requested" || result.Actor != productDeliveryManifestCompletionActor {
		return ProductDeliveryManifestCompletionReceipt{}, core.Integrity
	}
	result.RequestID = command.RequestID
	result.State = "ready"
	result.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return result, nil
}

func productDeliveryManifestDispositionRequestID(requestID string) string {
	return "delivery-ready-" + fingerprint(requestID)[:48]
}
