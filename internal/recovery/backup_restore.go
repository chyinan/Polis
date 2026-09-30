// pattern: Imperative Shell
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
)

type RestoreRecoveryBackupOptions struct {
	PackageRoot       string
	TargetDatabaseDSN string
	TargetBlobRoot    string
	RuntimeRoleName   string
	PGRestorePath     string
}

type RestoreRecoveryBackupResult struct {
	Status            string `json:"status"`
	GenerationID      string `json:"generationId"`
	DatabaseName      string `json:"databaseName"`
	BlobRoot          string `json:"blobRoot"`
	SchemaVersion     int64  `json:"schemaVersion"`
	RestoredBlobCount int    `json:"restoredBlobCount"`
	RestoredBlobBytes int64  `json:"restoredBlobBytes"`
}

type VerifyRestoredRecoveryGenerationResult struct {
	Status            string `json:"status"`
	GenerationID      string `json:"generationId"`
	ManifestSHA256    string `json:"manifestSha256"`
	DatabaseName      string `json:"databaseName"`
	BlobRoot          string `json:"blobRoot"`
	SchemaVersion     int64  `json:"schemaVersion"`
	VerifiedBlobCount int    `json:"verifiedBlobCount"`
	VerifiedBlobBytes int64  `json:"verifiedBlobBytes"`
}

type recoveryRestoreMarker struct {
	SchemaVersion  string `json:"schemaVersion"`
	GenerationID   string `json:"generationId"`
	ManifestSHA256 string `json:"manifestSha256"`
}

func RestoreRecoveryBackupPackage(ctx context.Context, options RestoreRecoveryBackupOptions) (RestoreRecoveryBackupResult, error) {
	var result RestoreRecoveryBackupResult
	report, err := VerifyRecoveryBackupPackage(options.PackageRoot)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(options.TargetDatabaseDSN) == "" || !filepath.IsAbs(options.TargetBlobRoot) || !core.ValidID(options.RuntimeRoleName) {
		return result, fmt.Errorf("%w: target DSN, absolute blob root and runtime role are required", ErrRecoveryBackupInvalid)
	}
	if options.RuntimeRoleName != report.RuntimeRoleName {
		return result, fmt.Errorf("%w: target runtime role differs from backup role metadata", ErrRecoveryBackupInvalid)
	}
	pgRestorePath, err := resolvePostgresUtility(options.PGRestorePath, "pg_restore")
	if err != nil {
		return result, err
	}
	clientMajor, err := postgresUtilityMajor(ctx, pgRestorePath)
	if err != nil || clientMajor != report.PostgresMajor {
		return result, fmt.Errorf("%w: pg_restore major version must match the backup server major version", ErrRecoveryBackupInvalid)
	}
	targetDatabase, targetMajor, targetEmpty, runtimeRoleExists, targetOwner, err := inspectRecoveryRestoreTarget(ctx, options.TargetDatabaseDSN, options.RuntimeRoleName, report.Extensions)
	if err != nil {
		return result, err
	}
	if targetMajor != report.PostgresMajor {
		return result, fmt.Errorf("%w: target PostgreSQL major differs from the backup", ErrRecoveryBackupInvalid)
	}
	if !runtimeRoleExists {
		return result, fmt.Errorf("%w: required runtime role %q is absent in the target cluster", ErrRecoveryBackupInvalid, options.RuntimeRoleName)
	}
	if !targetOwner {
		return result, fmt.Errorf("%w: restore connection must own the target database or be a superuser", ErrRecoveryBackupInvalid)
	}
	alreadyRestored, err := checkRestoreMarker(ctx, options.TargetDatabaseDSN, report)
	if err != nil {
		return result, err
	}
	if !targetEmpty && !alreadyRestored {
		return result, fmt.Errorf("%w: target database is non-empty and has no matching restore marker", ErrRecoveryBackupInvalid)
	}
	packageRoot, err := filepath.EvalSymlinks(options.PackageRoot)
	if err != nil {
		return result, err
	}
	blobParent, err := validateExistingDirectory(filepath.Dir(options.TargetBlobRoot))
	if err != nil {
		return result, fmt.Errorf("%w: blob-root parent: %v", ErrRecoveryBackupInvalid, err)
	}
	blobName := filepath.Base(options.TargetBlobRoot)
	if blobName == "." || blobName == ".." || blobName == string(filepath.Separator) {
		return result, fmt.Errorf("%w: CAS root must name a new child directory", ErrRecoveryBackupInvalid)
	}
	targetBlobRoot := filepath.Join(blobParent, blobName)
	if pathsOverlap(packageRoot, targetBlobRoot) {
		return result, fmt.Errorf("%w: restore CAS root must be outside the package directory", ErrRecoveryBackupInvalid)
	}
	if err = installRecoveryCAS(packageRoot, targetBlobRoot, report); err != nil {
		return result, err
	}
	if alreadyRestored {
		if err = grantRecoveryRuntimeAccess(ctx, options.TargetDatabaseDSN, options.RuntimeRoleName); err != nil {
			return result, fmt.Errorf("matching restore marker found but runtime grants could not be re-established: %w", err)
		}
		version, versionErr := restoredSchemaVersion(ctx, options.TargetDatabaseDSN)
		if versionErr != nil || version != report.SchemaVersion {
			return result, fmt.Errorf("%w: matching restore marker has a different schema version", ErrRecoveryBackupInvalid)
		}
		return RestoreRecoveryBackupResult{Status: "ALREADY_RESTORED", GenerationID: report.GenerationID, DatabaseName: targetDatabase, BlobRoot: targetBlobRoot, SchemaVersion: version, RestoredBlobCount: report.VerifiedBlobCount, RestoredBlobBytes: report.VerifiedBlobBytes}, nil
	}
	temporaryDirectory, err := os.MkdirTemp("", "polis-pg-restore-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temporaryDirectory)
	serviceName, pgEnv, cleanupPG, err := preparePostgresUtilityTarget(options.TargetDatabaseDSN, "", temporaryDirectory)
	if err != nil {
		return result, err
	}
	defer cleanupPG()
	dumpPath := filepath.Join(packageRoot, RecoveryBackupDatabaseName)
	command := exec.CommandContext(ctx, pgRestorePath, "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl", "--dbname=service="+serviceName, "--no-password", dumpPath)
	command.Env = postgresUtilityEnvironment(pgEnv)
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		return result, fmt.Errorf("pg_restore failed; target database transaction rolled back: %w: %s", commandErr, strings.TrimSpace(string(output)))
	}
	marker := recoveryRestoreMarker{SchemaVersion: RecoveryBackupPackageSchema, GenerationID: report.GenerationID, ManifestSHA256: report.ManifestSHA256}
	markerRaw, err := json.Marshal(marker)
	if err != nil {
		return result, err
	}
	if err = writeDatabaseRestoreMarker(ctx, options.TargetDatabaseDSN, markerRaw); err != nil {
		return result, fmt.Errorf("restore transaction committed but completion marker could not be written: %w", err)
	}
	if err = grantRecoveryRuntimeAccess(ctx, options.TargetDatabaseDSN, options.RuntimeRoleName); err != nil {
		return result, fmt.Errorf("restore completed but runtime grants failed; database and CAS were preserved: %w", err)
	}
	version, err := restoredSchemaVersion(ctx, options.TargetDatabaseDSN)
	if err != nil || version != report.SchemaVersion {
		return result, fmt.Errorf("%w: restored migration schema differs from backup", ErrRecoveryBackupInvalid)
	}
	return RestoreRecoveryBackupResult{
		Status: "RESTORED", GenerationID: report.GenerationID, DatabaseName: targetDatabase, BlobRoot: targetBlobRoot,
		SchemaVersion: version, RestoredBlobCount: report.VerifiedBlobCount, RestoredBlobBytes: report.VerifiedBlobBytes,
	}, nil
}

func VerifyRestoredRecoveryGeneration(ctx context.Context, packageRoot, targetDatabaseDSN, targetBlobRoot string) (VerifyRestoredRecoveryGenerationResult, error) {
	var result VerifyRestoredRecoveryGenerationResult
	if strings.TrimSpace(targetDatabaseDSN) == "" || !filepath.IsAbs(targetBlobRoot) {
		return result, fmt.Errorf("%w: target database DSN and absolute CAS root are required", ErrRecoveryBackupInvalid)
	}
	report, err := VerifyRecoveryBackupPackage(packageRoot)
	if err != nil {
		return result, err
	}
	canonicalPackageRoot, err := filepath.EvalSymlinks(packageRoot)
	if err != nil {
		return result, fmt.Errorf("%w: recovery package path cannot be resolved", ErrRecoveryBackupInvalid)
	}
	canonicalBlobRoot, err := validateExistingDirectory(targetBlobRoot)
	if err != nil {
		return result, fmt.Errorf("%w: restored CAS root is unavailable or unsafe", ErrRecoveryBackupInvalid)
	}
	if pathsOverlap(canonicalPackageRoot, canonicalBlobRoot) {
		return result, fmt.Errorf("%w: restored CAS root overlaps the recovery package", ErrRecoveryBackupInvalid)
	}
	markerMatches, err := checkRestoreMarker(ctx, targetDatabaseDSN, report)
	if err != nil {
		return result, wrapRecoveryGenerationDatabaseError(err, "target database restore marker could not be verified")
	}
	if !markerMatches {
		return result, fmt.Errorf("%w: target database has no matching restore marker", ErrRecoveryBackupInvalid)
	}
	entries, expectedMarker, err := loadRecoveryRestoreInventory(canonicalPackageRoot, report)
	if err != nil {
		return result, err
	}
	markerRaw, err := readBoundedRegularFile(canonicalBlobRoot+".polis-recovery.json", 4096)
	if err != nil {
		return result, fmt.Errorf("%w: restored CAS root marker is unavailable or unsafe", ErrRecoveryBackupInvalid)
	}
	var actualMarker recoveryRestoreMarker
	if err = json.Unmarshal(markerRaw, &actualMarker); err != nil || actualMarker != expectedMarker {
		return result, fmt.Errorf("%w: restored CAS root marker belongs to another recovery generation", ErrRecoveryBackupInvalid)
	}
	verifiedBytes, err := verifyRecoveryBackupCAS(canonicalBlobRoot, entries)
	if err != nil {
		return result, err
	}
	schemaVersion, err := restoredSchemaVersion(ctx, targetDatabaseDSN)
	if err != nil {
		return result, wrapRecoveryGenerationDatabaseError(err, "target database schema could not be verified")
	}
	if schemaVersion != report.SchemaVersion {
		return result, fmt.Errorf("%w: restored database schema differs from the recovery package", ErrRecoveryBackupInvalid)
	}
	databasePool, err := pgxpool.New(ctx, targetDatabaseDSN)
	if err != nil {
		return result, wrapRecoveryGenerationDatabaseError(err, "restored database cannot be reopened for identity verification")
	}
	defer databasePool.Close()
	var databaseName string
	if err = databasePool.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		return result, wrapRecoveryGenerationDatabaseError(err, "restored database identity cannot be read")
	}
	return VerifyRestoredRecoveryGenerationResult{
		Status: "PASSED", GenerationID: report.GenerationID, ManifestSHA256: report.ManifestSHA256,
		DatabaseName: databaseName, BlobRoot: canonicalBlobRoot, SchemaVersion: schemaVersion,
		VerifiedBlobCount: report.VerifiedBlobCount, VerifiedBlobBytes: verifiedBytes,
	}, nil
}

func wrapRecoveryGenerationDatabaseError(err error, message string) error {
	if errors.Is(err, ErrRecoveryBackupInvalid) {
		return err
	}
	return fmt.Errorf("%w: %s", ErrRecoveryBackupInvalid, message)
}

func inspectRecoveryRestoreTarget(ctx context.Context, dsn, runtimeRole string, requiredExtensions []RecoveryBackupExtension) (string, int, bool, bool, bool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return "", 0, false, false, false, fmt.Errorf("failed to connect to restore target: %w", err)
	}
	defer pool.Close()
	var databaseName string
	var versionNum int
	var relationCount int64
	var runtimeRoleExists bool
	var targetOwner bool
	err = pool.QueryRow(ctx, `SELECT current_database(),current_setting('server_version_num')::integer`).Scan(&databaseName, &versionNum)
	if err != nil {
		return "", 0, false, false, false, fmt.Errorf("failed to inspect restore target identity: %w", err)
	}
	if err = pool.QueryRow(ctx, `SELECT d.datdba=r.oid OR r.rolsuper FROM pg_database d JOIN pg_roles r ON r.rolname=current_user WHERE d.datname=current_database()`).Scan(&targetOwner); err != nil {
		return "", 0, false, false, false, err
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%'
AND c.relkind IN ('r','v','m','S','f','p')`).Scan(&relationCount); err != nil {
		return "", 0, false, false, false, err
	}
	if err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", runtimeRole).Scan(&runtimeRoleExists); err != nil {
		return "", 0, false, false, false, err
	}
	for _, extension := range requiredExtensions {
		var available bool
		if err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_available_extension_versions WHERE name=$1 AND version=$2)", extension.Name, extension.Version).Scan(&available); err != nil {
			return "", 0, false, false, false, err
		}
		if !available {
			return "", 0, false, false, false, fmt.Errorf("%w: required PostgreSQL extension %s@%s is unavailable", ErrRecoveryBackupInvalid, extension.Name, extension.Version)
		}
	}
	return databaseName, versionNum / 10000, relationCount == 0, runtimeRoleExists, targetOwner, nil
}

func installRecoveryCAS(packageRoot, targetRoot string, report RecoveryBackupVerificationReport) error {
	entries, marker, err := loadRecoveryRestoreInventory(packageRoot, report)
	if err != nil {
		return err
	}
	markerRaw, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	markerPath := targetRoot + ".polis-recovery.json"
	if existingMarkerRaw, markerErr := readBoundedRegularFile(markerPath, 4096); markerErr == nil {
		var existingMarker recoveryRestoreMarker
		if json.Unmarshal(existingMarkerRaw, &existingMarker) != nil || existingMarker != marker {
			return fmt.Errorf("%w: target CAS restore marker belongs to another recovery generation", ErrRecoveryBackupInvalid)
		}
	} else if !os.IsNotExist(markerErr) {
		return fmt.Errorf("%w: target CAS restore marker is unsafe", ErrRecoveryBackupInvalid)
	}
	if _, statErr := os.Lstat(targetRoot); statErr == nil {
		if _, verifyErr := verifyRecoveryBackupCAS(targetRoot, entries); verifyErr != nil {
			return fmt.Errorf("%w: existing target CAS root is not an exact package copy", ErrRecoveryBackupInvalid)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	} else {
		// Mkdir is the atomic no-overwrite reservation for this exact destination.
		// A failed/interrupted copy is preserved for inspection and never removed
		// automatically; a later attempt can adopt it only if every blob verifies.
		if _, _, err = copyVerifiedCAS(filepath.Join(packageRoot, "cas"), targetRoot); err != nil {
			return err
		}
		if _, err = verifyRecoveryBackupCAS(targetRoot, entries); err != nil {
			return err
		}
	}
	if _, err = os.Lstat(markerPath); os.IsNotExist(err) {
		if err = writeSyncedFile(markerPath, markerRaw, 0o600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func loadRecoveryRestoreInventory(packageRoot string, report RecoveryBackupVerificationReport) ([]RecoveryBackupCASEntry, recoveryRestoreMarker, error) {
	var backupManifest RecoveryBackupManifest
	manifestRaw, err := readBoundedRegularFile(filepath.Join(packageRoot, RecoveryBackupManifestName), 4<<20)
	if err != nil || json.Unmarshal(manifestRaw, &backupManifest) != nil {
		return nil, recoveryRestoreMarker{}, fmt.Errorf("%w: backup manifest unavailable during restore", ErrRecoveryBackupInvalid)
	}
	manifestHash := sha256.Sum256(manifestRaw)
	manifestSHA := hex.EncodeToString(manifestHash[:])
	if manifestSHA != report.ManifestSHA256 || backupManifest.GenerationID != report.GenerationID {
		return nil, recoveryRestoreMarker{}, fmt.Errorf("%w: backup manifest changed during restore", ErrRecoveryBackupInvalid)
	}
	casRaw, err := readBoundedRegularFile(filepath.Join(packageRoot, backupManifest.CAS.ManifestFileName), 64<<20)
	if err != nil {
		return nil, recoveryRestoreMarker{}, fmt.Errorf("%w: CAS manifest unavailable during restore", ErrRecoveryBackupInvalid)
	}
	casHash := sha256.Sum256(casRaw)
	if hex.EncodeToString(casHash[:]) != backupManifest.CAS.ManifestSHA256 {
		return nil, recoveryRestoreMarker{}, fmt.Errorf("%w: CAS manifest changed during restore", ErrRecoveryBackupInvalid)
	}
	var casManifest struct {
		SchemaVersion string                   `json:"schemaVersion"`
		Entries       []RecoveryBackupCASEntry `json:"entries"`
	}
	if json.Unmarshal(casRaw, &casManifest) != nil || casManifest.SchemaVersion != RecoveryBackupPackageSchema {
		return nil, recoveryRestoreMarker{}, fmt.Errorf("%w: CAS inventory is invalid", ErrRecoveryBackupInvalid)
	}
	if err = validateRecoveryCASEntries(casManifest.Entries, backupManifest.CAS.BlobCount, backupManifest.CAS.TotalBytes); err != nil {
		return nil, recoveryRestoreMarker{}, err
	}
	return casManifest.Entries, recoveryRestoreMarker{SchemaVersion: RecoveryBackupPackageSchema, GenerationID: report.GenerationID, ManifestSHA256: report.ManifestSHA256}, nil
}

func grantRecoveryRuntimeAccess(ctx context.Context, dsn, runtimeRole string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	roleIdentifier := pgx.Identifier{runtimeRole}.Sanitize()
	statements := []string{
		"GRANT USAGE ON SCHEMA public TO " + roleIdentifier,
		"GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO " + roleIdentifier,
		"GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO " + roleIdentifier,
	}
	for _, statement := range statements {
		if _, err = pool.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func restoredSchemaVersion(ctx context.Context, dsn string) (int64, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	var version int64
	if err = pool.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func writeDatabaseRestoreMarker(ctx context.Context, dsn string, marker []byte) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	var databaseName string
	if err = pool.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		return err
	}
	commentLiteral := ""
	if err = pool.QueryRow(ctx, "SELECT quote_literal($1)", "polis-recovery-restore@1:"+string(marker)).Scan(&commentLiteral); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, "COMMENT ON DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" IS "+commentLiteral)
	return err
}

func checkRestoreMarker(ctx context.Context, dsn string, report RecoveryBackupVerificationReport) (bool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return false, err
	}
	defer pool.Close()
	var marker *string
	if err = pool.QueryRow(ctx, `SELECT shobj_description((SELECT oid FROM pg_database WHERE datname=current_database()),'pg_database')`).Scan(&marker); err != nil {
		return false, err
	}
	if marker == nil {
		return false, nil
	}
	wantPrefix := "polis-recovery-restore@1:"
	if !strings.HasPrefix(*marker, wantPrefix) {
		return false, fmt.Errorf("%w: target database has an unrelated database comment", ErrRecoveryBackupInvalid)
	}
	var actual recoveryRestoreMarker
	if json.Unmarshal([]byte(strings.TrimPrefix(*marker, wantPrefix)), &actual) != nil {
		return false, ErrRecoveryBackupInvalid
	}
	if actual.SchemaVersion != RecoveryBackupPackageSchema || actual.GenerationID != report.GenerationID || actual.ManifestSHA256 != report.ManifestSHA256 {
		return false, fmt.Errorf("%w: target database was restored from a different recovery package", ErrRecoveryBackupInvalid)
	}
	return true, nil
}
