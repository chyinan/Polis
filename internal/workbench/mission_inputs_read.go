// pattern: Imperative Shell
package workbench

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresReadStore) ListMissionInputs(ctx context.Context, companyID, missionID string) ([]MissionInputView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return nil, err
	}
	if !companyIDPattern.MatchString(missionID) {
		return nil, fmt.Errorf("invalid mission scope")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND id=$2)", companyID, missionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errCompanyNotFound
	}
	rows, err := tx.Query(ctx, "SELECT input_id,mission_id,request_id,revision,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height,created_at FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 ORDER BY created_at DESC,input_id,revision DESC", companyID, missionID)
	if err != nil {
		return nil, err
	}
	items := make([]MissionInputView, 0)
	for rows.Next() {
		var item MissionInputView
		var revision, byteSize int64
		var width, height int32
		var createdAt time.Time
		if err = rows.Scan(&item.InputID, &item.MissionID, &item.RequestID, &revision, &item.SourceKind, &item.DisplayName, &item.MediaType, &byteSize, &item.ContentDigest, &item.State, &width, &height, &createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		item.CompanyID = companyID
		item.Revision = strconv.FormatInt(revision, 10)
		item.ByteSize = strconv.FormatInt(byteSize, 10)
		item.ImageWidth = strconv.FormatInt(int64(width), 10)
		item.ImageHeight = strconv.FormatInt(int64(height), 10)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

var _ MissionInputReader = (*PostgresReadStore)(nil)
