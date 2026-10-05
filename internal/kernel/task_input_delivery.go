// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
)

func (k *Kernel) EnsureTaskInputManifest(ctx context.Context, scope Scope, missionID, taskID, requestID string) error {
	if !core.ValidID(missionID) || !core.ValidID(taskID) || !core.ValidID(requestID) {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, scope, nil, requestID, "task.input.manifest.bind", struct{ MissionID, TaskID string }{missionID, taskID}, func(tx pgx.Tx) (Receipt, error) {
		var actualMission, taskState string
		if err := tx.QueryRow(ctx, `SELECT mission_id,state FROM tasks WHERE company_id=$1 AND id=$2 FOR UPDATE`, scope.company, taskID).Scan(&actualMission, &taskState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if actualMission != missionID {
			return Receipt{}, core.OutOfScope
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task_input_manifests WHERE company_id=$1 AND task_id=$2)`, scope.company, taskID).Scan(&exists); err != nil {
			return Receipt{}, err
		}
		if exists {
			return Receipt{ID: taskID, Status: "bound"}, nil
		}
		if taskState != "ready" {
			return Receipt{}, core.ConflictError{Reason: "input manifest cannot be created after Task work has started", CurrentState: taskState}
		}
		if err := bindMissionInputManifest(ctx, tx, scope, missionID, taskID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: taskID, Status: "bound"}, nil
	})
	return err
}

func (k *Kernel) TXPrepareProductTaskInputDelivery(ctx context.Context, binding Binding, input ProductTaskInputContext) (string, string, error) {
	if !core.ValidID(binding.session) || !core.ValidID(binding.task) || !validTaskInputDigest(input.ManifestDigest) || !validTaskInputDigest(input.Payload.PayloadDigest) || input.Payload.ManifestDigest != input.ManifestDigest {
		return "", "", core.Malformed
	}
	canonicalContext, err := k.ProductTaskInputContext(ctx, binding)
	if err != nil {
		return "", "", err
	}
	if err = verifyProductTaskInputContext(canonicalContext, input); err != nil {
		return "", "", err
	}
	verifiedPrompt := canonicalContext.Payload.PromptSection
	noRequiredInputs := len(input.Payload.Inputs) == 0 && len(input.Payload.Images) == 0 && len(input.Payload.CSVs) == 0 && len(input.Payload.Excluded) == 0
	refs := make([]intake.ModelInputDeliveryRef, 0, len(input.Payload.Inputs)+len(input.Payload.Images)+len(input.Payload.CSVs))
	for _, item := range input.Payload.Inputs {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	for _, item := range input.Payload.Images {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	for _, item := range input.Payload.CSVs {
		summaryBytes, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			return "", "", marshalErr
		}
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, MediaType: item.Reference.MediaType, ByteSize: item.Reference.ByteSize, ContentDigest: item.SourceDigest, Representation: "csv_table_summary", RepresentationBytes: int64(len(summaryBytes))})
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return "", "", err
	}
	exclusionsJSON, err := json.Marshal(input.Payload.Excluded)
	if err != nil {
		return "", "", err
	}
	// A WorkerSession has one Task input delivery. Keep this identifier stable
	// so retries of the same idempotent preparation return the original receipt.
	deliveryID := "task-input-" + binding.session
	requestID := "task-input-prepared-" + binding.session
	preparedReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "task.input.delivery.prepared", struct {
		DeliveryID string
		Manifest   string
		Payload    string
	}{deliveryID, input.ManifestDigest, input.Payload.PayloadDigest}, func(tx pgx.Tx) (Receipt, error) {
		var currentManifest string
		if err := tx.QueryRow(ctx, `SELECT manifest_digest FROM task_input_manifests WHERE company_id=$1 AND task_id=$2`, binding.scope.company, binding.task).Scan(&currentManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentManifest != canonicalContext.ManifestDigest {
			return Receipt{}, core.Conflict
		}
		if _, err := tx.Exec(ctx, `INSERT INTO task_input_delivery_attempts(company_id,delivery_id,task_id,session_id,phase,outcome,manifest_digest,payload_digest,input_refs,input_exclusions,provider_egress)
VALUES($1,$2,$3,$4,'prepared','prepared',$5,$6,$7,$8,0)`, binding.scope.company, deliveryID, binding.task, binding.session, input.ManifestDigest, input.Payload.PayloadDigest, refsJSON, exclusionsJSON); err != nil {
			return Receipt{}, err
		}
		if noRequiredInputs {
			if _, err := tx.Exec(ctx, `INSERT INTO task_input_delivery_attempts(company_id,delivery_id,task_id,session_id,phase,outcome,manifest_digest,payload_digest,input_refs,input_exclusions,provider_egress)
VALUES($1,$2,$3,$4,'final','not_required',$5,$6,$7,$8,0)`, binding.scope.company, deliveryID, binding.task, binding.session, input.ManifestDigest, input.Payload.PayloadDigest, refsJSON, exclusionsJSON); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: deliveryID, Status: "not_required"}, nil
		}
		return Receipt{ID: deliveryID, Status: "prepared"}, nil
	})
	if err != nil {
		return "", "", err
	}
	if noRequiredInputs {
		return "", verifiedPrompt, nil
	}
	return preparedReceipt.ID, verifiedPrompt, nil
}

func (k *Kernel) TXCompleteProductTaskInputDelivery(ctx context.Context, binding Binding, deliveryID, outcome string, providerEgress int) error {
	if !core.ValidID(binding.session) || !core.ValidID(deliveryID) || providerEgress < 0 || providerEgress > 100 {
		return core.Malformed
	}
	valid := (outcome == "provider_delivered" && providerEgress > 0) ||
		(outcome == "outcome_unknown" && providerEgress > 0) ||
		(outcome == "local_context_loaded" && providerEgress == 0) ||
		(outcome == "not_sent" && providerEgress == 0) ||
		(outcome == "not_required" && providerEgress == 0)
	if !valid {
		return core.Malformed
	}
	requestID := "task-input-final-" + binding.session
	_, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "task.input.delivery.final", struct {
		DeliveryID     string
		Outcome        string
		ProviderEgress int
	}{deliveryID, outcome, providerEgress}, func(tx pgx.Tx) (Receipt, error) {
		var manifestDigest, payloadDigest string
		var inputRefs, inputExclusions []byte
		if err := tx.QueryRow(ctx, `SELECT manifest_digest,payload_digest,input_refs,input_exclusions FROM task_input_delivery_attempts
WHERE company_id=$1 AND delivery_id=$2 AND session_id=$3 AND phase='prepared'`, binding.scope.company, deliveryID, binding.session).Scan(&manifestDigest, &payloadDigest, &inputRefs, &inputExclusions); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if outcome == "not_required" {
			var refs []intake.ModelInputDeliveryRef
			var exclusions []intake.ModelInputExclusion
			if json.Unmarshal(inputRefs, &refs) != nil || json.Unmarshal(inputExclusions, &exclusions) != nil || len(refs) != 0 || len(exclusions) != 0 {
				return Receipt{}, core.Denied
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO task_input_delivery_attempts(company_id,delivery_id,task_id,session_id,phase,outcome,manifest_digest,payload_digest,input_refs,input_exclusions,provider_egress)
VALUES($1,$2,$3,$4,'final',$5,$6,$7,$8,$9,$10)`, binding.scope.company, deliveryID, binding.task, binding.session, outcome, manifestDigest, payloadDigest, inputRefs, inputExclusions, providerEgress); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: deliveryID, Status: outcome}, nil
	})
	return err
}

func validTaskInputDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
