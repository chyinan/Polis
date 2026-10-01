// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type PeerReviewEvidence struct {
	SubjectRevision    string   `json:"subject_revision"`
	TaskInput          string   `json:"task_input"`
	IntegrationID      string   `json:"integration_id"`
	ContractRevisionID string   `json:"contract_revision_id"`
	BackendArtifactID  string   `json:"backend_artifact_id"`
	FrontendArtifactID string   `json:"frontend_artifact_id"`
	BackendDigest      string   `json:"backend_digest"`
	FrontendDigest     string   `json:"frontend_digest"`
	AllowedRefs        []string `json:"allowed_refs"`
}

type PeerIntegrationView struct {
	IntegrationID      string               `json:"integration_id"`
	Mission            string               `json:"mission"`
	Contract           PeerContractRevision `json:"contract"`
	BackendArtifactID  string               `json:"backend_artifact_id"`
	FrontendArtifactID string               `json:"frontend_artifact_id"`
	BackendDigest      string               `json:"backend_digest"`
	FrontendDigest     string               `json:"frontend_digest"`
	BackendContent     string               `json:"backend_content"`
	FrontendContent    string               `json:"frontend_content"`
}

func (k *Kernel) TXCreatePeerReview(ctx context.Context, s Scope, integrationID, subjectRevision, taskInput, key string) (Task, PeerReviewEvidence, error) {
	var task Task
	var evidence PeerReviewEvidence
	_, e := k.TXWrite(ctx, s, nil, key, "review.task.create", []string{integrationID, subjectRevision, taskInput}, func(tx pgx.Tx) (Receipt, error) {
		var mission, contractID, backendID, frontendID, state string
		if e := tx.QueryRow(ctx, "SELECT mission_id,contract_revision_id,backend_artifact_id,frontend_artifact_id,state FROM integration_candidates WHERE company_id=$1 AND id=$2", s.company, integrationID).Scan(&mission, &contractID, &backendID, &frontendID, &state); e != nil {
			return Receipt{}, e
		}
		if state != "passed" {
			return Receipt{}, core.Denied
		}
		var backendDigest, frontendDigest string
		if e := tx.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND verdict='candidate'", s.company, backendID).Scan(&backendDigest); e != nil {
			return Receipt{}, e
		}
		if e := tx.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND verdict='candidate'", s.company, frontendID).Scan(&frontendDigest); e != nil {
			return Receipt{}, e
		}
		id := newID()
		plan, e := json.Marshal(map[string]any{"review_of": integrationID, "independence": "emp-review", "contract": "r03-api@1", "backend_artifact": backendID, "frontend_artifact": frontendID, "contract_revision": contractID})
		if e != nil {
			return Receipt{}, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state,plan) VALUES($1,$2,$3,'emp-review','peer_review','ready',$4)", s.company, id, mission, plan); e != nil {
			return Receipt{}, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", s.company, id, frontendDigest); e != nil {
			return Receipt{}, e
		}
		task = Task{ID: id, Mission: mission, Owner: "emp-review", Kind: "peer_review", State: "ready"}
		evidence = PeerReviewEvidence{SubjectRevision: subjectRevision, TaskInput: taskInput, IntegrationID: integrationID, ContractRevisionID: contractID, BackendArtifactID: backendID, FrontendArtifactID: frontendID, BackendDigest: backendDigest, FrontendDigest: frontendDigest, AllowedRefs: []string{subjectRevision, integrationID, contractID, backendID, frontendID, backendDigest, frontendDigest, "backend.go", "frontend.go", "r03-api@1"}}
		return Receipt{ID: id, Status: "ready"}, nil
	})
	return task, evidence, e
}

func (k *Kernel) PeerIntegrationView(ctx context.Context, b Binding, evidence PeerReviewEvidence) (PeerIntegrationView, error) {
	var view PeerIntegrationView
	view.IntegrationID, view.BackendArtifactID, view.FrontendArtifactID = evidence.IntegrationID, evidence.BackendArtifactID, evidence.FrontendArtifactID
	if e := k.pool.QueryRow(ctx, "SELECT mission_id,contract_revision_id FROM integration_candidates WHERE company_id=$1 AND id=$2", b.scope.company, evidence.IntegrationID).Scan(&view.Mission, &view.Contract.ID); e != nil {
		return view, e
	}
	var backendDigest, frontendDigest string
	if e := k.pool.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2", b.scope.company, evidence.BackendArtifactID).Scan(&backendDigest); e != nil {
		return view, e
	}
	if e := k.pool.QueryRow(ctx, "SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2", b.scope.company, evidence.FrontendArtifactID).Scan(&frontendDigest); e != nil {
		return view, e
	}
	view.BackendDigest, view.FrontendDigest = backendDigest, frontendDigest
	backendContent, _ := readBlob(k.root, b.scope.company, backendDigest)
	frontendContent, _ := readBlob(k.root, b.scope.company, frontendDigest)
	view.BackendContent, view.FrontendContent = string(backendContent), string(frontendContent)
	view.Contract, _ = k.PeerContractRead(ctx, b, evidence.ContractRevisionID)
	return view, nil
}

func (k *Kernel) TXPeerReviewSubmit(ctx context.Context, b Binding, evidence PeerReviewEvidence, submission ReviewerSubmission, key string) (Receipt, error) {
	return k.TXWrite(ctx, b.scope, &b, key, "review.submit", submission, func(tx pgx.Tx) (Receipt, error) {
		var task, kind, owner, plan string
		if e := tx.QueryRow(ctx, "SELECT t.id,t.kind,t.owner,t.plan::text FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&task, &kind, &owner, &plan); e != nil {
			return Receipt{}, e
		}
		var planData struct {
			ReviewOf string `json:"review_of"`
		}
		if json.Unmarshal([]byte(plan), &planData) != nil || kind != "peer_review" || owner != "emp-review" || planData.ReviewOf != evidence.IntegrationID {
			return Receipt{}, core.Denied
		}
		var state string
		if e := tx.QueryRow(ctx, "SELECT state FROM integration_candidates WHERE company_id=$1 AND id=$2", b.scope.company, evidence.IntegrationID).Scan(&state); e != nil {
			return Receipt{}, e
		}
		if state != "passed" {
			return Receipt{}, core.Denied
		}
		findings, e := json.Marshal(submission.Findings)
		if e != nil {
			return Receipt{}, e
		}
		evidenceRefs, e := json.Marshal(submission.Evidence)
		if e != nil {
			return Receipt{}, e
		}
		limitations, e := json.Marshal(submission.Limitations)
		if e != nil {
			return Receipt{}, e
		}
		if e = requireMemoryTaskCleanTX(ctx, tx, b.scope.company, task); e != nil {
			return Receipt{}, e
		}
		id := newID()
		if _, e = tx.Exec(ctx, "INSERT INTO review_records(company_id,id,task_id,integration_id,verdict,findings,evidence,confidence,limitations) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", b.scope.company, id, task, evidence.IntegrationID, submission.Verdict, findings, evidenceRefs, submission.Confidence, limitations); e != nil {
			return Receipt{}, e
		}
		if _, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", b.scope.company, task); e != nil {
			return Receipt{}, e
		}
		return Receipt{ID: id, Status: submission.Verdict}, nil
	})
}

func (k *Kernel) PeerReviewRecord(ctx context.Context, s Scope, integrationID string) (ReviewerSubmission, error) {
	var out ReviewerSubmission
	var findings, evidence, limitations []byte
	if e := k.pool.QueryRow(ctx, "SELECT verdict,findings,evidence,confidence,limitations FROM review_records WHERE company_id=$1 AND integration_id=$2", s.company, integrationID).Scan(&out.Verdict, &findings, &evidence, &out.Confidence, &limitations); e != nil {
		return out, e
	}
	if e := json.Unmarshal(findings, &out.Findings); e != nil {
		return out, e
	}
	if e := json.Unmarshal(evidence, &out.Evidence); e != nil {
		return out, e
	}
	if e := json.Unmarshal(limitations, &out.Limitations); e != nil {
		return out, e
	}
	return out, nil
}
