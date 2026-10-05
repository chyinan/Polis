// pattern: Imperative Shell
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
)

type TaskInputManifestReader interface {
	GetTaskInputManifest(ctx context.Context, companyID, taskID string) (TaskInputManifestView, error)
}

func (s *PostgresReadStore) GetTaskInputManifest(ctx context.Context, companyID, taskID string) (TaskInputManifestView, error) {
	if !core.ValidID(companyID) || !core.ValidID(taskID) {
		return TaskInputManifestView{}, core.Malformed
	}
	var missionID, digest string
	var encoded []byte
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT mission_id,manifest_digest,manifest,created_at
FROM task_input_manifests WHERE company_id=$1 AND task_id=$2`, companyID, taskID).Scan(&missionID, &digest, &encoded, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskInputManifestView{}, core.OutOfScope
	}
	if err != nil {
		return TaskInputManifestView{}, err
	}
	var manifest intake.ModelInputManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return TaskInputManifestView{}, core.Integrity
	}
	deliveryStatus := manifest.DeliveryStatus
	var phase, outcome, workerState, attemptManifestDigest, payloadDigest string
	var inputRefsJSON, inputExclusionsJSON []byte
	var hasAttempt bool
	var inputRefs []intake.ModelInputDeliveryRef
	var inputExclusions []intake.ModelInputExclusion
	directoryFilesByInputID := make(map[string][]intake.DirectoryInputFile)
	contentByInputID := make(map[string][]byte)
	var payloadDigestView *string
	err = s.pool.QueryRow(ctx, `SELECT d.phase,d.outcome,s.state,d.input_refs,d.input_exclusions,d.manifest_digest,d.payload_digest
FROM task_input_delivery_attempts d
JOIN worker_sessions s ON s.company_id=d.company_id AND s.id=d.session_id
WHERE d.company_id=$1 AND d.task_id=$2
ORDER BY d.created_at DESC,CASE d.phase WHEN 'final' THEN 0 ELSE 1 END
LIMIT 1`, companyID, taskID).Scan(&phase, &outcome, &workerState, &inputRefsJSON, &inputExclusionsJSON, &attemptManifestDigest, &payloadDigest)
	if err == nil {
		hasAttempt = true
		if attemptManifestDigest != digest || !validDeliveryDigest(payloadDigest) {
			return TaskInputManifestView{}, core.Integrity
		}
		if err = json.Unmarshal(inputRefsJSON, &inputRefs); err != nil {
			return TaskInputManifestView{}, core.Integrity
		}
		if err = json.Unmarshal(inputExclusionsJSON, &inputExclusions); err != nil {
			return TaskInputManifestView{}, core.Integrity
		}
		if phase == "final" {
			deliveryStatus = outcome
		} else if workerState == "active" {
			deliveryStatus = "sending"
		} else {
			deliveryStatus = "outcome_unknown"
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TaskInputManifestView{}, err
	}
	if inputRefs == nil {
		inputRefs = []intake.ModelInputDeliveryRef{}
	}
	if inputExclusions == nil {
		inputExclusions = []intake.ModelInputExclusion{}
	}
	if hasAttempt {
		directoryArchives, sourceBytes := 0, int64(0)
		for _, candidate := range manifest.CandidateInputs {
			if !intake.IsInputArchiveSource(candidate.SourceKind) && !intake.ProviderTextInputEligible(candidate) && !intake.ProviderImageInputEligible(candidate) && !intake.ProviderCSVInputEligible(candidate) {
				continue
			}
			if intake.IsInputArchiveSource(candidate.SourceKind) {
				directoryArchives++
			}
			sourceBytes += candidate.ByteSize
			if directoryArchives > intake.MaxModelInputDirectoryArchives || sourceBytes > intake.MaxModelInputSourceBytes {
				return TaskInputManifestView{}, core.Integrity
			}
			blob, blobErr := s.readTaskInputBlob(companyID, candidate.ContentDigest, candidate.ByteSize)
			if blobErr != nil {
				return TaskInputManifestView{}, core.Integrity
			}
			contentByInputID[candidate.InputID] = blob
			if intake.IsInputArchiveSource(candidate.SourceKind) {
				files, extractErr := intake.ExtractVerifiedInputArchive(candidate.SourceKind, blob)
				if extractErr != nil {
					return TaskInputManifestView{}, core.Integrity
				}
				directoryFilesByInputID[candidate.InputID] = files
			}
		}
		canonicalContext, contextErr := intake.PrepareModelInputContextWithCSVTables(manifest, digest, contentByInputID)
		if contextErr != nil || canonicalContext.PayloadDigest != payloadDigest {
			legacyContext, legacyErr := intake.PrepareLegacyModelInputContext(manifest, digest, contentByInputID)
			if legacyErr != nil || legacyContext.PayloadDigest != payloadDigest {
				return TaskInputManifestView{}, core.Integrity
			}
			canonicalContext = legacyContext
		}
		expectedRefs := modelInputDeliveryRefs(canonicalContext)
		if !reflect.DeepEqual(expectedRefs, inputRefs) || !reflect.DeepEqual(canonicalContext.Excluded, inputExclusions) {
			return TaskInputManifestView{}, core.Integrity
		}
		payloadDigestView = &payloadDigest
	}
	view, err := newTaskInputManifestView(companyID, missionID, taskID, digest, deliveryStatus, payloadDigestView, manifest, inputRefs, inputExclusions, directoryFilesByInputID, hasAttempt, createdAt)
	if err != nil {
		return TaskInputManifestView{}, core.Integrity
	}
	return view, nil
}

func modelInputDeliveryRefs(payload intake.ModelInputContext) []intake.ModelInputDeliveryRef {
	refs := make([]intake.ModelInputDeliveryRef, 0, len(payload.Inputs)+len(payload.Images)+len(payload.CSVs))
	for _, item := range payload.Inputs {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	for _, item := range payload.Images {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	for _, item := range payload.CSVs {
		summaryBytes, _ := json.Marshal(item)
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, MediaType: item.Reference.MediaType, ByteSize: item.Reference.ByteSize, ContentDigest: item.SourceDigest, Representation: "csv_table_summary", RepresentationBytes: int64(len(summaryBytes))})
	}
	return refs
}
