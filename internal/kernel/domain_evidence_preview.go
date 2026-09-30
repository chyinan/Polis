// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/intake"
)

var ErrDomainEvidencePreviewUnsupported = errors.New("domain evidence source cannot be previewed")

type DomainEvidenceArtifactPreviewEntry struct {
	RelativePath  string `json:"relativePath"`
	FileName      string `json:"fileName"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentSHA256 string `json:"contentSha256"`
	Previewable   bool   `json:"previewable"`
	ReasonCode    string `json:"reasonCode"`
}

type DomainEvidenceArtifactPreviewManifest struct {
	CompanyID     string                               `json:"companyId"`
	RecordID      string                               `json:"recordId"`
	Area          domainworkflow.DomainEvidenceArea    `json:"area"`
	InputID       string                               `json:"inputId"`
	InputRevision int64                                `json:"inputRevision"`
	SourceDigest  string                               `json:"sourceDigest"`
	Entries       []DomainEvidenceArtifactPreviewEntry `json:"entries"`
}

type DomainEvidenceArtifactPreview struct {
	CompanyID     string
	RecordID      string
	Area          domainworkflow.DomainEvidenceArea
	InputID       string
	InputRevision int64
	SourceDigest  string
	FileName      string
	RelativePath  string
	MediaType     string
	ContentSHA256 string
	Content       []byte
}

type domainEvidencePreviewSource struct {
	CompanyID     string
	RecordID      string
	Area          domainworkflow.DomainEvidenceArea
	InputID       string
	InputRevision int64
	SourceDigest  string
	SourceKind    string
	DisplayName   string
	MediaType     string
	Content       []byte
}

func (k *Kernel) ListDomainEvidenceArtifactPreviewEntries(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea) (DomainEvidenceArtifactPreviewManifest, error) {
	source, err := k.readDomainEvidencePreviewSource(ctx, companyID, recordID, area)
	if err != nil {
		return DomainEvidenceArtifactPreviewManifest{}, err
	}
	entries, err := intake.ListDomainEvidencePreviewEntries(source.SourceKind, source.MediaType, source.DisplayName, source.Content)
	if err != nil {
		if isUnsupportedEvidencePreview(err) {
			return DomainEvidenceArtifactPreviewManifest{}, fmt.Errorf("%w: %s", ErrDomainEvidencePreviewUnsupported, err.Error())
		}
		return DomainEvidenceArtifactPreviewManifest{}, core.Integrity
	}
	projected := make([]DomainEvidenceArtifactPreviewEntry, 0, len(entries))
	for _, entry := range entries {
		projected = append(projected, DomainEvidenceArtifactPreviewEntry{
			RelativePath: entry.RelativePath, FileName: entry.FileName, MediaType: entry.MediaType,
			ByteSize: entry.ByteSize, ContentSHA256: entry.ContentSHA256, Previewable: entry.Previewable, ReasonCode: entry.ReasonCode,
		})
	}
	return DomainEvidenceArtifactPreviewManifest{
		CompanyID: source.CompanyID, RecordID: source.RecordID, Area: source.Area, InputID: source.InputID,
		InputRevision: source.InputRevision, SourceDigest: source.SourceDigest, Entries: projected,
	}, nil
}

func (k *Kernel) ReadDomainEvidenceArtifactPreview(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea, relativePath string) (DomainEvidenceArtifactPreview, error) {
	source, err := k.readDomainEvidencePreviewSource(ctx, companyID, recordID, area)
	if err != nil {
		return DomainEvidenceArtifactPreview{}, err
	}
	preparedPreview, err := intake.PrepareDomainEvidencePreview(source.SourceKind, source.MediaType, source.DisplayName, source.Content, relativePath)
	if err != nil {
		if isUnsupportedEvidencePreview(err) {
			return DomainEvidenceArtifactPreview{}, fmt.Errorf("%w: %s", ErrDomainEvidencePreviewUnsupported, err.Error())
		}
		return DomainEvidenceArtifactPreview{}, core.Integrity
	}
	return DomainEvidenceArtifactPreview{
		CompanyID: source.CompanyID, RecordID: source.RecordID, Area: source.Area, InputID: source.InputID,
		InputRevision: source.InputRevision, SourceDigest: source.SourceDigest, FileName: preparedPreview.FileName,
		RelativePath: relativePath, MediaType: preparedPreview.MediaType, ContentSHA256: preparedPreview.ContentSHA256,
		Content: preparedPreview.Content,
	}, nil
}

func (k *Kernel) readDomainEvidencePreviewSource(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea) (domainEvidencePreviewSource, error) {
	if !core.ValidID(companyID) || !core.ValidID(recordID) {
		return domainEvidencePreviewSource{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domainEvidencePreviewSource{}, err
	}
	defer tx.Rollback(ctx)
	var companyExists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)", companyID).Scan(&companyExists); err != nil {
		return domainEvidencePreviewSource{}, err
	}
	if !companyExists {
		return domainEvidencePreviewSource{}, core.OutOfScope
	}
	var record DomainEvidenceRecord
	var evidenceJSON, reasonsJSON []byte
	var sourceKind, displayName, mediaType, inputState string
	var byteSize int64
	source := domainEvidencePreviewSource{}
	err = tx.QueryRow(ctx, `SELECT s.record_id,s.profile_id,s.profile_revision,s.readiness_status,s.qualification_status,s.execution_enabled,s.evidence_digest,s.evidence,s.reason_codes,s.request_id,s.created_at::text,
i.area,i.input_id,i.input_revision,i.input_sha256,m.source_kind,m.display_name,m.media_type,m.byte_size,m.state
FROM domain_workflow_evidence_submissions s
JOIN domain_workflow_evidence_items i ON i.company_id=s.company_id AND i.record_id=s.record_id
JOIN mission_inputs m ON m.company_id=i.company_id AND m.input_id=i.input_id AND m.revision=i.input_revision AND m.content_digest=i.input_sha256
WHERE s.company_id=$1 AND s.record_id=$2 AND i.area=$3 AND s.readiness_status='ready_for_review' AND m.state IN ('usable','partial')`,
		companyID, recordID, area).Scan(
		&record.RecordID, &record.ProfileID, &record.ProfileRevision, &record.ReadinessStatus, &record.QualificationStatus, &record.ExecutionEnabled, &record.EvidenceDigest, &evidenceJSON, &reasonsJSON, &record.RequestID, &record.CreatedAt,
		&source.Area, &source.InputID, &source.InputRevision, &source.SourceDigest, &sourceKind, &displayName, &mediaType, &byteSize, &inputState)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainEvidencePreviewSource{}, core.OutOfScope
	}
	if err != nil {
		return domainEvidencePreviewSource{}, err
	}
	if err = json.Unmarshal(evidenceJSON, &record.Submission); err != nil {
		return domainEvidencePreviewSource{}, core.Integrity
	}
	if err = json.Unmarshal(reasonsJSON, &record.ReasonCodes); err != nil {
		return domainEvidencePreviewSource{}, core.Integrity
	}
	record.CompanyID = companyID
	if !validPersistedDomainEvidenceRecord(record) {
		return domainEvidencePreviewSource{}, core.Integrity
	}
	itemsMatch, err := domainEvidenceItemsMatch(ctx, tx, companyID, record)
	if err != nil {
		return domainEvidencePreviewSource{}, err
	}
	if !itemsMatch {
		return domainEvidencePreviewSource{}, core.Integrity
	}
	content, err := readBlob(k.root, companyID, source.SourceDigest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domainEvidencePreviewSource{}, core.Integrity
		}
		return domainEvidencePreviewSource{}, err
	}
	if int64(len(content)) != byteSize || inputState != "usable" && inputState != "partial" {
		return domainEvidencePreviewSource{}, core.Integrity
	}
	if err = tx.Commit(ctx); err != nil {
		return domainEvidencePreviewSource{}, err
	}
	source.CompanyID = companyID
	source.RecordID = recordID
	source.SourceKind = sourceKind
	source.DisplayName = displayName
	source.MediaType = mediaType
	source.Content = content
	return source, nil
}

func isUnsupportedEvidencePreview(err error) bool {
	var uploadError *intake.UploadError
	return errors.As(err, &uploadError) && uploadError.ReasonCode == "evidence_preview_unsupported"
}
