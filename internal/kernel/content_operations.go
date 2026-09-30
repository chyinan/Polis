// pattern: Imperative Shell

package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

const (
	maxContentOperationInputs = 1 << 20
	maxContentSourceEvents    = 200
	maxContentDrafts          = 100
	maxContentReviews         = 100
)

type DomainContentSourceState string

const (
	DomainContentSourceAuthorized DomainContentSourceState = "authorized"
	DomainContentSourceRevoked    DomainContentSourceState = "revoked"
)

type DomainContentSourceAuthorizationInput struct {
	InputID   string
	Revision  int64
	SHA256    string
	State     DomainContentSourceState
	Rationale string
	RequestID string
}

type DomainContentSourceEventRecord struct {
	CompanyID string                   `json:"companyId"`
	EventSeq  string                   `json:"eventSeq"`
	EventID   string                   `json:"eventId"`
	InputID   string                   `json:"inputId"`
	Revision  string                   `json:"revision"`
	SHA256    string                   `json:"sha256"`
	State     DomainContentSourceState `json:"state"`
	Rationale string                   `json:"rationale"`
	RequestID string                   `json:"requestId"`
	CreatedAt string                   `json:"createdAt"`
}

type DomainContentDraftInput struct {
	DraftInputID string
	Draft        domainworkflow.ContentDraft
	RequestID    string
}

type DomainContentDraftRecord struct {
	CompanyID         string   `json:"companyId"`
	DraftID           string   `json:"draftId"`
	DraftInputID      string   `json:"draftInputId"`
	DraftRevision     string   `json:"draftRevision"`
	DraftSHA256       string   `json:"draftSha256"`
	WriterEmployeeID  string   `json:"writerEmployeeId"`
	CriticalClaims    []string `json:"criticalClaims"`
	ConstraintsPassed bool     `json:"constraintsPassed"`
	RequestID         string   `json:"requestId"`
	CreatedAt         string   `json:"createdAt"`
}

type DomainContentReviewInput struct {
	DraftInputID  string
	DraftRevision int64
	Review        domainworkflow.ContentReview
	Sample        domainworkflow.ContentSampleEvidence
	RequestID     string
}

type DomainContentReviewRecord struct {
	CompanyID         string                               `json:"companyId"`
	ReviewID          string                               `json:"reviewId"`
	DraftInputID      string                               `json:"draftInputId"`
	DraftRevision     string                               `json:"draftRevision"`
	DraftSHA256       string                               `json:"draftSha256"`
	CorrectionID      string                               `json:"correctionId"`
	CheckerEmployeeID string                               `json:"checkerEmployeeId"`
	Outcome           domainworkflow.Outcome               `json:"outcome"`
	Stale             bool                                 `json:"stale"`
	Review            domainworkflow.ContentReview         `json:"review"`
	Sample            domainworkflow.ContentSampleEvidence `json:"sample"`
	ReasonCodes       []string                             `json:"reasonCodes"`
	RequestID         string                               `json:"requestId"`
	CreatedAt         string                               `json:"createdAt"`
}

type ContentOperationInputSnapshot struct {
	Reference domainworkflow.SourceReference
	MediaType string
	Bytes     []byte
}

func (k *Kernel) ReadContentOperationInput(ctx context.Context, companyID, inputID string, revision int64, maxBytes int64) (ContentOperationInputSnapshot, error) {
	if !core.ValidID(companyID) || !core.ValidID(inputID) || revision < 1 || maxBytes < 1 || maxBytes > 8<<20 {
		return ContentOperationInputSnapshot{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ContentOperationInputSnapshot{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, Scope{company: companyID}, nil); err != nil {
		return ContentOperationInputSnapshot{}, err
	}
	var companyState, digest, inputState, mediaType string
	var byteSize int64
	err = tx.QueryRow(ctx, `SELECT c.state,i.content_digest,i.state,i.media_type,i.byte_size
FROM companies c JOIN mission_inputs i ON i.company_id=c.id
WHERE c.id=$1 AND i.input_id=$2 AND i.revision=$3`, companyID, inputID, revision).Scan(&companyState, &digest, &inputState, &mediaType, &byteSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentOperationInputSnapshot{}, core.OutOfScope
	}
	if err != nil {
		return ContentOperationInputSnapshot{}, err
	}
	if companyState != "active" || inputState != "usable" || byteSize < 1 || byteSize > maxBytes || !(mediaType == "application/json" || strings.HasPrefix(mediaType, "text/")) {
		return ContentOperationInputSnapshot{}, core.Denied
	}
	if err = tx.Commit(ctx); err != nil {
		return ContentOperationInputSnapshot{}, err
	}
	content, err := readBlobBounded(k.root, companyID, digest, maxBytes)
	if err != nil {
		return ContentOperationInputSnapshot{}, err
	}
	if int64(len(content)) != byteSize {
		return ContentOperationInputSnapshot{}, core.Integrity
	}
	return ContentOperationInputSnapshot{Reference: domainworkflow.SourceReference{InputID: inputID, Revision: revision, SHA256: digest}, MediaType: mediaType, Bytes: content}, nil
}

func (k *Kernel) TXSetContentSourceAuthorization(ctx context.Context, companyID string, input DomainContentSourceAuthorizationInput) (DomainContentSourceEventRecord, error) {
	rationale := strings.TrimSpace(input.Rationale)
	if !core.ValidID(companyID) || !core.ValidID(input.InputID) || input.Revision < 1 || !validEnvironmentSHA256(input.SHA256) ||
		(input.State != DomainContentSourceAuthorized && input.State != DomainContentSourceRevoked) || rationale == "" || utf8.RuneCountInString(rationale) > 1000 || !core.ValidID(input.RequestID) {
		return DomainContentSourceEventRecord{}, core.Malformed
	}
	input.Rationale = rationale
	if prior, err := k.GetDomainContentSourceEventByRequest(ctx, companyID, input.RequestID); err == nil {
		if prior.InputID != input.InputID || prior.Revision != strconv.FormatInt(input.Revision, 10) || prior.SHA256 != input.SHA256 || prior.State != input.State || prior.Rationale != input.Rationale {
			return DomainContentSourceEventRecord{}, core.Conflict
		}
		return prior, nil
	} else if !errors.Is(err, core.OutOfScope) {
		return DomainContentSourceEventRecord{}, err
	}
	if input.State == DomainContentSourceAuthorized {
		snapshot, err := k.ReadContentOperationInput(ctx, companyID, input.InputID, input.Revision, maxContentOperationInputs)
		if err != nil {
			return DomainContentSourceEventRecord{}, err
		}
		if snapshot.Reference.SHA256 != input.SHA256 {
			return DomainContentSourceEventRecord{}, core.Integrity
		}
	}
	eventID := stableCapabilityID("content-source-event", companyID, input.RequestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "domain.content.source.authorization", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var digest, state string
		var size int64
		var mediaType string
		err := tx.QueryRow(ctx, `SELECT content_digest,state,media_type,byte_size FROM mission_inputs
WHERE company_id=$1 AND input_id=$2 AND revision=$3`, companyID, input.InputID, input.Revision).Scan(&digest, &state, &mediaType, &size)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if digest != input.SHA256 {
			return Receipt{}, core.Integrity
		}
		if input.State == DomainContentSourceRevoked {
			var currentState string
			err = tx.QueryRow(ctx, `SELECT state FROM domain_workflow_content_source_events
WHERE company_id=$1 AND input_id=$2 AND input_revision=$3 ORDER BY event_seq DESC LIMIT 1`, companyID, input.InputID, input.Revision).Scan(&currentState)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentState != string(DomainContentSourceAuthorized)) {
				return Receipt{}, core.Conflict
			}
			if err != nil {
				return Receipt{}, err
			}
		}
		if input.State == DomainContentSourceAuthorized && (state != "usable" || size < 1 || size > maxContentOperationInputs || !(mediaType == "application/json" || strings.HasPrefix(mediaType, "text/"))) {
			return Receipt{}, core.Denied
		}
		_, err = tx.Exec(ctx, `INSERT INTO domain_workflow_content_source_events(company_id,event_id,input_id,input_revision,source_sha256,state,rationale,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, companyID, eventID, input.InputID, input.Revision, input.SHA256, input.State, input.Rationale, input.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: string(input.State)}, nil
	})
	if err != nil {
		return DomainContentSourceEventRecord{}, err
	}
	return k.GetDomainContentSourceEventByRequest(ctx, companyID, input.RequestID)
}

func (k *Kernel) GetDomainContentSourceEventByRequest(ctx context.Context, companyID, requestID string) (DomainContentSourceEventRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return DomainContentSourceEventRecord{}, core.Malformed
	}
	var record DomainContentSourceEventRecord
	err := k.pool.QueryRow(ctx, `SELECT company_id,event_seq::text,event_id,input_id,input_revision::text,source_sha256,state,rationale,request_id,created_at::text
FROM domain_workflow_content_source_events WHERE company_id=$1 AND request_id=$2`, companyID, requestID).Scan(
		&record.CompanyID, &record.EventSeq, &record.EventID, &record.InputID, &record.Revision, &record.SHA256, &record.State, &record.Rationale, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentSourceEventRecord{}, core.OutOfScope
	}
	return record, err
}

func (k *Kernel) TXRegisterContentDraft(ctx context.Context, companyID string, input DomainContentDraftInput) (DomainContentDraftRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.DraftInputID) || !core.ValidID(input.RequestID) || !domainworkflow.ValidContentDraft(input.Draft) {
		return DomainContentDraftRecord{}, core.Malformed
	}
	if prior, found, err := k.GetDomainContentDraftByRequest(ctx, companyID, input.RequestID); err != nil {
		return DomainContentDraftRecord{}, err
	} else if found {
		if prior.DraftInputID != input.DraftInputID || prior.DraftRevision != strconv.FormatInt(input.Draft.Revision, 10) ||
			prior.DraftSHA256 != input.Draft.BodySHA256 || prior.WriterEmployeeID != input.Draft.WriterEmployeeID ||
			prior.ConstraintsPassed != input.Draft.ConstraintsPassed || !sameContentClaimIDs(prior.CriticalClaims, input.Draft.CriticalClaims) {
			return DomainContentDraftRecord{}, core.Conflict
		}
		return prior, nil
	}
	snapshot, err := k.ReadContentOperationInput(ctx, companyID, input.DraftInputID, input.Draft.Revision, maxContentOperationInputs)
	if err != nil {
		return DomainContentDraftRecord{}, err
	}
	if snapshot.Reference.SHA256 != input.Draft.BodySHA256 {
		return DomainContentDraftRecord{}, core.Integrity
	}
	draftID := stableCapabilityID("content-draft", companyID, input.DraftInputID, strconv.FormatInt(input.Draft.Revision, 10))
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "domain.content.draft.register", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var writerExists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)", companyID, input.Draft.WriterEmployeeID).Scan(&writerExists); err != nil {
			return Receipt{}, err
		}
		if !writerExists {
			return Receipt{}, core.OutOfScope
		}
		claimsJSON, err := json.Marshal(input.Draft.CriticalClaims)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		_, err = tx.Exec(ctx, `INSERT INTO domain_workflow_content_drafts(company_id,draft_input_id,draft_revision,draft_sha256,writer_employee_id,critical_claims,constraints_passed,request_id)
VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8)`, companyID, input.DraftInputID, input.Draft.Revision, input.Draft.BodySHA256, input.Draft.WriterEmployeeID, claimsJSON, input.Draft.ConstraintsPassed, input.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: draftID, Status: "drafted", Revision: input.Draft.Revision}, nil
	})
	if err != nil {
		return DomainContentDraftRecord{}, err
	}
	return k.GetDomainContentDraft(ctx, companyID, input.DraftInputID, input.Draft.Revision)
}

func (k *Kernel) GetDomainContentDraft(ctx context.Context, companyID, inputID string, revision int64) (DomainContentDraftRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(inputID) || revision < 1 {
		return DomainContentDraftRecord{}, core.Malformed
	}
	return readDomainContentDraft(ctx, k.pool, companyID, inputID, revision)
}

func (k *Kernel) GetDomainContentDraftByRequest(ctx context.Context, companyID, requestID string) (DomainContentDraftRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return DomainContentDraftRecord{}, false, core.Malformed
	}
	var inputID string
	var revision int64
	err := k.pool.QueryRow(ctx, "SELECT draft_input_id,draft_revision FROM domain_workflow_content_drafts WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(&inputID, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentDraftRecord{}, false, nil
	}
	if err != nil {
		return DomainContentDraftRecord{}, false, err
	}
	record, err := k.GetDomainContentDraft(ctx, companyID, inputID, revision)
	return record, err == nil, err
}

func (k *Kernel) ListDomainContentSourceEvents(ctx context.Context, companyID string) ([]DomainContentSourceEventRecord, error) {
	if !core.ValidID(companyID) {
		return nil, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	events, err := listDomainContentSourceEvents(ctx, tx, companyID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

type domainContentSourceQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listDomainContentSourceEvents(ctx context.Context, queryer domainContentSourceQueryer, companyID string) ([]DomainContentSourceEventRecord, error) {
	rows, err := queryer.Query(ctx, `SELECT company_id,event_seq::text,event_id,input_id,input_revision::text,source_sha256,state,rationale,request_id,created_at::text
FROM domain_workflow_content_source_events WHERE company_id=$1 ORDER BY event_seq DESC LIMIT $2`, companyID, maxContentSourceEvents)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DomainContentSourceEventRecord, 0)
	for rows.Next() {
		var record DomainContentSourceEventRecord
		if err = rows.Scan(&record.CompanyID, &record.EventSeq, &record.EventID, &record.InputID, &record.Revision, &record.SHA256, &record.State, &record.Rationale, &record.RequestID, &record.CreatedAt); err != nil {
			return nil, err
		}
		if !core.ValidID(record.CompanyID) || record.CompanyID != companyID || !core.ValidID(record.EventID) || !core.ValidID(record.InputID) ||
			!validEnvironmentSHA256(record.SHA256) || (record.State != DomainContentSourceAuthorized && record.State != DomainContentSourceRevoked) {
			return nil, core.Integrity
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (k *Kernel) listDomainContentDrafts(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainContentDraftRecord, error) {
	rows, err := tx.Query(ctx, `SELECT draft_input_id,draft_revision,draft_sha256,writer_employee_id,critical_claims,constraints_passed,request_id,created_at::text
FROM domain_workflow_content_drafts WHERE company_id=$1 ORDER BY created_at DESC,draft_input_id,draft_revision DESC LIMIT $2`, companyID, maxContentDrafts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DomainContentDraftRecord, 0)
	for rows.Next() {
		var record DomainContentDraftRecord
		var revision int64
		var claimsJSON []byte
		if err = rows.Scan(&record.DraftInputID, &revision, &record.DraftSHA256, &record.WriterEmployeeID, &claimsJSON, &record.ConstraintsPassed, &record.RequestID, &record.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(claimsJSON, &record.CriticalClaims); err != nil {
			return nil, core.Integrity
		}
		record.CompanyID, record.DraftRevision = companyID, strconv.FormatInt(revision, 10)
		record.DraftID = stableCapabilityID("content-draft", companyID, record.DraftInputID, record.DraftRevision)
		if !domainworkflow.ValidContentDraft(domainworkflow.ContentDraft{Revision: revision, WriterEmployeeID: record.WriterEmployeeID, BodySHA256: record.DraftSHA256, CriticalClaims: record.CriticalClaims, ConstraintsPassed: record.ConstraintsPassed}) {
			return nil, core.Integrity
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

type domainContentDraftQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readDomainContentDraft(ctx context.Context, queryer domainContentDraftQueryer, companyID, inputID string, revision int64) (DomainContentDraftRecord, error) {
	var record DomainContentDraftRecord
	var claimsJSON []byte
	err := queryer.QueryRow(ctx, `SELECT draft_sha256,writer_employee_id,critical_claims,constraints_passed,request_id,created_at::text
FROM domain_workflow_content_drafts WHERE company_id=$1 AND draft_input_id=$2 AND draft_revision=$3`, companyID, inputID, revision).Scan(
		&record.DraftSHA256, &record.WriterEmployeeID, &claimsJSON, &record.ConstraintsPassed, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainContentDraftRecord{}, core.OutOfScope
	}
	if err != nil {
		return DomainContentDraftRecord{}, err
	}
	if err = json.Unmarshal(claimsJSON, &record.CriticalClaims); err != nil {
		return DomainContentDraftRecord{}, core.Integrity
	}
	record.CompanyID, record.DraftInputID = companyID, inputID
	record.DraftRevision = strconv.FormatInt(revision, 10)
	record.DraftID = stableCapabilityID("content-draft", companyID, inputID, record.DraftRevision)
	if !domainworkflow.ValidContentDraft(domainworkflow.ContentDraft{Revision: revision, WriterEmployeeID: record.WriterEmployeeID, BodySHA256: record.DraftSHA256, CriticalClaims: record.CriticalClaims, ConstraintsPassed: record.ConstraintsPassed}) {
		return DomainContentDraftRecord{}, core.Integrity
	}
	return record, nil
}
