// pattern: Imperative Shell
package workbench

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ArtifactDeliveryReader interface {
	GetArtifactDeliveryManifest(ctx context.Context, companyID, artifactID string) (ArtifactDeliveryManifestResponse, error)
	GetArtifactDeliveryPackage(ctx context.Context, companyID, artifactID string) (ArtifactDeliveryPackage, error)
}

func (s *PostgresReadStore) GetArtifactDeliveryManifest(ctx context.Context, companyID, artifactID string) (ArtifactDeliveryManifestResponse, error) {
	if err := validateCompanyID(companyID); err != nil || !core.ValidID(artifactID) {
		return ArtifactDeliveryManifestResponse{}, core.Malformed
	}
	var manifest ArtifactDeliveryManifestView
	var byteSize, workspaceRevision int64
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT a.id,a.task_id,a.digest,a.bytes,a.state,a.verdict,
q.checkpoint_id,q.check_id,q.validation_binding_digest,q.workspace_digest,q.workspace_revision,q.runner_revision,q.created_at
FROM artifacts a
JOIN task_validation_artifact_qualifications q ON q.company_id=a.company_id AND q.artifact_id=a.id AND q.task_id=a.task_id
WHERE a.company_id=$1 AND a.id=$2`, companyID, artifactID).Scan(
		&manifest.ArtifactID, &manifest.TaskID, &manifest.Content.SHA256, &byteSize, &manifest.State, &manifest.Verdict,
		&manifest.Qualification.CheckpointID, &manifest.Qualification.ValidationReceiptID, &manifest.Qualification.TaskValidationBindingDigest,
		&manifest.Qualification.WorkspaceDigest, &workspaceRevision, &manifest.Qualification.RunnerRevision, &createdAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArtifactDeliveryManifestResponse{}, errCompanyNotFound
	}
	if err != nil {
		return ArtifactDeliveryManifestResponse{}, err
	}
	var deliveryBlocked bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM tasks t
 JOIN mission_change_requests r ON r.company_id=t.company_id AND r.mission_id=t.mission_id AND r.block_previous_results
 JOIN LATERAL (SELECT state FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
 WHERE t.company_id=$1 AND t.id=$2 AND latest.state NOT IN ('declined','superseded')
)`, companyID, manifest.TaskID).Scan(&deliveryBlocked); err != nil {
		return ArtifactDeliveryManifestResponse{}, err
	}
	if deliveryBlocked {
		return ArtifactDeliveryManifestResponse{}, core.ConflictError{Reason: "artifact delivery is blocked by an unresolved formal requirement change", CurrentState: "change_review"}
	}
	if byteSize <= 0 || manifest.State != "ready" || !validDeliverySHA256(manifest.Content.SHA256) {
		return ArtifactDeliveryManifestResponse{}, core.Integrity
	}
	manifest.SchemaVersion = ArtifactDeliveryManifestSchema
	manifest.CompanyID = companyID
	manifest.Content.FileName = "artifact.bin"
	manifest.Content.ContentType = "application/octet-stream"
	manifest.Content.ByteSize = stringValue(byteSize)
	manifest.Qualification.WorkspaceRevision = stringValue(workspaceRevision)
	manifest.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	_, digest, err := canonicalArtifactDeliveryManifest(manifest)
	if err != nil {
		return ArtifactDeliveryManifestResponse{}, core.Integrity
	}
	return ArtifactDeliveryManifestResponse{Manifest: manifest, ManifestSHA256: digest}, nil
}

func (s *PostgresReadStore) GetArtifactDeliveryPackage(ctx context.Context, companyID, artifactID string) (ArtifactDeliveryPackage, error) {
	manifestResponse, err := s.GetArtifactDeliveryManifest(ctx, companyID, artifactID)
	if err != nil {
		return ArtifactDeliveryPackage{}, err
	}
	artifact, err := s.GetArtifact(ctx, companyID, artifactID)
	if err != nil {
		return ArtifactDeliveryPackage{}, err
	}
	if !artifact.ContentAvailable || artifact.Digest != manifestResponse.Manifest.Content.SHA256 || artifact.Bytes != manifestResponse.Manifest.Content.ByteSize {
		return ArtifactDeliveryPackage{}, core.Integrity
	}
	packaged, err := buildArtifactDeliveryPackage(manifestResponse.Manifest, []byte(artifact.Content))
	if err != nil {
		return ArtifactDeliveryPackage{}, core.Integrity
	}
	return packaged, nil
}
