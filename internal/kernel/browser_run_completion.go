// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type BrowserRunSuccessInput struct {
	RunID     string
	Manifest  OperationEvidenceManifest
	RequestID string
}

// TXCompleteBrowserRunSuccess appends the success event after an external
// browser runner has produced immutable Artifact rows. The runner itself is
// deliberately not called here: this is the database-bound completion fence.
func (k *Kernel) TXCompleteBrowserRunSuccess(ctx context.Context, binding Binding, input BrowserRunSuccessInput) (BrowserRunRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(input.RunID) || !core.ValidID(input.RequestID) {
		return BrowserRunRecord{}, core.Malformed
	}
	if err := ValidateOperationEvidenceManifest(input.Manifest, binding.scope.company, input.Manifest.MissionID, binding.task, input.RunID); err != nil {
		return BrowserRunRecord{}, core.Integrity
	}
	canonicalManifest, err := CanonicalOperationEvidenceJSON(mustMarshalOperationEvidenceManifest(input.Manifest))
	if err != nil {
		return BrowserRunRecord{}, core.Integrity
	}
	manifestSHA := digestCapabilityBytes(canonicalManifest)
	_, err = k.TXWrite(ctx, binding.scope, &binding, input.RequestID, "browser.run.complete", input, func(tx pgx.Tx) (Receipt, error) {
		var missionID, taskID, sessionID string
		var generation int
		if err := tx.QueryRow(ctx, `SELECT mission_id,task_id,session_id,service_generation
FROM browser_runs WHERE company_id=$1 AND run_id=$2 FOR UPDATE`, binding.scope.company, input.RunID).Scan(&missionID, &taskID, &sessionID, &generation); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if taskID != binding.task || sessionID != binding.session || missionID != input.Manifest.MissionID || generation < 1 {
			return Receipt{}, core.OutOfScope
		}
		var currentState string
		if err := tx.QueryRow(ctx, `SELECT state FROM browser_run_events WHERE company_id=$1 AND run_id=$2 ORDER BY event_seq DESC LIMIT 1`, binding.scope.company, input.RunID).Scan(&currentState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		} else if err != nil {
			return Receipt{}, err
		}
		if currentState != "blocked" && currentState != "requested" && currentState != "running" {
			return Receipt{}, core.Conflict
		}
		if err := validateOperationEvidenceArtifacts(ctx, tx, input.Manifest); err != nil {
			return Receipt{}, core.Integrity
		}
		eventID := stableCapabilityID("browser-run-success-event", binding.scope.company, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO browser_run_events(company_id,event_id,run_id,state,reason_code,evidence_manifest_sha256,evidence,actor,request_id)
VALUES($1,$2,$3,'succeeded','browser_run_succeeded',$4,$5::jsonb,$6,$7)`, binding.scope.company, eventID, input.RunID, manifestSHA, canonicalManifest, binding.employee, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.RunID, Status: "succeeded"}, nil
	})
	if err != nil {
		return BrowserRunRecord{}, err
	}
	return k.GetBrowserRun(ctx, binding.scope.company, input.RunID)
}
