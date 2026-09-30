// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
)

type TaskInputManifestRecord struct {
	CompanyID string
	MissionID string
	TaskID    string
	Digest    string
	Manifest  intake.ModelInputManifest
	CreatedAt time.Time
}

func bindMissionInputManifest(ctx context.Context, tx pgx.Tx, scope Scope, missionID, taskID string) error {
	rows, err := tx.Query(ctx, `SELECT input_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state
FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 ORDER BY input_id,revision`, scope.company, missionID)
	if err != nil {
		return err
	}
	references := make([]intake.MissionInputReference, 0)
	for rows.Next() {
		var reference intake.MissionInputReference
		if err = rows.Scan(&reference.InputID, &reference.Revision, &reference.RequestID, &reference.SourceKind, &reference.DisplayName, &reference.MediaType, &reference.ByteSize, &reference.ContentDigest, &reference.State); err != nil {
			rows.Close()
			return err
		}
		references = append(references, reference)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	manifest, digest, err := intake.PrepareModelInputManifest(scope.company, missionID, taskID, references)
	if err != nil {
		return core.Integrity
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO task_input_manifests(company_id,task_id,mission_id,schema_version,manifest_digest,delivery_status,manifest)
VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.company, taskID, missionID, manifest.SchemaVersion, digest, manifest.DeliveryStatus, encoded)
	return err
}

func (k *Kernel) TaskInputManifest(ctx context.Context, scope Scope, taskID string) (TaskInputManifestRecord, error) {
	if !core.ValidID(taskID) {
		return TaskInputManifestRecord{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return TaskInputManifestRecord{}, err
	}
	defer tx.Rollback(ctx)
	var record TaskInputManifestRecord
	var encoded []byte
	err = tx.QueryRow(ctx, `SELECT company_id,mission_id,task_id,manifest_digest,manifest,created_at
FROM task_input_manifests WHERE company_id=$1 AND task_id=$2`, scope.company, taskID).Scan(&record.CompanyID, &record.MissionID, &record.TaskID, &record.Digest, &encoded, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskInputManifestRecord{}, core.OutOfScope
	}
	if err != nil {
		return TaskInputManifestRecord{}, err
	}
	if err = json.Unmarshal(encoded, &record.Manifest); err != nil {
		return TaskInputManifestRecord{}, core.Integrity
	}
	if record.CompanyID != scope.company || record.Manifest.CompanyID != record.CompanyID || record.Manifest.MissionID != record.MissionID || record.Manifest.TaskID != record.TaskID || intake.VerifyModelInputManifest(record.Manifest, record.Digest) != nil {
		return TaskInputManifestRecord{}, core.Integrity
	}
	if err = tx.Commit(ctx); err != nil {
		return TaskInputManifestRecord{}, err
	}
	return record, nil
}
