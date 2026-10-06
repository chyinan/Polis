// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type BrowserRunRecord struct {
	CompanyID            string          `json:"companyId"`
	RunID                string          `json:"runId"`
	MissionID            string          `json:"missionId"`
	TaskID               string          `json:"taskId"`
	ServiceTaskID        string          `json:"serviceTaskId"`
	SessionID            string          `json:"sessionId"`
	ServiceJobID         string          `json:"serviceJobId"`
	ServiceGeneration    int             `json:"serviceGeneration"`
	TargetOrigin         string          `json:"targetOrigin"`
	PlanSHA256           string          `json:"planSha256"`
	BrowserBuild         string          `json:"browserBuild"`
	ExecutionEnvironment string          `json:"executionEnvironment"`
	InputRevision        string          `json:"inputRevision"`
	ViewportWidth        int             `json:"viewportWidth"`
	ViewportHeight       int             `json:"viewportHeight"`
	Locale               string          `json:"locale"`
	Timezone             string          `json:"timezone"`
	State                string          `json:"state"`
	ReasonCode           string          `json:"reasonCode"`
	EvidenceManifestSHA  string          `json:"evidenceManifestSha256,omitempty"`
	Evidence             json.RawMessage `json:"evidence"`
	Actor                string          `json:"actor"`
	RequestID            string          `json:"requestId"`
	CreatedAt            string          `json:"createdAt"`
}

func (k *Kernel) TXRequestBrowserRun(ctx context.Context, binding Binding, input BrowserRunRequest, requestID string) (BrowserRunRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(requestID) {
		return BrowserRunRecord{}, core.Malformed
	}
	normalized, err := normalizeBrowserRunRequest(input)
	if err != nil {
		return BrowserRunRecord{}, err
	}
	runID := stableCapabilityID("browser-run", binding.scope.company, requestID)
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "browser.run.request", normalized, func(tx pgx.Tx) (Receipt, error) {
		var currentMission string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id
FROM tasks t JOIN worker_sessions s ON s.company_id=t.company_id AND s.task_id=t.id
WHERE t.company_id=$1 AND t.id=$2 AND s.id=$3 AND s.state='active'`, binding.scope.company, binding.task, binding.session).Scan(&currentMission); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var serviceTaskID, serviceMissionID, readiness string
		var generation int
		if err := tx.QueryRow(ctx, `SELECT j.task_id,t.mission_id,endpoint.generation,endpoint.readiness
FROM job_runs j
JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
JOIN LATERAL (SELECT generation,readiness FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
WHERE j.company_id=$1 AND j.job_id=$2 AND j.kind='service'`, binding.scope.company, normalized.ServiceJobID).Scan(&serviceTaskID, &serviceMissionID, &generation, &readiness); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if serviceMissionID != currentMission {
			return Receipt{}, core.OutOfScope
		}
		if generation != normalized.ServiceGeneration {
			return Receipt{}, core.ConflictError{Reason: "browser target service generation is not current", CurrentState: readiness}
		}
		evidence := json.RawMessage(`{"execution":"disabled","qualification":"not_run"}`)
		if _, err := tx.Exec(ctx, `INSERT INTO browser_runs(
company_id,run_id,mission_id,task_id,service_task_id,session_id,service_job_id,service_generation,target_origin,plan_sha256,browser_build,execution_environment,input_revision,viewport_width,viewport_height,locale,timezone,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, binding.scope.company, runID, currentMission, binding.task, serviceTaskID, binding.session, normalized.ServiceJobID, normalized.ServiceGeneration, normalized.TargetOrigin, normalized.PlanSHA256, normalized.BrowserBuild, normalized.ExecutionEnvironment, normalized.InputRevision, normalized.ViewportWidth, normalized.ViewportHeight, normalized.Locale, normalized.Timezone, requestID); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("browser-run-event", binding.scope.company, requestID)
		if _, err := tx.Exec(ctx, `INSERT INTO browser_run_events(company_id,event_id,run_id,state,reason_code,evidence,actor,request_id)
VALUES($1,$2,$3,'blocked',$4,$5::jsonb,$6,$7)`, binding.scope.company, eventID, runID, BrowserRunBlockedReason, evidence, binding.employee, requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: runID, Status: "blocked"}, nil
	})
	if err != nil {
		return BrowserRunRecord{}, err
	}
	return k.GetBrowserRun(ctx, binding.scope.company, writeReceipt.ID)
}

func (k *Kernel) GetBrowserRun(ctx context.Context, companyID, runID string) (BrowserRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(runID) {
		return BrowserRunRecord{}, core.Malformed
	}
	return readBrowserRun(ctx, k.pool, companyID, runID, "", "")
}

func (k *Kernel) ProductTaskBrowserRunResults(ctx context.Context, binding Binding, runID string) (BrowserRunRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(runID) {
		return BrowserRunRecord{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return BrowserRunRecord{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return BrowserRunRecord{}, err
	}
	record, err := readBrowserRun(ctx, tx, binding.scope.company, runID, binding.task, binding.session)
	if err != nil {
		return BrowserRunRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BrowserRunRecord{}, err
	}
	return record, nil
}

type browserRunQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readBrowserRun(ctx context.Context, queryer browserRunQueryer, companyID, runID, taskID, sessionID string) (BrowserRunRecord, error) {
	var record BrowserRunRecord
	var evidence []byte
	var createdAt time.Time
	query := `SELECT b.company_id,b.run_id,b.mission_id,b.task_id,b.service_task_id,b.session_id,b.service_job_id,b.service_generation,b.target_origin,b.plan_sha256,b.browser_build,b.execution_environment,b.input_revision,b.viewport_width,b.viewport_height,b.locale,b.timezone,e.state,e.reason_code,COALESCE(e.evidence_manifest_sha256,''),e.evidence,e.actor,e.request_id,b.created_at
FROM browser_runs b JOIN LATERAL (SELECT state,reason_code,evidence_manifest_sha256,evidence,actor,request_id FROM browser_run_events WHERE company_id=b.company_id AND run_id=b.run_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE b.company_id=$1 AND b.run_id=$2`
	args := []any{companyID, runID}
	if taskID != "" || sessionID != "" {
		if taskID == "" || sessionID == "" {
			return BrowserRunRecord{}, core.Malformed
		}
		query += ` AND b.task_id=$3 AND b.session_id=$4`
		args = append(args, taskID, sessionID)
	}
	err := queryer.QueryRow(ctx, query, args...).Scan(&record.CompanyID, &record.RunID, &record.MissionID, &record.TaskID, &record.ServiceTaskID, &record.SessionID, &record.ServiceJobID, &record.ServiceGeneration, &record.TargetOrigin, &record.PlanSHA256, &record.BrowserBuild, &record.ExecutionEnvironment, &record.InputRevision, &record.ViewportWidth, &record.ViewportHeight, &record.Locale, &record.Timezone, &record.State, &record.ReasonCode, &record.EvidenceManifestSHA, &evidence, &record.Actor, &record.RequestID, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BrowserRunRecord{}, core.OutOfScope
	}
	if err != nil {
		return BrowserRunRecord{}, err
	}
	if !json.Valid(evidence) {
		return BrowserRunRecord{}, core.Integrity
	}
	if record.State == "succeeded" {
		canonicalEvidence, canonicalErr := CanonicalOperationEvidenceJSON(evidence)
		if canonicalErr != nil || !validCapabilityDigest(record.EvidenceManifestSHA) || digestCapabilityBytes(canonicalEvidence) != record.EvidenceManifestSHA {
			return BrowserRunRecord{}, core.Integrity
		}
		var manifest OperationEvidenceManifest
		if err := json.Unmarshal(canonicalEvidence, &manifest); err != nil || ValidateOperationEvidenceManifest(manifest, record.CompanyID, record.MissionID, record.TaskID, record.RunID) != nil {
			return BrowserRunRecord{}, core.Integrity
		}
		for _, artifact := range manifest.Artifacts {
			var artifactTaskID, artifactMissionID, artifactDigest, artifactState, artifactVerdict string
			if err := queryer.QueryRow(ctx, `SELECT a.task_id,t.mission_id,a.digest,a.state,a.verdict
FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND a.id=$2`, record.CompanyID, artifact.ArtifactID).Scan(&artifactTaskID, &artifactMissionID, &artifactDigest, &artifactState, &artifactVerdict); err != nil {
				return BrowserRunRecord{}, core.Integrity
			}
			if artifactTaskID != record.TaskID || artifactMissionID != record.MissionID || artifactDigest != artifact.Digest || artifactState != "ready" || (artifactVerdict != "candidate" && artifactVerdict != "passed") {
				return BrowserRunRecord{}, core.Integrity
			}
		}
	}
	record.Evidence = append(json.RawMessage(nil), evidence...)
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}
