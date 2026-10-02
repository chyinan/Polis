// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	maxCASCompanyEntries        = 4096
	maxCASCollectionPageEntries = 128
)

type CASCollectionReport struct {
	CompanyID       string   `json:"companyId"`
	AfterDigest     string   `json:"afterDigest,omitempty"`
	NextAfterDigest string   `json:"nextAfterDigest,omitempty"`
	Scanned         int      `json:"scanned"`
	Referenced      int      `json:"referenced"`
	Eligible        int      `json:"eligible"`
	Deleted         []string `json:"deleted"`
	DryRun          bool     `json:"dryRun"`
	Complete        bool     `json:"complete"`
}

// putBlobWithClaim protects CAS bytes written before their durable database
// owner is committed. The short-lived advisory lock coordinates the writer
// with collection; the durable claim covers a later owner transaction or a
// process crash between the file write and that transaction.
func (k *Kernel) putBlobWithClaim(ctx context.Context, companyID string, content []byte) (string, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || len(content) == 0 {
		return "", core.Malformed
	}
	digest := casDigest(content)
	unlock, err := k.lockCASLifecycle(ctx, companyID)
	if err != nil {
		return "", err
	}
	defer unlock()
	if err = k.registerCASWriteClaim(ctx, companyID, digest); err != nil {
		return "", err
	}
	storedDigest, err := putBlob(k.root, companyID, content)
	if err != nil {
		return "", err
	}
	if storedDigest != digest {
		return "", core.Integrity
	}
	return digest, nil
}

// putBlobInTX is used only by callbacks that already hold the Company row
// lock. The claim and database owner then commit atomically, while a collector
// must wait for the same lock before looking at the CAS directory.
func (k *Kernel) putBlobInTX(ctx context.Context, tx pgx.Tx, companyID string, content []byte) (string, error) {
	if k == nil || ctx == nil || tx == nil || !core.ValidID(companyID) || len(content) == 0 {
		return "", core.Malformed
	}
	digest := casDigest(content)
	if err := insertCASBlobWriteClaimTX(ctx, tx, companyID, digest); err != nil {
		return "", err
	}
	storedDigest, err := putBlob(k.root, companyID, content)
	if err != nil {
		return "", err
	}
	if storedDigest != digest {
		return "", core.Integrity
	}
	return digest, nil
}

func casDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func (k *Kernel) lockCASLifecycle(ctx context.Context, companyID string) (func(), error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || k.lockPool == nil {
		return nil, core.Malformed
	}
	connection, err := k.lockPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := capabilitySourceAdvisoryLockKey(companyID, "cas-blob-lifecycle")
	for {
		var locked bool
		if err = connection.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
			discardCapabilitySourceLockConnection(connection)
			return nil, err
		}
		if locked {
			break
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			connection.Release()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return func() { releaseCapabilitySourceLockKeys(connection, []int64{key}) }, nil
}

func (k *Kernel) registerCASWriteClaim(ctx context.Context, companyID, digest string) error {
	if k == nil || ctx == nil || !core.ValidID(companyID) || !validSHA256(digest) {
		return core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, Scope{company: companyID}, nil, false); err != nil {
		return err
	}
	if err = insertCASBlobWriteClaimTX(ctx, tx, companyID, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertCASBlobWriteClaimTX(ctx context.Context, tx pgx.Tx, companyID, digest string) error {
	if !core.ValidID(companyID) || !validSHA256(digest) {
		return core.Malformed
	}
	_, err := tx.Exec(ctx, `INSERT INTO cas_blob_write_claims(company_id,digest,expires_at)
VALUES($1,$2,clock_timestamp()+interval '24 hours')
ON CONFLICT(company_id,digest) DO UPDATE SET expires_at=GREATEST(cas_blob_write_claims.expires_at,EXCLUDED.expires_at),updated_at=clock_timestamp()`,
		companyID, digest)
	return err
}

// CollectOrphanCASBlobs reports or removes unreferenced company CAS objects.
// Collection is dry-run by default at the command layer. It serializes with
// CAS-first writers and all company-locked database writes, checks every
// company-owned public table for the digest, and fails closed on an unknown
// directory entry or any database/filesystem error.
func (k *Kernel) CollectOrphanCASBlobs(ctx context.Context, companyID, afterDigest string, limit int, apply bool) (CASCollectionReport, error) {
	report := CASCollectionReport{CompanyID: companyID, AfterDigest: afterDigest, Deleted: []string{}, DryRun: !apply, Complete: true}
	if k == nil || ctx == nil || !core.ValidID(companyID) || limit < 1 || limit > maxCASCollectionPageEntries ||
		(afterDigest != "" && !validSHA256(afterDigest)) {
		return CASCollectionReport{}, core.Malformed
	}
	unlock, err := k.lockCASLifecycle(ctx, companyID)
	if err != nil {
		return CASCollectionReport{}, err
	}
	defer unlock()
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return CASCollectionReport{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, Scope{company: companyID}, nil, false); err != nil {
		return CASCollectionReport{}, err
	}
	if apply {
		if _, err = tx.Exec(ctx, "DELETE FROM cas_blob_write_claims WHERE company_id=$1 AND expires_at<=clock_timestamp()", companyID); err != nil {
			return CASCollectionReport{}, err
		}
	}
	baseRoot, err := os.OpenRoot(k.root)
	if err != nil {
		return CASCollectionReport{}, err
	}
	companyInfo, err := baseRoot.Lstat(companyID)
	if errors.Is(err, os.ErrNotExist) {
		baseRoot.Close()
		if err = tx.Commit(ctx); err != nil {
			return CASCollectionReport{}, err
		}
		return report, nil
	}
	if err != nil {
		baseRoot.Close()
		return CASCollectionReport{}, err
	}
	if companyInfo.Mode()&os.ModeSymlink != 0 || !companyInfo.IsDir() {
		baseRoot.Close()
		return CASCollectionReport{}, core.Denied
	}
	root, err := baseRoot.OpenRoot(companyID)
	baseRoot.Close()
	if err != nil {
		return CASCollectionReport{}, err
	}
	openedInfo, err := root.Stat(".")
	if err != nil {
		root.Close()
		return CASCollectionReport{}, err
	}
	if !os.SameFile(companyInfo, openedInfo) {
		root.Close()
		return CASCollectionReport{}, core.Denied
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return CASCollectionReport{}, err
	}
	entries, readErr := directory.ReadDir(maxCASCompanyEntries + 1)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		directory.Close()
		return CASCollectionReport{}, readErr
	}
	if len(entries) > maxCASCompanyEntries {
		directory.Close()
		return CASCollectionReport{}, core.TooLarge
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	digests := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") {
			continue
		}
		if !validSHA256(name) {
			directory.Close()
			return CASCollectionReport{}, core.Integrity
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			directory.Close()
			return CASCollectionReport{}, infoErr
		}
		if !info.Mode().IsRegular() {
			directory.Close()
			return CASCollectionReport{}, core.Integrity
		}
		if name > afterDigest {
			digests = append(digests, name)
		}
	}
	if closeErr := directory.Close(); closeErr != nil {
		return CASCollectionReport{}, closeErr
	}
	pageDigests := digests
	if len(pageDigests) > limit {
		pageDigests = pageDigests[:limit]
		report.Complete = false
		report.NextAfterDigest = pageDigests[len(pageDigests)-1]
	}
	refs, err := casDigestsReferencedTX(ctx, tx, companyID, pageDigests)
	if err != nil {
		return CASCollectionReport{}, err
	}
	deletedAny := false
	for _, digest := range pageDigests {
		report.Scanned++
		if _, referenced := refs[digest]; referenced {
			report.Referenced++
			continue
		}
		report.Eligible++
		if apply {
			if err = root.Remove(digest); err != nil && !errors.Is(err, os.ErrNotExist) {
				return CASCollectionReport{}, err
			}
			report.Deleted = append(report.Deleted, digest)
			deletedAny = true
		}
	}
	if deletedAny {
		if err = syncParentDirectory(root); err != nil && (!parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported)) {
			return CASCollectionReport{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return CASCollectionReport{}, err
	}
	return report, nil
}

func casDigestsReferencedTX(ctx context.Context, tx pgx.Tx, companyID string, digests []string) (map[string]struct{}, error) {
	referenced := make(map[string]struct{}, len(digests))
	if len(digests) == 0 {
		return referenced, nil
	}
	for _, digest := range digests {
		if !validSHA256(digest) {
			return nil, core.Integrity
		}
	}
	rows, err := tx.Query(ctx, `SELECT c.relname
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND c.relkind IN ('r','p') AND c.relname!='cas_blob_write_claims'
AND EXISTS(SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=c.oid AND a.attname='company_id' AND NOT a.attisdropped)
ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	tables := make([]string, 0, 64)
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(tables) == 0 {
		return nil, core.Integrity
	}
	queries := make([]string, 0, len(tables))
	for _, table := range tables {
		queries = append(queries, fmt.Sprintf("SELECT m.vals[1] AS digest FROM %s AS owner CROSS JOIN LATERAL regexp_matches(to_jsonb(owner)::text,$2,'g') AS m(vals) WHERE owner.company_id=$1", pgx.Identifier{"public", table}.Sanitize()))
	}
	pattern := "(" + strings.Join(digests, "|") + ")"
	query := "SELECT DISTINCT digest FROM (" + strings.Join(queries, " UNION ALL ") + ") AS refs"
	rows, err = tx.Query(ctx, query, companyID, pattern)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var digest string
		if err = rows.Scan(&digest); err != nil {
			rows.Close()
			return nil, err
		}
		if validSHA256(digest) {
			referenced[digest] = struct{}{}
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT digest FROM cas_blob_write_claims
WHERE company_id=$1 AND expires_at>clock_timestamp() AND digest=ANY($2::text[])`, companyID, digests)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var digest string
		if err = rows.Scan(&digest); err != nil {
			rows.Close()
			return nil, err
		}
		referenced[digest] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	return referenced, nil
}
