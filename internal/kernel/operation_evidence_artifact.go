// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const maxOperationEvidenceArtifactBytes int64 = 8 << 20

type OperationEvidenceArtifactInput struct {
	OperationID string
	Kind        string
	Content     []byte
	RequestID   string
}

// TXStoreOperationEvidenceArtifact stores one immutable CAS-backed evidence
// Artifact for an existing BrowserRun or ResearchOperation. It does not mark
// the operation succeeded; the operation completion fence does that later.
func (k *Kernel) TXStoreOperationEvidenceArtifact(ctx context.Context, binding Binding, input OperationEvidenceArtifactInput) (OperationEvidenceArtifactRef, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(input.OperationID) || !core.ValidID(input.RequestID) || !validOperationEvidenceKind(input.Kind) || len(input.Content) < 1 || int64(len(input.Content)) > maxOperationEvidenceArtifactBytes {
		return OperationEvidenceArtifactRef{}, core.Malformed
	}
	digest := digestCapabilityBytes(input.Content)
	if _, err := putBlob(k.root, binding.scope.company, input.Content); err != nil {
		return OperationEvidenceArtifactRef{}, err
	}
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, input.RequestID, "operation.evidence.artifact", []any{input.OperationID, input.Kind, digest, len(input.Content)}, func(tx pgx.Tx) (Receipt, error) {
		var taskID, missionID, sessionID string
		err := tx.QueryRow(ctx, `SELECT task_id,mission_id,session_id
FROM research_operations WHERE company_id=$1 AND operation_id=$2`, binding.scope.company, input.OperationID).Scan(&taskID, &missionID, &sessionID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT task_id,mission_id,session_id
FROM browser_runs WHERE company_id=$1 AND run_id=$2`, binding.scope.company, input.OperationID).Scan(&taskID, &missionID, &sessionID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if taskID != binding.task || sessionID != binding.session {
			return Receipt{}, core.OutOfScope
		}
		artifactID := newID()
		if _, err := tx.Exec(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract,artifact_kind)
VALUES($1,$2,$3,$4,$5,$6,'ready','candidate','operation-evidence@1','operation_evidence')`, binding.scope.company, artifactID, taskID, binding.employee, digest, len(input.Content)); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: artifactID, Status: "candidate"}, nil
	})
	if err != nil {
		return OperationEvidenceArtifactRef{}, err
	}
	return OperationEvidenceArtifactRef{ArtifactID: writeReceipt.ID, Digest: digest, Kind: input.Kind}, nil
}
