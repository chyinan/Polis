// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	memoryRevocationOverlaySchema = "polis-memory-revocation@1"
	maxMemoryRevocationCompanies  = 4096
	maxMemoryRevocationRecords    = 100000
	maxMemoryRevocationPage       = 256
	maxMemoryRevocationBytes      = 4096
)

// MemoryRecordRevocationOverlay contains no memory text or user-supplied
// rationale. It lives outside recovery generations and is never included in
// a database/CAS backup, so restoring an older generation cannot clear it.
type MemoryRecordRevocationOverlay struct {
	SchemaVersion  string `json:"schemaVersion"`
	CompanyID      string `json:"companyId"`
	RecordID       string `json:"recordId"`
	OperationID    string `json:"operationId"`
	ContentSHA256  string `json:"contentSha256"`
	SourceRevision int64  `json:"sourceRevision"`
	ReasonCode     string `json:"reasonCode"`
	CreatedAt      string `json:"createdAt"`
}

type MemoryRecordRevocationPreview struct {
	CompanyID                string `json:"companyId"`
	RecordID                 string `json:"recordId"`
	Kind                     string `json:"kind"`
	Scope                    string `json:"scope"`
	Sensitivity              string `json:"sensitivity"`
	LatestRevision           int64  `json:"latestRevision"`
	ContentSHA256            string `json:"contentSha256"`
	State                    string `json:"state"`
	DependencyCount          int64  `json:"dependencyCount"`
	ActiveWorkerSessionCount int64  `json:"activeWorkerSessionCount"`
	Revoked                  bool   `json:"revoked"`
	RevocationOperationID    string `json:"revocationOperationId,omitempty"`
	RevocationReasonCode     string `json:"revocationReasonCode,omitempty"`
	RevokedAt                string `json:"revokedAt,omitempty"`
}

func (k *Kernel) GetMemoryRecordRevocationPreview(ctx context.Context, scope Scope, recordID string) (MemoryRecordRevocationPreview, error) {
	preview := MemoryRecordRevocationPreview{CompanyID: scope.company, RecordID: recordID}
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(recordID) {
		return MemoryRecordRevocationPreview{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	overlay, hasOverlay := k.memoryRecordRevocation(scope.company, recordID)
	err = tx.QueryRow(ctx, `SELECT r.record_kind,r.scope_kind,r.sensitivity,v.revision,v.content_sha256,s.state
FROM memory_records r JOIN LATERAL (SELECT revision,content_sha256 FROM memory_record_revisions
 WHERE company_id=r.company_id AND record_id=r.record_id ORDER BY revision DESC LIMIT 1) v ON true
JOIN LATERAL (SELECT state FROM memory_revision_state_events e WHERE e.company_id=r.company_id AND e.record_id=r.record_id AND e.revision=v.revision
 ORDER BY e.company_seq DESC LIMIT 1) s ON true
WHERE r.company_id=$1 AND r.record_id=$2`, scope.company, recordID).Scan(
		&preview.Kind, &preview.Scope, &preview.Sensitivity, &preview.LatestRevision, &preview.ContentSHA256, &preview.State)
	if errors.Is(err, pgx.ErrNoRows) {
		if !hasOverlay {
			return MemoryRecordRevocationPreview{}, core.OutOfScope
		}
		preview.LatestRevision, preview.ContentSHA256, preview.Revoked = overlay.SourceRevision, overlay.ContentSHA256, true
		preview.RevocationOperationID, preview.RevocationReasonCode, preview.RevokedAt = overlay.OperationID, overlay.ReasonCode, overlay.CreatedAt
	} else if err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	if hasOverlay {
		preview.Revoked = true
		preview.RevocationOperationID, preview.RevocationReasonCode, preview.RevokedAt = overlay.OperationID, overlay.ReasonCode, overlay.CreatedAt
	}
	if !preview.Revoked {
		if err = tx.QueryRow(ctx, `SELECT operation_id,reason_code,created_at::text FROM memory_record_revocation_overlays
WHERE company_id=$1 AND record_id=$2`, scope.company, recordID).Scan(
			&preview.RevocationOperationID, &preview.RevocationReasonCode, &preview.RevokedAt); err == nil {
			preview.Revoked = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return MemoryRecordRevocationPreview{}, err
		}
	}
	if preview.Revoked {
		preview.State = "revoked"
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM memory_dependencies WHERE company_id=$1 AND record_id=$2`, scope.company, recordID).Scan(&preview.DependencyCount); err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(DISTINCT s.id) FROM memory_dependencies d
JOIN worker_sessions s ON s.company_id=d.company_id AND s.task_id=d.bound_task_id
WHERE d.company_id=$1 AND d.record_id=$2 AND s.state!='stopped'`, scope.company, recordID).Scan(&preview.ActiveWorkerSessionCount); err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemoryRecordRevocationPreview{}, err
	}
	return preview, nil
}

func (k *Kernel) initializeMemoryRevocationOverlays() error {
	if k == nil || k.root == "" {
		return core.Malformed
	}
	configuredRoot := strings.TrimSpace(os.Getenv("POLIS_MEMORY_REVOCATION_ROOT"))
	if configuredRoot == "" {
		configuredRoot = filepath.Join(filepath.Dir(k.root), "memory-revocations")
	}
	overlayRoot, err := filepath.Abs(configuredRoot)
	if err != nil {
		return err
	}
	createErr := os.Mkdir(overlayRoot, 0700)
	if createErr != nil && !errors.Is(createErr, os.ErrExist) {
		return createErr
	}
	info, err := os.Lstat(overlayRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return core.Denied
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return core.Denied
	}
	canonicalOverlay, err := filepath.EvalSymlinks(overlayRoot)
	if err != nil {
		return err
	}
	canonicalCAS, err := filepath.EvalSymlinks(k.root)
	if err != nil {
		return err
	}
	if pathsOverlap(canonicalOverlay, canonicalCAS) {
		return fmt.Errorf("%w: memory revocation overlay root must be separate from CAS", core.Denied)
	}
	if createErr == nil {
		parentRoot, openErr := os.OpenRoot(filepath.Dir(canonicalOverlay))
		if openErr != nil {
			return openErr
		}
		syncErr := syncParentDirectory(parentRoot)
		closeErr := parentRoot.Close()
		if syncErr != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(syncErr, ErrParentDirectorySyncUnsupported)) {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	k.memoryRevocationRoot = canonicalOverlay
	k.memoryRevocations = make(map[string]MemoryRecordRevocationOverlay)
	return k.loadMemoryRevocationOverlayFiles()
}

func pathsOverlap(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)))
	}
	return within(left, right) || within(right, left)
}

func (k *Kernel) loadMemoryRevocationOverlayFiles() error {
	root, err := os.OpenRoot(k.memoryRevocationRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	companies, readErr := directory.ReadDir(maxMemoryRevocationCompanies + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(companies) > maxMemoryRevocationCompanies {
		return core.TooLarge
	}
	loadedRecords := 0
	for _, companyEntry := range companies {
		companyID := companyEntry.Name()
		if !core.ValidID(companyID) {
			return core.Integrity
		}
		companyInfo, infoErr := companyEntry.Info()
		if infoErr != nil {
			return infoErr
		}
		if companyInfo.Mode()&os.ModeSymlink != 0 || !companyInfo.IsDir() {
			return core.Integrity
		}
		if err = validateMemoryRevocationDirectoryPermissions(companyInfo); err != nil {
			return err
		}
		companyRoot, openErr := root.OpenRoot(companyID)
		if openErr != nil {
			return openErr
		}
		companyDirectory, openErr := companyRoot.Open(".")
		if openErr != nil {
			companyRoot.Close()
			return openErr
		}
		records, recordErr := companyDirectory.ReadDir(maxMemoryRevocationRecords + 1)
		recordCloseErr := companyDirectory.Close()
		if recordErr != nil && !errors.Is(recordErr, io.EOF) {
			companyRoot.Close()
			return recordErr
		}
		if recordCloseErr != nil {
			companyRoot.Close()
			return recordCloseErr
		}
		if len(records) > maxMemoryRevocationRecords {
			companyRoot.Close()
			return core.TooLarge
		}
		if len(records) > maxMemoryRevocationRecords-loadedRecords {
			companyRoot.Close()
			return core.TooLarge
		}
		for _, recordEntry := range records {
			recordID := recordEntry.Name()
			if strings.HasPrefix(recordID, ".stage-") && core.ValidID(strings.TrimPrefix(recordID, ".stage-")) {
				continue
			}
			if !core.ValidID(recordID) {
				companyRoot.Close()
				return core.Integrity
			}
			recordInfo, statErr := recordEntry.Info()
			if statErr != nil {
				companyRoot.Close()
				return statErr
			}
			if recordInfo.Mode()&os.ModeSymlink != 0 || !recordInfo.Mode().IsRegular() || recordInfo.Size() < 0 || recordInfo.Size() > maxMemoryRevocationBytes {
				companyRoot.Close()
				return core.Integrity
			}
			if runtime.GOOS != "windows" && recordInfo.Mode().Perm()&0077 != 0 {
				companyRoot.Close()
				return core.Denied
			}
			raw, fileErr := companyRoot.ReadFile(recordID)
			if fileErr != nil {
				companyRoot.Close()
				return fileErr
			}
			var overlay MemoryRecordRevocationOverlay
			if err = json.Unmarshal(raw, &overlay); err != nil || !validMemoryRevocationOverlay(overlay) || overlay.CompanyID != companyID || overlay.RecordID != recordID {
				companyRoot.Close()
				return core.Integrity
			}
			k.memoryRevocations[memoryRevocationKey(companyID, recordID)] = overlay
			loadedRecords++
		}
		if err = companyRoot.Close(); err != nil {
			return err
		}
	}
	return nil
}

func validMemoryRevocationOverlay(overlay MemoryRecordRevocationOverlay) bool {
	if overlay.SchemaVersion != memoryRevocationOverlaySchema || !core.ValidID(overlay.CompanyID) || !core.ValidID(overlay.RecordID) ||
		!core.ValidID(overlay.OperationID) || !validSHA256(overlay.ContentSHA256) || overlay.SourceRevision < 1 ||
		(overlay.ReasonCode != "incorrect" && overlay.ReasonCode != "sensitive" && overlay.ReasonCode != "requested" && overlay.ReasonCode != "other") {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, overlay.CreatedAt)
	return err == nil
}

func validateMemoryRevocationDirectoryPermissions(info os.FileInfo) error {
	if info == nil {
		return core.Integrity
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return core.Denied
	}
	return nil
}

func memoryRevocationKey(companyID, recordID string) string { return companyID + "\x00" + recordID }

func (k *Kernel) memoryRecordRevocation(companyID, recordID string) (MemoryRecordRevocationOverlay, bool) {
	if k == nil {
		return MemoryRecordRevocationOverlay{}, false
	}
	k.memoryRevocationMu.RLock()
	defer k.memoryRevocationMu.RUnlock()
	overlay, ok := k.memoryRevocations[memoryRevocationKey(companyID, recordID)]
	return overlay, ok
}

func (k *Kernel) memoryRecordRevokedTX(ctx context.Context, tx pgx.Tx, companyID, recordID string) (bool, error) {
	if _, ok := k.memoryRecordRevocation(companyID, recordID); ok {
		return true, nil
	}
	var revoked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_record_revocation_overlays WHERE company_id=$1 AND record_id=$2)`, companyID, recordID).Scan(&revoked)
	return revoked, err
}

func (k *Kernel) memoryRevocationSnapshot() []MemoryRecordRevocationOverlay {
	k.memoryRevocationMu.RLock()
	defer k.memoryRevocationMu.RUnlock()
	items := make([]MemoryRecordRevocationOverlay, 0, len(k.memoryRevocations))
	for _, overlay := range k.memoryRevocations {
		items = append(items, overlay)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CompanyID != items[j].CompanyID {
			return items[i].CompanyID < items[j].CompanyID
		}
		return items[i].RecordID < items[j].RecordID
	})
	return items
}

func (k *Kernel) persistMemoryRevocationOverlay(overlay MemoryRecordRevocationOverlay) error {
	if k == nil || !validMemoryRevocationOverlay(overlay) {
		return core.Malformed
	}
	key := memoryRevocationKey(overlay.CompanyID, overlay.RecordID)
	k.memoryRevocationMu.Lock()
	defer k.memoryRevocationMu.Unlock()
	if current, ok := k.memoryRevocations[key]; ok {
		if sameMemoryRevocation(current, overlay) {
			return nil
		}
		return core.Conflict
	}
	root, err := os.OpenRoot(k.memoryRevocationRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	mkdirErr := root.Mkdir(overlay.CompanyID, 0700)
	if mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
		return mkdirErr
	}
	if mkdirErr == nil {
		if err = syncParentDirectory(root); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
			return err
		}
	}
	companyInfo, err := root.Lstat(overlay.CompanyID)
	if err != nil {
		return err
	}
	if companyInfo.Mode()&os.ModeSymlink != 0 || !companyInfo.IsDir() {
		return core.Denied
	}
	if err = validateMemoryRevocationDirectoryPermissions(companyInfo); err != nil {
		return err
	}
	companyRoot, err := root.OpenRoot(overlay.CompanyID)
	if err != nil {
		return err
	}
	defer companyRoot.Close()
	if existing, readErr := companyRoot.ReadFile(overlay.RecordID); readErr == nil {
		var prior MemoryRecordRevocationOverlay
		if json.Unmarshal(existing, &prior) != nil || !validMemoryRevocationOverlay(prior) || prior.CompanyID != overlay.CompanyID || prior.RecordID != overlay.RecordID {
			return core.Integrity
		}
		if sameMemoryRevocation(prior, overlay) {
			if err = syncParentDirectory(companyRoot); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
				return err
			}
			k.memoryRevocations[key] = prior
			return nil
		}
		return core.Conflict
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	raw, err := json.Marshal(overlay)
	if err != nil {
		return err
	}
	stage := ".stage-" + newID()
	file, err := companyRoot.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer companyRoot.Remove(stage)
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = companyRoot.Chmod(stage, 0400); err != nil {
		return err
	}
	if err = companyRoot.Link(stage, overlay.RecordID); err != nil {
		if errors.Is(err, os.ErrExist) {
			if existing, readErr := companyRoot.ReadFile(overlay.RecordID); readErr == nil {
				var prior MemoryRecordRevocationOverlay
				if json.Unmarshal(existing, &prior) == nil && validMemoryRevocationOverlay(prior) && sameMemoryRevocation(prior, overlay) {
					if err = syncParentDirectory(companyRoot); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
						return err
					}
					k.memoryRevocations[key] = prior
					return nil
				}
			}
		}
		return err
	}
	removeErr := companyRoot.Remove(stage)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		// The linked final name is authoritative now; still flush it before
		// returning so a failed SQL commit cannot leave a memory-only barrier.
		if err = syncParentDirectory(companyRoot); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
			return err
		}
		k.memoryRevocations[key] = overlay
		return removeErr
	}
	if err = syncParentDirectory(companyRoot); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
		return err
	}
	// Do not let the in-memory retry fast path report success until the final
	// directory entry has passed the durability check above.
	k.memoryRevocations[key] = overlay
	return nil
}

func sameMemoryRevocation(left, right MemoryRecordRevocationOverlay) bool {
	return left.CompanyID == right.CompanyID && left.RecordID == right.RecordID && left.OperationID == right.OperationID &&
		left.ContentSHA256 == right.ContentSHA256 && left.SourceRevision == right.SourceRevision && left.ReasonCode == right.ReasonCode
}

func (k *Kernel) reconcileMemoryRevocationOverlays(ctx context.Context) error {
	items := k.memoryRevocationSnapshot()
	for start := 0; start < len(items); {
		companyID := items[start].CompanyID
		end := start
		for end < len(items) && items[end].CompanyID == companyID && end-start < maxMemoryRevocationPage {
			end++
		}
		tx, err := k.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err = k.guardWithSessionMode(ctx, tx, Scope{company: companyID}, nil, false); errors.Is(err, core.OutOfScope) {
			_ = tx.Rollback(ctx)
			start = end
			continue
		} else if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		for _, overlay := range items[start:end] {
			if err = k.applyMemoryRevocationOverlayTX(ctx, tx, overlay, true); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (k *Kernel) applyMemoryRevocationOverlayTX(ctx context.Context, tx pgx.Tx, overlay MemoryRecordRevocationOverlay, emitOverlayEvent bool) error {
	tag, err := tx.Exec(ctx, `INSERT INTO memory_record_revocation_overlays(company_id,record_id,operation_id,content_sha256,source_revision,reason_code,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(company_id,record_id) DO NOTHING`, overlay.CompanyID, overlay.RecordID,
		overlay.OperationID, overlay.ContentSHA256, overlay.SourceRevision, overlay.ReasonCode, overlay.CreatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var operationID, contentSHA, reasonCode string
		var sourceRevision int64
		if err = tx.QueryRow(ctx, `SELECT operation_id,content_sha256,source_revision,reason_code FROM memory_record_revocation_overlays
WHERE company_id=$1 AND record_id=$2`, overlay.CompanyID, overlay.RecordID).Scan(&operationID, &contentSHA, &sourceRevision, &reasonCode); err != nil {
			return err
		}
		if operationID != overlay.OperationID || contentSHA != overlay.ContentSHA256 || sourceRevision != overlay.SourceRevision || reasonCode != overlay.ReasonCode {
			return core.Integrity
		}
		return nil
	}
	if emitOverlayEvent {
		if err = appendEvent(ctx, tx, Scope{company: overlay.CompanyID}, "memory.record.revocation_overlay_applied", map[string]any{
			"record_id": overlay.RecordID, "operation_id": overlay.OperationID, "content_sha256": overlay.ContentSHA256,
			"source_revision": overlay.SourceRevision, "reason_code": overlay.ReasonCode,
		}); err != nil {
			return err
		}
	}
	type dependency struct {
		id, taskID, risk string
		revision         int64
	}
	rows, err := tx.Query(ctx, `SELECT dependency_id,record_revision,COALESCE(bound_task_id,''),risk_level
FROM memory_dependencies WHERE company_id=$1 AND record_id=$2 ORDER BY dependency_id`, overlay.CompanyID, overlay.RecordID)
	if err != nil {
		return err
	}
	dependencies := make([]dependency, 0)
	for rows.Next() {
		var item dependency
		if err = rows.Scan(&item.id, &item.revision, &item.taskID, &item.risk); err != nil {
			rows.Close()
			return err
		}
		dependencies = append(dependencies, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range dependencies {
		reason := "memory record revoked; establish a new dependency before resuming work"
		if err = appendEvent(ctx, tx, Scope{company: overlay.CompanyID}, "memory.dependency.frozen", map[string]any{
			"dependency_id": item.id, "record_id": overlay.RecordID, "record_revision": item.revision,
			"operation_id": overlay.OperationID, "reason": reason,
		}); err != nil {
			return err
		}
		seq, seqErr := currentCompanySequenceTX(ctx, tx, overlay.CompanyID)
		if seqErr != nil {
			return seqErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_dependency_invalidation_events(company_id,company_seq,dependency_id,state,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,'frozen','source_revoked',$4,'local-owner',$5)`, overlay.CompanyID, seq, item.id, overlay.OperationID, reason); err != nil {
			return err
		}
		if item.taskID == "" {
			continue
		}
		if err = appendEvent(ctx, tx, Scope{company: overlay.CompanyID}, "memory.task.frozen", map[string]any{
			"task_id": item.taskID, "dependency_id": item.id, "record_id": overlay.RecordID,
			"record_revision": item.revision, "operation_id": overlay.OperationID, "risk_level": item.risk, "reason": reason,
		}); err != nil {
			return err
		}
		seq, seqErr = currentCompanySequenceTX(ctx, tx, overlay.CompanyID)
		if seqErr != nil {
			return seqErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_task_state_events(company_id,company_seq,task_id,dependency_id,state,risk_level,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,$4,'frozen',$5,'source_revoked',$6,'local-owner',$7)`, overlay.CompanyID, seq, item.taskID, item.id, item.risk, overlay.OperationID, reason); err != nil {
			return err
		}
	}
	return nil
}
