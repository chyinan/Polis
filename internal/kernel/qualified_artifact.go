// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// Runs entirely under TXWrite's company row lock, shared by contract.accept,
// workspace.replace and checkpoint creation. No pre-transaction check grants
// finalization authority. The existing workspace blob is already durable.
func (k *Kernel) TXSubmitQualifiedPeer(ctx context.Context, b Binding, w Task, key string, content []byte) (Receipt, error) {
	digest := digestPeerContent(content)
	return k.TXWrite(ctx, b.scope, &b, key, "artifact.submit", []string{w.ID, digest}, func(tx pgx.Tx) (Receipt, error) {
		task, err := checkWork(ctx, tx, b, w)
		if err != nil {
			return Receipt{}, err
		}
		if b.session == "" || task.State != "working" || (task.Kind != "peer_backend" && task.Kind != "peer_frontend") {
			return Receipt{}, core.Denied
		}
		if err = requireMemoryTaskCleanTX(ctx, tx, b.scope.company, task.ID); err != nil {
			return Receipt{}, err
		}
		var checkpointID, contractID string
		var revision int64
		err = tx.QueryRow(ctx, `SELECT cp.id,c.id,ws.revision
FROM worker_workspaces ws
JOIN contract_revisions c ON c.company_id=ws.company_id AND c.mission_id=$4 AND c.state='accepted'
JOIN worker_sessions s ON s.company_id=ws.company_id AND s.task_id=ws.task_id AND s.id=$2
JOIN worker_checkpoints cp ON cp.company_id=s.company_id AND cp.session_id=s.id AND cp.digest=ws.digest
WHERE ws.company_id=$1 AND ws.task_id=$3 AND ws.digest=$5
 AND cp.data->>'kind'='qualified'
 AND cp.data->>'finalization_state'='current'
 AND cp.data->>'workspace_digest'=ws.digest
 AND cp.data->>'workspace_revision'=ws.revision::text
 AND cp.data->>'contract_revision_id'=c.id
 AND cp.data->>'acceptance_checker_revision'=$6
 AND cp.data->>'checkpoint_policy_revision'=$7
 AND cp.data->>'artifact_eligibility_policy_revision'=$8
 AND cp.data->>'contract_supersession_policy_revision'=$9
ORDER BY cp.id LIMIT 1`, b.scope.company, b.session, task.ID, task.Mission, digest, core.PeerAcceptanceCheckerRevision, core.CheckpointPolicyRevision, core.ArtifactEligibilityPolicyRevision, core.PeerContractSupersessionPolicyRevision).Scan(&checkpointID, &contractID, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		}
		if err != nil {
			return Receipt{}, err
		}
		stored, err := readBlob(k.root, b.scope.company, digest)
		if err != nil {
			return Receipt{}, err
		}
		if !bytes.Equal(stored, content) {
			return Receipt{}, core.Integrity
		}
		id := newID()
		if _, err = tx.Exec(ctx, "INSERT INTO artifact_staging(company_id,id,task_id,digest) VALUES($1,$2,$3,$4)", b.scope.company, id, task.ID, digest); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,contract) VALUES($1,$2,$3,$4,$5,$6,'ready','r03-api@1')", b.scope.company, id, task.ID, b.employee, digest, len(content)); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO artifact_qualifications(company_id,artifact_id,checkpoint_id,workspace_revision,workspace_digest,contract_revision_id,acceptance_checker_revision,checkpoint_policy_revision,artifact_eligibility_policy_revision,contract_supersession_policy_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, b.scope.company, id, checkpointID, revision, digest, contractID, core.PeerAcceptanceCheckerRevision, core.CheckpointPolicyRevision, core.ArtifactEligibilityPolicyRevision, core.PeerContractSupersessionPolicyRevision); err != nil {
			return Receipt{}, err
		}
		_, err = tx.Exec(ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2", b.scope.company, task.ID)
		return Receipt{ID: id, Status: "candidate"}, err
	})
}
