// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

type DomainEvidenceReviewRequest struct {
	domainworkflow.DomainEvidenceReview
	RequestID string `json:"requestId"`
}

type DomainEvidenceReviewRecord struct {
	CompanyID              string                                            `json:"companyId"`
	ReviewID               string                                            `json:"reviewId"`
	RecordID               string                                            `json:"recordId"`
	Outcome                domainworkflow.DomainEvidenceReviewOutcome        `json:"outcome"`
	ReviewerEmployeeID     string                                            `json:"reviewerEmployeeId"`
	Rationale              string                                            `json:"rationale"`
	ReviewContractRevision int16                                             `json:"reviewContractRevision"`
	PreviewedEvidence      []domainworkflow.DomainEvidencePreviewAttestation `json:"previewedEvidence"`
	RequestID              string                                            `json:"requestId"`
	CreatedAt              string                                            `json:"createdAt"`
}

type domainEvidenceReviewReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (k *Kernel) TXRecordDomainEvidenceReview(ctx context.Context, companyID, recordID string, request DomainEvidenceReviewRequest) (DomainEvidenceReviewRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(recordID) || !core.ValidID(request.RequestID) {
		return DomainEvidenceReviewRecord{}, core.Malformed
	}
	scope := k.LocalScope(companyID)
	reviewID := domainEvidenceReviewRecordID(companyID, recordID)
	input := struct {
		RecordID  string
		Review    domainworkflow.DomainEvidenceReview
		RequestID string
	}{recordID, request.DomainEvidenceReview, request.RequestID}
	_, err := k.TXWrite(ctx, scope, nil, request.RequestID, "domain.evidence.review", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		submission, err := readDomainEvidenceSubmissionForReview(ctx, tx, companyID, recordID)
		if err != nil {
			return Receipt{}, err
		}
		if reasons := domainworkflow.ValidateDomainEvidenceReview(submission, request.DomainEvidenceReview); len(reasons) != 0 {
			return Receipt{}, core.Malformed
		}
		var reviewerRole string
		if err = tx.QueryRow(ctx, "SELECT role_name FROM employees WHERE company_id=$1 AND id=$2", companyID, request.ReviewerEmployeeID).Scan(&reviewerRole); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if reviewerRole != "review" {
			return Receipt{}, core.Denied
		}
		for _, attestation := range request.PreviewedEvidence {
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
		previewedJSON, err := json.Marshal(request.PreviewedEvidence)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		_, err = tx.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,previewed_evidence,review_contract_revision,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,1,$8)`, companyID, reviewID, recordID, request.Outcome, request.ReviewerEmployeeID, request.Rationale, previewedJSON, request.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: reviewID, Status: string(request.Outcome)}, nil
	})
	if err != nil {
		return DomainEvidenceReviewRecord{}, err
	}
	review, err := readDomainEvidenceReview(ctx, k.pool, companyID, recordID)
	if err != nil {
		return DomainEvidenceReviewRecord{}, err
	}
	if review == nil || review.ReviewID != reviewID || review.Outcome != request.Outcome || review.ReviewerEmployeeID != request.ReviewerEmployeeID || review.Rationale != request.Rationale || review.ReviewContractRevision != 1 || !sameDomainEvidencePreviewAttestations(review.PreviewedEvidence, request.PreviewedEvidence) || review.RequestID != request.RequestID {
		return DomainEvidenceReviewRecord{}, core.Integrity
	}
	return *review, nil
}

func readDomainEvidenceSubmissionForReview(ctx context.Context, querier domainEvidenceReviewReader, companyID, recordID string) (domainworkflow.DomainEvidenceSubmission, error) {
	var record DomainEvidenceRecord
	var evidenceJSON, reasonsJSON []byte
	err := querier.QueryRow(ctx, `SELECT record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id,created_at::text
FROM domain_workflow_evidence_submissions WHERE company_id=$1 AND record_id=$2 FOR SHARE`, companyID, recordID).Scan(&record.RecordID, &record.ProfileID, &record.ProfileRevision, &record.ReadinessStatus, &record.QualificationStatus, &record.ExecutionEnabled, &record.EvidenceDigest, &evidenceJSON, &reasonsJSON, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainworkflow.DomainEvidenceSubmission{}, core.OutOfScope
	}
	if err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, err
	}
	if err = json.Unmarshal(evidenceJSON, &record.Submission); err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	if err = json.Unmarshal(reasonsJSON, &record.ReasonCodes); err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	if !validPersistedDomainEvidenceRecord(record) {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	itemsMatch, err := domainEvidenceItemsMatch(ctx, querier, companyID, record)
	if err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, err
	}
	if !itemsMatch {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	return record.Submission, nil
}

func readDomainEvidenceReview(ctx context.Context, querier domainEvidenceReviewReader, companyID, recordID string) (*DomainEvidenceReviewRecord, error) {
	var review DomainEvidenceReviewRecord
	var previewedJSON []byte
	err := querier.QueryRow(ctx, `SELECT review_id,record_id,outcome,reviewer_employee_id,rationale,previewed_evidence,review_contract_revision,request_id,created_at::text
FROM domain_workflow_evidence_reviews WHERE company_id=$1 AND record_id=$2`, companyID, recordID).Scan(&review.ReviewID, &review.RecordID, &review.Outcome, &review.ReviewerEmployeeID, &review.Rationale, &previewedJSON, &review.ReviewContractRevision, &review.RequestID, &review.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(previewedJSON, &review.PreviewedEvidence); err != nil {
		return nil, core.Integrity
	}
	review.CompanyID = companyID
	if review.ReviewID != domainEvidenceReviewRecordID(companyID, recordID) || review.RecordID != recordID || !core.ValidID(review.ReviewerEmployeeID) || !core.ValidID(review.RequestID) || !domainEvidenceReviewOutcomeValid(review.Outcome) || strings.TrimSpace(review.Rationale) == "" || len([]rune(review.Rationale)) > 2000 || (review.ReviewContractRevision != 0 && review.ReviewContractRevision != 1) {
		return nil, core.Integrity
	}
	if review.PreviewedEvidence == nil {
		review.PreviewedEvidence = []domainworkflow.DomainEvidencePreviewAttestation{}
	}
	return &review, nil
}

func sameDomainEvidencePreviewAttestations(left, right []domainworkflow.DomainEvidencePreviewAttestation) bool {
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

func validPersistedDomainEvidenceReview(submission domainworkflow.DomainEvidenceSubmission, review DomainEvidenceReviewRecord) bool {
	switch review.ReviewContractRevision {
	case 0:
		return len(review.PreviewedEvidence) == 0
	case 1:
		return len(domainworkflow.ValidateDomainEvidenceReview(submission, domainworkflow.DomainEvidenceReview{
			Outcome: review.Outcome, ReviewerEmployeeID: review.ReviewerEmployeeID, Rationale: review.Rationale,
			PreviewedEvidence: review.PreviewedEvidence,
		})) == 0
	default:
		return false
	}
}

func domainEvidenceReviewOutcomeValid(outcome domainworkflow.DomainEvidenceReviewOutcome) bool {
	return outcome == domainworkflow.DomainEvidenceReviewAccepted || outcome == domainworkflow.DomainEvidenceReviewRejected || outcome == domainworkflow.DomainEvidenceReviewNeedsMore
}

func domainEvidenceReviewRecordID(companyID, recordID string) string {
	identity := sha256.Sum256([]byte(companyID + "\x00" + recordID))
	return "domain-review-" + hex.EncodeToString(identity[:16])
}
