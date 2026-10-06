// pattern: Imperative Shell
package kernel

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type operationEvidenceArtifactQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validateOperationEvidenceArtifacts(ctx context.Context, queryer operationEvidenceArtifactQueryer, manifest OperationEvidenceManifest) error {
	for _, artifact := range manifest.Artifacts {
		var taskID, missionID, digest, state, verdict string
		if err := queryer.QueryRow(ctx, `SELECT a.task_id,t.mission_id,a.digest,a.state,a.verdict
FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND a.id=$2`, manifest.CompanyID, artifact.ArtifactID).Scan(&taskID, &missionID, &digest, &state, &verdict); err != nil {
			return err
		}
		if taskID != manifest.TaskID || missionID != manifest.MissionID || digest != artifact.Digest || state != "ready" || (verdict != "candidate" && verdict != "passed") {
			return pgx.ErrNoRows
		}
	}
	return nil
}
