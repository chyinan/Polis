// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ResearchOperationSuccessInput struct {
	OperationID string
	Envelope    ResearchOperationEvidenceEnvelope
	RequestID   string
}

// TXCompleteResearchOperationSuccess is the database-bound handoff from a
// qualified retrieval executor to the immutable operation ledger. It never
// performs retrieval itself.
func (k *Kernel) TXCompleteResearchOperationSuccess(ctx context.Context, binding Binding, input ResearchOperationSuccessInput) (ResearchOperationRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(input.OperationID) || !core.ValidID(input.RequestID) {
		return ResearchOperationRecord{}, core.Malformed
	}
	_, err := k.TXWrite(ctx, binding.scope, &binding, input.RequestID, "research.operation.complete", input, func(tx pgx.Tx) (Receipt, error) {
		var missionID, taskID, sessionID, kind string
		if err := tx.QueryRow(ctx, `SELECT mission_id,task_id,session_id,kind
FROM research_operations WHERE company_id=$1 AND operation_id=$2 FOR UPDATE`, binding.scope.company, input.OperationID).Scan(&missionID, &taskID, &sessionID, &kind); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if taskID != binding.task || sessionID != binding.session {
			return Receipt{}, core.OutOfScope
		}
		rawEnvelope, err := json.Marshal(input.Envelope)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		if _, err := ValidateResearchOperationEvidenceEnvelope(rawEnvelope, binding.scope.company, missionID, binding.task, input.OperationID); err != nil {
			return Receipt{}, core.Integrity
		}
		if err := validateOperationEvidenceArtifacts(ctx, tx, input.Envelope.EvidenceManifest); err != nil {
			return Receipt{}, core.Integrity
		}
		var currentState string
		if err := tx.QueryRow(ctx, `SELECT state FROM research_operation_events WHERE company_id=$1 AND operation_id=$2 ORDER BY event_seq DESC LIMIT 1`, binding.scope.company, input.OperationID).Scan(&currentState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		} else if err != nil {
			return Receipt{}, err
		}
		if currentState != "unavailable" && currentState != "requested" && currentState != "running" {
			return Receipt{}, core.Conflict
		}
		reasonCode := "research_" + kind + "_succeeded"
		eventID := stableCapabilityID("research-operation-success-event", binding.scope.company, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO research_operation_events(company_id,event_id,operation_id,state,reason_code,result,actor,request_id)
VALUES($1,$2,$3,'succeeded',$4,$5::jsonb,$6,$7)`, binding.scope.company, eventID, input.OperationID, reasonCode, rawEnvelope, binding.employee, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.OperationID, Status: "succeeded"}, nil
	})
	if err != nil {
		return ResearchOperationRecord{}, err
	}
	return k.GetResearchOperation(ctx, binding.scope.company, input.OperationID)
}
