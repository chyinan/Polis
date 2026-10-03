// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	SharedMissionArtifactPageSize       = 32
	maxSharedMissionArtifactReadHistory = 32
	sharedMissionArtifactContentLimit   = core.MaxContent
)

type SharedMissionArtifactSummary struct {
	ArtifactID   string `json:"artifactId"`
	TaskID       string `json:"taskId"`
	Author       string `json:"author"`
	Digest       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	Verdict      string `json:"verdict"`
	TextReadable bool   `json:"textReadable"`
}

type SharedMissionArtifactPage struct {
	MissionID         string                         `json:"missionId"`
	Artifacts         []SharedMissionArtifactSummary `json:"artifacts"`
	Truncated         bool                           `json:"truncated"`
	NextAfterArtifact string                         `json:"nextAfterArtifactId,omitempty"`
}

type SharedMissionArtifactRead struct {
	ReadReference   string `json:"readReference"`
	ArtifactID      string `json:"artifactId"`
	TaskID          string `json:"taskId"`
	Author          string `json:"author"`
	Digest          string `json:"sha256"`
	Bytes           int64  `json:"bytes"`
	Verdict         string `json:"verdict"`
	ContentBoundary string `json:"contentBoundary"`
	Content         string `json:"content"`
}

type SharedMissionArtifactReadUse struct {
	ReadReference string `json:"readReference"`
	ArtifactID    string `json:"artifactId"`
	SourceTaskID  string `json:"sourceTaskId"`
	ReaderTaskID  string `json:"readerTaskId"`
	Author        string `json:"author"`
	EmployeeID    string `json:"employeeId"`
	SessionID     string `json:"sessionId"`
	Digest        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	Verdict       string `json:"verdict"`
	ReadAt        string `json:"readAt,omitempty"`
}

// ListSharedMissionArtifacts derives the requester scope from the persisted
// provider WorkerSession. It exposes only published Artifacts whose content is
// pinned by a digest, from another Task in the same active Mission.
func (k *Kernel) ListSharedMissionArtifacts(ctx context.Context, binding Binding, afterArtifactID string) (SharedMissionArtifactPage, error) {
	if afterArtifactID != "" && !core.ValidID(afterArtifactID) {
		return SharedMissionArtifactPage{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return SharedMissionArtifactPage{}, err
	}
	defer tx.Rollback(ctx)
	missionID, taskID, err := k.sharedMissionArtifactScope(ctx, tx, binding)
	if err != nil {
		return SharedMissionArtifactPage{}, err
	}
	rows, err := tx.Query(ctx, `SELECT a.id,a.task_id,a.author,a.digest,a.bytes,a.verdict
FROM artifacts a
JOIN tasks source_task ON source_task.company_id=a.company_id AND source_task.id=a.task_id
WHERE a.company_id=$1 AND source_task.mission_id=$2 AND source_task.id<>$3 AND a.id>$4
  AND a.author=source_task.owner AND a.state='ready' AND a.verdict IN ('candidate','passed')
ORDER BY a.id LIMIT $5`, binding.scope.company, missionID, taskID, afterArtifactID, SharedMissionArtifactPageSize+1)
	if err != nil {
		return SharedMissionArtifactPage{}, err
	}
	items := make([]SharedMissionArtifactSummary, 0, SharedMissionArtifactPageSize+1)
	for rows.Next() {
		var item SharedMissionArtifactSummary
		if err = rows.Scan(&item.ArtifactID, &item.TaskID, &item.Author, &item.Digest, &item.Bytes, &item.Verdict); err != nil {
			rows.Close()
			return SharedMissionArtifactPage{}, err
		}
		if !core.ValidID(item.ArtifactID) || !core.ValidID(item.TaskID) || !core.ValidID(item.Author) ||
			!validTaskInputDigest(item.Digest) || item.Bytes < 1 || item.Bytes > sharedMissionArtifactContentLimit ||
			(item.Verdict != "candidate" && item.Verdict != "passed") {
			rows.Close()
			return SharedMissionArtifactPage{}, core.Integrity
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return SharedMissionArtifactPage{}, err
	}
	page := SharedMissionArtifactPage{MissionID: missionID, Artifacts: items}
	if len(items) > SharedMissionArtifactPageSize {
		page.Truncated = true
		page.Artifacts = items[:SharedMissionArtifactPageSize]
		page.NextAfterArtifact = page.Artifacts[len(page.Artifacts)-1].ArtifactID
	}
	for index, item := range page.Artifacts {
		content, readErr := readBlobBounded(k.root, binding.scope.company, item.Digest, sharedMissionArtifactContentLimit)
		if readErr != nil || int64(len(content)) != item.Bytes {
			return SharedMissionArtifactPage{}, core.Integrity
		}
		page.Artifacts[index].TextReadable = utf8.Valid(content)
	}
	if err = tx.Commit(ctx); err != nil {
		return SharedMissionArtifactPage{}, err
	}
	return page, nil
}

// TXReadSharedMissionArtifact returns a published same-Mission Artifact only
// after checking the current WorkerSession and re-reading verified CAS bytes.
// The append-only event records the exact immutable version that was read.
func (k *Kernel) TXReadSharedMissionArtifact(ctx context.Context, binding Binding, key, artifactID string) (Receipt, SharedMissionArtifactRead, error) {
	if !core.ValidID(artifactID) || len(key) == 0 || len(key) > 256 {
		return Receipt{}, SharedMissionArtifactRead{}, core.Malformed
	}
	var document SharedMissionArtifactRead
	receipt, err := k.TXWrite(ctx, binding.scope, &binding, key, "task.shared_artifact.read", struct{ ArtifactID string }{artifactID}, func(tx pgx.Tx) (Receipt, error) {
		var readErr error
		document, readErr = k.readSharedMissionArtifact(ctx, tx, binding, artifactID)
		if readErr != nil {
			return Receipt{}, readErr
		}
		reference := newID()
		document.ReadReference = reference
		use := SharedMissionArtifactReadUse{
			ReadReference: reference, ArtifactID: document.ArtifactID, SourceTaskID: document.TaskID, ReaderTaskID: binding.task,
			Author: document.Author, EmployeeID: binding.employee, SessionID: binding.session, Digest: document.Digest,
			Bytes: document.Bytes, Verdict: document.Verdict,
		}
		if readErr = appendEvent(ctx, tx, binding.scope, "worker.shared_artifact.read", use); readErr != nil {
			return Receipt{}, readErr
		}
		return Receipt{ID: reference, Status: "read"}, nil
	})
	if err != nil {
		return Receipt{}, SharedMissionArtifactRead{}, err
	}
	if document.ArtifactID == "" {
		// A replay must re-check the active session and current Artifact grant
		// before returning previously requested content.
		tx, beginErr := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if beginErr != nil {
			return Receipt{}, SharedMissionArtifactRead{}, beginErr
		}
		defer tx.Rollback(ctx)
		if beginErr = k.guard(ctx, tx, binding.scope, &binding); beginErr != nil {
			return Receipt{}, SharedMissionArtifactRead{}, beginErr
		}
		document, beginErr = k.readSharedMissionArtifact(ctx, tx, binding, artifactID)
		if beginErr != nil {
			return Receipt{}, SharedMissionArtifactRead{}, beginErr
		}
		if beginErr = tx.Commit(ctx); beginErr != nil {
			return Receipt{}, SharedMissionArtifactRead{}, beginErr
		}
		document.ReadReference = receipt.ID
	}
	return receipt, document, nil
}

func (k *Kernel) readSharedMissionArtifact(ctx context.Context, tx pgx.Tx, binding Binding, artifactID string) (SharedMissionArtifactRead, error) {
	var document SharedMissionArtifactRead
	missionID, taskID, err := k.sharedMissionArtifactScope(ctx, tx, binding)
	if err != nil {
		return document, err
	}
	var contentDigest string
	err = tx.QueryRow(ctx, `SELECT a.id,a.task_id,a.author,a.digest,a.bytes,a.verdict
FROM artifacts a
JOIN tasks source_task ON source_task.company_id=a.company_id AND source_task.id=a.task_id
WHERE a.company_id=$1 AND source_task.mission_id=$2 AND source_task.id<>$3 AND a.id=$4
  AND a.author=source_task.owner AND a.state='ready' AND a.verdict IN ('candidate','passed')`,
		binding.scope.company, missionID, taskID, artifactID).Scan(&document.ArtifactID, &document.TaskID, &document.Author, &contentDigest, &document.Bytes, &document.Verdict)
	if errors.Is(err, pgx.ErrNoRows) {
		return SharedMissionArtifactRead{}, core.OutOfScope
	}
	if err != nil {
		return SharedMissionArtifactRead{}, err
	}
	if !core.ValidID(document.ArtifactID) || !core.ValidID(document.TaskID) || !core.ValidID(document.Author) ||
		!validTaskInputDigest(contentDigest) || document.Bytes < 1 || document.Bytes > sharedMissionArtifactContentLimit ||
		(document.Verdict != "candidate" && document.Verdict != "passed") {
		return SharedMissionArtifactRead{}, core.Integrity
	}
	content, err := readBlobBounded(k.root, binding.scope.company, contentDigest, sharedMissionArtifactContentLimit)
	if err != nil || int64(len(content)) != document.Bytes {
		return SharedMissionArtifactRead{}, core.Integrity
	}
	if !utf8.Valid(content) {
		return SharedMissionArtifactRead{}, core.Denied
	}
	document.Digest = contentDigest
	document.ContentBoundary = "untrusted_shared_mission_artifact_utf8"
	document.Content = string(content)
	return document, nil
}

func (k *Kernel) sharedMissionArtifactScope(ctx context.Context, tx pgx.Tx, binding Binding) (string, string, error) {
	if _, err := k.checkSession(ctx, tx, binding, false); err != nil {
		return "", "", err
	}
	taskID, err := k.requireProductTaskWorking(ctx, tx, binding)
	if err != nil {
		return "", "", err
	}
	if taskID != binding.task {
		return "", "", core.StaleEpoch
	}
	var missionID, owner string
	err = tx.QueryRow(ctx, `SELECT mission_id,owner FROM tasks WHERE company_id=$1 AND id=$2`, binding.scope.company, taskID).Scan(&missionID, &owner)
	if err != nil {
		return "", "", err
	}
	if owner != binding.employee || !core.ValidID(missionID) {
		return "", "", core.Denied
	}
	return missionID, taskID, nil
}

func (k *Kernel) SharedMissionArtifactReadHistory(ctx context.Context, binding Binding) ([]SharedMissionArtifactReadUse, bool, error) {
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, `SELECT payload->>'readReference',payload->>'artifactId',payload->>'sourceTaskId',payload->>'readerTaskId',payload->>'author',payload->>'employeeId',payload->>'sessionId',payload->>'sha256',(payload->>'bytes')::bigint,payload->>'verdict',payload->>'occurred_at'
FROM events WHERE company_id=$1 AND kind='worker.shared_artifact.read' AND payload->>'readerTaskId'=$2
ORDER BY company_seq DESC LIMIT $3`, binding.scope.company, binding.task, maxSharedMissionArtifactReadHistory+1)
	if err != nil {
		return nil, false, err
	}
	items := make([]SharedMissionArtifactReadUse, 0, maxSharedMissionArtifactReadHistory+1)
	for rows.Next() {
		var item SharedMissionArtifactReadUse
		if err = rows.Scan(&item.ReadReference, &item.ArtifactID, &item.SourceTaskID, &item.ReaderTaskID, &item.Author, &item.EmployeeID, &item.SessionID, &item.Digest, &item.Bytes, &item.Verdict, &item.ReadAt); err != nil {
			rows.Close()
			return nil, false, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > maxSharedMissionArtifactReadHistory
	if truncated {
		items = items[:maxSharedMissionArtifactReadHistory]
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return items, truncated, nil
}
