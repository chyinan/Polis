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

type DomainEvidenceSubstantiveAssessmentRequest struct {
	domainworkflow.DomainEvidenceSubstantiveReview
	EvidenceDigest string `json:"evidenceDigest"`
	RequestID      string `json:"requestId"`
}

type DomainEvidenceSubstantiveAssessmentRecord struct {
	CompanyID          string                                            `json:"companyId"`
	AssessmentID       string                                            `json:"assessmentId"`
	RecordID           string                                            `json:"recordId"`
	EvidenceDigest     string                                            `json:"evidenceDigest"`
	Outcome            domainworkflow.DomainEvidenceSubstantiveOutcome   `json:"outcome"`
	ReviewerEmployeeID string                                            `json:"reviewerEmployeeId"`
	AreaAssessments    []domainworkflow.DomainEvidenceAreaAssessment     `json:"areaAssessments"`
	PreviewedEvidence  []domainworkflow.DomainEvidencePreviewAttestation `json:"previewedEvidence"`
	RequestID          string                                            `json:"requestId"`
	CreatedAt          string                                            `json:"createdAt"`
}

func (k *Kernel) TXRecordDomainEvidenceSubstantiveAssessment(ctx context.Context, companyID, recordID string, request DomainEvidenceSubstantiveAssessmentRequest) (DomainEvidenceSubstantiveAssessmentRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(recordID) || !core.ValidID(request.RequestID) || !validTaskInputDigest(request.EvidenceDigest) {
		return DomainEvidenceSubstantiveAssessmentRecord{}, core.Malformed
	}
	scope := k.LocalScope(companyID)
	assessmentID := domainEvidenceSubstantiveAssessmentID(companyID, recordID)
	input := struct {
		RecordID       string
		EvidenceDigest string
		Review         domainworkflow.DomainEvidenceSubstantiveReview
		RequestID      string
	}{recordID, request.EvidenceDigest, request.DomainEvidenceSubstantiveReview, request.RequestID}
	_, err := k.TXWrite(ctx, scope, nil, request.RequestID, "domain.evidence.substantive_assessment", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		submission, err := readDomainEvidenceSubmissionForReview(ctx, tx, companyID, recordID)
		if err != nil {
			return Receipt{}, err
		}
		if domainEvidenceSubmissionDigestValue(submission) != request.EvidenceDigest {
			return Receipt{}, core.ConflictError{Reason: "evidence submission digest changed before substantive review", CurrentState: "evidence_digest_changed"}
		}
		referenceReview, err := readDomainEvidenceReview(ctx, tx, companyID, recordID)
		if err != nil {
			return Receipt{}, err
		}
		if referenceReview == nil {
			return Receipt{}, core.ConflictError{Reason: "preview-bound evidence-reference review is required first", CurrentState: "reference_review_missing"}
		}
		if !validPersistedDomainEvidenceReview(submission, *referenceReview) {
			return Receipt{}, core.Integrity
		}
		var assessmentExists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM domain_workflow_substantive_assessments WHERE company_id=$1 AND record_id=$2)", companyID, recordID).Scan(&assessmentExists); err != nil {
			return Receipt{}, err
		}
		if assessmentExists {
			return Receipt{}, core.Conflict
		}
		assessment, reasons := domainworkflow.EvaluateDomainEvidenceSubstantiveReview(submission, referenceReview.Outcome, referenceReview.ReviewContractRevision, request.DomainEvidenceSubstantiveReview)
		if len(reasons) != 0 {
			return Receipt{}, core.Malformed
		}
		var reviewerRole string
		if err = tx.QueryRow(ctx, "SELECT role_name FROM employees WHERE company_id=$1 AND id=$2", companyID, assessment.ReviewerEmployeeID).Scan(&reviewerRole); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if reviewerRole != "review" {
			return Receipt{}, core.Denied
		}
		for _, attestation := range assessment.PreviewedEvidence {
			preview, previewErr := k.ReadDomainEvidenceArtifactPreview(ctx, companyID, recordID, attestation.Area, attestation.RelativePath)
			if errors.Is(previewErr, ErrDomainEvidencePreviewUnsupported) {
				return Receipt{}, core.Denied
			}
			if previewErr != nil {
				return Receipt{}, previewErr
			}
			if preview.SourceDigest != attestation.SourceDigest || preview.ContentSHA256 != attestation.ContentDigest || preview.MediaType != attestation.MediaType {
				return Receipt{}, core.Integrity
			}
		}
		areaJSON, err := json.Marshal(assessment.AreaAssessments)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		previewJSON, err := json.Marshal(assessment.PreviewedEvidence)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		if _, err = tx.Exec(ctx, `INSERT INTO domain_workflow_substantive_assessments(company_id,assessment_id,record_id,evidence_digest,outcome,reviewer_employee_id,area_assessments,previewed_evidence,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, companyID, assessmentID, recordID, domainEvidenceSubmissionDigestValue(submission), assessment.Outcome, assessment.ReviewerEmployeeID, areaJSON, previewJSON, request.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		for _, areaAssessment := range assessment.AreaAssessments {
			if _, err = tx.Exec(ctx, `INSERT INTO domain_workflow_substantive_area_assessments(company_id,record_id,area,outcome,rationale)
VALUES($1,$2,$3,$4,$5)`, companyID, recordID, areaAssessment.Area, areaAssessment.Outcome, areaAssessment.Rationale); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: assessmentID, Status: string(assessment.Outcome)}, nil
	})
	if err != nil {
		return DomainEvidenceSubstantiveAssessmentRecord{}, err
	}
	ledger, err := k.ListDomainEvidence(ctx, companyID)
	if err != nil {
		return DomainEvidenceSubstantiveAssessmentRecord{}, err
	}
	for _, record := range ledger.Submissions {
		if record.RecordID == recordID && record.Assessment != nil {
			assessment := record.Assessment
			if assessment.AssessmentID == assessmentID && assessment.EvidenceDigest == record.EvidenceDigest && assessment.RequestID == request.RequestID {
				return *assessment, nil
			}
			return DomainEvidenceSubstantiveAssessmentRecord{}, core.Integrity
		}
	}
	return DomainEvidenceSubstantiveAssessmentRecord{}, core.Integrity
}

func domainEvidenceSubmissionDigestValue(submission domainworkflow.DomainEvidenceSubmission) string {
	raw, err := json.Marshal(submission)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func domainEvidenceSubstantiveAssessmentID(companyID, recordID string) string {
	identity := sha256.Sum256([]byte(companyID + "\x00" + recordID))
	return "domain-substantive-" + hex.EncodeToString(identity[:16])
}

func readDomainEvidenceSubstantiveAssessment(ctx context.Context, querier domainEvidenceReviewReader, companyID, recordID string) (*DomainEvidenceSubstantiveAssessmentRecord, error) {
	var record DomainEvidenceSubstantiveAssessmentRecord
	var areaJSON, previewJSON []byte
	err := querier.QueryRow(ctx, `SELECT assessment_id,record_id,evidence_digest,outcome,reviewer_employee_id,area_assessments,previewed_evidence,request_id,created_at::text
FROM domain_workflow_substantive_assessments WHERE company_id=$1 AND record_id=$2`, companyID, recordID).Scan(
		&record.AssessmentID, &record.RecordID, &record.EvidenceDigest, &record.Outcome, &record.ReviewerEmployeeID, &areaJSON, &previewJSON, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(areaJSON, &record.AreaAssessments) != nil || json.Unmarshal(previewJSON, &record.PreviewedEvidence) != nil {
		return nil, core.Integrity
	}
	if record.AreaAssessments == nil || record.PreviewedEvidence == nil {
		return nil, core.Integrity
	}
	rows, err := querier.Query(ctx, `SELECT area,outcome,rationale FROM domain_workflow_substantive_area_assessments WHERE company_id=$1 AND record_id=$2 ORDER BY area`, companyID, recordID)
	if err != nil {
		return nil, err
	}
	normalized := make(map[domainworkflow.DomainEvidenceArea]domainworkflow.DomainEvidenceAreaAssessment, len(record.AreaAssessments))
	for _, item := range record.AreaAssessments {
		normalized[item.Area] = item
	}
	count := 0
	for rows.Next() {
		var item domainworkflow.DomainEvidenceAreaAssessment
		if err = rows.Scan(&item.Area, &item.Outcome, &item.Rationale); err != nil {
			rows.Close()
			return nil, err
		}
		if expected, ok := normalized[item.Area]; !ok || expected != item {
			rows.Close()
			return nil, core.Integrity
		}
		delete(normalized, item.Area)
		count++
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if count != len(record.AreaAssessments) || len(normalized) != 0 || record.AssessmentID != domainEvidenceSubstantiveAssessmentID(companyID, recordID) || !core.ValidID(record.RequestID) || !core.ValidID(record.ReviewerEmployeeID) {
		return nil, core.Integrity
	}
	record.CompanyID = companyID
	return &record, nil
}

func validPersistedDomainEvidenceSubstantiveAssessment(submission domainworkflow.DomainEvidenceSubmission, evidenceDigest string, referenceReview *DomainEvidenceReviewRecord, assessment DomainEvidenceSubstantiveAssessmentRecord) bool {
	if referenceReview == nil || assessment.EvidenceDigest != evidenceDigest || !core.ValidID(assessment.CompanyID) || !core.ValidID(assessment.RecordID) || !core.ValidID(assessment.RequestID) ||
		!core.ValidID(assessment.ReviewerEmployeeID) || assessment.AssessmentID != domainEvidenceSubstantiveAssessmentID(assessment.CompanyID, assessment.RecordID) {
		return false
	}
	result, reasons := domainworkflow.EvaluateDomainEvidenceSubstantiveReview(submission, referenceReview.Outcome, referenceReview.ReviewContractRevision, domainworkflow.DomainEvidenceSubstantiveReview{
		ReviewerEmployeeID: assessment.ReviewerEmployeeID,
		AreaAssessments:    assessment.AreaAssessments,
		PreviewedEvidence:  assessment.PreviewedEvidence,
	})
	return len(reasons) == 0 && result.Outcome == assessment.Outcome && sameDomainEvidenceAreaAssessments(result.AreaAssessments, assessment.AreaAssessments)
}

func sameDomainEvidenceAreaAssessments(left, right []domainworkflow.DomainEvidenceAreaAssessment) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
