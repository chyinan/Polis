// pattern: Imperative Shell
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"crypto/sha256"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/taskvalidation"
)

type PostgresReadStore struct {
	pool                 *pgxpool.Pool
	blobRoot             string
	executorFingerprints map[string]environment.EnvironmentExecutorFingerprint
}

func NewPostgresReadStore(ctx context.Context, dsn string) (*PostgresReadStore, error) {
	return newPostgresReadStore(ctx, dsn, "")
}

func NewPostgresReadStoreWithBlobRoot(ctx context.Context, dsn, blobRoot string) (*PostgresReadStore, error) {
	return newPostgresReadStore(ctx, dsn, blobRoot)
}

func newPostgresReadStore(ctx context.Context, dsn, blobRoot string) (*PostgresReadStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "polis_r0_") {
		return nil, errors.New("workbench read store requires a dedicated polis_r0 database")
	}
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	executorFingerprints := make(map[string]environment.EnvironmentExecutorFingerprint)
	for _, profileID := range []string{environment.WindowsNodeNPMProfile, environment.LinuxNodeNPMProfile} {
		if fingerprint, fingerprintErr := environment.CurrentEnvironmentExecutorFingerprint(profileID); fingerprintErr == nil {
			executorFingerprints[profileID] = fingerprint
		}
	}
	store := &PostgresReadStore{pool: pool, blobRoot: blobRoot, executorFingerprints: executorFingerprints}
	if err := store.ensureSchema(ctx); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresReadStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresReadStore) ensureSchema(ctx context.Context) error {
	var version int64
	if err := s.pool.QueryRow(ctx, "SELECT COALESCE(max(version_id),0) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil {
		return err
	}
	if version < 77 {
		return fmt.Errorf("workbench read store requires schema 77 or newer, got %d", version)
	}
	return nil
}

func (s *PostgresReadStore) GetCompanyOverview(ctx context.Context, companyID string) (CompanyOverviewView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return CompanyOverviewView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CompanyOverviewView{}, err
	}
	defer tx.Rollback(ctx)
	view, err := readOverview(ctx, tx, companyID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CompanyOverviewView{}, err
	}
	return view, nil
}

func (s *PostgresReadStore) ListActivity(ctx context.Context, query ActivityQuery) (ActivityView, error) {
	if err := validateCompanyID(query.CompanyID); err != nil {
		return ActivityView{}, err
	}
	if err := validateActivityLimit(query.Limit); err != nil {
		return ActivityView{}, err
	}
	snapshot, err := parseSnapshotCursor(query.SnapshotCursor)
	if err != nil {
		return ActivityView{}, err
	}
	cursor, err := parseActivityCursor(query.Cursor)
	if err != nil {
		return ActivityView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ActivityView{}, err
	}
	defer tx.Rollback(ctx)
	var currentSequence int64
	if err := tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", query.CompanyID).Scan(&currentSequence); errors.Is(err, pgx.ErrNoRows) {
		return ActivityView{}, errCompanyNotFound
	} else if err != nil {
		return ActivityView{}, err
	}
	if snapshot > currentSequence {
		return ActivityView{}, fmt.Errorf("snapshot cursor is ahead of company state")
	}
	if cursor != nil {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM events WHERE company_id=$1 AND company_seq=$2 AND company_seq<=$3)", query.CompanyID, *cursor, snapshot).Scan(&exists); err != nil {
			return ActivityView{}, err
		}
		if !exists {
			return ActivityView{}, errInvalidActivityCursor
		}
	}
	rows, err := tx.Query(ctx, `SELECT company_seq,kind,payload::text,observed
FROM events
WHERE company_id=$1 AND company_seq<=$2 AND ($3::bigint IS NULL OR company_seq<=$3)
ORDER BY company_seq DESC LIMIT $4`, query.CompanyID, snapshot, cursor, query.Limit)
	if err != nil {
		return ActivityView{}, err
	}
	events, err := scanEvents(rows)
	if err != nil {
		return ActivityView{}, err
	}
	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	items := make([]ActivityEvent, 0, len(events))
	for _, event := range events {
		items = append(items, eventView(query.CompanyID, event, observedAt))
	}
	var nextCursor *string
	if len(events) == query.Limit {
		var next int64
		err = tx.QueryRow(ctx, `SELECT company_seq FROM events
WHERE company_id=$1 AND company_seq<$2 AND company_seq<=$3
ORDER BY company_seq DESC LIMIT 1`, query.CompanyID, events[len(events)-1].Sequence, snapshot).Scan(&next)
		if err == nil {
			value := cursorForSequence(next)
			nextCursor = &value
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ActivityView{}, err
		}
	}
	meta := viewMeta(query.CompanyID, snapshot, observedAt)
	if err := tx.Commit(ctx); err != nil {
		return ActivityView{}, err
	}
	return ActivityView{Meta: meta, Items: items, NextCursor: nextCursor}, nil
}

const streamCatchupBatchSize = 256

func (s *PostgresReadStore) StreamActivity(ctx context.Context, companyID string, cursor int64, emit func(ActivityEvent) error) error {
	if err := validateCompanyID(companyID); err != nil {
		return err
	}
	if cursor < 0 {
		return fmt.Errorf("invalid stream cursor")
	}
	connection, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	if _, err = connection.Exec(ctx, "LISTEN polis_company_events"); err != nil {
		return err
	}
	var currentSequence int64
	if err = connection.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", companyID).Scan(&currentSequence); errors.Is(err, pgx.ErrNoRows) {
		return errCompanyNotFound
	} else if err != nil {
		return err
	}
	if cursor > currentSequence {
		return fmt.Errorf("stream cursor is ahead of company state")
	}
	for {
		events, err := readActivityEventsAfter(ctx, connection, companyID, cursor)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err = emit(eventView(companyID, event, time.Now().UTC().Format(time.RFC3339Nano))); err != nil {
				return err
			}
			cursor = event.Sequence
		}
		if len(events) == streamCatchupBatchSize {
			continue
		}
		notification, err := connection.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if notification.Channel != "polis_company_events" || notification.Payload != companyID {
			continue
		}
	}
}

func (s *PostgresReadStore) ListOperatorInstructions(ctx context.Context, companyID string) ([]OperatorInstructionView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT i.id,i.mission_id,i.task_id,i.employee_id,i.content,i.state,i.created_at,
(SELECT COALESCE(jsonb_agg(jsonb_build_object('employeeId',r.employee_id,'outcome',r.outcome,'summary',r.summary,'respondedAt',r.responded_at) ORDER BY r.responded_at,r.employee_id),'[]'::jsonb)
 FROM operator_instruction_responses r WHERE r.company_id=i.company_id AND r.instruction_id=i.id)
FROM operator_instructions i
WHERE i.company_id=$1 ORDER BY i.created_at DESC,i.id DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]OperatorInstructionView, 0)
	for rows.Next() {
		var item OperatorInstructionView
		var taskID, employeeID *string
		var createdAt time.Time
		var responseJSON []byte
		if err = rows.Scan(&item.InstructionID, &item.MissionID, &taskID, &employeeID, &item.Content, &item.State, &createdAt, &responseJSON); err != nil {
			return nil, err
		}
		item.TaskID = taskID
		item.EmployeeID = employeeID
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		item.Responses = make([]OperatorInstructionResponseView, 0)
		if len(responseJSON) > 0 {
			if err = json.Unmarshal(responseJSON, &item.Responses); err != nil {
				return nil, err
			}
		}
		if len(item.Responses) > 0 {
			latest := item.Responses[len(item.Responses)-1]
			item.ResponseOutcome = &latest.Outcome
			item.ResponseSummary = &latest.Summary
			item.RespondedByEmployeeID = &latest.EmployeeID
			item.RespondedAt = &latest.RespondedAt
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresReadStore) ListCollaboration(ctx context.Context, companyID string) ([]CollaborationItem, error) {
	if err := validateCompanyID(companyID); err != nil {
		return nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT m.id,m.mission_id,m.task_id,m.sender,m.recipient,m.body,m.kind,m.delivery_state,
m.contract_revision_id,m.task_revision,o.id,o.state,COALESCE(o.evidence_ref,o.evidence_id,'')
FROM messages m
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
WHERE m.company_id=$1 ORDER BY m.id DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CollaborationItem, 0)
	for rows.Next() {
		var item CollaborationItem
		var contractID, obligationID, obligationState, evidence *string
		if err = rows.Scan(&item.MessageID, &item.MissionID, &item.TaskID, &item.SenderEmployeeID, &item.RecipientEmployeeID, &item.Content, &item.Kind, &item.DeliveryState, &contractID, &item.TaskRevision, &obligationID, &obligationState, &evidence); err != nil {
			return nil, err
		}
		item.ContractRevisionID = contractID
		item.ObligationID = obligationID
		item.ObligationState = obligationState
		if evidence != nil && *evidence != "" {
			item.EvidenceRef = evidence
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresReadStore) GetWorkspace(ctx context.Context, companyID, taskID string) (WorkspaceView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return WorkspaceView{}, err
	}
	if !companyIDPattern.MatchString(taskID) {
		return WorkspaceView{}, fmt.Errorf("invalid task scope")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return WorkspaceView{}, err
	}
	defer tx.Rollback(ctx)
	var missionID, digest string
	var revision int64
	if err = tx.QueryRow(ctx, `SELECT w.revision,w.digest,t.mission_id
FROM worker_workspaces w JOIN tasks t ON t.company_id=w.company_id AND t.id=w.task_id
WHERE w.company_id=$1 AND w.task_id=$2`, companyID, taskID).Scan(&revision, &digest, &missionID); errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceView{}, errCompanyNotFound
	} else if err != nil {
		return WorkspaceView{}, err
	}
	checkpointRows, err := readTaskCheckpoints(ctx, tx, companyID, taskID)
	if err != nil {
		return WorkspaceView{}, err
	}
	checkpoints := make([]CheckpointSummary, 0, len(checkpointRows))
	for _, row := range checkpointRows {
		checkpoint, viewErr := checkpointView(row)
		if viewErr != nil {
			return WorkspaceView{}, viewErr
		}
		checkpoints = append(checkpoints, checkpoint)
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkspaceView{}, err
	}
	content, available, err := s.readAuthorizedBlob(companyID, digest)
	if err != nil {
		return WorkspaceView{}, err
	}
	file := WorkspaceFile{Path: "workspace.txt", Bytes: stringValue(int64(len(content))), Content: content}
	if !available {
		file.Bytes = "不可得"
	}
	_ = missionID
	return WorkspaceView{TaskID: taskID, Revision: stringValue(revision), Digest: digest, Files: []WorkspaceFile{file}, ChangedFiles: []string{"workspace.txt"}, Checkpoints: checkpoints}, nil
}

func (s *PostgresReadStore) GetArtifact(ctx context.Context, companyID, artifactID string) (ArtifactDetailView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return ArtifactDetailView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ArtifactDetailView{}, err
	}
	defer tx.Rollback(ctx)
	var view ArtifactDetailView
	var bytes int64
	if err = tx.QueryRow(ctx, `SELECT id,task_id,digest,bytes,state,verdict FROM artifacts WHERE company_id=$1 AND id=$2 AND artifact_kind='deliverable'`, companyID, artifactID).Scan(&view.ArtifactID, &view.TaskID, &view.Digest, &bytes, &view.State, &view.Verdict); errors.Is(err, pgx.ErrNoRows) {
		return ArtifactDetailView{}, errCompanyNotFound
	} else if err != nil {
		return ArtifactDetailView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ArtifactDetailView{}, err
	}
	view.Bytes = stringValue(bytes)
	content, available, err := s.readAuthorizedBlob(companyID, view.Digest)
	if err != nil {
		return ArtifactDetailView{}, err
	}
	view.Content = content
	view.ContentAvailable = available
	return view, nil
}

func (s *PostgresReadStore) GetOperations(ctx context.Context, companyID string) (OperationsView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return OperationsView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return OperationsView{}, err
	}
	defer tx.Rollback(ctx)
	var used, limit, workers int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(tool_calls_used),0),COALESCE(sum(NULLIF(tool_call_limit,0)),0),count(*) FILTER (WHERE state!='stopped') FROM worker_sessions WHERE company_id=$1`, companyID).Scan(&used, &limit, &workers); err != nil {
		return OperationsView{}, err
	}
	var lastError *string
	var errorKind string
	if err = tx.QueryRow(ctx, `SELECT kind FROM events WHERE company_id=$1 AND (kind ILIKE '%failed%' OR kind ILIKE '%inconclusive%') ORDER BY company_seq DESC LIMIT 1`, companyID).Scan(&errorKind); err == nil {
		lastError = pointer(errorKind)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return OperationsView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return OperationsView{}, err
	}
	quality := "unavailable"
	limitText := "不可得"
	if limit > 0 {
		quality = "reported"
		limitText = stringValue(limit)
	}
	casStatus := "unavailable"
	if s.blobRoot != "" {
		if info, statErr := os.Stat(filepath.Clean(s.blobRoot)); statErr == nil && info.IsDir() {
			casStatus = "ready"
		}
	}
	return OperationsView{CompanyID: companyID, ToolCallsUsed: stringValue(used), ToolCallsLimit: limitText, ToolBudgetQuality: quality, InputTokens: nil, OutputTokens: nil, ElapsedRuntime: nil, WorkerCount: stringValue(workers), PostgreSQLStatus: "ready", CASStatus: casStatus, EventStreamStatus: "ready", LastRuntimeError: lastError}, nil
}

// HasActiveWork is used only for the desktop graceful-quit guard. It reads
// authoritative Mission/WorkerSession state in one snapshot and never mutates
// product state.
func (s *PostgresReadStore) HasActiveWork(ctx context.Context) (bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM missions WHERE state IN ('active','paused','closing')
		UNION ALL
		SELECT 1 FROM worker_sessions WHERE state <> 'stopped'
	)`).Scan(&active); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return active, nil
}

func (s *PostgresReadStore) ListNotifications(ctx context.Context, companyID string) (NotificationsView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return NotificationsView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return NotificationsView{}, err
	}
	defer tx.Rollback(ctx)
	view := NotificationsView{Routes: []NotificationRouteView{}, Deliveries: []NotificationDeliveryView{}}
	routes, err := tx.Query(ctx, `SELECT id,adapter,enabled,destination,status,safety_alias,credential_ref,route_revision,qualification_status,qualified_until FROM notification_routes WHERE company_id=$1 ORDER BY id`, companyID)
	if err != nil {
		return NotificationsView{}, err
	}
	for routes.Next() {
		var route NotificationRouteView
		var revision int64
		var qualifiedUntil pgtype.Timestamptz
		if err = routes.Scan(&route.RouteID, &route.Adapter, &route.Enabled, &route.Destination, &route.Status, &route.SafetyAlias, &route.CredentialRef, &revision, &route.QualificationStatus, &qualifiedUntil); err != nil {
			routes.Close()
			return NotificationsView{}, err
		}
		route.Destination = notificationDestinationForView(route.Adapter, route.Destination)
		route.RouteRevision = strconv.FormatInt(revision, 10)
		if qualifiedUntil.Valid {
			route.QualifiedUntil = qualifiedUntil.Time.UTC().Format(time.RFC3339Nano)
		}
		view.Routes = append(view.Routes, route)
	}
	routes.Close()
	if err = routes.Err(); err != nil {
		return NotificationsView{}, err
	}
	deliveries, err := tx.Query(ctx, `SELECT id,intent_id,adapter,state,error_code,created_at FROM notification_deliveries WHERE company_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, companyID)
	if err != nil {
		return NotificationsView{}, err
	}
	for deliveries.Next() {
		var delivery NotificationDeliveryView
		var createdAt time.Time
		if err = deliveries.Scan(&delivery.DeliveryID, &delivery.IntentID, &delivery.Adapter, &delivery.State, &delivery.ErrorCode, &createdAt); err != nil {
			deliveries.Close()
			return NotificationsView{}, err
		}
		delivery.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		view.Deliveries = append(view.Deliveries, delivery)
	}
	deliveries.Close()
	if err = deliveries.Err(); err != nil {
		return NotificationsView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return NotificationsView{}, err
	}
	return view, nil
}

func (s *PostgresReadStore) readAuthorizedBlob(companyID, digest string) (string, bool, error) {
	if s.blobRoot == "" {
		return "", false, nil
	}
	if len(digest) != 64 {
		return "", false, fmt.Errorf("invalid content digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", false, fmt.Errorf("invalid content digest")
	}
	root := filepath.Clean(s.blobRoot)
	path := filepath.Join(root, companyID, digest)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("content path escaped CAS root")
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if len(content) > 128*1024 {
		return string(content[:128*1024]), true, nil
	}
	hash := sha256.Sum256(content)
	if hex.EncodeToString(hash[:]) != digest {
		return "", false, fmt.Errorf("content digest does not match artifact")
	}
	return string(content), true, nil
}

func readTaskCheckpoints(ctx context.Context, tx pgx.Tx, companyID, taskID string) ([]checkpointRow, error) {
	rows, err := tx.Query(ctx, `SELECT c.id,c.session_id,s.employee_id,s.state,s.epoch,c.digest,c.data
FROM worker_checkpoints c JOIN worker_sessions s ON s.company_id=c.company_id AND s.id=c.session_id
WHERE c.company_id=$1 AND s.task_id=$2 ORDER BY c.id DESC`, companyID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]checkpointRow, 0)
	for rows.Next() {
		var row checkpointRow
		if err = rows.Scan(&row.ID, &row.SessionID, &row.EmployeeID, &row.SessionState, &row.SessionEpoch, &row.Digest, &row.Data); err != nil {
			return nil, err
		}
		row.TaskID = taskID
		items = append(items, row)
	}
	return items, rows.Err()
}

func readActivityEventsAfter(ctx context.Context, connection *pgxpool.Conn, companyID string, cursor int64) ([]eventRow, error) {
	rows, err := connection.Query(ctx, `SELECT company_seq,kind,payload::text,observed
FROM events WHERE company_id=$1 AND company_seq>$2 ORDER BY company_seq ASC LIMIT $3`, companyID, cursor, streamCatchupBatchSize)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

type eventRow struct {
	Sequence int64
	Kind     string
	Payload  string
	Observed bool
}

func scanEvents(rows pgx.Rows) ([]eventRow, error) {
	defer rows.Close()
	items := make([]eventRow, 0)
	for rows.Next() {
		var item eventRow
		if err := rows.Scan(&item.Sequence, &item.Kind, &item.Payload, &item.Observed); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type missionRow struct {
	ID                 string
	Title              string
	Goal               string
	State              string
	Contract           string
	AcceptanceContract *taskvalidation.AcceptanceContract
}

type taskRow struct {
	ID         string
	MissionID  string
	Owner      string
	Kind       string
	State      string
	Generation int64
}

type employeeRow struct {
	ID                     string
	Epoch                  int64
	SessionID              string
	SessionTask            string
	SessionState           string
	Profile                string
	SessionEpoch           int64
	ToolLimit              int64
	ToolUsed               int64
	ScheduleState          pgtype.Text
	ScheduleWorkGeneration pgtype.Int8
	ScheduleCheckedGen     pgtype.Int8
	ScheduleNextDueAt      pgtype.Timestamptz
	SchedulePauseReason    string
}

type revisionRow struct {
	ID       string
	Revision int64
	Endpoint string
	State    string
	Digest   string
	Proposer string
	Accepter string
}

type obligationRow struct {
	ID          string
	TaskID      string
	Owner       string
	State       string
	EvidenceRef string
	EvidenceID  string
	MessageBody string
}

type artifactRow struct {
	ID              string
	TaskID          string
	Author          string
	Digest          string
	Bytes           int64
	State           string
	Verdict         string
	ContractID      string
	QualificationID string
	CheckpointID    string
}

type checkpointRow struct {
	ID                string
	TaskID            string
	EmployeeID        string
	SessionID         string
	SessionState      string
	SessionEpoch      int64
	Digest            string
	Data              []byte
	ArtifactID        string
	Kind              string
	WorkspaceRevision int64
}

type checkpointPayload struct {
	Kind              string `json:"kind"`
	ValidationStatus  string `json:"validation_status"`
	FinalizationState string `json:"finalization_state"`
	WorkspaceDigest   string `json:"workspace_digest"`
	WorkspaceRevision int64  `json:"workspace_revision"`
}

type taskRevisionRow struct {
	TaskID     string
	ContractID string
	Revision   int64
}

type workspaceRow struct {
	TaskID   string
	Revision int64
}

type reviewRow struct {
	TaskID  string
	Verdict string
}

func readOverview(ctx context.Context, tx pgx.Tx, companyID string) (CompanyOverviewView, error) {
	var companySequence int64
	var companyName string
	var workspaceRoot string
	if err := tx.QueryRow(ctx, "SELECT company_seq,name,workspace_root FROM companies WHERE id=$1", companyID).Scan(&companySequence, &companyName, &workspaceRoot); errors.Is(err, pgx.ErrNoRows) {
		return CompanyOverviewView{}, errCompanyNotFound
	} else if err != nil {
		return CompanyOverviewView{}, err
	}
	var mission missionRow
	var acceptanceJSON []byte
	missionErr := tx.QueryRow(ctx, `SELECT id,title,goal,state,contract,acceptance_contract FROM missions
WHERE company_id=$1
ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'paused' THEN 1 WHEN 'closing' THEN 2 ELSE 3 END,id
LIMIT 1`, companyID).Scan(&mission.ID, &mission.Title, &mission.Goal, &mission.State, &mission.Contract, &acceptanceJSON)
	if errors.Is(missionErr, pgx.ErrNoRows) {
		mission = missionRow{ID: "unavailable", State: "draft", Contract: "unavailable"}
	} else if missionErr != nil {
		return CompanyOverviewView{}, missionErr
	}
	if len(acceptanceJSON) > 0 {
		var acceptance taskvalidation.AcceptanceContract
		if json.Unmarshal(acceptanceJSON, &acceptance) != nil || taskvalidation.ValidateContract(&acceptance) != nil {
			return CompanyOverviewView{}, fmt.Errorf("mission acceptance contract is invalid")
		}
		mission.AcceptanceContract = &acceptance
	}
	closeout, err := readMissionCloseout(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	tasks, err := readTasks(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	employees, err := readEmployees(ctx, tx, companyID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	revisions, err := readRevisions(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	taskRevisions, err := readTaskRevisions(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	workspaces, err := readWorkspaces(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	artifacts, err := readArtifacts(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	checkpoints, err := readCheckpoints(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	obligations, err := readObligations(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	interventions, err := readHumanInterventions(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	reviews, err := readReviews(ctx, tx, companyID, mission.ID)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	activityRows, err := readRecentEvents(ctx, tx, companyID, companySequence)
	if err != nil {
		return CompanyOverviewView{}, err
	}
	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	meta := viewMeta(companyID, companySequence, observedAt)
	currentContract := currentRevision(revisions)
	taskViews := make([]TaskSummary, 0, len(tasks))
	for _, task := range tasks {
		taskViews = append(taskViews, taskView(task, taskRevisions, workspaces, artifacts, reviews))
	}
	obligationViews := make([]ObligationSummary, 0, len(obligations))
	for _, obligation := range obligations {
		evidence := firstNonEmpty(obligation.EvidenceRef, obligation.EvidenceID)
		var evidenceRef *string
		if evidence != "" {
			evidenceRef = pointer(evidence)
		}
		note := strings.TrimSpace(obligation.MessageBody)
		if note == "" {
			note = "message body unavailable"
		}
		obligationViews = append(obligationViews, ObligationSummary{ObligationID: obligation.ID, MessageID: obligation.ID, OwnerEmployeeID: obligation.Owner, State: mapObligationState(obligation.State), EvidenceRef: evidenceRef, Note: note})
	}
	artifactViews := make([]ArtifactSummary, 0, len(artifacts))
	for _, artifact := range artifacts {
		var contractID *string
		if artifact.ContractID != "" {
			contractID = pointer(artifact.ContractID)
		}
		var qualificationID *string
		if artifact.QualificationID != "" {
			qualificationID = pointer(artifact.QualificationID)
		}
		var checkpointID *string
		if artifact.CheckpointID != "" {
			checkpointID = pointer(artifact.CheckpointID)
		}
		artifactViews = append(artifactViews, ArtifactSummary{ArtifactID: artifact.ID, TaskID: artifact.TaskID, AuthorEmployeeID: artifact.Author, Digest: artifact.Digest, Bytes: stringValue(artifact.Bytes), State: mapArtifactState(artifact.State), Verdict: mapArtifactVerdict(artifact.Verdict), ContractRevisionID: contractID, QualificationID: qualificationID, CheckpointID: checkpointID})
	}
	checkpointViews := make([]CheckpointSummary, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		view, err := checkpointView(checkpoint)
		if err != nil {
			return CompanyOverviewView{}, err
		}
		checkpointViews = append(checkpointViews, view)
	}
	employeeViews, team, resources := employeeViews(employees, tasks, obligationViews, observedAt)
	if slicesContainReconcile(employees) {
		meta.RecoveryState = "recovery_required"
	}
	recentActivity := make([]ActivityEvent, 0, len(activityRows))
	for _, event := range activityRows {
		recentActivity = append(recentActivity, eventView(companyID, event, observedAt))
	}
	attention := attentionViews(obligationViews)
	for _, intervention := range interventions {
		attention = append(attention, humanInterventionAttention(intervention))
	}
	missionView := MissionSummary{MissionID: mission.ID, Title: firstNonEmpty(mission.Title, mission.ID), Goal: missionGoal(mission), State: mapMissionState(mission.State), Contract: mission.Contract, AcceptanceContract: mission.AcceptanceContract, CurrentContractRevision: currentContract, NextMilestone: "不可得", VerifiedMilestones: "0", MilestoneTotal: "0", Milestones: []Milestone{}, Closeout: closeout}
	description := "runtime company scope from PostgreSQL"
	if workspaceRoot != "" {
		description = "workspace root configured: " + workspaceRoot
	}
	return CompanyOverviewView{Meta: meta, Company: CompanyView{CompanyID: companyID, Name: firstNonEmpty(companyName, companyID), Description: description}, Mission: missionView, Team: team, Employees: employeeViews, Tasks: taskViews, Obligations: obligationViews, Artifacts: artifactViews, Checkpoints: checkpointViews, Resources: resources, Attention: attention, RecentActivity: recentActivity}, nil
}

func readMissionCloseout(ctx context.Context, tx pgx.Tx, companyID, missionID string) (*MissionCloseoutSummary, error) {
	var closeout MissionCloseoutSummary
	var terminalOutcome pgtype.Text
	var finishedAt pgtype.Timestamptz
	var openedAt time.Time
	var report []byte
	err := tx.QueryRow(ctx, `SELECT requested_outcome,rationale,acceptance_artifact_ids,request_id,opened_at,
terminal_outcome,closeout_report,finished_at FROM mission_closeouts WHERE company_id=$1 AND mission_id=$2`, companyID, missionID).Scan(
		&closeout.RequestedOutcome, &closeout.Rationale, &closeout.AcceptanceArtifactIDs, &closeout.RequestID, &openedAt,
		&terminalOutcome, &report, &finishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	closeout.OpenedAt = openedAt.UTC().Format(time.RFC3339Nano)
	if terminalOutcome.Valid {
		value := terminalOutcome.String
		closeout.TerminalOutcome = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC().Format(time.RFC3339Nano)
		closeout.FinishedAt = &value
	}
	if len(report) > 0 {
		if !json.Valid(report) {
			return nil, fmt.Errorf("Mission closeout report is invalid")
		}
		closeout.Report = append(json.RawMessage(nil), report...)
	}
	if closeout.AcceptanceArtifactIDs == nil {
		closeout.AcceptanceArtifactIDs = []string{}
	}
	return &closeout, nil
}

func readHumanInterventions(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]humanInterventionRow, error) {
	rows, err := tx.Query(ctx, `SELECT hi.id,hi.severity,hi.reason_code,hi.protection_action,hi.required_action,hi.state,
COALESCE(latest_delivery.state,ni.state,'unavailable'),hi.evidence_refs
FROM human_interventions hi
LEFT JOIN notification_intents ni ON ni.company_id=hi.company_id AND ni.intervention_id=hi.id AND ni.event_kind='human_intervention.open'
LEFT JOIN LATERAL (
  SELECT nd.state FROM notification_deliveries nd
  WHERE nd.company_id=ni.company_id AND nd.intent_id=ni.id
  ORDER BY nd.attempt_count DESC,nd.created_at DESC,nd.id LIMIT 1
) latest_delivery ON true
WHERE hi.company_id=$1 AND hi.state IN ('open','acknowledged')
AND hi.expires_at>clock_timestamp() AND (hi.mission_id IS NULL OR hi.mission_id=$2 OR $2='unavailable')
ORDER BY CASE hi.severity WHEN 'critical' THEN 0 ELSE 1 END,hi.created_at DESC,hi.id LIMIT 100`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]humanInterventionRow, 0)
	for rows.Next() {
		var item humanInterventionRow
		var evidenceJSON []byte
		if err = rows.Scan(&item.ID, &item.Severity, &item.ReasonCode, &item.ProtectionAction, &item.RequiredAction, &item.WorkflowState, &item.NotificationState, &evidenceJSON); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(evidenceJSON, &item.EvidenceRefs); err != nil {
			return nil, core.Integrity
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readTasks(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]taskRow, error) {
	if missionID == "unavailable" {
		return []taskRow{}, nil
	}
	rows, err := tx.Query(ctx, "SELECT id,mission_id,owner,kind,state,generation FROM tasks WHERE company_id=$1 AND mission_id=$2 ORDER BY id", companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]taskRow, 0)
	for rows.Next() {
		var item taskRow
		if err := rows.Scan(&item.ID, &item.MissionID, &item.Owner, &item.Kind, &item.State, &item.Generation); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readEmployees(ctx context.Context, tx pgx.Tx, companyID string) ([]employeeRow, error) {
	rows, err := tx.Query(ctx, `SELECT e.id,e.epoch,
COALESCE(ws.id,''),COALESCE(ws.task_id,''),COALESCE(ws.state,''),COALESCE(ws.profile,''),
COALESCE(ws.epoch,e.epoch),COALESCE(ws.tool_call_limit,0),COALESCE(ws.tool_calls_used,0),
es.state,es.work_generation,es.checked_generation,es.next_due_at,COALESCE(es.pause_reason,'')
FROM employees e
LEFT JOIN employee_schedules es ON es.company_id=e.company_id AND es.employee_id=e.id
LEFT JOIN LATERAL (
  SELECT id,task_id,state,profile,epoch,tool_call_limit,tool_calls_used
  FROM worker_sessions
  WHERE company_id=e.company_id AND employee_id=e.id
  ORDER BY (state!='stopped') DESC,epoch DESC,id DESC LIMIT 1
) ws ON true
WHERE e.company_id=$1 ORDER BY e.id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]employeeRow, 0)
	for rows.Next() {
		var item employeeRow
		if err := rows.Scan(&item.ID, &item.Epoch, &item.SessionID, &item.SessionTask, &item.SessionState, &item.Profile, &item.SessionEpoch, &item.ToolLimit, &item.ToolUsed,
			&item.ScheduleState, &item.ScheduleWorkGeneration, &item.ScheduleCheckedGen, &item.ScheduleNextDueAt, &item.SchedulePauseReason); err != nil {
			return nil, err
		}
		if item.ScheduleState.Valid && (!item.ScheduleWorkGeneration.Valid || !item.ScheduleCheckedGen.Valid) {
			return nil, fmt.Errorf("employee schedule generation is incomplete")
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readRevisions(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]revisionRow, error) {
	if missionID == "unavailable" {
		return []revisionRow{}, nil
	}
	rows, err := tx.Query(ctx, `SELECT id,revision,endpoint,state,digest,proposer,COALESCE(accepter,'')
FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 ORDER BY revision`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]revisionRow, 0)
	for rows.Next() {
		var item revisionRow
		if err := rows.Scan(&item.ID, &item.Revision, &item.Endpoint, &item.State, &item.Digest, &item.Proposer, &item.Accepter); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readTaskRevisions(ctx context.Context, tx pgx.Tx, companyID, missionID string) (map[string]taskRevisionRow, error) {
	result := make(map[string]taskRevisionRow)
	if missionID == "unavailable" {
		return result, nil
	}
	rows, err := tx.Query(ctx, `SELECT tr.task_id,tr.contract_revision_id,tr.revision
FROM task_revisions tr JOIN tasks t ON t.company_id=tr.company_id AND t.id=tr.task_id
WHERE tr.company_id=$1 AND t.mission_id=$2 ORDER BY tr.task_id,tr.revision DESC`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item taskRevisionRow
		if err := rows.Scan(&item.TaskID, &item.ContractID, &item.Revision); err != nil {
			return nil, err
		}
		if _, exists := result[item.TaskID]; !exists {
			result[item.TaskID] = item
		}
	}
	return result, rows.Err()
}

func readWorkspaces(ctx context.Context, tx pgx.Tx, companyID, missionID string) (map[string]workspaceRow, error) {
	result := make(map[string]workspaceRow)
	if missionID == "unavailable" {
		return result, nil
	}
	rows, err := tx.Query(ctx, `SELECT w.task_id,w.revision FROM worker_workspaces w
JOIN tasks t ON t.company_id=w.company_id AND t.id=w.task_id
WHERE w.company_id=$1 AND t.mission_id=$2 ORDER BY w.task_id`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item workspaceRow
		if err := rows.Scan(&item.TaskID, &item.Revision); err != nil {
			return nil, err
		}
		result[item.TaskID] = item
	}
	return result, rows.Err()
}

func readArtifacts(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]artifactRow, error) {
	if missionID == "unavailable" {
		return []artifactRow{}, nil
	}
	rows, err := tx.Query(ctx, `SELECT a.id,a.task_id,a.author,a.digest,a.bytes,a.state,
CASE WHEN EXISTS (
 SELECT 1 FROM mission_change_requests r
 JOIN LATERAL (SELECT state FROM mission_change_request_events e WHERE e.company_id=r.company_id AND e.change_request_id=r.change_request_id ORDER BY event_seq DESC LIMIT 1) latest ON true
 WHERE r.company_id=a.company_id AND r.mission_id=t.mission_id AND r.block_previous_results AND latest.state NOT IN ('declined','superseded')
) THEN 'invalidated' ELSE a.verdict END,
COALESCE(aq.contract_revision_id,''),COALESCE(product_aq.check_id,''),COALESCE(product_aq.checkpoint_id,aq.checkpoint_id,'')
FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
LEFT JOIN artifact_qualifications aq ON aq.company_id=a.company_id AND aq.artifact_id=a.id
LEFT JOIN task_validation_artifact_qualifications product_aq ON product_aq.company_id=a.company_id AND product_aq.artifact_id=a.id
WHERE a.company_id=$1 AND t.mission_id=$2 AND a.artifact_kind='deliverable' ORDER BY a.id`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]artifactRow, 0)
	for rows.Next() {
		var item artifactRow
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Author, &item.Digest, &item.Bytes, &item.State, &item.Verdict, &item.ContractID, &item.QualificationID, &item.CheckpointID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readCheckpoints(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]checkpointRow, error) {
	if missionID == "unavailable" {
		return []checkpointRow{}, nil
	}
	rows, err := tx.Query(ctx, `SELECT c.id,s.task_id,s.employee_id,c.session_id,s.state,s.epoch,c.digest,c.data::text,
COALESCE(product_aq.artifact_id,legacy_aq.artifact_id,'')
FROM worker_checkpoints c
JOIN worker_sessions s ON s.company_id=c.company_id AND s.id=c.session_id
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
LEFT JOIN task_validation_artifact_qualifications product_aq ON product_aq.company_id=c.company_id AND product_aq.checkpoint_id=c.id
LEFT JOIN artifact_qualifications legacy_aq ON legacy_aq.company_id=c.company_id AND legacy_aq.checkpoint_id=c.id
WHERE c.company_id=$1 AND t.mission_id=$2
ORDER BY s.task_id,s.epoch DESC,c.id DESC`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rowsByTask := make(map[string][]checkpointRow)
	for rows.Next() {
		var item checkpointRow
		if err := rows.Scan(&item.ID, &item.TaskID, &item.EmployeeID, &item.SessionID, &item.SessionState, &item.SessionEpoch, &item.Digest, &item.Data, &item.ArtifactID); err != nil {
			return nil, err
		}
		var payload checkpointPayload
		if err := json.Unmarshal(item.Data, &payload); err != nil {
			return nil, fmt.Errorf("checkpoint %s has invalid data: %w", item.ID, err)
		}
		item.Kind = payload.Kind
		item.WorkspaceRevision = payload.WorkspaceRevision
		rowsByTask[item.TaskID] = append(rowsByTask[item.TaskID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	selected := make([]checkpointRow, 0, len(rowsByTask))
	for _, taskRows := range rowsByTask {
		selected = append(selected, selectCurrentCheckpoint(taskRows))
	}
	sort.Slice(selected, func(left, right int) bool {
		if selected[left].TaskID != selected[right].TaskID {
			return selected[left].TaskID < selected[right].TaskID
		}
		return selected[left].ID < selected[right].ID
	})
	return selected, nil
}

func checkpointView(row checkpointRow) (CheckpointSummary, error) {
	var payload checkpointPayload
	if err := json.Unmarshal(row.Data, &payload); err != nil {
		return CheckpointSummary{}, fmt.Errorf("checkpoint %s has invalid data: %w", row.ID, err)
	}
	kind := payload.Kind
	if kind == "" {
		kind = "unknown"
	}
	qualificationState := "unverified"
	if kind == "qualified" && (payload.ValidationStatus == "" || payload.ValidationStatus == "PASS") {
		qualificationState = "qualified"
	} else if kind == "progress" {
		qualificationState = "progress"
	}
	state := payload.FinalizationState
	if state == "" {
		state = "persisted"
	}
	workspaceDigest := firstNonEmpty(payload.WorkspaceDigest, row.Digest)
	var artifactID *string
	if row.ArtifactID != "" {
		artifactID = pointer(row.ArtifactID)
	}
	var workspaceRevision *string
	if payload.WorkspaceRevision > 0 {
		workspaceRevision = pointer(stringValue(payload.WorkspaceRevision))
	}
	var digest *string
	if workspaceDigest != "" {
		digest = pointer(workspaceDigest)
	}
	return CheckpointSummary{CheckpointID: row.ID, TaskID: row.TaskID, EmployeeID: row.EmployeeID, SessionID: row.SessionID, SessionState: row.SessionState, SessionEpoch: stringValue(row.SessionEpoch), Kind: kind, State: state, QualificationState: qualificationState, WorkspaceRevision: workspaceRevision, WorkspaceDigest: digest, ArtifactID: artifactID}, nil
}

func readObligations(ctx context.Context, tx pgx.Tx, companyID, missionID string) ([]obligationRow, error) {
	if missionID == "unavailable" {
		return []obligationRow{}, nil
	}
	rows, err := tx.Query(ctx, `SELECT o.id,o.task_id,o.owner,o.state,COALESCE(o.evidence_ref,''),COALESCE(o.evidence_id,''),COALESCE(m.body,'')
FROM obligations o JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
LEFT JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
WHERE o.company_id=$1 AND t.mission_id=$2 ORDER BY o.id`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]obligationRow, 0)
	for rows.Next() {
		var item obligationRow
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Owner, &item.State, &item.EvidenceRef, &item.EvidenceID, &item.MessageBody); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readReviews(ctx context.Context, tx pgx.Tx, companyID, missionID string) (map[string]string, error) {
	result := make(map[string]string)
	if missionID == "unavailable" {
		return result, nil
	}
	rows, err := tx.Query(ctx, `SELECT rr.task_id,rr.verdict FROM review_records rr
JOIN tasks t ON t.company_id=rr.company_id AND t.id=rr.task_id
WHERE rr.company_id=$1 AND t.mission_id=$2`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID, verdict string
		if err := rows.Scan(&taskID, &verdict); err != nil {
			return nil, err
		}
		result[taskID] = verdict
	}
	return result, rows.Err()
}

func readRecentEvents(ctx context.Context, tx pgx.Tx, companyID string, sequence int64) ([]eventRow, error) {
	rows, err := tx.Query(ctx, `SELECT company_seq,kind,payload::text,observed
FROM events WHERE company_id=$1 AND company_seq<=$2 ORDER BY company_seq DESC LIMIT 50`, companyID, sequence)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

func viewMeta(companyID string, sequence int64, observedAt string) ViewMeta {
	return ViewMeta{SchemaVersion: viewSchemaVersion, CompanyID: companyID, EntityRevision: snapshotCursorForSequence(sequence), SnapshotCursor: snapshotCursorForSequence(sequence), ObservedAt: observedAt, DataMode: "real", Freshness: "fresh", SourceLabel: "PostgreSQL runtime read model", RecoveryState: "operational"}
}

func slicesContainReconcile(rows []employeeRow) bool {
	for _, row := range rows {
		if row.SessionState == "reconcile_required" {
			return true
		}
	}
	return false
}

func contractView(row revisionRow) ContractRevisionSummary {
	var accepter *string
	if row.Accepter != "" {
		accepter = pointer(row.Accepter)
	}
	return ContractRevisionSummary{RevisionID: row.ID, Revision: stringValue(row.Revision), Endpoint: row.Endpoint, State: mapRevisionState(row.State), Digest: row.Digest, ProposerEmployeeID: row.Proposer, AccepterEmployeeID: accepter}
}

func currentRevision(rows []revisionRow) *ContractRevisionSummary {
	var chosen *revisionRow
	for index := range rows {
		row := rows[index]
		if chosen == nil || (row.State == "accepted" && chosen.State != "accepted") || (row.State == chosen.State && row.Revision > chosen.Revision) {
			chosen = &row
		}
	}
	if chosen == nil {
		return nil
	}
	view := contractView(*chosen)
	return &view
}

func taskView(row taskRow, revisions map[string]taskRevisionRow, workspaces map[string]workspaceRow, artifacts []artifactRow, reviews map[string]string) TaskSummary {
	var contractID *string
	if revision, exists := revisions[row.ID]; exists {
		contractID = pointer(revision.ContractID)
	}
	var workspaceRevision *string
	if workspace, exists := workspaces[row.ID]; exists {
		workspaceRevision = pointer(stringValue(workspace.Revision))
	}
	acceptance := reviews[row.ID]
	artifactInvalidated := false
	for _, artifact := range artifacts {
		artifactInvalidated = artifactInvalidated || (artifact.TaskID == row.ID && artifact.Verdict == "invalidated")
	}
	if artifactInvalidated {
		acceptance = "inconclusive"
	} else if acceptance == "" {
		acceptance = "not_started"
		for _, artifact := range artifacts {
			if artifact.TaskID != row.ID {
				continue
			}
			if artifact.Verdict == "invalidated" {
				acceptance = "inconclusive"
			} else if artifact.Verdict == "passed" {
				acceptance = "passed"
			} else if artifact.Verdict == "failed" || artifact.Verdict == "invalidated" {
				acceptance = "failed"
			} else {
				acceptance = "candidate"
			}
		}
	}
	return TaskSummary{TaskID: row.ID, Title: row.Kind, Kind: row.Kind, State: mapTaskState(row.State), OwnerEmployeeID: row.Owner, Generation: stringValue(row.Generation), ContractRevisionID: contractID, WorkspaceRevision: workspaceRevision, Acceptance: mapAcceptance(acceptance), DependencyLabel: nil}
}

func employeeViews(rows []employeeRow, tasks []taskRow, obligations []ObligationSummary, observedAt string) ([]EmployeeSummary, TeamSummary, ResourceSummary) {
	views := make([]EmployeeSummary, 0, len(rows))
	team := TeamSummary{Total: stringValue(int64(len(rows))), Working: "0", Sleeping: "0", Waiting: "0", Stopped: "0"}
	var toolUsed, toolLimit int64
	hasSession := false
	for _, row := range rows {
		status, tone, reason := employeeStatus(row.SessionState)
		var sessionID, sessionState, profile *string
		if row.SessionID != "" {
			hasSession = true
			sessionID = pointer(row.SessionID)
			sessionState = pointer(row.SessionState)
			profile = pointer(row.Profile)
			toolUsed += row.ToolUsed
			toolLimit += row.ToolLimit
		}
		var currentTask *EntityRef
		for _, task := range tasks {
			if task.Owner == row.ID && task.State != "completed" && task.State != "cancelled" {
				currentTask = &EntityRef{Kind: "task", ID: task.ID, Label: task.ID}
				break
			}
		}
		openCount := int64(0)
		for _, obligation := range obligations {
			if obligation.OwnerEmployeeID == row.ID && obligation.State != "fulfilled" && obligation.State != "declined" && obligation.State != "superseded" {
				openCount++
			}
		}
		limit, used, remaining := employeeBudget(row)
		views = append(views, EmployeeSummary{EmployeeID: row.ID, DisplayName: row.ID, Role: employeeRole(row.ID), RoleRevision: nil, Epoch: stringValue(row.Epoch), SessionID: sessionID, SessionState: sessionState, Profile: profile, CurrentTask: currentTask, Status: EmployeeStatusView{Primary: status, Tone: tone, Reason: reason, ActiveModelRequests: "不可得", InFlightTools: "不可得", ObservedAt: observedAt}, Schedule: employeeScheduleView(row), ToolBudget: ToolBudgetView{Limit: limit, Used: used, Remaining: remaining, Quality: budgetQuality(row.SessionID)}, Qualification: QualificationView{Status: "unverified", EvidenceID: nil, PolicyRevision: nil}, OpenObligationCount: stringValue(openCount)})
		switch status {
		case "working":
			team.Working = incrementString(team.Working)
		case "sleeping":
			team.Sleeping = incrementString(team.Sleeping)
		case "stopped":
			team.Stopped = incrementString(team.Stopped)
		default:
			team.Waiting = incrementString(team.Waiting)
		}
	}
	resource := ResourceSummary{ToolBudgetQuality: "unavailable", MoneyQuality: "unavailable", MoneyAmount: nil, Currency: nil, AsOf: observedAt, Note: "money and token usage are not exposed by this read model"}
	if hasSession {
		limitText := stringValue(toolLimit)
		if toolLimit == 0 {
			limitText = "unbounded"
		}
		resource = ResourceSummary{ToolCallsUsed: stringValue(toolUsed), ToolCallsLimit: limitText, ToolBudgetQuality: "reported", MoneyQuality: "unavailable", MoneyAmount: nil, Currency: nil, AsOf: observedAt, Note: "tool call budget is reported from WorkerSession; it is not a token or money budget"}
	} else {
		resource.ToolCallsUsed = "不可得"
		resource.ToolCallsLimit = "不可得"
	}
	return views, team, resource
}

func employeeBudget(row employeeRow) (*string, *string, *string) {
	if row.SessionID == "" {
		return nil, nil, nil
	}
	used := pointer(stringValue(row.ToolUsed))
	if row.ToolLimit == 0 {
		return nil, used, nil
	}
	remaining := row.ToolLimit - row.ToolUsed
	if remaining < 0 {
		remaining = 0
	}
	return pointer(stringValue(row.ToolLimit)), used, pointer(stringValue(remaining))
}

func employeeScheduleView(row employeeRow) *EmployeeScheduleView {
	if !row.ScheduleState.Valid {
		return nil
	}
	var nextDueAt *string
	if row.ScheduleNextDueAt.Valid {
		nextDueAt = pointer(row.ScheduleNextDueAt.Time.UTC().Format(time.RFC3339Nano))
	}
	return &EmployeeScheduleView{
		State:             row.ScheduleState.String,
		WorkGeneration:    stringValue(row.ScheduleWorkGeneration.Int64),
		CheckedGeneration: stringValue(row.ScheduleCheckedGen.Int64),
		NextDueAt:         nextDueAt,
		PauseReason:       row.SchedulePauseReason,
	}
}

func eventView(companyID string, row eventRow, observedAt string) ActivityEvent {
	kind, tone := presentationForEventKind(row.Kind)
	payload := make(map[string]any)
	_ = json.Unmarshal([]byte(row.Payload), &payload)
	subjectID := stringFromPayload(payload, "id")
	if subjectID == "" {
		subjectID = fmt.Sprintf("company-seq-%d", row.Sequence)
	}
	subjectKind := "runtime_event"
	switch {
	case strings.Contains(row.Kind, "mission"):
		subjectKind = "mission"
	case strings.Contains(row.Kind, "worker"):
		subjectKind = "worker_session"
	case strings.Contains(row.Kind, "contract"):
		subjectKind = "contract_revision"
	case strings.Contains(row.Kind, "artifact"):
		subjectKind = "artifact"
	case strings.Contains(row.Kind, "checkpoint"):
		subjectKind = "checkpoint"
	case strings.Contains(row.Kind, "collab") || strings.Contains(row.Kind, "obligation"):
		subjectKind = "message"
	case strings.Contains(row.Kind, "task"):
		subjectKind = "task"
	}
	detail := fmt.Sprintf("authoritative company_seq=%d; runtime event kind=%s", row.Sequence, row.Kind)
	metadata := map[string]string{"runtimeKind": row.Kind, "observed": fmt.Sprintf("%t", row.Observed)}
	if payloadID := stringFromPayload(payload, "id"); payloadID != "" {
		metadata["subjectId"] = payloadID
	}
	return ActivityEvent{ID: fmt.Sprintf("%s:%d", companyID, row.Sequence), CompanySeq: stringValue(row.Sequence), OccurredAt: eventOccurredAt(payload), Kind: kind, Actor: ActivityActor{Kind: "system", ID: "runtime", Label: "Polis runtime"}, Subject: EntityRef{Kind: subjectKind, ID: subjectID, Label: row.Kind}, Summary: row.Kind, Detail: detail, Tone: tone, EvidenceRefs: []string{}, Metadata: metadata}
}

func eventOccurredAt(payload map[string]any) string {
	for _, key := range []string{"occurred_at", "occurredAt", "created_at", "createdAt"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "unavailable"
}

func stringFromPayload(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func attentionViews(obligations []ObligationSummary) []AttentionItem {
	items := make([]AttentionItem, 0)
	for _, obligation := range obligations {
		if obligation.State == "fulfilled" || obligation.State == "declined" || obligation.State == "superseded" {
			continue
		}
		items = append(items, AttentionItem{ID: obligation.ObligationID, Tone: "warning", Title: "待处理 Obligation", Description: obligation.Note, Subject: EntityRef{Kind: "obligation", ID: obligation.ObligationID, Label: obligation.ObligationID}, EvidenceRefs: []string{}})
	}
	return items
}

func employeeStatus(sessionState string) (string, string, string) {
	switch sessionState {
	case "active", "validating":
		return "working", "info", "WorkerSession is active or validating"
	case "stopped":
		return "stopped", "neutral", "WorkerSession is stopped"
	case "stopping", "restoring", "activation_pending_environment":
		return "wake_pending", "warning", "WorkerSession is transitioning"
	case "reconcile_required":
		return "waiting_external", "warning", "WorkerSession requires reconciliation"
	default:
		return "sleeping", "neutral", "no active WorkerSession"
	}
}

func budgetQuality(sessionID string) string {
	if sessionID == "" {
		return "unavailable"
	}
	return "reported"
}

func employeeRole(id string) string {
	switch id {
	case "emp-planning":
		return "Planning Employee"
	case "emp-backend":
		return "Backend Employee"
	case "emp-frontend":
		return "Frontend Employee"
	case "emp-review":
		return "Review Employee"
	default:
		return "Employee"
	}
}

func missionGoal(mission missionRow) string {
	if mission.ID == "unavailable" {
		return "当前公司没有可读取的 Mission。"
	}
	return firstNonEmpty(mission.Goal, "Mission state and contract are read from the authoritative runtime tables.")
}

func incrementString(value string) string {
	var current int64
	_, _ = fmt.Sscan(value, &current)
	return stringValue(current + 1)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapMissionState(value string) string {
	if value == "active" || value == "paused" || value == "closing" || value == "succeeded" || value == "ended_not_met" || value == "draft" || value == "cancelled" {
		return value
	}
	return "draft"
}

func mapTaskState(value string) string {
	if value == "ready" || value == "working" || value == "candidate" || value == "completed" || value == "blocked" || value == "cancelled" {
		return value
	}
	return "blocked"
}

func mapRevisionState(value string) string {
	if value == "proposed" || value == "accepted" || value == "superseded" {
		return value
	}
	return "proposed"
}

func mapObligationState(value string) string {
	switch value {
	case "pending", "observed", "applied", "fulfilled", "declined", "superseded":
		return value
	default:
		return "pending"
	}
}

func mapArtifactState(value string) string {
	if value == "ready" || value == "missing" || value == "corrupt" {
		return value
	}
	return "corrupt"
}

func mapArtifactVerdict(value string) string {
	if value == "candidate" || value == "passed" || value == "failed" || value == "invalidated" {
		return value
	}
	return "invalidated"
}

func mapAcceptance(value string) string {
	if value == "not_started" || value == "candidate" || value == "passed" || value == "failed" || value == "inconclusive" {
		return value
	}
	return "inconclusive"
}
