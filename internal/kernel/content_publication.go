// pattern: Imperative Shell

package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

const maxContentPublications = 100

type DomainContentPublicationReceipt struct {
	SchemaVersion       string `json:"schemaVersion"`
	Mode                string `json:"mode"`
	PublicationID       string `json:"publicationId"`
	ReviewID            string `json:"reviewId"`
	DraftInputID        string `json:"draftInputId"`
	DraftRevision       string `json:"draftRevision"`
	DraftSHA256         string `json:"draftSha256"`
	ExternalSideEffects bool   `json:"externalSideEffects"`
}

type DomainContentPublicationRecord struct {
	CompanyID           string                          `json:"companyId"`
	PublicationID       string                          `json:"publicationId"`
	ReviewID            string                          `json:"reviewId"`
	DraftInputID        string                          `json:"draftInputId"`
	DraftRevision       string                          `json:"draftRevision"`
	DraftSHA256         string                          `json:"draftSha256"`
	Mode                string                          `json:"mode"`
	ExternalSideEffects bool                            `json:"externalSideEffects"`
	ReceiptSHA256       string                          `json:"receiptSha256"`
	Receipt             DomainContentPublicationReceipt `json:"receipt"`
	RequestID           string                          `json:"requestId"`
	CreatedAt           string                          `json:"createdAt"`
}

type DomainContentCorrectionRecord struct {
	CompanyID               string `json:"companyId"`
	CorrectionID            string `json:"correctionId"`
	PublicationID           string `json:"publicationId"`
	CorrectionDraftInputID  string `json:"correctionDraftInputId"`
	CorrectionDraftRevision string `json:"correctionDraftRevision"`
	CorrectionDraftSHA256   string `json:"correctionDraftSha256"`
	Rationale               string `json:"rationale"`
	State                   string `json:"state"`
	RequestID               string `json:"requestId"`
	CreatedAt               string `json:"createdAt"`
}

type DomainContentFeedbackCategory string

const (
	DomainContentFeedbackPositive            DomainContentFeedbackCategory = "positive"
	DomainContentFeedbackNegative            DomainContentFeedbackCategory = "negative"
	DomainContentFeedbackMixed               DomainContentFeedbackCategory = "mixed"
	DomainContentFeedbackInconclusive        DomainContentFeedbackCategory = "inconclusive"
	DomainContentFeedbackCorrectionRequested DomainContentFeedbackCategory = "correction_requested"
)

type DomainContentFeedbackInput struct {
	Category  DomainContentFeedbackCategory
	Note      string
	RequestID string
}

type DomainContentFeedbackRecord struct {
	CompanyID     string                        `json:"companyId"`
	FeedbackID    string                        `json:"feedbackId"`
	PublicationID string                        `json:"publicationId"`
	Category      DomainContentFeedbackCategory `json:"category"`
	Note          string                        `json:"note"`
	State         string                        `json:"state"`
	RequestID     string                        `json:"requestId"`
	CreatedAt     string                        `json:"createdAt"`
}

func (k *Kernel) TXSimulateContentPublication(ctx context.Context, companyID, reviewID, requestID string) (DomainContentPublicationRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(reviewID) || !core.ValidID(requestID) {
		return DomainContentPublicationRecord{}, core.Malformed
	}
	if prior, found, err := k.GetDomainContentPublicationByRequest(ctx, companyID, requestID); err != nil {
		return DomainContentPublicationRecord{}, err
	} else if found {
		if prior.ReviewID != reviewID {
			return DomainContentPublicationRecord{}, core.Conflict
		}
		return prior, nil
	}
	review, err := k.GetDomainContentReview(ctx, companyID, reviewID)
	if err != nil {
		return DomainContentPublicationRecord{}, err
	}
	if review.Outcome != domainworkflow.OutcomeAccepted || review.Stale {
		return DomainContentPublicationRecord{}, core.Conflict
	}
	draftRevision, err := strconv.ParseInt(review.DraftRevision, 10, 64)
	if err != nil {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	draftSnapshot, err := k.ReadContentOperationInput(ctx, companyID, review.DraftInputID, draftRevision, maxContentOperationInputs)
	if err != nil {
		return DomainContentPublicationRecord{}, err
	}
	if draftSnapshot.Reference.SHA256 != review.DraftSHA256 {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	references, ok := contentReviewReferences(review.Review, review.Sample)
	if !ok {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	publicationID := stableCapabilityID("content-publication", companyID, requestID)
	receipt := DomainContentPublicationReceipt{
		SchemaVersion: "polis-content-publication-simulation@1", Mode: "simulation", PublicationID: publicationID,
		ReviewID: review.ReviewID, DraftInputID: review.DraftInputID, DraftRevision: review.DraftRevision,
		DraftSHA256: review.DraftSHA256, ExternalSideEffects: false,
	}
	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	receiptDigest := sha256.Sum256(receiptBytes)
	receiptSHA256 := hex.EncodeToString(receiptDigest[:])
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, requestID, "domain.content.publication.simulate", struct{ ReviewID, RequestID string }{reviewID, requestID}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var outcome, draftID, draftDigest string
		var draftRevision int64
		err := tx.QueryRow(ctx, `SELECT r.outcome,r.draft_input_id,r.draft_revision,r.draft_sha256
FROM domain_workflow_content_reviews r WHERE r.company_id=$1 AND r.review_id=$2`, companyID, reviewID).Scan(&outcome, &draftID, &draftRevision, &draftDigest)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		var latestRevision int64
		if err = tx.QueryRow(ctx, `SELECT max(draft_revision) FROM domain_workflow_content_drafts WHERE company_id=$1 AND draft_input_id=$2`, companyID, draftID).Scan(&latestRevision); err != nil {
			return Receipt{}, err
		}
		if outcome != string(domainworkflow.OutcomeAccepted) || latestRevision != draftRevision || draftDigest != review.DraftSHA256 {
			return Receipt{}, core.Conflict
		}
		for _, reference := range references {
			if err = verifyCurrentContentSourceAuthorization(ctx, tx, companyID, reference); err != nil {
				return Receipt{}, err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO domain_workflow_content_publication_simulations(
company_id,publication_id,review_id,draft_input_id,draft_revision,draft_sha256,mode,external_side_effects,receipt_sha256,receipt_json,request_id)
VALUES($1,$2,$3,$4,$5,$6,'simulation',false,$7,$8::jsonb,$9)`, companyID, publicationID, reviewID, review.DraftInputID, draftRevision, review.DraftSHA256, receiptSHA256, receiptBytes, requestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: publicationID, Status: "simulated", Revision: draftRevision}, nil
	})
	if err != nil {
		return DomainContentPublicationRecord{}, err
	}
	return k.GetDomainContentPublication(ctx, companyID, publicationID)
}

func (k *Kernel) GetDomainContentPublication(ctx context.Context, companyID, publicationID string) (DomainContentPublicationRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(publicationID) {
		return DomainContentPublicationRecord{}, core.Malformed
	}
	return readDomainContentPublication(ctx, k.pool, companyID, publicationID)
}

func (k *Kernel) GetDomainContentPublicationByRequest(ctx context.Context, companyID, requestID string) (DomainContentPublicationRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return DomainContentPublicationRecord{}, false, core.Malformed
	}
	var publicationID string
	err := k.pool.QueryRow(ctx, "SELECT publication_id FROM domain_workflow_content_publication_simulations WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(&publicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentPublicationRecord{}, false, nil
	}
	if err != nil {
		return DomainContentPublicationRecord{}, false, err
	}
	record, err := k.GetDomainContentPublication(ctx, companyID, publicationID)
	return record, err == nil, err
}

func (k *Kernel) listDomainContentPublications(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainContentPublicationRecord, error) {
	rows, err := tx.Query(ctx, `SELECT publication_id FROM domain_workflow_content_publication_simulations
WHERE company_id=$1 ORDER BY created_at DESC,publication_id DESC LIMIT $2`, companyID, maxContentPublications)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, maxContentPublications)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]DomainContentPublicationRecord, 0, len(ids))
	for _, id := range ids {
		record, readErr := readDomainContentPublication(ctx, tx, companyID, id)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, record)
	}
	return result, nil
}

type contentPublicationQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readDomainContentPublication(ctx context.Context, queryer contentPublicationQueryer, companyID, publicationID string) (DomainContentPublicationRecord, error) {
	var record DomainContentPublicationRecord
	var revision int64
	var rawReceipt []byte
	err := queryer.QueryRow(ctx, `SELECT publication_id,review_id,draft_input_id,draft_revision,draft_sha256,mode,external_side_effects,receipt_sha256,receipt_json,request_id,created_at::text
FROM domain_workflow_content_publication_simulations WHERE company_id=$1 AND publication_id=$2`, companyID, publicationID).Scan(
		&record.PublicationID, &record.ReviewID, &record.DraftInputID, &revision, &record.DraftSHA256, &record.Mode, &record.ExternalSideEffects,
		&record.ReceiptSHA256, &rawReceipt, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentPublicationRecord{}, core.OutOfScope
	}
	if err != nil {
		return DomainContentPublicationRecord{}, err
	}
	if err = json.Unmarshal(rawReceipt, &record.Receipt); err != nil {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	canonicalReceipt, err := json.Marshal(record.Receipt)
	if err != nil {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	digest := sha256.Sum256(canonicalReceipt)
	record.CompanyID, record.DraftRevision = companyID, strconv.FormatInt(revision, 10)
	if hex.EncodeToString(digest[:]) != record.ReceiptSHA256 || record.Mode != "simulation" || record.ExternalSideEffects ||
		record.Receipt.SchemaVersion != "polis-content-publication-simulation@1" || record.Receipt.Mode != record.Mode ||
		record.Receipt.PublicationID != publicationID || record.Receipt.ReviewID != record.ReviewID ||
		record.Receipt.DraftInputID != record.DraftInputID || record.Receipt.DraftRevision != record.DraftRevision ||
		record.Receipt.DraftSHA256 != record.DraftSHA256 || record.Receipt.ExternalSideEffects {
		return DomainContentPublicationRecord{}, core.Integrity
	}
	return record, nil
}

func (k *Kernel) TXRecordContentCorrection(ctx context.Context, companyID, publicationID, draftInputID string, draftRevision int64, rationale, requestID string) (DomainContentCorrectionRecord, error) {
	rationale = strings.TrimSpace(rationale)
	if !core.ValidID(companyID) || !core.ValidID(publicationID) || !core.ValidID(draftInputID) || draftRevision < 1 ||
		rationale == "" || utf8.RuneCountInString(rationale) > 2000 || !core.ValidID(requestID) {
		return DomainContentCorrectionRecord{}, core.Malformed
	}
	draft, err := k.GetDomainContentDraft(ctx, companyID, draftInputID, draftRevision)
	if err != nil {
		return DomainContentCorrectionRecord{}, err
	}
	draftSnapshot, err := k.ReadContentOperationInput(ctx, companyID, draftInputID, draftRevision, maxContentOperationInputs)
	if err != nil {
		return DomainContentCorrectionRecord{}, err
	}
	if draftSnapshot.Reference.SHA256 != draft.DraftSHA256 {
		return DomainContentCorrectionRecord{}, core.Integrity
	}
	correctionID := stableCapabilityID("content-correction", companyID, requestID)
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, requestID, "domain.content.correction.record", struct {
		PublicationID, DraftInputID string
		DraftRevision               int64
		Rationale                   string
	}{publicationID, draftInputID, draftRevision, rationale}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var publishedInputID string
		var publishedRevision int64
		if err := tx.QueryRow(ctx, "SELECT draft_input_id,draft_revision FROM domain_workflow_content_publication_simulations WHERE company_id=$1 AND publication_id=$2", companyID, publicationID).Scan(&publishedInputID, &publishedRevision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if draftInputID != publishedInputID || draftRevision <= publishedRevision {
			return Receipt{}, core.Conflict
		}
		var draftSHA string
		if err := tx.QueryRow(ctx, "SELECT draft_sha256 FROM domain_workflow_content_drafts WHERE company_id=$1 AND draft_input_id=$2 AND draft_revision=$3", companyID, draftInputID, draftRevision).Scan(&draftSHA); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var latestDraftRevision int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(draft_revision),0) FROM domain_workflow_content_drafts
WHERE company_id=$1 AND draft_input_id=$2`, companyID, draftInputID).Scan(&latestDraftRevision); err != nil {
			return Receipt{}, err
		}
		if draftRevision != latestDraftRevision {
			return Receipt{}, core.Conflict
		}
		_, err := tx.Exec(ctx, `INSERT INTO domain_workflow_content_corrections(company_id,correction_id,publication_id,correction_draft_input_id,correction_draft_revision,correction_draft_sha256,rationale,state,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'review_required',$8)`, companyID, correctionID, publicationID, draftInputID, draftRevision, draftSHA, rationale, requestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: correctionID, Status: "review_required", Revision: draftRevision}, nil
	})
	if err != nil {
		return DomainContentCorrectionRecord{}, err
	}
	return readDomainContentCorrection(ctx, k.pool, companyID, correctionID)
}

func (k *Kernel) TXRecordContentFeedback(ctx context.Context, companyID, publicationID string, input DomainContentFeedbackInput) (DomainContentFeedbackRecord, error) {
	note := strings.TrimSpace(input.Note)
	if !core.ValidID(companyID) || !core.ValidID(publicationID) || !core.ValidID(input.RequestID) || note == "" || utf8.RuneCountInString(note) > 2000 ||
		(input.Category != DomainContentFeedbackPositive && input.Category != DomainContentFeedbackNegative && input.Category != DomainContentFeedbackMixed && input.Category != DomainContentFeedbackInconclusive && input.Category != DomainContentFeedbackCorrectionRequested) {
		return DomainContentFeedbackRecord{}, core.Malformed
	}
	state := "recorded"
	if input.Category == DomainContentFeedbackNegative || input.Category == DomainContentFeedbackCorrectionRequested {
		state = "review_required"
	}
	feedbackID := stableCapabilityID("content-feedback", companyID, input.RequestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "domain.content.feedback.record", struct {
		PublicationID string
		Category      DomainContentFeedbackCategory
		Note          string
	}{publicationID, input.Category, note}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM domain_workflow_content_publication_simulations WHERE company_id=$1 AND publication_id=$2)", companyID, publicationID).Scan(&exists); err != nil {
			return Receipt{}, err
		}
		if !exists {
			return Receipt{}, core.OutOfScope
		}
		_, err := tx.Exec(ctx, `INSERT INTO domain_workflow_content_feedback(company_id,feedback_id,publication_id,category,note,state,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7)`, companyID, feedbackID, publicationID, input.Category, note, state, input.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: feedbackID, Status: state}, nil
	})
	if err != nil {
		return DomainContentFeedbackRecord{}, err
	}
	return readDomainContentFeedback(ctx, k.pool, companyID, feedbackID)
}

func (k *Kernel) listDomainContentCorrections(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainContentCorrectionRecord, error) {
	rows, err := tx.Query(ctx, `SELECT correction_id FROM domain_workflow_content_corrections WHERE company_id=$1 ORDER BY created_at DESC,correction_id DESC LIMIT 100`, companyID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, 100)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]DomainContentCorrectionRecord, 0, len(ids))
	for _, id := range ids {
		record, readErr := readDomainContentCorrection(ctx, tx, companyID, id)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, record)
	}
	return result, nil
}

func (k *Kernel) listDomainContentFeedback(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainContentFeedbackRecord, error) {
	rows, err := tx.Query(ctx, `SELECT feedback_id FROM domain_workflow_content_feedback WHERE company_id=$1 ORDER BY created_at DESC,feedback_id DESC LIMIT 100`, companyID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, 100)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]DomainContentFeedbackRecord, 0, len(ids))
	for _, id := range ids {
		record, readErr := readDomainContentFeedback(ctx, tx, companyID, id)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, record)
	}
	return result, nil
}

func readDomainContentCorrection(ctx context.Context, queryer contentPublicationQueryer, companyID, correctionID string) (DomainContentCorrectionRecord, error) {
	var record DomainContentCorrectionRecord
	var revision int64
	err := queryer.QueryRow(ctx, `SELECT correction_id,publication_id,correction_draft_input_id,correction_draft_revision,correction_draft_sha256,rationale,state,request_id,created_at::text
FROM domain_workflow_content_corrections WHERE company_id=$1 AND correction_id=$2`, companyID, correctionID).Scan(&record.CorrectionID, &record.PublicationID, &record.CorrectionDraftInputID, &revision, &record.CorrectionDraftSHA256, &record.Rationale, &record.State, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentCorrectionRecord{}, core.OutOfScope
	}
	if err != nil {
		return DomainContentCorrectionRecord{}, err
	}
	record.CompanyID, record.CorrectionDraftRevision = companyID, strconv.FormatInt(revision, 10)
	if record.State != "review_required" || record.Rationale == "" || !validEnvironmentSHA256(record.CorrectionDraftSHA256) {
		return DomainContentCorrectionRecord{}, core.Integrity
	}
	return record, nil
}

func readDomainContentFeedback(ctx context.Context, queryer contentPublicationQueryer, companyID, feedbackID string) (DomainContentFeedbackRecord, error) {
	var record DomainContentFeedbackRecord
	err := queryer.QueryRow(ctx, `SELECT feedback_id,publication_id,category,note,state,request_id,created_at::text FROM domain_workflow_content_feedback WHERE company_id=$1 AND feedback_id=$2`, companyID, feedbackID).Scan(&record.FeedbackID, &record.PublicationID, &record.Category, &record.Note, &record.State, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentFeedbackRecord{}, core.OutOfScope
	}
	if err != nil {
		return DomainContentFeedbackRecord{}, err
	}
	record.CompanyID = companyID
	if record.Note == "" || (record.State != "recorded" && record.State != "review_required") {
		return DomainContentFeedbackRecord{}, core.Integrity
	}
	return record, nil
}
