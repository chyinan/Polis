// pattern: Imperative Shell
package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationAdvisoryLockKey int64 = 714209832
const migrationExecutionEvidenceSchemaVersion int64 = 40

var gooseMigrationProcessMu sync.Mutex

type MigrationExecutionEvidence struct {
	VersionID         int64     `json:"versionId"`
	MigrationSHA256   string    `json:"migrationSha256"`
	RecordedByBuildID string    `json:"recordedByBuildId"`
	ExecutionResult   string    `json:"executionResult"`
	RecordedAt        time.Time `json:"recordedAt"`
}

type migrationSource struct {
	sha256 string
}

// A pair of historical SQL migrations omitted Goose's Up marker. Normalize
// only the stream consumed by Goose; the checked-in migration bytes and their
// manifest digests stay unchanged for migration execution evidence.
var legacyGooseUpMarkerFiles = map[string]struct{}{
	"migrations/00078_memory_cas_retention_pins.sql": {},
	"migrations/00079_cas_blob_write_claims.sql":     {},
}

type gooseMigrationSourceFS struct{ source fs.FS }

func (source gooseMigrationSourceFS) Open(name string) (fs.File, error) {
	file, err := source.source.Open(name)
	if err != nil {
		return nil, err
	}
	if _, needsMarker := legacyGooseUpMarkerFiles[name]; !needsMarker {
		return file, nil
	}
	content, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	return &gooseMigrationFile{reader: bytes.NewReader(append([]byte("-- +goose Up\n"), content...)), info: info}, nil
}

type gooseMigrationFile struct {
	reader *bytes.Reader
	info   fs.FileInfo
}

func (file *gooseMigrationFile) Read(buffer []byte) (int, error) { return file.reader.Read(buffer) }
func (file *gooseMigrationFile) Close() error                    { return nil }
func (file *gooseMigrationFile) Stat() (fs.FileInfo, error)      { return file.info, nil }

func MigrateToVersion(ctx context.Context, dsn string, targetVersion int64) error {
	gooseMigrationProcessMu.Lock()
	defer gooseMigrationProcessMu.Unlock()
	if targetVersion < 0 {
		return fmt.Errorf("migration target version must be nonnegative")
	}
	if err := verifyMigrationManifest(migrations, migrationHashManifest); err != nil {
		return fmt.Errorf("refusing migrations because the embedded SQL set failed its hash manifest: %w", err)
	}
	config, err := parseMigrationDSN(dsn)
	if err != nil {
		return err
	}
	database := stdlib.OpenDB(*config)
	defer database.Close()

	lockConn, err := database.Conn(ctx)
	if err != nil {
		return err
	}
	defer lockConn.Close()
	if err = executeSessionAdvisoryLock(ctx, lockConn, "SELECT pg_advisory_lock($1)"); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_ = executeSessionAdvisoryLock(context.Background(), lockConn, "SELECT pg_advisory_unlock($1)")
	}()

	goose.SetBaseFS(gooseMigrationSourceFS{source: migrations})
	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	sources, err := migrationSourceCatalog(migrations)
	if err != nil {
		return err
	}
	beforeVersions, err := appliedMigrationVersions(ctx, database)
	if err != nil {
		return err
	}
	evidenceTableExistedBefore, err := databaseHasRelation(ctx, database, "migration_execution_evidence")
	if err != nil {
		return err
	}
	if len(beforeVersions) > 0 && beforeVersions[len(beforeVersions)-1] >= migrationExecutionEvidenceSchemaVersion && !evidenceTableExistedBefore {
		return fmt.Errorf("schema %d is missing the required migration execution evidence table", beforeVersions[len(beforeVersions)-1])
	}
	if evidenceTableExistedBefore {
		if err = verifyRecordedMigrationEvidence(ctx, database, sources, beforeVersions); err != nil {
			return fmt.Errorf("refusing to continue because recorded migration evidence does not match this build: %w", err)
		}
	}
	if targetVersion > 0 {
		if _, ok := sources[targetVersion]; !ok {
			return fmt.Errorf("migration target version %d is not embedded", targetVersion)
		}
	}
	if len(beforeVersions) > 0 && targetVersion > 0 && beforeVersions[len(beforeVersions)-1] > targetVersion {
		return fmt.Errorf("refusing migration downgrade from version %d to %d", beforeVersions[len(beforeVersions)-1], targetVersion)
	}
	if targetVersion > 0 {
		if err = goose.UpToContext(ctx, database, "migrations", targetVersion); err != nil {
			return err
		}
	} else if err = goose.UpContext(ctx, database, "migrations"); err != nil {
		return err
	}
	afterVersions, err := appliedMigrationVersions(ctx, database)
	if err != nil {
		return err
	}
	if targetVersion > 0 && len(afterVersions) > 0 && afterVersions[len(afterVersions)-1] > targetVersion {
		return fmt.Errorf("database is already at version %d, above requested migration target %d", afterVersions[len(afterVersions)-1], targetVersion)
	}
	evidenceTableExistsAfter, err := databaseHasRelation(ctx, database, "migration_execution_evidence")
	if err != nil {
		return err
	}
	if len(afterVersions) > 0 && afterVersions[len(afterVersions)-1] >= migrationExecutionEvidenceSchemaVersion && !evidenceTableExistsAfter {
		return fmt.Errorf("schema %d is missing the required migration execution evidence table after Goose completed", afterVersions[len(afterVersions)-1])
	}
	if !evidenceTableExistsAfter {
		return nil
	}
	return recordMigrationExecutionEvidence(ctx, database, sources, beforeVersions, afterVersions)
}

func parseMigrationDSN(dsn string) (*pgx.ConnConfig, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cfg.Database, "polis_r0_") {
		return nil, fmt.Errorf("refusing non-R0 database %q", cfg.Database)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = make(map[string]string)
	}
	cfg.RuntimeParams["search_path"] = "public"
	return cfg, nil
}

func migrationSourceCatalog(files fs.FS) (map[int64]migrationSource, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, err
	}
	sources := make(map[int64]migrationSource, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := parseGooseMigrationVersion(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("invalid embedded migration %q: %w", entry.Name(), err)
		}
		content, err := fs.ReadFile(files, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(content)
		sources[int64(version)] = migrationSource{sha256: hex.EncodeToString(digest[:])}
	}
	return sources, nil
}

func executeSessionAdvisoryLock(ctx context.Context, connection *sql.Conn, statement string) error {
	rows, err := connection.QueryContext(ctx, statement, migrationAdvisoryLockKey)
	if err != nil {
		return err
	}
	if !rows.Next() {
		rowsErr := rows.Err()
		_ = rows.Close()
		if rowsErr != nil {
			return rowsErr
		}
		return fmt.Errorf("migration advisory lock statement returned no result row")
	}
	if rows.Next() {
		_ = rows.Close()
		return fmt.Errorf("migration advisory lock statement returned unexpected extra rows")
	}
	rowsErr := rows.Err()
	closeErr := rows.Close()
	if rowsErr != nil {
		return rowsErr
	}
	return closeErr
}

func appliedMigrationVersions(ctx context.Context, database *sql.DB) ([]int64, error) {
	var exists bool
	if err := database.QueryRowContext(ctx, "SELECT to_regclass('public.goose_db_version') IS NOT NULL").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return []int64{}, nil
	}
	rows, err := database.QueryContext(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied AND version_id > 0 ORDER BY version_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]int64, 0)
	for rows.Next() {
		var version int64
		if err = rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func databaseHasRelation(ctx context.Context, database *sql.DB, relation string) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+relation).Scan(&exists)
	return exists, err
}

func recordMigrationExecutionEvidence(ctx context.Context, database *sql.DB, sources map[int64]migrationSource, before, applied []int64) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	existing := make(map[int64]MigrationExecutionEvidence)
	rows, err := transaction.QueryContext(ctx, `SELECT version_id,migration_sha256,recorded_by_build_id,execution_result,recorded_at
FROM migration_execution_evidence ORDER BY version_id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var record MigrationExecutionEvidence
		if err = rows.Scan(&record.VersionID, &record.MigrationSHA256, &record.RecordedByBuildID, &record.ExecutionResult, &record.RecordedAt); err != nil {
			rows.Close()
			return err
		}
		existing[record.VersionID] = record
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	appliedSet := versionSet(applied)
	beforeSet := versionSet(before)
	var highestRecorded int64
	for version, record := range existing {
		source, sourceExists := sources[version]
		if !sourceExists || !appliedSet[version] || source.sha256 != record.MigrationSHA256 {
			return fmt.Errorf("migration execution evidence for version %d does not match applied source", version)
		}
		if version > highestRecorded {
			highestRecorded = version
		}
	}
	for _, version := range applied {
		if _, sourceExists := sources[version]; !sourceExists {
			return fmt.Errorf("applied Goose migration version %d is not embedded in this build", version)
		}
		if _, recorded := existing[version]; !recorded && version < highestRecorded {
			return fmt.Errorf("migration execution evidence is missing applied version %d before a later recorded version", version)
		}
	}
	buildID := currentMigrationBuildID()
	for _, version := range applied {
		if _, recorded := existing[version]; recorded {
			continue
		}
		executionResult := "applied_during_this_run"
		if beforeSet[version] {
			executionResult = "observed_preexisting"
		}
		if _, err = transaction.ExecContext(ctx, `INSERT INTO migration_execution_evidence(version_id,migration_sha256,recorded_by_build_id,execution_result)
VALUES($1,$2,$3,$4)`, version, sources[version].sha256, buildID, executionResult); err != nil {
			return fmt.Errorf("record migration evidence for version %d: %w", version, err)
		}
	}
	return transaction.Commit()
}

func verifyRecordedMigrationEvidence(ctx context.Context, database *sql.DB, sources map[int64]migrationSource, applied []int64) error {
	rows, err := database.QueryContext(ctx, `SELECT version_id,migration_sha256,recorded_by_build_id,execution_result
FROM migration_execution_evidence ORDER BY version_id`)
	if err != nil {
		return err
	}
	recorded := make(map[int64]MigrationExecutionEvidence)
	for rows.Next() {
		var record MigrationExecutionEvidence
		if err = rows.Scan(&record.VersionID, &record.MigrationSHA256, &record.RecordedByBuildID, &record.ExecutionResult); err != nil {
			rows.Close()
			return err
		}
		recorded[record.VersionID] = record
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	appliedSet := versionSet(applied)
	var highestRecorded int64
	for version, record := range recorded {
		source, sourceExists := sources[version]
		if !sourceExists || !appliedSet[version] || source.sha256 != record.MigrationSHA256 || record.RecordedByBuildID == "" {
			return fmt.Errorf("migration evidence row %d is stale, forged or outside the applied schema", version)
		}
		if record.ExecutionResult != "applied_during_this_run" && record.ExecutionResult != "observed_preexisting" {
			return fmt.Errorf("migration evidence row %d has an unknown execution result", version)
		}
		if version > highestRecorded {
			highestRecorded = version
		}
	}
	for _, version := range applied {
		if _, sourceExists := sources[version]; !sourceExists {
			return fmt.Errorf("applied Goose migration version %d is not embedded in this build", version)
		}
		if _, exists := recorded[version]; !exists && version < highestRecorded {
			return fmt.Errorf("migration evidence is missing applied version %d before a later recorded version", version)
		}
	}
	return nil
}

func versionSet(versions []int64) map[int64]bool {
	set := make(map[int64]bool, len(versions))
	for _, version := range versions {
		set[version] = true
	}
	return set
}

func currentMigrationBuildID() string {
	version := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				version += ";vcs=" + setting.Value
			}
			if setting.Key == "vcs.modified" {
				version += ";modified=" + setting.Value
			}
		}
	}
	buildID := "go=" + runtime.Version() + ";" + version
	if len(buildID) > 512 {
		return buildID[:512]
	}
	return buildID
}

func ReadMigrationExecutionEvidence(ctx context.Context, dsn string) ([]MigrationExecutionEvidence, error) {
	config, err := parseMigrationDSN(dsn)
	if err != nil {
		return nil, err
	}
	database := stdlib.OpenDB(*config)
	defer database.Close()
	lockConn, err := database.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer lockConn.Close()
	if err = executeSessionAdvisoryLock(ctx, lockConn, "SELECT pg_advisory_lock($1)"); err != nil {
		return nil, fmt.Errorf("acquire migration-status advisory lock: %w", err)
	}
	defer func() {
		_ = executeSessionAdvisoryLock(context.Background(), lockConn, "SELECT pg_advisory_unlock($1)")
	}()
	applied, err := appliedMigrationVersions(ctx, database)
	if err != nil {
		return nil, err
	}
	exists, err := databaseHasRelation(ctx, database, "migration_execution_evidence")
	if err != nil {
		return nil, err
	}
	if !exists {
		if len(applied) > 0 && applied[len(applied)-1] >= migrationExecutionEvidenceSchemaVersion {
			return nil, fmt.Errorf("schema %d is missing the required migration execution evidence table", applied[len(applied)-1])
		}
		return nil, fmt.Errorf("migration execution evidence is unavailable; apply the migration ledger first")
	}
	sources, err := migrationSourceCatalog(migrations)
	if err != nil {
		return nil, err
	}
	if err = verifyRecordedMigrationEvidence(ctx, database, sources, applied); err != nil {
		return nil, fmt.Errorf("migration execution ledger failed verification: %w", err)
	}
	rows, err := database.QueryContext(ctx, `SELECT version_id,migration_sha256,recorded_by_build_id,execution_result,recorded_at
FROM migration_execution_evidence ORDER BY version_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]MigrationExecutionEvidence, 0)
	for rows.Next() {
		var record MigrationExecutionEvidence
		if err = rows.Scan(&record.VersionID, &record.MigrationSHA256, &record.RecordedByBuildID, &record.ExecutionResult, &record.RecordedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(records) != len(applied) {
		return nil, fmt.Errorf("migration execution ledger is incomplete: recorded %d of %d applied versions; run polis migrate to reconcile", len(records), len(applied))
	}
	for index, version := range applied {
		if records[index].VersionID != version {
			return nil, fmt.Errorf("migration execution ledger is incomplete at applied version %d; run polis migrate to reconcile", version)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].VersionID < records[j].VersionID })
	return records, nil
}
