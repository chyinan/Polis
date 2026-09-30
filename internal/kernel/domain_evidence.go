// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

const maxDomainEvidenceSubmissions = 100

type DomainEvidenceRecord struct {
	CompanyID           string                                     `json:"companyId"`
	RecordID            string                                     `json:"recordId"`
	ProfileID           string                                     `json:"profileId"`
	ProfileRevision     string                                     `json:"profileRevision"`
	ReadinessStatus     domainworkflow.DomainEvidenceStatus        `json:"readinessStatus"`
	QualificationStatus string                                     `json:"qualificationStatus"`
	ExecutionEnabled    bool                                       `json:"executionEnabled"`
	EvidenceDigest      string                                     `json:"evidenceDigest"`
	Submission          domainworkflow.DomainEvidenceSubmission    `json:"submission"`
	ReasonCodes         []string                                   `json:"reasonCodes"`
	RequestID           string                                     `json:"requestId"`
	CreatedAt           string                                     `json:"createdAt"`
	Review              *DomainEvidenceReviewRecord                `json:"review"`
	Assessment          *DomainEvidenceSubstantiveAssessmentRecord `json:"assessment"`
}

type DomainEvidenceLedger struct {
	CompanyID              string                             `json:"companyId"`
	Profiles               []domainworkflow.WorkflowProfile   `json:"profiles"`
	Submissions            []DomainEvidenceRecord             `json:"submissions"`
	Qualifications         []DomainProfileQualificationRecord `json:"qualifications"`
	ResearchSimulationRuns []ResearchSimulationRunRecord      `json:"researchSimulationRuns"`
	ContentSourceEvents    []DomainContentSourceEventRecord   `json:"contentSourceEvents"`
	ContentDrafts          []DomainContentDraftRecord         `json:"contentDrafts"`
	ContentReviews         []DomainContentReviewRecord        `json:"contentReviews"`
	ContentPublications    []DomainContentPublicationRecord   `json:"contentPublications"`
	ContentCorrections     []DomainContentCorrectionRecord    `json:"contentCorrections"`
	ContentFeedback        []DomainContentFeedbackRecord      `json:"contentFeedback"`
}

type domainEvidenceItemQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (k *Kernel) ListDomainEvidence(ctx context.Context, companyID string) (DomainEvidenceLedger, error) {
	if !core.ValidID(companyID) {
		return DomainEvidenceLedger{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	defer tx.Rollback(ctx)
	var companyExists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)", companyID).Scan(&companyExists); err != nil {
		return DomainEvidenceLedger{}, err
	}
	if !companyExists {
		return DomainEvidenceLedger{}, core.OutOfScope
	}
	ledger := DomainEvidenceLedger{
		CompanyID: companyID, Profiles: domainworkflow.ReferenceWorkflows(),
		Submissions: []DomainEvidenceRecord{}, Qualifications: []DomainProfileQualificationRecord{}, ResearchSimulationRuns: []ResearchSimulationRunRecord{},
		ContentSourceEvents: []DomainContentSourceEventRecord{}, ContentDrafts: []DomainContentDraftRecord{}, ContentReviews: []DomainContentReviewRecord{},
		ContentPublications: []DomainContentPublicationRecord{}, ContentCorrections: []DomainContentCorrectionRecord{}, ContentFeedback: []DomainContentFeedbackRecord{},
	}
	rows, err := tx.Query(ctx, `SELECT record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id,created_at::text
FROM domain_workflow_evidence_submissions WHERE company_id=$1
ORDER BY created_at DESC,record_id DESC LIMIT $2`, companyID, maxDomainEvidenceSubmissions)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	for rows.Next() {
		var record DomainEvidenceRecord
		var evidenceJSON, reasonsJSON []byte
		if err = rows.Scan(&record.RecordID, &record.ProfileID, &record.ProfileRevision, &record.ReadinessStatus, &record.QualificationStatus, &record.ExecutionEnabled, &record.EvidenceDigest, &evidenceJSON, &reasonsJSON, &record.RequestID, &record.CreatedAt); err != nil {
			rows.Close()
			return DomainEvidenceLedger{}, err
		}
		if err = json.Unmarshal(evidenceJSON, &record.Submission); err != nil {
			rows.Close()
			return DomainEvidenceLedger{}, core.Integrity
		}
		if err = json.Unmarshal(reasonsJSON, &record.ReasonCodes); err != nil {
			rows.Close()
			return DomainEvidenceLedger{}, core.Integrity
		}
		record.CompanyID = companyID
		if !validPersistedDomainEvidenceRecord(record) {
			rows.Close()
			return DomainEvidenceLedger{}, core.Integrity
		}
		ledger.Submissions = append(ledger.Submissions, record)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return DomainEvidenceLedger{}, err
	}
	for index := range ledger.Submissions {
		record := &ledger.Submissions[index]
		itemsMatch, matchErr := domainEvidenceItemsMatch(ctx, tx, companyID, *record)
		if matchErr != nil {
			return DomainEvidenceLedger{}, matchErr
		}
		if !itemsMatch {
			return DomainEvidenceLedger{}, core.Integrity
		}
		review, reviewErr := readDomainEvidenceReview(ctx, tx, companyID, record.RecordID)
		if reviewErr != nil {
			return DomainEvidenceLedger{}, reviewErr
		}
		if review != nil && !validPersistedDomainEvidenceReview(record.Submission, *review) {
			return DomainEvidenceLedger{}, core.Integrity
		}
		record.Review = review
		assessment, assessmentErr := readDomainEvidenceSubstantiveAssessment(ctx, tx, companyID, record.RecordID)
		if assessmentErr != nil {
			return DomainEvidenceLedger{}, assessmentErr
		}
		if assessment != nil && !validPersistedDomainEvidenceSubstantiveAssessment(record.Submission, record.EvidenceDigest, review, *assessment) {
			return DomainEvidenceLedger{}, core.Integrity
		}
		record.Assessment = assessment
	}
	ledger.Qualifications, err = k.listCurrentDomainProfileQualifications(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ResearchSimulationRuns, err = k.listResearchSimulationRuns(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentSourceEvents, err = listDomainContentSourceEvents(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentDrafts, err = k.listDomainContentDrafts(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentReviews, err = k.listDomainContentReviews(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentPublications, err = k.listDomainContentPublications(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentCorrections, err = k.listDomainContentCorrections(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	ledger.ContentFeedback, err = k.listDomainContentFeedback(ctx, tx, companyID)
	if err != nil {
		return DomainEvidenceLedger{}, err
	}
	for _, qualification := range ledger.Qualifications {
		for index := range ledger.Profiles {
			if ledger.Profiles[index].ID == qualification.ProfileID && ledger.Profiles[index].Revision == qualification.ProfileRevision {
				ledger.Profiles[index].QualificationStatus = string(qualification.Decision)
				ledger.Profiles[index].ExecutionEnabled = false
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return DomainEvidenceLedger{}, err
	}
	return ledger, nil
}

func (k *Kernel) TXRecordDomainEvidence(ctx context.Context, companyID string, submission domainworkflow.DomainEvidenceSubmission, requestID string) (DomainEvidenceRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return DomainEvidenceRecord{}, core.Malformed
	}
	if submission.Evidence == nil {
		submission.Evidence = []domainworkflow.DomainEvidence{}
	}
	readiness := domainworkflow.EvaluateDomainEvidence(submission)
	if readiness.Status == domainworkflow.DomainEvidenceRejected {
		return DomainEvidenceRecord{}, core.Malformed
	}
	evidenceDigest, evidenceJSON, err := domainEvidenceSubmissionDigest(submission)
	if err != nil {
		return DomainEvidenceRecord{}, core.Malformed
	}
	recordID := domainEvidenceRecordID(companyID, requestID)
	input := struct {
		Submission domainworkflow.DomainEvidenceSubmission
		RequestID  string
	}{submission, requestID}
	scope := k.LocalScope(companyID)
	_, err = k.TXWrite(ctx, scope, nil, requestID, "domain.evidence.submit", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		for _, evidence := range submission.Evidence {
			var assessorExists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)", companyID, evidence.AssessedByEmployeeID).Scan(&assessorExists); err != nil {
				return Receipt{}, err
			}
			if !assessorExists {
				return Receipt{}, core.OutOfScope
			}
			var artifactExists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3 AND content_digest=$4 AND state IN ('usable','partial'))`, companyID, evidence.Artifact.ID, evidence.Artifact.Revision, evidence.Artifact.SHA256).Scan(&artifactExists); err != nil {
				return Receipt{}, err
			}
			if !artifactExists {
				return Receipt{}, core.OutOfScope
			}
		}
		reasonCodesJSON, err := json.Marshal(readiness.ReasonCodes)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		_, err = tx.Exec(ctx, `INSERT INTO domain_workflow_evidence_submissions(company_id,record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, companyID, recordID, submission.ProfileID, submission.ProfileRevision, readiness.Status, readiness.QualificationStatus, readiness.ExecutionEnabled, evidenceDigest, evidenceJSON, reasonCodesJSON, requestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		for _, evidence := range submission.Evidence {
			_, err := tx.Exec(ctx, `INSERT INTO domain_workflow_evidence_items(company_id,record_id,area,input_id,input_revision,input_sha256,method_sha256,assessed_by_employee_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, companyID, recordID, evidence.Area, evidence.Artifact.ID, evidence.Artifact.Revision, evidence.Artifact.SHA256, evidence.MethodSHA256, evidence.AssessedByEmployeeID)
			if err != nil {
				if isUniqueViolation(err) {
					return Receipt{}, core.Conflict
				}
				return Receipt{}, err
			}
		}
		return Receipt{ID: recordID, Status: string(readiness.Status)}, nil
	})
	if err != nil {
		return DomainEvidenceRecord{}, err
	}
	var record DomainEvidenceRecord
	var evidenceJSONRead, reasonsJSON []byte
	err = k.pool.QueryRow(ctx, `SELECT record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id,created_at::text
FROM domain_workflow_evidence_submissions WHERE company_id=$1 AND record_id=$2`, companyID, recordID).Scan(&record.RecordID, &record.ProfileID, &record.ProfileRevision, &record.ReadinessStatus, &record.QualificationStatus, &record.ExecutionEnabled, &record.EvidenceDigest, &evidenceJSONRead, &reasonsJSON, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainEvidenceRecord{}, core.Integrity
	}
	if err != nil {
		return DomainEvidenceRecord{}, err
	}
	if err = json.Unmarshal(evidenceJSONRead, &record.Submission); err != nil {
		return DomainEvidenceRecord{}, core.Integrity
	}
	if err = json.Unmarshal(reasonsJSON, &record.ReasonCodes); err != nil {
		return DomainEvidenceRecord{}, core.Integrity
	}
	record.CompanyID = companyID
	itemsMatch, matchErr := domainEvidenceItemsMatch(ctx, k.pool, companyID, record)
	if matchErr != nil {
		return DomainEvidenceRecord{}, matchErr
	}
	if record.EvidenceDigest != evidenceDigest || !validPersistedDomainEvidenceRecord(record) || !itemsMatch {
		return DomainEvidenceRecord{}, core.Integrity
	}
	return record, nil
}

func domainEvidenceRecordID(companyID, requestID string) string {
	identity := sha256.Sum256([]byte(companyID + "\x00" + requestID))
	return "domain-evidence-" + hex.EncodeToString(identity[:16])
}

func domainEvidenceSubmissionDigest(submission domainworkflow.DomainEvidenceSubmission) (string, []byte, error) {
	raw, err := json.Marshal(submission)
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), raw, nil
}

func validPersistedDomainEvidenceRecord(record DomainEvidenceRecord) bool {
	digest, _, err := domainEvidenceSubmissionDigest(record.Submission)
	if err != nil || digest != record.EvidenceDigest {
		return false
	}
	readiness := domainworkflow.EvaluateDomainEvidence(record.Submission)
	if readiness.Status != record.ReadinessStatus || readiness.QualificationStatus != record.QualificationStatus || readiness.ExecutionEnabled != record.ExecutionEnabled || len(readiness.ReasonCodes) != len(record.ReasonCodes) {
		return false
	}
	for index, reason := range readiness.ReasonCodes {
		if record.ReasonCodes[index] != reason {
			return false
		}
	}
	return true
}

func domainEvidenceItemsMatch(ctx context.Context, querier domainEvidenceItemQuerier, companyID string, record DomainEvidenceRecord) (bool, error) {
	expected := make(map[domainworkflow.DomainEvidenceArea]domainworkflow.DomainEvidence, len(record.Submission.Evidence))
	for _, item := range record.Submission.Evidence {
		expected[item.Area] = item
	}
	rows, err := querier.Query(ctx, `SELECT area,input_id,input_revision,input_sha256,method_sha256,assessed_by_employee_id
FROM domain_workflow_evidence_items WHERE company_id=$1 AND record_id=$2 ORDER BY area`, companyID, record.RecordID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var area string
		var inputID, inputSHA256, methodSHA256, assessorID string
		var revision int64
		if err = rows.Scan(&area, &inputID, &revision, &inputSHA256, &methodSHA256, &assessorID); err != nil {
			return false, err
		}
		key := domainworkflow.DomainEvidenceArea(area)
		item, exists := expected[key]
		if !exists || item.Artifact.ID != inputID || item.Artifact.Revision != revision || item.Artifact.SHA256 != inputSHA256 || item.MethodSHA256 != methodSHA256 || item.AssessedByEmployeeID != assessorID {
			return false, nil
		}
		delete(expected, key)
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	return len(expected) == 0, nil
}
