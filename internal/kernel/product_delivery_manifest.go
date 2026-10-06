// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const durableProductDeliveryManifestSchema = "polis-durable-delivery-manifest@1"

type durableProductDeliveryManifest struct {
	SchemaVersion string                                  `json:"schemaVersion"`
	DeliveryID    string                                  `json:"deliveryId"`
	Revision      string                                  `json:"revision"`
	CompanyID     string                                  `json:"companyId"`
	MissionID     string                                  `json:"missionId"`
	TaskID        string                                  `json:"taskId"`
	ArtifactID    string                                  `json:"artifactId"`
	State         string                                  `json:"state"`
	Artifact      durableProductDeliveryArtifact          `json:"artifact"`
	Sections      []durableProductDeliveryManifestSection `json:"sections"`
	CreatedAt     string                                  `json:"createdAt"`
}

type durableProductDeliveryArtifact struct {
	FileName string `json:"fileName"`
	ByteSize string `json:"byteSize"`
	SHA256   string `json:"sha256"`
}

type durableProductDeliveryManifestSection struct {
	Key    string `json:"key"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

// persistInitialProductDeliveryManifest records the first bounded delivery
// revision and its separate initial user disposition in the Artifact
// publication transaction. Missing evidence stays explicit; this slice does
// not claim the delivery is ready or that a user accepted it.
func persistInitialProductDeliveryManifest(
	ctx context.Context,
	tx pgx.Tx,
	companyID, missionID, taskID, artifactID string,
	artifactBytes int,
	artifactSHA256 string,
	checkpointID string,
	qualification productQualification,
) error {
	var createdAt time.Time
	if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&createdAt); err != nil {
		return err
	}
	createdAt = createdAt.UTC()
	byteSize := fmt.Sprintf("%d", artifactBytes)
	manifest := durableProductDeliveryManifest{
		SchemaVersion: durableProductDeliveryManifestSchema,
		DeliveryID:    artifactID,
		Revision:      "1",
		CompanyID:     companyID,
		MissionID:     missionID,
		TaskID:        taskID,
		ArtifactID:    artifactID,
		State:         "assembling",
		Artifact: durableProductDeliveryArtifact{
			FileName: "artifact.bin",
			ByteSize: byteSize,
			SHA256:   artifactSHA256,
		},
		Sections: []durableProductDeliveryManifestSection{
			{Key: "source_inputs", State: "unavailable", Detail: "Source-input provenance is not captured by this delivery path."},
			{Key: "environment_build", State: "unavailable", Detail: "Environment and build evidence is not captured by this delivery path."},
			{Key: "file_inventory", State: "available", Detail: fmt.Sprintf("artifact.bin; bytes=%s; sha256=%s", byteSize, artifactSHA256)},
			{Key: "run_instructions", State: "unavailable", Detail: "Reproducible run instructions are not captured by this delivery path."},
			{Key: "verification", State: "available", Detail: fmt.Sprintf("checkpoint=%s; validationReceipt=%s; taskValidationBindingDigest=%s; workspaceDigest=%s; workspaceRevision=%d; runnerRevision=%s", checkpointID, qualification.CheckID, qualification.BindingDigest, qualification.WorkspaceDigest, qualification.WorkspaceRevision, qualification.RunnerRevision)},
			{Key: "limitations", State: "unavailable", Detail: "Known limitations have not been reviewed; this initial revision contains only one opaque artifact.bin."},
			{Key: "license_source", State: "unavailable", Detail: "License and source-attribution evidence is not captured by this delivery path."},
			{Key: "feedback", State: "not_requested", Detail: "No user feedback has been requested; download or preview is not acceptance."},
		},
		CreatedAt: createdAt.Format(time.RFC3339Nano),
	}
	for _, section := range manifest.Sections {
		if len(section.Detail) > 512 {
			return fmt.Errorf("delivery manifest section detail exceeds 512 bytes")
		}
	}
	manifestBytes, err := marshalDurableProductDeliveryManifest(manifest)
	if err != nil {
		return err
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	manifestSHA256 := hex.EncodeToString(manifestDigest[:])
	if _, err = tx.Exec(ctx, `INSERT INTO delivery_manifest_revisions(
company_id,delivery_id,revision,mission_id,task_id,artifact_id,state,manifest,manifest_sha256,created_at)
VALUES($1,$2,1,$3,$4,$2,'assembling',$5,$6,$7)`,
		companyID, artifactID, missionID, taskID, manifestBytes, manifestSHA256, createdAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO delivery_user_dispositions(
company_id,delivery_id,revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at)
VALUES($1,$2,1,1,'not_requested','system',$3,$4,NULL,$5)`,
		companyID, artifactID,
		"Initial delivery does not request user feedback; download or preview is not acceptance.",
		artifactID, createdAt)
	return err
}
