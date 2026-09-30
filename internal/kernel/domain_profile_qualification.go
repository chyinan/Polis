package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

type DomainProfileQualificationDecision string

const (
	DomainProfileQualificationQualified DomainProfileQualificationDecision = "qualified"
	DomainProfileQualificationRevoked   DomainProfileQualificationDecision = "revoked"
)

type DomainProfileQualificationInput struct {
	ProfileID             string
	ProfileRevision       string
	Decision              DomainProfileQualificationDecision
	EvidenceInputID       string
	EvidenceInputRevision int64
	Rationale             string
	RequestID             string
}

type DomainProfileQualificationRecord struct {
	CompanyID             string                             `json:"companyId"`
	EventID               string                             `json:"eventId"`
	ProfileID             string                             `json:"profileId"`
	ProfileRevision       string                             `json:"profileRevision"`
	Decision              DomainProfileQualificationDecision `json:"decision"`
	EvidenceInputID       string                             `json:"evidenceInputId"`
	EvidenceInputRevision int64                              `json:"evidenceInputRevision,string"`
	EvidenceSHA256        string                             `json:"evidenceSha256"`
	Rationale             string                             `json:"rationale"`
	Actor                 string                             `json:"actor"`
	RequestID             string                             `json:"requestId"`
	CreatedAt             string                             `json:"createdAt"`
}

func (k *Kernel) TXRecordDomainProfileQualification(ctx context.Context, companyID string, input DomainProfileQualificationInput) (DomainProfileQualificationRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RequestID) || !validDomainProfileQualificationRevision(input.ProfileID, input.ProfileRevision) ||
		(input.Decision != DomainProfileQualificationQualified && input.Decision != DomainProfileQualificationRevoked) {
		return DomainProfileQualificationRecord{}, core.Malformed
	}
	rationale := strings.TrimSpace(input.Rationale)
	if rationale == "" || utf8.RuneCountInString(rationale) > 2000 || strings.ContainsRune(rationale, '\x00') {
		return DomainProfileQualificationRecord{}, core.Malformed
	}
	input.Rationale = rationale
	var report domainworkflow.DomainProfileQualificationEvidence
	evidenceSHA256 := ""
	if input.Decision == DomainProfileQualificationQualified {
		if !core.ValidID(input.EvidenceInputID) || input.EvidenceInputRevision < 1 {
			return DomainProfileQualificationRecord{}, core.Malformed
		}
		var err error
		report, evidenceSHA256, err = k.loadDomainProfileQualificationEvidence(ctx, k.pool, companyID, input.EvidenceInputID, input.EvidenceInputRevision, input.ProfileID, input.ProfileRevision)
		if err != nil {
			return DomainProfileQualificationRecord{}, err
		}
	} else if input.EvidenceInputID != "" || input.EvidenceInputRevision != 0 {
		return DomainProfileQualificationRecord{}, core.Malformed
	}
	eventID := stableCapabilityID("domain-profile-qualification", companyID, input.RequestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "domain.profile.qualification", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var previous string
		err := tx.QueryRow(ctx, `SELECT decision FROM domain_workflow_profile_qualification_events
WHERE company_id=$1 AND profile_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, companyID, input.ProfileID).Scan(&previous)
		if errors.Is(err, pgx.ErrNoRows) {
			previous = ""
		} else if err != nil {
			return Receipt{}, err
		}
		if input.Decision == DomainProfileQualificationRevoked && previous != string(DomainProfileQualificationQualified) {
			return Receipt{}, core.Conflict
		}
		if input.Decision == DomainProfileQualificationQualified && previous == string(DomainProfileQualificationQualified) {
			return Receipt{}, core.Conflict
		}
		if input.Decision == DomainProfileQualificationQualified {
			if err = verifyDomainProfileQualificationReferences(ctx, tx, companyID, report); err != nil {
				return Receipt{}, err
			}
			var currentDigest string
			var currentState string
			if err = tx.QueryRow(ctx, `SELECT content_digest,state FROM mission_inputs
WHERE company_id=$1 AND input_id=$2 AND revision=$3`, companyID, input.EvidenceInputID, input.EvidenceInputRevision).Scan(&currentDigest, &currentState); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			}
			if currentDigest != evidenceSHA256 || currentState != "usable" {
				return Receipt{}, core.Integrity
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO domain_workflow_profile_qualification_events(
company_id,event_id,profile_id,profile_revision,decision,evidence_input_id,evidence_input_revision,evidence_sha256,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,0),NULLIF($8,''),$9,'local-owner',$10)`, companyID, eventID, input.ProfileID, input.ProfileRevision, string(input.Decision), input.EvidenceInputID, input.EvidenceInputRevision, evidenceSHA256, input.Rationale, input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: string(input.Decision)}, nil
	})
	if err != nil {
		return DomainProfileQualificationRecord{}, err
	}
	return readDomainProfileQualificationRecord(ctx, k.pool, companyID, eventID)
}

type domainProfileQualificationMissionInputReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (k *Kernel) loadDomainProfileQualificationEvidence(ctx context.Context, reader domainProfileQualificationMissionInputReader, companyID, inputID string, revision int64, profileID, profileRevision string) (domainworkflow.DomainProfileQualificationEvidence, string, error) {
	var expectedDigest, mediaType, state string
	var byteSize int64
	err := reader.QueryRow(ctx, `SELECT content_digest,media_type,state,byte_size FROM mission_inputs
WHERE company_id=$1 AND input_id=$2 AND revision=$3`, companyID, inputID, revision).Scan(&expectedDigest, &mediaType, &state, &byteSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainworkflow.DomainProfileQualificationEvidence{}, "", core.OutOfScope
	}
	if err != nil {
		return domainworkflow.DomainProfileQualificationEvidence{}, "", err
	}
	if state != "usable" || mediaType != "application/json" || byteSize < 1 || byteSize > domainworkflow.MaxDomainProfileQualificationEvidenceBytes {
		return domainworkflow.DomainProfileQualificationEvidence{}, "", core.Denied
	}
	content, err := readBlobBounded(k.root, companyID, expectedDigest, domainworkflow.MaxDomainProfileQualificationEvidenceBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domainworkflow.DomainProfileQualificationEvidence{}, "", core.Integrity
		}
		return domainworkflow.DomainProfileQualificationEvidence{}, "", err
	}
	digest := sha256.Sum256(content)
	actualDigest := hex.EncodeToString(digest[:])
	if int64(len(content)) != byteSize || actualDigest != expectedDigest {
		return domainworkflow.DomainProfileQualificationEvidence{}, "", core.Integrity
	}
	report, err := domainworkflow.ValidateDomainProfileQualificationEvidence(content, profileID, profileRevision)
	if err != nil {
		return domainworkflow.DomainProfileQualificationEvidence{}, "", core.Malformed
	}
	return report, actualDigest, nil
}

func (k *Kernel) listCurrentDomainProfileQualifications(ctx context.Context, tx pgx.Tx, companyID string) ([]DomainProfileQualificationRecord, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(profile_id)
company_id,event_id,profile_id,profile_revision,decision,COALESCE(evidence_input_id,''),COALESCE(evidence_input_revision,0),COALESCE(evidence_sha256,''),rationale,actor,request_id,created_at::text
FROM domain_workflow_profile_qualification_events
WHERE company_id=$1 ORDER BY profile_id,event_seq DESC`, companyID)
	if err != nil {
		return nil, err
	}
	records := make([]DomainProfileQualificationRecord, 0, len(domainworkflow.ReferenceWorkflows()))
	for rows.Next() {
		var record DomainProfileQualificationRecord
		if err = rows.Scan(&record.CompanyID, &record.EventID, &record.ProfileID, &record.ProfileRevision, &record.Decision,
			&record.EvidenceInputID, &record.EvidenceInputRevision, &record.EvidenceSHA256, &record.Rationale, &record.Actor, &record.RequestID, &record.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if !validDomainProfileQualificationRevision(record.ProfileID, record.ProfileRevision) {
			rows.Close()
			return nil, core.Integrity
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for _, record := range records {
		if record.Decision != DomainProfileQualificationQualified {
			continue
		}
		report, digest, readErr := k.loadDomainProfileQualificationEvidence(ctx, tx, companyID, record.EvidenceInputID, record.EvidenceInputRevision, record.ProfileID, record.ProfileRevision)
		if readErr != nil || digest != record.EvidenceSHA256 {
			return nil, core.Integrity
		}
		if verifyErr := verifyDomainProfileQualificationReferences(ctx, tx, companyID, report); verifyErr != nil {
			return nil, core.Integrity
		}
	}
	return records, nil
}

func verifyDomainProfileQualificationReferences(ctx context.Context, reader domainEvidenceReviewReader, companyID string, report domainworkflow.DomainProfileQualificationEvidence) error {
	for _, areaEvidence := range report.Areas {
		for _, reference := range areaEvidence.Cases {
			submission, err := readDomainEvidenceSubmissionForProfileQualification(ctx, reader, companyID, reference.RecordID)
			if err != nil {
				if errors.Is(err, core.OutOfScope) {
					return core.Denied
				}
				return err
			}
			if submission.ProfileID != report.ProfileID || submission.ProfileRevision != report.ProfileRevision {
				return core.Denied
			}
			submissionDigest, _, err := domainEvidenceSubmissionDigest(submission)
			if err != nil {
				return core.Integrity
			}
			if submissionDigest != reference.EvidenceDigest {
				return core.Integrity
			}
			review, err := readDomainEvidenceReview(ctx, reader, companyID, reference.RecordID)
			if err != nil {
				return err
			}
			assessment, err := readDomainEvidenceSubstantiveAssessment(ctx, reader, companyID, reference.RecordID)
			if err != nil {
				return err
			}
			if review == nil || assessment == nil || assessment.AssessmentID != reference.AssessmentID ||
				!validPersistedDomainEvidenceReview(submission, *review) ||
				!validPersistedDomainEvidenceSubstantiveAssessment(submission, submissionDigest, review, *assessment) ||
				assessment.Outcome != domainworkflow.DomainEvidenceSubstantiveAccepted {
				return core.Denied
			}
			areaAccepted := false
			for _, areaAssessment := range assessment.AreaAssessments {
				if areaAssessment.Area == areaEvidence.Area && areaAssessment.Outcome == domainworkflow.DomainAssessmentAccepted {
					areaAccepted = true
					break
				}
			}
			if !areaAccepted {
				return core.Denied
			}
		}
	}
	return nil
}

func readDomainEvidenceSubmissionForProfileQualification(ctx context.Context, reader domainEvidenceReviewReader, companyID, recordID string) (domainworkflow.DomainEvidenceSubmission, error) {
	var record DomainEvidenceRecord
	var evidenceJSON, reasonsJSON []byte
	err := reader.QueryRow(ctx, `SELECT record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id,created_at::text
FROM domain_workflow_evidence_submissions WHERE company_id=$1 AND record_id=$2`, companyID, recordID).Scan(
		&record.RecordID, &record.ProfileID, &record.ProfileRevision, &record.ReadinessStatus, &record.QualificationStatus,
		&record.ExecutionEnabled, &record.EvidenceDigest, &evidenceJSON, &reasonsJSON, &record.RequestID, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainworkflow.DomainEvidenceSubmission{}, core.OutOfScope
	}
	if err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, err
	}
	if json.Unmarshal(evidenceJSON, &record.Submission) != nil || json.Unmarshal(reasonsJSON, &record.ReasonCodes) != nil || !validPersistedDomainEvidenceRecord(record) {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	itemsMatch, err := domainEvidenceItemsMatch(ctx, reader, companyID, record)
	if err != nil {
		return domainworkflow.DomainEvidenceSubmission{}, err
	}
	if !itemsMatch {
		return domainworkflow.DomainEvidenceSubmission{}, core.Integrity
	}
	return record.Submission, nil
}

type domainProfileQualificationReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readDomainProfileQualificationRecord(ctx context.Context, reader domainProfileQualificationReader, companyID, eventID string) (DomainProfileQualificationRecord, error) {
	var record DomainProfileQualificationRecord
	err := reader.QueryRow(ctx, `SELECT company_id,event_id,profile_id,profile_revision,decision,COALESCE(evidence_input_id,''),COALESCE(evidence_input_revision,0),COALESCE(evidence_sha256,''),rationale,actor,request_id,created_at::text
FROM domain_workflow_profile_qualification_events WHERE company_id=$1 AND event_id=$2`, companyID, eventID).Scan(
		&record.CompanyID, &record.EventID, &record.ProfileID, &record.ProfileRevision, &record.Decision,
		&record.EvidenceInputID, &record.EvidenceInputRevision, &record.EvidenceSHA256, &record.Rationale, &record.Actor, &record.RequestID, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainProfileQualificationRecord{}, core.OutOfScope
	}
	return record, err
}

func validDomainProfileQualificationRevision(profileID, profileRevision string) bool {
	for _, profile := range domainworkflow.ReferenceWorkflows() {
		if profile.ID == profileID && profile.Revision == profileRevision {
			return true
		}
	}
	return false
}
