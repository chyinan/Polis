// pattern: Imperative Shell

package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

type DomainContentReviewRequest struct {
	Review       domainworkflow.ContentReview
	Sample       domainworkflow.ContentSampleEvidence
	CorrectionID string
	RequestID    string
}

func (k *Kernel) TXRecordContentReview(ctx context.Context, companyID, draftInputID string, draftRevision int64, request DomainContentReviewRequest) (DomainContentReviewRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(draftInputID) || draftRevision < 1 || !core.ValidID(request.RequestID) ||
		request.Review.DraftRevision != draftRevision || request.Sample.DraftRevision != draftRevision || len(request.Review.Claims) == 0 || len(request.Review.Claims) > 100 ||
		len(request.Sample.SampledClaimIDs) == 0 || len(request.Sample.SampledClaimIDs) > 100 || (request.CorrectionID != "" && !core.ValidID(request.CorrectionID)) {
		return DomainContentReviewRecord{}, core.Malformed
	}
	if prior, found, err := k.GetDomainContentReviewByRequest(ctx, companyID, request.RequestID); err != nil {
		return DomainContentReviewRecord{}, err
	} else if found {
		if !sameDomainContentReviewRequest(prior, draftInputID, draftRevision, request) {
			return DomainContentReviewRecord{}, core.Conflict
		}
		return prior, nil
	}
	draft, err := k.GetDomainContentDraft(ctx, companyID, draftInputID, draftRevision)
	if err != nil {
		return DomainContentReviewRecord{}, err
	}
	if request.Sample.DraftSHA256 != draft.DraftSHA256 {
		return DomainContentReviewRecord{}, core.Integrity
	}
	body, err := k.ReadContentOperationInput(ctx, companyID, draftInputID, draftRevision, maxContentOperationInputs)
	if err != nil {
		return DomainContentReviewRecord{}, err
	}
	if body.Reference.SHA256 != draft.DraftSHA256 {
		return DomainContentReviewRecord{}, core.Integrity
	}
	references, ok := contentReviewReferences(request.Review, request.Sample)
	if !ok {
		return DomainContentReviewRecord{}, core.Malformed
	}
	authorizedSources := make([]domainworkflow.ContentSourceCatalogEntry, 0, len(references))
	for _, reference := range references {
		event, found, sourceErr := k.GetCurrentDomainContentSourceAuthorization(ctx, companyID, reference.InputID, reference.Revision)
		if sourceErr != nil {
			return DomainContentReviewRecord{}, sourceErr
		}
		if !found || event.State != DomainContentSourceAuthorized || event.SHA256 != reference.SHA256 {
			return DomainContentReviewRecord{}, core.Denied
		}
		source, sourceErr := k.ReadContentOperationInput(ctx, companyID, reference.InputID, reference.Revision, maxContentOperationInputs)
		if sourceErr != nil {
			return DomainContentReviewRecord{}, sourceErr
		}
		if source.Reference != reference {
			return DomainContentReviewRecord{}, core.Integrity
		}
		authorizedSources = append(authorizedSources, domainworkflow.ContentSourceCatalogEntry{CompanyID: companyID, Reference: reference})
	}
	domainDraft := domainworkflow.ContentDraft{
		Revision: draftRevision, WriterEmployeeID: draft.WriterEmployeeID, BodySHA256: draft.DraftSHA256,
		CriticalClaims: draft.CriticalClaims, ConstraintsPassed: draft.ConstraintsPassed,
	}
	decision := domainworkflow.EvaluateContentReviewWithEvidence(companyID, domainDraft, request.Review, authorizedSources, request.Sample)
	reviewID := stableCapabilityID("content-review", companyID, request.RequestID)
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, request.RequestID, "domain.content.review", struct {
		DraftInputID  string
		DraftRevision int64
		Request       DomainContentReviewRequest
	}{draftInputID, draftRevision, request}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var currentDraft DomainContentDraftRecord
		var claimsJSON []byte
		if err := tx.QueryRow(ctx, `SELECT draft_sha256,writer_employee_id,critical_claims,constraints_passed
FROM domain_workflow_content_drafts WHERE company_id=$1 AND draft_input_id=$2 AND draft_revision=$3`, companyID, draftInputID, draftRevision).Scan(
			&currentDraft.DraftSHA256, &currentDraft.WriterEmployeeID, &claimsJSON, &currentDraft.ConstraintsPassed); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			}
			return Receipt{}, err
		}
		if currentDraft.DraftSHA256 != draft.DraftSHA256 || !currentDraft.ConstraintsPassed || currentDraft.WriterEmployeeID == request.Review.CheckerEmployeeID {
			return Receipt{}, core.Conflict
		}
		if err := json.Unmarshal(claimsJSON, &currentDraft.CriticalClaims); err != nil || !domainworkflow.ValidContentDraft(domainworkflow.ContentDraft{
			Revision: draftRevision, WriterEmployeeID: currentDraft.WriterEmployeeID, BodySHA256: currentDraft.DraftSHA256,
			CriticalClaims: currentDraft.CriticalClaims, ConstraintsPassed: currentDraft.ConstraintsPassed,
		}) {
			return Receipt{}, core.Integrity
		}
		var reviewerRole string
		if err := tx.QueryRow(ctx, "SELECT role_name FROM employees WHERE company_id=$1 AND id=$2", companyID, request.Review.CheckerEmployeeID).Scan(&reviewerRole); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if reviewerRole != "review" {
			return Receipt{}, core.Denied
		}
		if err := verifyDomainContentCorrectionReviewTX(ctx, tx, companyID, draftInputID, draftRevision, request.CorrectionID); err != nil {
			return Receipt{}, err
		}
		for _, reference := range references {
			if err := verifyCurrentContentSourceAuthorization(ctx, tx, companyID, reference); err != nil {
				return Receipt{}, err
			}
		}
		reviewJSON, marshalErr := json.Marshal(request.Review)
		if marshalErr != nil {
			return Receipt{}, core.Integrity
		}
		sampleJSON, marshalErr := json.Marshal(request.Sample)
		if marshalErr != nil {
			return Receipt{}, core.Integrity
		}
		reasonsJSON, marshalErr := json.Marshal(decision.ReasonCodes)
		if marshalErr != nil {
			return Receipt{}, core.Integrity
		}
		_, insertErr := tx.Exec(ctx, `INSERT INTO domain_workflow_content_reviews(
company_id,review_id,draft_input_id,draft_revision,draft_sha256,checker_employee_id,outcome,review,sample,reason_codes,correction_id,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,NULLIF($11,''),$12)`, companyID, reviewID, draftInputID, draftRevision, draft.DraftSHA256,
			request.Review.CheckerEmployeeID, decision.Outcome, reviewJSON, sampleJSON, reasonsJSON, request.CorrectionID, request.RequestID)
		if insertErr != nil {
			if isUniqueViolation(insertErr) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, insertErr
		}
		return Receipt{ID: reviewID, Status: string(decision.Outcome), Revision: draftRevision}, nil
	})
	if err != nil {
		return DomainContentReviewRecord{}, err
	}
	return k.GetDomainContentReview(ctx, companyID, reviewID)
}

func (k *Kernel) GetDomainContentReviewByRequest(ctx context.Context, companyID, requestID string) (DomainContentReviewRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return DomainContentReviewRecord{}, false, core.Malformed
	}
	var reviewID string
	err := k.pool.QueryRow(ctx, "SELECT review_id FROM domain_workflow_content_reviews WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(&reviewID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentReviewRecord{}, false, nil
	}
	if err != nil {
		return DomainContentReviewRecord{}, false, err
	}
	record, err := k.GetDomainContentReview(ctx, companyID, reviewID)
	return record, err == nil, err
}

func (k *Kernel) GetDomainContentReview(ctx context.Context, companyID, reviewID string) (DomainContentReviewRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(reviewID) {
		return DomainContentReviewRecord{}, core.Malformed
	}
	return readDomainContentReview(ctx, k.pool, companyID, reviewID)
}

func (k *Kernel) listDomainContentReviews(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainContentReviewRecord, error) {
	rows, err := tx.Query(ctx, `SELECT review_id FROM domain_workflow_content_reviews WHERE company_id=$1
ORDER BY created_at DESC,review_id DESC LIMIT $2`, companyID, maxContentReviews)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, maxContentReviews)
	for rows.Next() {
		var reviewID string
		if err = rows.Scan(&reviewID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, reviewID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]DomainContentReviewRecord, 0, len(ids))
	for _, reviewID := range ids {
		record, readErr := readDomainContentReview(ctx, tx, companyID, reviewID)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, record)
	}
	return result, nil
}

func readDomainContentReview(ctx context.Context, queryer domainContentDraftQueryer, companyID, reviewID string) (DomainContentReviewRecord, error) {
	var record DomainContentReviewRecord
	var revision int64
	var reviewJSON, sampleJSON, reasonsJSON []byte
	err := queryer.QueryRow(ctx, `SELECT review_id,draft_input_id,draft_revision,draft_sha256,checker_employee_id,outcome,review,sample,reason_codes,COALESCE(correction_id,''),request_id,created_at::text
FROM domain_workflow_content_reviews WHERE company_id=$1 AND review_id=$2`, companyID, reviewID).Scan(
		&record.ReviewID, &record.DraftInputID, &revision, &record.DraftSHA256, &record.CheckerEmployeeID, &record.Outcome,
		&reviewJSON, &sampleJSON, &reasonsJSON, &record.CorrectionID, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentReviewRecord{}, core.OutOfScope
	}
	if err != nil {
		return DomainContentReviewRecord{}, err
	}
	if err = json.Unmarshal(reviewJSON, &record.Review); err != nil {
		return DomainContentReviewRecord{}, core.Integrity
	}
	if err = json.Unmarshal(sampleJSON, &record.Sample); err != nil {
		return DomainContentReviewRecord{}, core.Integrity
	}
	if err = json.Unmarshal(reasonsJSON, &record.ReasonCodes); err != nil {
		return DomainContentReviewRecord{}, core.Integrity
	}
	record.CompanyID = companyID
	record.DraftRevision = strconv.FormatInt(revision, 10)
	var latestRevision int64
	if err = queryer.QueryRow(ctx, `SELECT COALESCE(max(draft_revision),0) FROM domain_workflow_content_drafts
WHERE company_id=$1 AND draft_input_id=$2`, companyID, record.DraftInputID).Scan(&latestRevision); err != nil {
		return DomainContentReviewRecord{}, err
	}
	record.Stale = latestRevision > revision
	var correctionReviewPending bool
	if record.CorrectionID == "" {
		if err = queryer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domain_workflow_content_corrections
WHERE company_id=$1 AND correction_draft_input_id=$2 AND correction_draft_revision=$3)`, companyID, record.DraftInputID, revision).Scan(&correctionReviewPending); err != nil {
			return DomainContentReviewRecord{}, err
		}
	} else {
		if err = queryer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domain_workflow_content_corrections c
WHERE c.company_id=$1 AND c.correction_draft_input_id=$2 AND c.correction_draft_revision=$3
AND NOT EXISTS(SELECT 1 FROM domain_workflow_content_reviews r WHERE r.company_id=c.company_id AND r.correction_id=c.correction_id AND r.outcome='accepted'))`,
			companyID, record.DraftInputID, revision).Scan(&correctionReviewPending); err != nil {
			return DomainContentReviewRecord{}, err
		}
	}
	record.Stale = record.Stale || correctionReviewPending
	if record.Review.DraftRevision != revision || record.Review.CheckerEmployeeID != record.CheckerEmployeeID ||
		record.Sample.DraftRevision != revision || record.Sample.DraftSHA256 != record.DraftSHA256 ||
		(record.Outcome != domainworkflow.OutcomeAccepted && record.Outcome != domainworkflow.OutcomeInconclusive && record.Outcome != domainworkflow.OutcomeRejected) {
		return DomainContentReviewRecord{}, core.Integrity
	}
	return record, nil
}

func (k *Kernel) GetCurrentDomainContentSourceAuthorization(ctx context.Context, companyID, inputID string, revision int64) (DomainContentSourceEventRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(inputID) || revision < 1 {
		return DomainContentSourceEventRecord{}, false, core.Malformed
	}
	var record DomainContentSourceEventRecord
	err := k.pool.QueryRow(ctx, `SELECT company_id,event_seq::text,event_id,input_id,input_revision::text,source_sha256,state,rationale,request_id,created_at::text
FROM domain_workflow_content_source_events WHERE company_id=$1 AND input_id=$2 AND input_revision=$3 ORDER BY event_seq DESC LIMIT 1`, companyID, inputID, revision).Scan(
		&record.CompanyID, &record.EventSeq, &record.EventID, &record.InputID, &record.Revision, &record.SHA256, &record.State, &record.Rationale, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentSourceEventRecord{}, false, nil
	}
	return record, err == nil, err
}

func verifyCurrentContentSourceAuthorization(ctx context.Context, tx pgx.Tx, companyID string, reference domainworkflow.SourceReference) error {
	var state, digest string
	err := tx.QueryRow(ctx, `SELECT state,source_sha256 FROM domain_workflow_content_source_events
WHERE company_id=$1 AND input_id=$2 AND input_revision=$3 ORDER BY event_seq DESC LIMIT 1`, companyID, reference.InputID, reference.Revision).Scan(&state, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	}
	if err != nil {
		return err
	}
	if state != string(DomainContentSourceAuthorized) || digest != reference.SHA256 {
		return core.Denied
	}
	var sourceState, sourceDigest string
	err = tx.QueryRow(ctx, "SELECT state,content_digest FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3", companyID, reference.InputID, reference.Revision).Scan(&sourceState, &sourceDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if sourceState != "usable" || sourceDigest != reference.SHA256 {
		return core.Integrity
	}
	return nil
}

func verifyDomainContentCorrectionReviewTX(ctx context.Context, tx pgx.Tx, companyID, draftInputID string, draftRevision int64, correctionID string) error {
	if correctionID == "" {
		var pending bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domain_workflow_content_corrections
WHERE company_id=$1 AND correction_draft_input_id=$2 AND correction_draft_revision=$3)`, companyID, draftInputID, draftRevision).Scan(&pending)
		if err != nil {
			return err
		}
		if pending {
			return core.Conflict
		}
		return nil
	}
	var correctionInputID, state string
	var correctionRevision int64
	err := tx.QueryRow(ctx, `SELECT correction_draft_input_id,correction_draft_revision,state FROM domain_workflow_content_corrections
WHERE company_id=$1 AND correction_id=$2`, companyID, correctionID).Scan(&correctionInputID, &correctionRevision, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if correctionInputID != draftInputID || correctionRevision != draftRevision || state != "review_required" {
		return core.Conflict
	}
	return nil
}

func contentReviewReferences(review domainworkflow.ContentReview, sample domainworkflow.ContentSampleEvidence) ([]domainworkflow.SourceReference, bool) {
	if len(review.Claims) == 0 || len(review.Claims) > 100 || len(sample.SampledClaimIDs) == 0 || len(sample.SampledClaimIDs) > 100 || !domainworkflow.ValidContentSourceReference(sample.Plan) {
		return nil, false
	}
	byKey := make(map[string]domainworkflow.SourceReference)
	for _, reference := range append([]domainworkflow.SourceReference{sample.Plan}, contentClaimSources(review.Claims)...) {
		if !domainworkflow.ValidContentSourceReference(reference) {
			return nil, false
		}
		key := fmt.Sprintf("%s\x00%d", reference.InputID, reference.Revision)
		if prior, exists := byKey[key]; exists && prior.SHA256 != reference.SHA256 {
			return nil, false
		}
		byKey[key] = reference
		if len(byKey) > 100 {
			return nil, false
		}
	}
	result := make([]domainworkflow.SourceReference, 0, len(byKey))
	for _, reference := range byKey {
		result = append(result, reference)
	}
	return result, true
}

func contentClaimSources(claims []domainworkflow.ClaimReview) []domainworkflow.SourceReference {
	result := make([]domainworkflow.SourceReference, 0)
	for _, claim := range claims {
		result = append(result, claim.Sources...)
	}
	return result
}

func sameDomainContentReviewRequest(record DomainContentReviewRecord, draftInputID string, draftRevision int64, request DomainContentReviewRequest) bool {
	if record.DraftInputID != draftInputID || record.DraftRevision != strconv.FormatInt(draftRevision, 10) || record.CorrectionID != request.CorrectionID || record.RequestID != request.RequestID {
		return false
	}
	priorReview, priorErr := json.Marshal(record.Review)
	requestedReview, requestedErr := json.Marshal(request.Review)
	priorSample, sampleErr := json.Marshal(record.Sample)
	requestedSample, requestedErr := json.Marshal(request.Sample)
	return priorErr == nil && requestedErr == nil && sampleErr == nil &&
		bytes.Equal(priorReview, requestedReview) && bytes.Equal(priorSample, requestedSample)
}
