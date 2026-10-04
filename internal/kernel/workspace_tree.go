// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	workspaceTreePageSize            = 32
	workspaceTreeMaxFiles            = 512
	workspaceTreeMaxFileBytes        = 2 << 20
	workspaceTreeMaxTotalBytes       = 16 << 20
	workspaceTreeMaxSnapshotsPerTask = 32
)

type WorkspaceTreeEntry struct {
	RelativePath   string `json:"relativePath"`
	Digest         string `json:"sha256"`
	Bytes          int64  `json:"bytes"`
	FileRevision   int64  `json:"fileRevision"`
	SourceRevision int64  `json:"sourceRevision"`
	ContentType    string `json:"contentType"`
}

type WorkspaceTreePage struct {
	RootBindingID string               `json:"rootBindingId"`
	TaskID        string               `json:"taskId"`
	Revision      int64                `json:"revision"`
	ReadReference string               `json:"readReference"`
	Entries       []WorkspaceTreeEntry `json:"entries"`
	Truncated     bool                 `json:"truncated"`
	NextCursor    string               `json:"nextCursor,omitempty"`
	NextAfterPath string               `json:"-"`
	Complete      bool                 `json:"complete"`
}

type workspaceTreeCursor struct {
	RootBindingID string `json:"rootBindingId"`
	TaskID        string `json:"taskId"`
	Revision      int64  `json:"revision"`
	AfterPath     string `json:"afterPath"`
	QuerySHA256   string `json:"querySha256,omitempty"`
}

type WorkspaceTreeRead struct {
	RootBindingID string `json:"rootBindingId"`
	TaskID        string `json:"taskId"`
	ReadReference string `json:"readReference"`
	RelativePath  string `json:"relativePath"`
	Digest        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	FileRevision  int64  `json:"fileRevision"`
	Revision      int64  `json:"workspaceRevision"`
	ContentType   string `json:"contentType"`
	Content       string `json:"content"`
}

type WorkspaceTreeSnapshot struct {
	ArtifactID    string               `json:"artifactId"`
	ReadReference string               `json:"readReference,omitempty"`
	TaskID        string               `json:"taskId"`
	RootBindingID string               `json:"rootBindingId"`
	Revision      int64                `json:"workspaceRevision"`
	Digest        string               `json:"manifestSha256"`
	Bytes         int64                `json:"manifestBytes"`
	FileCount     int                  `json:"fileCount"`
	Verdict       string               `json:"verdict"`
	Entries       []WorkspaceTreeEntry `json:"entries"`
}

type workspaceTreeManifest struct {
	Version       string               `json:"version"`
	CompanyID     string               `json:"companyId"`
	MissionID     string               `json:"missionId"`
	TaskID        string               `json:"taskId"`
	RootBindingID string               `json:"rootBindingId"`
	Revision      int64                `json:"workspaceRevision"`
	Entries       []WorkspaceTreeEntry `json:"entries"`
}

func ValidWorkspaceRelativePath(path string) bool {
	if path == "" || !utf8.ValidString(path) || len(path) > 1024 || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.ContainsAny(path, "\\%:\x00") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." || len(segment) > 255 || strings.TrimSpace(segment) != segment {
			return false
		}
		for _, r := range segment {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}

// bindProductWorkspaceTreeTX creates the logical private file tree from the
// existing Task workspace on first admission. The stored paths are database
// relative paths over immutable CAS blobs; they do not claim to be host mounts.
func (k *Kernel) bindProductWorkspaceTreeTX(ctx context.Context, tx pgx.Tx, binding Binding, task Task, digest string, legacyRevision int64) error {
	var rootID, writerSession string
	err := tx.QueryRow(ctx, `SELECT id,writer_session_id FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2 FOR UPDATE`, binding.scope.company, task.ID).Scan(&rootID, &writerSession)
	if err == nil {
		if writerSession != binding.session {
			var oldState string
			if err = tx.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2", binding.scope.company, writerSession).Scan(&oldState); err != nil {
				return err
			}
			if oldState != "stopped" {
				return core.Conflict
			}
			_, err = tx.Exec(ctx, `UPDATE worker_workspace_roots SET writer_session_id=$3,writer_epoch=$4 WHERE company_id=$1 AND id=$2`, binding.scope.company, rootID, binding.session, binding.epoch)
		}
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if legacyRevision < 1 {
		return core.Integrity
	}
	content, err := readBlobBounded(k.root, binding.scope.company, digest, workspaceTreeMaxFileBytes)
	if err != nil || !utf8.Valid(content) {
		return core.Integrity
	}
	path := "workspace.txt"
	if task.Kind == core.TaskKindCompat {
		path = "formatter.go"
	}
	rootID = newID()
	if _, err = tx.Exec(ctx, `INSERT INTO worker_workspace_roots(company_id,id,task_id,mission_id,owner,read_write_class,revision,writer_session_id,writer_epoch)
VALUES($1,$2,$3,$4,$5,'task_private',1,$6,$7)`, binding.scope.company, rootID, task.ID, task.Mission, task.Owner, binding.session, binding.epoch); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO worker_workspace_files(company_id,workspace_id,relative_path,digest,bytes,file_revision,source_revision,content_type)
	VALUES($1,$2,$3,$4,$5,1,$6,'text/utf-8')`, binding.scope.company, rootID, path, digest, len(content), legacyRevision)
	return err
}

func (k *Kernel) requireWorkspaceTreeWriterTX(ctx context.Context, tx pgx.Tx, binding Binding, expectedRevision int64, lock string) (string, int64, string, error) {
	if _, err := k.checkSession(ctx, tx, binding, true); err != nil {
		return "", 0, "", err
	}
	taskID, err := k.requireProductTaskWorking(ctx, tx, binding)
	if err != nil || taskID != binding.task {
		if err == nil {
			err = core.StaleEpoch
		}
		return "", 0, "", err
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT owner FROM tasks WHERE company_id=$1 AND id=$2`, binding.scope.company, taskID).Scan(&owner); err != nil {
		return "", 0, "", err
	}
	if owner != binding.employee {
		return "", 0, "", core.Denied
	}
	var rootID, class, writerSession string
	var revision, writerEpoch int64
	query := `SELECT r.id,r.read_write_class,r.revision,r.writer_session_id,r.writer_epoch,r.owner,r.mission_id,t.owner,t.mission_id
FROM worker_workspace_roots r JOIN tasks t ON t.company_id=r.company_id AND t.id=r.task_id WHERE r.company_id=$1 AND r.task_id=$2`
	if lock == "update" {
		query += " FOR UPDATE OF r"
	} else {
		query += " FOR SHARE OF r"
	}
	var rootOwner, rootMission, taskOwner, taskMission string
	if err = tx.QueryRow(ctx, query, binding.scope.company, binding.task).Scan(&rootID, &class, &revision, &writerSession, &writerEpoch, &rootOwner, &rootMission, &taskOwner, &taskMission); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = core.Conflict
		}
		return "", 0, "", err
	}
	if class != "task_private" || rootOwner != binding.employee || taskOwner != binding.employee || rootMission != taskMission || writerSession != binding.session || writerEpoch != binding.epoch {
		return "", 0, "", core.StaleEpoch
	}
	if expectedRevision > 0 && revision != expectedRevision {
		return "", revision, "", core.Conflict
	}
	return rootID, revision, taskID, nil
}

func (k *Kernel) ListProductWorkspaceTree(ctx context.Context, binding Binding, afterCursor string) (WorkspaceTreePage, error) {
	cursor, err := decodeWorkspaceTreeCursor(afterCursor, "")
	if err != nil {
		return WorkspaceTreePage{}, err
	}
	return k.listProductWorkspaceTreePage(ctx, binding, cursor)
}

func (k *Kernel) listProductWorkspaceTreePage(ctx context.Context, binding Binding, cursor workspaceTreeCursor) (WorkspaceTreePage, error) {
	page := WorkspaceTreePage{Entries: []WorkspaceTreeEntry{}, Complete: true}
	if cursor.AfterPath != "" && !ValidWorkspaceRelativePath(cursor.AfterPath) {
		return page, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return page, err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, binding.scope, &binding, false); err != nil {
		return page, err
	}
	rootID, revision, taskID, err := k.requireWorkspaceTreeWriterTX(ctx, tx, binding, 0, "share")
	if err != nil {
		return page, err
	}
	if (cursor.RootBindingID != "" && cursor.RootBindingID != rootID) || (cursor.TaskID != "" && cursor.TaskID != taskID) {
		return page, core.OutOfScope
	}
	if cursor.Revision > 0 && cursor.Revision != revision {
		return page, core.Conflict
	}
	rows, err := tx.Query(ctx, `SELECT relative_path,digest,bytes,file_revision,source_revision,content_type
FROM worker_workspace_files WHERE company_id=$1 AND workspace_id=$2 AND relative_path COLLATE "C">$3 COLLATE "C" ORDER BY relative_path COLLATE "C" LIMIT $4`, binding.scope.company, rootID, cursor.AfterPath, workspaceTreePageSize+1)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var entry WorkspaceTreeEntry
		if err = rows.Scan(&entry.RelativePath, &entry.Digest, &entry.Bytes, &entry.FileRevision, &entry.SourceRevision, &entry.ContentType); err != nil {
			rows.Close()
			return page, err
		}
		if !ValidWorkspaceRelativePath(entry.RelativePath) || !validSHA256(entry.Digest) || entry.Bytes < 1 || entry.Bytes > workspaceTreeMaxFileBytes ||
			entry.FileRevision < 1 || entry.SourceRevision < 1 || entry.ContentType != "text/utf-8" {
			rows.Close()
			return WorkspaceTreePage{}, core.Integrity
		}
		if cursor.AfterPath != "" && strings.HasPrefix(entry.RelativePath, cursor.AfterPath+"/") {
			rows.Close()
			return WorkspaceTreePage{}, core.Integrity
		}
		page.Entries = append(page.Entries, entry)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Entries) > workspaceTreePageSize {
		page.Truncated = true
		page.Complete = false
		page.Entries = page.Entries[:workspaceTreePageSize]
		page.NextAfterPath = page.Entries[len(page.Entries)-1].RelativePath
	}
	if page.Truncated {
		page.NextCursor = encodeWorkspaceTreeCursor(workspaceTreeCursor{RootBindingID: rootID, TaskID: taskID, Revision: revision, AfterPath: page.NextAfterPath})
	}
	page.RootBindingID, page.TaskID, page.Revision = rootID, taskID, revision
	page.ReadReference = newID()
	paths := make([]string, len(page.Entries))
	for index, entry := range page.Entries {
		paths[index] = entry.RelativePath
	}
	if err = appendEvent(ctx, tx, binding.scope, "worker.workspace.tree.list", map[string]any{
		"readReference": page.ReadReference, "taskId": taskID, "workspaceId": rootID, "employeeId": binding.employee,
		"sessionId": binding.session, "epoch": binding.epoch, "workspaceRevision": revision, "afterRelativePath": cursor.AfterPath,
		"relativePaths": paths, "truncated": page.Truncated, "complete": page.Complete,
	}); err != nil {
		return page, err
	}
	return page, tx.Commit(ctx)
}

func (k *Kernel) SearchProductWorkspaceTree(ctx context.Context, binding Binding, query, afterCursor string) (WorkspaceTreePage, error) {
	if query == "" || len(query) > 256 || !utf8.ValidString(query) {
		return WorkspaceTreePage{}, core.Malformed
	}
	queryDigest := casDigest([]byte(query))
	cursor, err := decodeWorkspaceTreeCursor(afterCursor, queryDigest)
	if err != nil {
		return WorkspaceTreePage{}, err
	}
	out := WorkspaceTreePage{Entries: []WorkspaceTreeEntry{}, Complete: true}
	afterPath := cursor.AfterPath
	fileCursor := workspaceTreeCursor{RootBindingID: cursor.RootBindingID, TaskID: cursor.TaskID, Revision: cursor.Revision}
	for {
		page, pageErr := k.listProductWorkspaceTreePage(ctx, binding, fileCursor)
		if pageErr != nil {
			return WorkspaceTreePage{}, pageErr
		}
		if out.RootBindingID == "" {
			out.RootBindingID, out.TaskID, out.Revision = page.RootBindingID, page.TaskID, page.Revision
			fileCursor.RootBindingID, fileCursor.TaskID, fileCursor.Revision = page.RootBindingID, page.TaskID, page.Revision
		}
		for _, entry := range page.Entries {
			if entry.RelativePath <= afterPath {
				continue
			}
			content, readErr := readBlobBounded(k.root, binding.scope.company, entry.Digest, workspaceTreeMaxFileBytes)
			if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
				return WorkspaceTreePage{}, core.Integrity
			}
			if strings.Contains(string(content), query) {
				out.Entries = append(out.Entries, entry)
				if len(out.Entries) > workspaceTreePageSize {
					out.Entries = out.Entries[:workspaceTreePageSize]
					out.Truncated, out.Complete = true, false
					out.NextAfterPath = out.Entries[len(out.Entries)-1].RelativePath
					out.NextCursor = encodeWorkspaceTreeCursor(workspaceTreeCursor{RootBindingID: out.RootBindingID, TaskID: out.TaskID, Revision: out.Revision, AfterPath: out.NextAfterPath, QuerySHA256: queryDigest})
					break
				}
			}
		}
		if out.Truncated || !page.Truncated {
			break
		}
		fileCursor.AfterPath = page.NextAfterPath
	}
	if _, err = k.recordWorkspaceTreeRead(ctx, binding, "worker.workspace.tree.search", map[string]any{
		"workspaceId": out.RootBindingID, "workspaceRevision": out.Revision, "querySha256": queryDigest,
		"afterRelativePath": afterPath, "relativePaths": workspaceTreePaths(out.Entries), "truncated": out.Truncated, "complete": out.Complete,
	}); err != nil {
		return WorkspaceTreePage{}, err
	}
	return out, nil
}

func encodeWorkspaceTreeCursor(cursor workspaceTreeCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeWorkspaceTreeCursor(token, expectedQuerySHA256 string) (workspaceTreeCursor, error) {
	if token == "" {
		return workspaceTreeCursor{}, nil
	}
	if len(token) > 4096 {
		return workspaceTreeCursor{}, core.Malformed
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return workspaceTreeCursor{}, core.Malformed
	}
	var cursor workspaceTreeCursor
	if err = json.Unmarshal(raw, &cursor); err != nil || !core.ValidID(cursor.RootBindingID) || !core.ValidID(cursor.TaskID) || cursor.Revision < 1 || !ValidWorkspaceRelativePath(cursor.AfterPath) || cursor.QuerySHA256 != expectedQuerySHA256 {
		return workspaceTreeCursor{}, core.Malformed
	}
	return cursor, nil
}

func (k *Kernel) ReadProductWorkspaceFile(ctx context.Context, binding Binding, path string) (WorkspaceTreeRead, error) {
	var out WorkspaceTreeRead
	if !ValidWorkspaceRelativePath(path) {
		return out, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, binding.scope, &binding, false); err != nil {
		return out, err
	}
	rootID, revision, taskID, err := k.requireWorkspaceTreeWriterTX(ctx, tx, binding, 0, "share")
	if err != nil {
		return out, err
	}
	var entry WorkspaceTreeEntry
	if err = tx.QueryRow(ctx, `SELECT relative_path,digest,bytes,file_revision,source_revision,content_type FROM worker_workspace_files
WHERE company_id=$1 AND workspace_id=$2 AND relative_path=$3`, binding.scope.company, rootID, path).Scan(&entry.RelativePath, &entry.Digest, &entry.Bytes, &entry.FileRevision, &entry.SourceRevision, &entry.ContentType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, core.OutOfScope
		}
		return out, err
	}
	content, err := readBlobBounded(k.root, binding.scope.company, entry.Digest, workspaceTreeMaxFileBytes)
	if err != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
		return out, core.Integrity
	}
	out = WorkspaceTreeRead{RootBindingID: rootID, TaskID: taskID, ReadReference: newID(), RelativePath: path, Digest: entry.Digest, Bytes: entry.Bytes,
		FileRevision: entry.FileRevision, Revision: revision, ContentType: entry.ContentType, Content: string(content)}
	if err = appendEvent(ctx, tx, binding.scope, "worker.workspace.file.read", map[string]any{
		"readReference": out.ReadReference, "workspaceId": rootID, "taskId": taskID, "relativePath": path,
		"sha256": entry.Digest, "bytes": entry.Bytes, "fileRevision": entry.FileRevision, "workspaceRevision": revision,
		"employeeId": binding.employee, "sessionId": binding.session, "epoch": binding.epoch,
	}); err != nil {
		return WorkspaceTreeRead{}, err
	}
	return out, tx.Commit(ctx)
}

func (k *Kernel) WriteProductWorkspaceFile(ctx context.Context, binding Binding, key, path string, expectedRevision int64, content string) (Receipt, error) {
	if !ValidWorkspaceRelativePath(path) || expectedRevision < 1 || content == "" || len(content) > workspaceTreeMaxFileBytes || !utf8.ValidString(content) {
		return Receipt{}, core.Malformed
	}
	if path == "formatter.go" && len(content) > core.MaxContent {
		return Receipt{}, core.TooLarge
	}
	preflight, err := k.pool.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	if err = k.guard(ctx, preflight, binding.scope, &binding); err == nil {
		_, _, _, err = k.requireWorkspaceTreeWriterTX(ctx, preflight, binding, expectedRevision, "share")
	}
	_ = preflight.Rollback(ctx)
	if err != nil {
		return Receipt{}, err
	}
	digest, err := k.putBlobWithClaim(ctx, binding.scope.company, []byte(content))
	if err != nil {
		return Receipt{}, err
	}
	return k.TXWrite(ctx, binding.scope, &binding, key, "workspace.tree.write", []any{binding.task, path, expectedRevision, digest}, func(tx pgx.Tx) (Receipt, error) {
		rootID, revision, _, checkErr := k.requireWorkspaceTreeWriterTX(ctx, tx, binding, expectedRevision, "update")
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		var count int
		var total int64
		if checkErr = tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(bytes),0) FROM worker_workspace_files WHERE company_id=$1 AND workspace_id=$2`, binding.scope.company, rootID).Scan(&count, &total); checkErr != nil {
			return Receipt{}, checkErr
		}
		var oldBytes int64
		checkErr = tx.QueryRow(ctx, `SELECT bytes FROM worker_workspace_files WHERE company_id=$1 AND workspace_id=$2 AND relative_path=$3`, binding.scope.company, rootID, path).Scan(&oldBytes)
		if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
			return Receipt{}, checkErr
		}
		if errors.Is(checkErr, pgx.ErrNoRows) {
			if count >= workspaceTreeMaxFiles {
				return Receipt{}, core.TooLarge
			}
			var conflict bool
			if checkErr = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_workspace_files WHERE company_id=$1 AND workspace_id=$2 AND
(left(relative_path,length($3)+1)=$3||'/' OR left($3,length(relative_path)+1)=relative_path||'/'))`, binding.scope.company, rootID, path).Scan(&conflict); checkErr != nil {
				return Receipt{}, checkErr
			}
			if conflict {
				return Receipt{}, core.Conflict
			}
		}
		if total-oldBytes+int64(len(content)) > workspaceTreeMaxTotalBytes {
			return Receipt{}, core.TooLarge
		}
		next := revision + 1
		if _, checkErr = tx.Exec(ctx, `INSERT INTO worker_workspace_files(company_id,workspace_id,relative_path,digest,bytes,file_revision,source_revision,content_type)
VALUES($1,$2,$3,$4,$5,$6,$6,'text/utf-8') ON CONFLICT(company_id,workspace_id,relative_path) DO UPDATE SET digest=EXCLUDED.digest,bytes=EXCLUDED.bytes,file_revision=EXCLUDED.file_revision,source_revision=EXCLUDED.source_revision,content_type=EXCLUDED.content_type`, binding.scope.company, rootID, path, digest, len(content), next); checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, `UPDATE worker_workspace_roots SET revision=$3 WHERE company_id=$1 AND id=$2 AND revision=$4`, binding.scope.company, rootID, next, revision); checkErr != nil {
			return Receipt{}, checkErr
		}
		if path == "formatter.go" {
			var sourceRevision int64
			if checkErr = tx.QueryRow(ctx, `UPDATE worker_workspaces SET digest=$3,revision=revision+1 WHERE company_id=$1 AND task_id=$2 RETURNING revision`, binding.scope.company, binding.task, digest).Scan(&sourceRevision); checkErr != nil {
				return Receipt{}, checkErr
			}
			if _, checkErr = tx.Exec(ctx, `UPDATE worker_workspace_files SET source_revision=$4 WHERE company_id=$1 AND workspace_id=$2 AND relative_path=$3`, binding.scope.company, rootID, path, sourceRevision); checkErr != nil {
				return Receipt{}, checkErr
			}
		}
		return Receipt{ID: digest, Status: "persisted", Revision: next}, nil
	})
}

func (k *Kernel) DeleteProductWorkspaceFile(ctx context.Context, binding Binding, key, path string, expectedRevision int64) (Receipt, error) {
	if !ValidWorkspaceRelativePath(path) || expectedRevision < 1 {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, binding.scope, &binding, key, "workspace.tree.delete", []any{binding.task, path, expectedRevision}, func(tx pgx.Tx) (Receipt, error) {
		rootID, revision, _, err := k.requireWorkspaceTreeWriterTX(ctx, tx, binding, expectedRevision, "update")
		if err != nil {
			return Receipt{}, err
		}
		result, err := tx.Exec(ctx, `DELETE FROM worker_workspace_files WHERE company_id=$1 AND workspace_id=$2 AND relative_path=$3`, binding.scope.company, rootID, path)
		if err != nil {
			return Receipt{}, err
		}
		if result.RowsAffected() != 1 {
			return Receipt{}, core.OutOfScope
		}
		next := revision + 1
		if _, err = tx.Exec(ctx, `UPDATE worker_workspace_roots SET revision=$3 WHERE company_id=$1 AND id=$2 AND revision=$4`, binding.scope.company, rootID, next, revision); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: rootID, Status: "deleted", Revision: next}, nil
	})
}

func WorkspaceTreeCanSubmit(ctx context.Context, tx pgx.Tx, companyID, taskID string) (bool, error) {
	var rootExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2)`, companyID, taskID).Scan(&rootExists); err != nil || !rootExists {
		return !rootExists, err
	}
	var safe bool
	err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(relative_path='formatter.go') FROM worker_workspace_roots r
JOIN worker_workspace_files f ON f.company_id=r.company_id AND f.workspace_id=r.id
WHERE r.company_id=$1 AND r.task_id=$2`, companyID, taskID).Scan(&safe)
	return safe, err
}

func (k *Kernel) CreateProductWorkspaceSnapshot(ctx context.Context, binding Binding, key string, expectedRevision int64) (Receipt, WorkspaceTreeSnapshot, error) {
	var snapshot WorkspaceTreeSnapshot
	if expectedRevision < 1 {
		return Receipt{}, snapshot, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, binding.scope, &binding, key, "workspace.snapshot", []any{binding.task, expectedRevision}, func(tx pgx.Tx) (Receipt, error) {
		rootID, revision, taskID, txErr := k.requireWorkspaceTreeWriterTX(ctx, tx, binding, expectedRevision, "update")
		if txErr != nil {
			return Receipt{}, txErr
		}
		var missionID, contract string
		if txErr = tx.QueryRow(ctx, `SELECT t.mission_id,m.contract FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE t.company_id=$1 AND t.id=$2 AND t.owner=$3 AND t.state='working' AND m.state='active'`, binding.scope.company, taskID, binding.employee).Scan(&missionID, &contract); txErr != nil {
			return Receipt{}, txErr
		}
		var existingSnapshots int
		if txErr = tx.QueryRow(ctx, `SELECT count(*) FROM artifacts WHERE company_id=$1 AND task_id=$2 AND artifact_kind='workspace_snapshot'`, binding.scope.company, taskID).Scan(&existingSnapshots); txErr != nil {
			return Receipt{}, txErr
		}
		if existingSnapshots >= workspaceTreeMaxSnapshotsPerTask {
			return Receipt{}, core.TooLarge
		}
		rows, txErr := tx.Query(ctx, `SELECT relative_path,digest,bytes,file_revision,source_revision,content_type FROM worker_workspace_files
WHERE company_id=$1 AND workspace_id=$2 ORDER BY relative_path COLLATE "C"`, binding.scope.company, rootID)
		if txErr != nil {
			return Receipt{}, txErr
		}
		entries := make([]WorkspaceTreeEntry, 0, 32)
		var total int64
		for rows.Next() {
			var entry WorkspaceTreeEntry
			if txErr = rows.Scan(&entry.RelativePath, &entry.Digest, &entry.Bytes, &entry.FileRevision, &entry.SourceRevision, &entry.ContentType); txErr != nil {
				rows.Close()
				return Receipt{}, txErr
			}
			if !ValidWorkspaceRelativePath(entry.RelativePath) || entry.ContentType != "text/utf-8" || !validSHA256(entry.Digest) || entry.Bytes < 1 || entry.Bytes > workspaceTreeMaxFileBytes {
				rows.Close()
				return Receipt{}, core.Integrity
			}
			content, readErr := readBlobBounded(k.root, binding.scope.company, entry.Digest, workspaceTreeMaxFileBytes)
			if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
				rows.Close()
				return Receipt{}, core.Integrity
			}
			total += entry.Bytes
			if total > workspaceTreeMaxTotalBytes {
				rows.Close()
				return Receipt{}, core.TooLarge
			}
			entries = append(entries, entry)
		}
		rows.Close()
		if txErr = rows.Err(); txErr != nil {
			return Receipt{}, txErr
		}
		if len(entries) == 0 || len(entries) > workspaceTreeMaxFiles {
			return Receipt{}, core.Conflict
		}
		if !validWorkspaceTreeEntrySequence(entries) {
			return Receipt{}, core.Integrity
		}
		manifest := workspaceTreeManifest{Version: "polis-workspace-snapshot@1", CompanyID: binding.scope.company, MissionID: missionID, TaskID: taskID, RootBindingID: rootID, Revision: revision, Entries: entries}
		manifestBytes, txErr := json.Marshal(manifest)
		if txErr != nil {
			return Receipt{}, txErr
		}
		if len(manifestBytes) > 8<<20 {
			return Receipt{}, core.TooLarge
		}
		manifestDigest := casDigest(manifestBytes)
		artifactID := newID()
		if _, txErr = tx.Exec(ctx, `INSERT INTO artifact_staging(company_id,id,task_id,digest,artifact_kind,workspace_id,workspace_revision)
VALUES($1,$2,$3,$4,'workspace_snapshot',$5,$6)`, binding.scope.company, artifactID, taskID, manifestDigest, rootID, revision); txErr != nil {
			return Receipt{}, txErr
		}
		storedDigest, txErr := k.putBlobInTX(ctx, tx, binding.scope.company, manifestBytes)
		if txErr != nil || storedDigest != manifestDigest {
			if txErr != nil {
				return Receipt{}, txErr
			}
			return Receipt{}, core.Integrity
		}
		if _, txErr = tx.Exec(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract,artifact_kind,workspace_id,workspace_revision)
SELECT $1,$2,$3,$4,$5,$6,'ready','draft_not_accepted',contract,'workspace_snapshot',$7,$8 FROM missions WHERE company_id=$1 AND id=$9`,
			binding.scope.company, artifactID, taskID, binding.employee, manifestDigest, len(manifestBytes), rootID, revision, missionID); txErr != nil {
			return Receipt{}, txErr
		}
		for _, entry := range entries {
			if _, txErr = tx.Exec(ctx, `INSERT INTO workspace_snapshot_files(company_id,artifact_id,workspace_id,relative_path,digest,bytes,file_revision)
VALUES($1,$2,$3,$4,$5,$6,$7)`, binding.scope.company, artifactID, rootID, entry.RelativePath, entry.Digest, entry.Bytes, entry.FileRevision); txErr != nil {
				return Receipt{}, txErr
			}
		}
		if _, txErr = tx.Exec(ctx, `DELETE FROM artifact_staging WHERE company_id=$1 AND id=$2`, binding.scope.company, artifactID); txErr != nil {
			return Receipt{}, txErr
		}
		snapshot = WorkspaceTreeSnapshot{ArtifactID: artifactID, TaskID: taskID, RootBindingID: rootID, Revision: revision, Digest: manifestDigest, Bytes: int64(len(manifestBytes)), FileCount: len(entries), Verdict: "draft_not_accepted", Entries: entries}
		return Receipt{ID: artifactID, Status: "draft_not_accepted", Revision: revision}, nil
	})
	if err != nil {
		return Receipt{}, WorkspaceTreeSnapshot{}, err
	}
	if snapshot.ArtifactID == "" {
		// Exact retries replay the immutable Artifact ID; reload its frozen manifest.
		snapshot, err = k.ReadProductWorkspaceSnapshot(ctx, binding, receipt.ID)
		if err != nil {
			return Receipt{}, WorkspaceTreeSnapshot{}, err
		}
	}
	return receipt, snapshot, nil
}

func (k *Kernel) ReadProductWorkspaceSnapshot(ctx context.Context, binding Binding, artifactID string) (WorkspaceTreeSnapshot, error) {
	var snapshot WorkspaceTreeSnapshot
	if !core.ValidID(artifactID) {
		return snapshot, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return snapshot, err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, binding.scope, &binding, false); err != nil {
		return snapshot, err
	}
	readerTask, err := k.requireProductTaskWorking(ctx, tx, binding)
	if err != nil || readerTask != binding.task {
		if err == nil {
			err = core.StaleEpoch
		}
		return snapshot, err
	}
	var readerMission, sourceTask, sourceMission, author, workspaceID, digest, state, verdict string
	var revision, byteCount int64
	err = tx.QueryRow(ctx, `SELECT reader_task.mission_id,a.task_id,source_task.mission_id,a.author,a.workspace_id,a.digest,a.state,a.verdict,a.workspace_revision,a.bytes
FROM worker_sessions s JOIN tasks reader_task ON reader_task.company_id=s.company_id AND reader_task.id=s.task_id
JOIN artifacts a ON a.company_id=s.company_id AND a.id=$3
JOIN tasks source_task ON source_task.company_id=a.company_id AND source_task.id=a.task_id
WHERE s.company_id=$1 AND s.id=$2 AND s.state='active'`, binding.scope.company, binding.session, artifactID).Scan(&readerMission, &sourceTask, &sourceMission, &author, &workspaceID, &digest, &state, &verdict, &revision, &byteCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return snapshot, core.OutOfScope
	}
	if err != nil {
		return snapshot, err
	}
	if readerMission != sourceMission || (sourceTask == binding.task && author != binding.employee) || author == "" || state != "ready" || verdict != "draft_not_accepted" || workspaceID == "" || revision < 1 || byteCount < 1 || byteCount > 8<<20 || !validSHA256(digest) {
		return snapshot, core.OutOfScope
	}
	manifestBytes, err := readBlobBounded(k.root, binding.scope.company, digest, 8<<20)
	if err != nil || int64(len(manifestBytes)) != byteCount {
		return snapshot, core.Integrity
	}
	var manifest workspaceTreeManifest
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil || manifest.Version != "polis-workspace-snapshot@1" || manifest.CompanyID != binding.scope.company || manifest.MissionID != sourceMission || manifest.TaskID != sourceTask || manifest.RootBindingID != workspaceID || manifest.Revision != revision || len(manifest.Entries) == 0 || len(manifest.Entries) > workspaceTreeMaxFiles {
		return snapshot, core.Integrity
	}
	entries := make([]WorkspaceTreeEntry, len(manifest.Entries))
	fileRows, err := tx.Query(ctx, `SELECT relative_path,digest,bytes,file_revision FROM workspace_snapshot_files
WHERE company_id=$1 AND artifact_id=$2 ORDER BY relative_path COLLATE "C"`, binding.scope.company, artifactID)
	if err != nil {
		return snapshot, err
	}
	var total int64
	for index, entry := range manifest.Entries {
		var path, fileDigest string
		var fileBytes, fileRevision int64
		if !fileRows.Next() {
			fileRows.Close()
			return snapshot, core.Integrity
		}
		if err = fileRows.Scan(&path, &fileDigest, &fileBytes, &fileRevision); err != nil {
			fileRows.Close()
			return snapshot, err
		}
		if path != entry.RelativePath || fileDigest != entry.Digest || fileBytes != entry.Bytes || fileRevision != entry.FileRevision {
			fileRows.Close()
			return snapshot, core.Integrity
		}
		if !ValidWorkspaceRelativePath(entry.RelativePath) || entry.ContentType != "text/utf-8" || !validSHA256(entry.Digest) || entry.Bytes < 1 || entry.Bytes > workspaceTreeMaxFileBytes || entry.FileRevision < 1 {
			fileRows.Close()
			return snapshot, core.Integrity
		}
		content, readErr := readBlobBounded(k.root, binding.scope.company, entry.Digest, workspaceTreeMaxFileBytes)
		if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
			fileRows.Close()
			return snapshot, core.Integrity
		}
		total += entry.Bytes
		if total > workspaceTreeMaxTotalBytes {
			fileRows.Close()
			return snapshot, core.Integrity
		}
		entries[index] = entry
	}
	if fileRows.Next() {
		fileRows.Close()
		return snapshot, core.Integrity
	}
	if err = fileRows.Err(); err != nil {
		fileRows.Close()
		return snapshot, err
	}
	fileRows.Close()
	if !validWorkspaceTreeEntrySequence(entries) {
		return snapshot, core.Integrity
	}
	snapshot = WorkspaceTreeSnapshot{ArtifactID: artifactID, TaskID: sourceTask, RootBindingID: workspaceID, Revision: revision, Digest: digest, Bytes: byteCount, FileCount: len(entries), Verdict: verdict, Entries: entries}
	snapshot.ReadReference = newID()
	if err = appendEvent(ctx, tx, binding.scope, "worker.workspace.snapshot.read", map[string]any{
		"readReference": snapshot.ReadReference, "artifactId": artifactID, "sha256": digest, "sourceTaskId": sourceTask,
		"readerTaskId": binding.task, "workspaceId": workspaceID, "workspaceRevision": revision,
		"employeeId": binding.employee, "sessionId": binding.session, "epoch": binding.epoch,
	}); err != nil {
		return WorkspaceTreeSnapshot{}, err
	}
	return snapshot, tx.Commit(ctx)
}

func (k *Kernel) ReadProductWorkspaceSnapshotFile(ctx context.Context, binding Binding, artifactID, path string) (WorkspaceTreeRead, error) {
	var out WorkspaceTreeRead
	if !core.ValidID(artifactID) || !ValidWorkspaceRelativePath(path) {
		return out, core.Malformed
	}
	// Snapshot verification checks the immutable manifest and every file entry;
	// the exact-path read then returns the one selected regular UTF-8 CAS value.
	snapshot, err := k.ReadProductWorkspaceSnapshot(ctx, binding, artifactID)
	if err != nil {
		return out, err
	}
	for _, entry := range snapshot.Entries {
		if entry.RelativePath != path {
			continue
		}
		content, readErr := readBlobBounded(k.root, binding.scope.company, entry.Digest, workspaceTreeMaxFileBytes)
		if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
			return out, core.Integrity
		}
		read := WorkspaceTreeRead{RootBindingID: snapshot.RootBindingID, TaskID: snapshot.TaskID, RelativePath: path, Digest: entry.Digest,
			Bytes: entry.Bytes, FileRevision: entry.FileRevision, Revision: snapshot.Revision, ContentType: entry.ContentType, Content: string(content)}
		read.ReadReference, err = k.recordWorkspaceTreeRead(ctx, binding, "worker.workspace.snapshot.file.read", map[string]any{
			"artifactId": artifactID, "sourceTaskId": snapshot.TaskID, "workspaceId": snapshot.RootBindingID, "workspaceRevision": snapshot.Revision,
			"relativePath": path, "sha256": entry.Digest, "bytes": entry.Bytes, "fileRevision": entry.FileRevision,
		})
		if err != nil {
			return WorkspaceTreeRead{}, err
		}
		return read, nil
	}
	return out, core.OutOfScope
}

func workspaceTreePaths(entries []WorkspaceTreeEntry) []string {
	paths := make([]string, len(entries))
	for index, entry := range entries {
		paths[index] = entry.RelativePath
	}
	return paths
}

func validWorkspaceTreeEntrySequence(entries []WorkspaceTreeEntry) bool {
	for index, entry := range entries {
		if !ValidWorkspaceRelativePath(entry.RelativePath) {
			return false
		}
		if index == 0 {
			continue
		}
		previous := entries[index-1].RelativePath
		if previous >= entry.RelativePath || strings.HasPrefix(entry.RelativePath, previous+"/") {
			return false
		}
	}
	return true
}

func (k *Kernel) recordWorkspaceTreeRead(ctx context.Context, binding Binding, kind string, payload map[string]any) (string, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, binding.scope, &binding, false); err != nil {
		return "", err
	}
	taskID, err := k.requireProductTaskWorking(ctx, tx, binding)
	if err != nil || taskID != binding.task {
		if err == nil {
			err = core.StaleEpoch
		}
		return "", err
	}
	readReference := newID()
	payload["readReference"] = readReference
	payload["taskId"] = taskID
	payload["employeeId"] = binding.employee
	payload["sessionId"] = binding.session
	payload["epoch"] = binding.epoch
	if err = appendEvent(ctx, tx, binding.scope, kind, payload); err != nil {
		return "", err
	}
	return readReference, tx.Commit(ctx)
}
