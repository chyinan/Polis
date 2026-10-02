// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
)

type MissionInputRevision struct {
	CompanyID     string
	InputID       string
	MissionID     string
	Revision      int64
	RequestID     string
	SourceKind    string
	DisplayName   string
	MediaType     string
	ByteSize      int64
	ContentDigest string
	State         string
	ImageWidth    int
	ImageHeight   int
}

func (k *Kernel) TXAddMissionInput(ctx context.Context, scope Scope, missionID, inputID, requestID string, prepared intake.PreparedUpload, content []byte) (MissionInputRevision, error) {
	if !core.ValidID(missionID) || !core.ValidID(requestID) || (inputID != "" && !core.ValidID(inputID)) {
		return MissionInputRevision{}, core.Malformed
	}
	if err := intake.VerifyPreparedUpload(prepared, content); err != nil {
		return MissionInputRevision{}, core.Integrity
	}
	requestFingerprint := fingerprint(struct {
		MissionID string
		InputID   string
		RequestID string
		Prepared  intake.PreparedUpload
	}{missionID, inputID, requestID, prepared})

	reservation, err := k.TXReserveMissionInput(ctx, scope, missionID, inputID, requestID, requestFingerprint, prepared)
	if err != nil {
		return MissionInputRevision{}, err
	}
	if reservation.State == string(intake.StateUsable) || reservation.State == string(intake.StatePartial) || reservation.State == string(intake.StateUnsupported) {
		return reservation, nil
	}
	if reservation.State != "uploading" && reservation.State != "stored" {
		return MissionInputRevision{}, core.Denied
	}

	digest, err := k.putBlobWithClaim(ctx, scope.company, content)
	if err != nil {
		return MissionInputRevision{}, err
	}
	if digest != prepared.ContentDigest {
		return MissionInputRevision{}, core.Integrity
	}
	if reservation.State == "uploading" {
		if err = k.TXMarkMissionInputStored(ctx, scope, reservation, requestFingerprint); err != nil {
			return MissionInputRevision{}, err
		}
	}

	receipt, err := k.TXWrite(ctx, scope, nil, requestID, "mission.input.add", struct {
		InputID            string
		MissionID          string
		Revision           int64
		RequestFingerprint string
		State              string
	}{reservation.InputID, missionID, reservation.Revision, requestFingerprint, string(prepared.State)}, func(tx pgx.Tx) (Receipt, error) {
		var currentState, storedFingerprint string
		err := tx.QueryRow(ctx, "SELECT state,COALESCE(request_fingerprint,'') FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3 AND request_id=$4 FOR UPDATE", scope.company, reservation.InputID, reservation.Revision, requestID).Scan(&currentState, &storedFingerprint)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if storedFingerprint != requestFingerprint || currentState != "stored" {
			return Receipt{}, core.Conflict
		}
		_, err = tx.Exec(ctx, "UPDATE mission_inputs SET state=$4,updated_at=clock_timestamp() WHERE company_id=$1 AND input_id=$2 AND revision=$3", scope.company, reservation.InputID, reservation.Revision, string(prepared.State))
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: reservation.InputID, Status: string(prepared.State)}, nil
	})
	if err != nil {
		return MissionInputRevision{}, err
	}
	return k.missionInputByRequest(ctx, scope.company, requestID, receipt.ID)
}

func (k *Kernel) TXReserveMissionInput(ctx context.Context, scope Scope, missionID, inputID, requestID, requestFingerprint string, prepared intake.PreparedUpload) (MissionInputRevision, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return MissionInputRevision{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, scope, nil); err != nil {
		return MissionInputRevision{}, err
	}

	var existing MissionInputRevision
	var existingFingerprint string
	err = tx.QueryRow(ctx, "SELECT company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height,COALESCE(request_fingerprint,'') FROM mission_inputs WHERE company_id=$1 AND request_id=$2 FOR UPDATE", scope.company, requestID).Scan(
		&existing.CompanyID, &existing.InputID, &existing.MissionID, &existing.Revision, &existing.RequestID,
		&existing.SourceKind, &existing.DisplayName, &existing.MediaType, &existing.ByteSize, &existing.ContentDigest,
		&existing.State, &existing.ImageWidth, &existing.ImageHeight, &existingFingerprint)
	if err == nil {
		if existingFingerprint != requestFingerprint || existing.MissionID != missionID {
			return MissionInputRevision{}, core.Conflict
		}
		if err = tx.Commit(ctx); err != nil {
			return MissionInputRevision{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MissionInputRevision{}, err
	}

	var missionState string
	err = tx.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2 FOR UPDATE", scope.company, missionID).Scan(&missionState)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionInputRevision{}, core.OutOfScope
	}
	if err != nil {
		return MissionInputRevision{}, err
	}
	if !intake.MissionAllowsInput(missionState) {
		return MissionInputRevision{}, core.Denied
	}

	currentInputID := inputID
	revision := int64(1)
	if currentInputID == "" {
		currentInputID = newID()
	} else {
		var existingMissionID, existingState string
		var latestRevision int64
		err = tx.QueryRow(ctx, "SELECT mission_id,revision,state FROM mission_inputs WHERE company_id=$1 AND input_id=$2 ORDER BY revision DESC LIMIT 1 FOR UPDATE", scope.company, currentInputID).Scan(&existingMissionID, &latestRevision, &existingState)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && existingMissionID != missionID) {
			return MissionInputRevision{}, core.OutOfScope
		}
		if err != nil {
			return MissionInputRevision{}, err
		}
		if existingState == "uploading" || existingState == "stored" {
			return MissionInputRevision{}, core.Conflict
		}
		revision = latestRevision + 1
	}

	_, err = tx.Exec(ctx, "INSERT INTO mission_inputs (company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height,request_fingerprint) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'uploading',$11,$12,$13)",
		scope.company, currentInputID, missionID, revision, requestID, prepared.SourceKind, prepared.DisplayName,
		prepared.MediaType, prepared.ByteSize, prepared.ContentDigest, prepared.ImageWidth, prepared.ImageHeight, requestFingerprint)
	if err != nil {
		return MissionInputRevision{}, err
	}
	if err = appendEvent(ctx, tx, scope, "mission.input.uploading", map[string]any{"id": currentInputID, "mission_id": missionID, "revision": revision, "state": "uploading"}); err != nil {
		return MissionInputRevision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MissionInputRevision{}, err
	}
	return MissionInputRevision{
		CompanyID: scope.company, InputID: currentInputID, MissionID: missionID, Revision: revision,
		RequestID: requestID, SourceKind: prepared.SourceKind, DisplayName: prepared.DisplayName, MediaType: prepared.MediaType,
		ByteSize: prepared.ByteSize, ContentDigest: prepared.ContentDigest, State: "uploading",
		ImageWidth: prepared.ImageWidth, ImageHeight: prepared.ImageHeight,
	}, nil
}

func (k *Kernel) TXMarkMissionInputStored(ctx context.Context, scope Scope, reservation MissionInputRevision, requestFingerprint string) error {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, scope, nil); err != nil {
		return err
	}
	var state, storedFingerprint string
	err = tx.QueryRow(ctx, "SELECT state,COALESCE(request_fingerprint,'') FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3 AND request_id=$4 FOR UPDATE", scope.company, reservation.InputID, reservation.Revision, reservation.RequestID).Scan(&state, &storedFingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if storedFingerprint != requestFingerprint {
		return core.Conflict
	}
	if state == "stored" {
		return tx.Commit(ctx)
	}
	if state != "uploading" {
		return core.Conflict
	}
	if _, err = tx.Exec(ctx, "UPDATE mission_inputs SET state='stored',updated_at=clock_timestamp() WHERE company_id=$1 AND input_id=$2 AND revision=$3", scope.company, reservation.InputID, reservation.Revision); err != nil {
		return err
	}
	if err = appendEvent(ctx, tx, scope, "mission.input.stored", map[string]any{"id": reservation.InputID, "mission_id": reservation.MissionID, "revision": reservation.Revision, "state": "stored"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (k *Kernel) missionInputByRequest(ctx context.Context, companyID, requestID, expectedInputID string) (MissionInputRevision, error) {
	var result MissionInputRevision
	err := k.pool.QueryRow(ctx, "SELECT company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height FROM mission_inputs WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(
		&result.CompanyID, &result.InputID, &result.MissionID, &result.Revision, &result.RequestID, &result.SourceKind,
		&result.DisplayName, &result.MediaType, &result.ByteSize, &result.ContentDigest, &result.State, &result.ImageWidth, &result.ImageHeight)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionInputRevision{}, core.Integrity
	}
	if err != nil {
		return MissionInputRevision{}, err
	}
	if result.InputID != expectedInputID {
		return MissionInputRevision{}, core.Integrity
	}
	return result, nil
}
