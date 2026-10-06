// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
)

const DurableDeliveryManifestSchema = "polis-durable-delivery-manifest@1"

type DurableDeliveryManifestSection struct {
	Key    string `json:"key"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type DurableDeliveryManifestArtifact struct {
	FileName string `json:"fileName"`
	ByteSize string `json:"byteSize"`
	SHA256   string `json:"sha256"`
}

type DurableDeliveryManifestView struct {
	SchemaVersion string                           `json:"schemaVersion"`
	DeliveryID    string                           `json:"deliveryId"`
	Revision      string                           `json:"revision"`
	CompanyID     string                           `json:"companyId"`
	MissionID     string                           `json:"missionId"`
	TaskID        string                           `json:"taskId"`
	ArtifactID    string                           `json:"artifactId"`
	State         string                           `json:"state"`
	Artifact      DurableDeliveryManifestArtifact  `json:"artifact"`
	Sections      []DurableDeliveryManifestSection `json:"sections"`
	CreatedAt     string                           `json:"createdAt"`
}

type DurableDeliveryUserDispositionView struct {
	Revision         string `json:"revision"`
	ManifestRevision string `json:"manifestRevision"`
	State            string `json:"state"`
	Actor            string `json:"actor"`
	Reason           string `json:"reason"`
	RequestID        string `json:"requestId"`
	FeedbackDeadline string `json:"feedbackDeadline"`
	CreatedAt        string `json:"createdAt"`
}

type DurableDeliveryLifecycleResponse struct {
	Manifest        DurableDeliveryManifestView        `json:"manifest"`
	ManifestSHA256  string                             `json:"manifestSha256"`
	UserDisposition DurableDeliveryUserDispositionView `json:"userDisposition"`
}

type DurableArtifactDeliveryLifecycleReader interface {
	GetDurableArtifactDeliveryLifecycle(ctx context.Context, companyID, artifactID string) (DurableDeliveryLifecycleResponse, error)
}

func (s *PostgresReadStore) GetDurableArtifactDeliveryLifecycle(ctx context.Context, companyID, artifactID string) (DurableDeliveryLifecycleResponse, error) {
	if err := validateCompanyID(companyID); err != nil || !core.ValidID(artifactID) {
		return DurableDeliveryLifecycleResponse{}, core.Malformed
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return DurableDeliveryLifecycleResponse{}, err
	}
	defer tx.Rollback(ctx)
	var rawManifest string
	var manifestSHA256 string
	var storedRevision, storedMissionID, storedTaskID, storedArtifactID, storedState string
	var artifactDigest, artifactState string
	var artifactBytes int64
	var disposition DurableDeliveryUserDispositionView
	var dispositionCreatedAt time.Time
	var feedbackDeadline pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT r.revision::text,r.mission_id,r.task_id,r.artifact_id,r.state,r.manifest::text,r.manifest_sha256,
	a.digest,a.bytes,a.state,
 d.revision::text,d.manifest_revision::text,d.state,d.actor,COALESCE(d.reason,''),COALESCE(d.request_id,''),d.feedback_deadline,d.created_at
FROM delivery_manifest_revisions r
JOIN artifacts a ON a.company_id=r.company_id AND a.id=r.artifact_id AND a.task_id=r.task_id
JOIN LATERAL (
 SELECT revision,manifest_revision,state,actor,reason,request_id,feedback_deadline,created_at
 FROM delivery_user_dispositions
 WHERE company_id=r.company_id AND delivery_id=r.delivery_id AND manifest_revision=r.revision
 ORDER BY revision DESC LIMIT 1
) d ON true
	WHERE r.company_id=$1 AND r.delivery_id=$2 AND r.artifact_id=$2
	ORDER BY r.revision DESC LIMIT 1`, companyID, artifactID).Scan(
		&storedRevision, &storedMissionID, &storedTaskID, &storedArtifactID, &storedState, &rawManifest, &manifestSHA256,
		&artifactDigest, &artifactBytes, &artifactState,
		&disposition.Revision, &disposition.ManifestRevision, &disposition.State, &disposition.Actor,
		&disposition.Reason, &disposition.RequestID, &feedbackDeadline, &dispositionCreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DurableDeliveryLifecycleResponse{}, errCompanyNotFound
	}
	if err != nil {
		return DurableDeliveryLifecycleResponse{}, err
	}
	var manifest DurableDeliveryManifestView
	decoder := json.NewDecoder(bytes.NewBufferString(rawManifest))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return DurableDeliveryLifecycleResponse{}, core.Integrity
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return DurableDeliveryLifecycleResponse{}, core.Integrity
	}
	_, digest, err := canonicalDurableDeliveryManifest(manifest)
	manifestByteSize, _ := strconv.ParseInt(manifest.Artifact.ByteSize, 10, 64)
	if err != nil || digest != manifestSHA256 || manifest.CompanyID != companyID || manifest.ArtifactID != artifactID || manifest.DeliveryID != artifactID || manifest.Revision != storedRevision || manifest.MissionID != storedMissionID || manifest.TaskID != storedTaskID || manifest.ArtifactID != storedArtifactID || manifest.State != storedState || artifactDigest != manifest.Artifact.SHA256 || artifactBytes != manifestByteSize || ((manifest.State == "assembling" || manifest.State == "ready") && artifactState != "ready") {
		return DurableDeliveryLifecycleResponse{}, core.Integrity
	}
	var deliveryBlocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM tasks t
 JOIN mission_change_requests r ON r.company_id=t.company_id AND r.mission_id=t.mission_id AND r.block_previous_results
 JOIN LATERAL (SELECT state FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
 WHERE t.company_id=$1 AND t.id=$2 AND latest.state NOT IN ('declined','superseded')
)`, companyID, manifest.TaskID).Scan(&deliveryBlocked); err != nil {
		return DurableDeliveryLifecycleResponse{}, err
	}
	if deliveryBlocked {
		return DurableDeliveryLifecycleResponse{}, core.ConflictError{Reason: "artifact delivery is blocked by an unresolved formal requirement change", CurrentState: "change_review"}
	}
	disposition.CreatedAt = dispositionCreatedAt.UTC().Format(time.RFC3339Nano)
	if feedbackDeadline.Valid {
		disposition.FeedbackDeadline = feedbackDeadline.Time.UTC().Format(time.RFC3339Nano)
	}
	if !validDurableDeliveryDisposition(disposition) || disposition.ManifestRevision != manifest.Revision {
		return DurableDeliveryLifecycleResponse{}, core.Integrity
	}
	if err = tx.Commit(ctx); err != nil {
		return DurableDeliveryLifecycleResponse{}, err
	}
	return DurableDeliveryLifecycleResponse{Manifest: manifest, ManifestSHA256: manifestSHA256, UserDisposition: disposition}, nil
}

func canonicalDurableDeliveryManifest(manifest DurableDeliveryManifestView) ([]byte, string, error) {
	manifestRevision, revisionErr := strconv.ParseInt(manifest.Revision, 10, 64)
	byteSize, byteSizeErr := strconv.ParseInt(manifest.Artifact.ByteSize, 10, 64)
	_, timeErr := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
	if manifest.SchemaVersion != DurableDeliveryManifestSchema || !core.ValidID(manifest.DeliveryID) || manifestRevision <= 0 || revisionErr != nil || !core.ValidID(manifest.CompanyID) || !core.ValidID(manifest.MissionID) || !core.ValidID(manifest.TaskID) || !core.ValidID(manifest.ArtifactID) || !validDurableDeliveryState(manifest.State) || manifest.Artifact.FileName != "artifact.bin" || byteSize <= 0 || byteSize > 64*1024*1024 || byteSizeErr != nil || !validDeliverySHA256(manifest.Artifact.SHA256) || timeErr != nil || !validDurableDeliverySections(manifest.Sections, manifest.State) {
		return nil, "", errors.New("durable delivery manifest is incomplete or invalid")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(hash[:]), nil
}

func validDurableDeliveryState(state string) bool {
	switch state {
	case "assembling", "ready", "invalidated", "withdrawn":
		return true
	default:
		return false
	}
}

func validDurableDeliverySections(sections []DurableDeliveryManifestSection, manifestState string) bool {
	const required = 8
	if len(sections) != required {
		return false
	}
	expected := []string{"source_inputs", "environment_build", "file_inventory", "run_instructions", "verification", "limitations", "license_source", "feedback"}
	for index, section := range sections {
		if section.Key != expected[index] || section.Detail == "" || len(section.Detail) > 512 {
			return false
		}
		if section.State != "available" && section.State != "unavailable" && section.State != "missing" && section.State != "not_requested" {
			return false
		}
	}
	if sections[2].State != "available" || sections[4].State != "available" {
		return false
	}
	if manifestState == "ready" {
		for index, section := range sections {
			if index != 7 && section.State != "available" {
				return false
			}
		}
	}
	return true
}

func validDurableDeliveryDisposition(disposition DurableDeliveryUserDispositionView) bool {
	revision, revisionErr := strconv.ParseInt(disposition.Revision, 10, 64)
	manifestRevision, manifestRevisionErr := strconv.ParseInt(disposition.ManifestRevision, 10, 64)
	_, createdAtErr := time.Parse(time.RFC3339Nano, disposition.CreatedAt)
	if revisionErr != nil || manifestRevisionErr != nil || revision <= 0 || manifestRevision <= 0 || disposition.Actor == "" || len(disposition.Actor) > 128 || len(disposition.Reason) > 4096 || len(disposition.RequestID) > 80 || len(disposition.FeedbackDeadline) > 64 || createdAtErr != nil {
		return false
	}
	switch disposition.State {
	case "not_requested", "awaiting_feedback", "accepted", "changes_requested":
		if disposition.State == "awaiting_feedback" {
			if disposition.FeedbackDeadline == "" {
				return false
			}
			if _, err := time.Parse(time.RFC3339Nano, disposition.FeedbackDeadline); err != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}
