// pattern: Imperative Shell
package recovery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
)

type CreateRecoveryBackupOptions struct {
	DatabaseDSN     string
	BlobRoot        string
	OutputRoot      string
	PGDumpPath      string
	RuntimeRoleName string
}

type CreateRecoveryBackupResult struct {
	PackagePath  string                           `json:"packagePath"`
	Verification RecoveryBackupVerificationReport `json:"verification"`
}

func CreateRecoveryBackupPackage(ctx context.Context, options CreateRecoveryBackupOptions) (CreateRecoveryBackupResult, error) {
	var result CreateRecoveryBackupResult
	if strings.TrimSpace(options.DatabaseDSN) == "" || !filepath.IsAbs(options.BlobRoot) || !filepath.IsAbs(options.OutputRoot) {
		return result, fmt.Errorf("%w: a database DSN and absolute CAS/output paths are required", ErrRecoveryBackupInvalid)
	}
	blobRoot, err := validateExistingDirectory(options.BlobRoot)
	if err != nil {
		return result, fmt.Errorf("%w: CAS root: %v", ErrRecoveryBackupInvalid, err)
	}
	outputRoot, err := validateExistingDirectory(options.OutputRoot)
	if err != nil {
		return result, fmt.Errorf("%w: output root: %v", ErrRecoveryBackupInvalid, err)
	}
	if pathsOverlap(blobRoot, outputRoot) {
		return result, fmt.Errorf("%w: output root and CAS root must not overlap", ErrRecoveryBackupInvalid)
	}
	pgDumpPath, err := resolvePostgresUtility(options.PGDumpPath, "pg_dump")
	if err != nil {
		return result, err
	}
	runtimeRoleName := options.RuntimeRoleName
	if runtimeRoleName == "" {
		runtimeRoleName = "polis_runtime"
	}
	window, err := acquireRecoveryBackupWindow(ctx, options.DatabaseDSN, runtimeRoleName)
	if err != nil {
		return result, err
	}
	defer window.Close()
	server := window.Database
	clientMajor, err := postgresUtilityMajor(ctx, pgDumpPath)
	if err != nil || clientMajor != server.PostgresMajor {
		return result, fmt.Errorf("%w: pg_dump major version must match the server major version", ErrRecoveryBackupInvalid)
	}
	generationID, err := newRecoveryGenerationID()
	if err != nil {
		return result, err
	}
	stageRoot, err := os.MkdirTemp(outputRoot, ".polis-recovery-stage-")
	if err != nil {
		return result, err
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			_ = os.RemoveAll(stageRoot)
		}
	}()
	if err = os.Chmod(stageRoot, 0o700); err != nil {
		return result, err
	}
	dumpPath := filepath.Join(stageRoot, RecoveryBackupDatabaseName)
	pgTargetDir, err := os.MkdirTemp("", "polis-pg-command-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(pgTargetDir)
	serviceName, pgEnv, cleanupPG, err := preparePostgresUtilityTarget(options.DatabaseDSN, "", pgTargetDir)
	if err != nil {
		return result, err
	}
	defer cleanupPG()
	command := exec.CommandContext(ctx, pgDumpPath, "--format=custom", "--no-owner", "--no-privileges", "--file", dumpPath, "--dbname=service="+serviceName, "--no-password")
	command.Env = postgresUtilityEnvironment(pgEnv)
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		return result, fmt.Errorf("pg_dump failed: %w: %s", commandErr, strings.TrimSpace(string(output)))
	}
	if err = os.Chmod(dumpPath, 0o600); err != nil {
		return result, err
	}
	dumpSHA, dumpSize, err := hashRegularFile(dumpPath)
	if err != nil || dumpSize <= 0 {
		return result, fmt.Errorf("%w: pg_dump output is missing or invalid", ErrRecoveryBackupInvalid)
	}
	casStage := filepath.Join(stageRoot, "cas")
	entries, totalCASBytes, err := copyVerifiedCAS(blobRoot, casStage)
	if err != nil {
		return result, err
	}
	casManifest := struct {
		SchemaVersion string                   `json:"schemaVersion"`
		Entries       []RecoveryBackupCASEntry `json:"entries"`
	}{SchemaVersion: RecoveryBackupPackageSchema, Entries: entries}
	casRaw, err := json.Marshal(casManifest)
	if err != nil {
		return result, err
	}
	if len(casRaw) > 64<<20 {
		return result, fmt.Errorf("%w: CAS manifest exceeds the 64 MiB backup limit", ErrRecoveryBackupInvalid)
	}
	casHash := sha256.Sum256(casRaw)
	casSHA := hex.EncodeToString(casHash[:])
	if err = writeSyncedFile(filepath.Join(stageRoot, RecoveryBackupCASName), casRaw, 0o600); err != nil {
		return result, err
	}
	manifest := RecoveryBackupManifest{
		SchemaVersion: RecoveryBackupPackageSchema, Status: "COMPLETE", GenerationID: generationID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), CredentialsIncluded: false,
		Database: RecoveryBackupDatabase{
			FileName: RecoveryBackupDatabaseName, ByteSize: dumpSize, SHA256: dumpSHA,
			DatabaseName: server.DatabaseName, DatabaseOwner: server.DatabaseOwner, RuntimeRoleName: server.RuntimeRoleName,
			PostgresMajor: server.PostgresMajor, SchemaVersion: server.SchemaVersion, Extensions: server.Extensions,
		},
		CAS: RecoveryBackupCAS{ManifestFileName: RecoveryBackupCASName, ManifestSHA256: casSHA, BlobCount: len(entries), TotalBytes: totalCASBytes},
	}
	if err = ValidateRecoveryBackupManifest(manifest); err != nil {
		return result, err
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return result, err
	}
	manifestHash := sha256.Sum256(manifestRaw)
	manifestSHA := hex.EncodeToString(manifestHash[:])
	if err = writeSyncedFile(filepath.Join(stageRoot, RecoveryBackupManifestName), manifestRaw, 0o600); err != nil {
		return result, err
	}
	if err = writeSyncedFile(filepath.Join(stageRoot, RecoveryBackupCompleteName), []byte(manifestSHA+"\n"), 0o600); err != nil {
		return result, err
	}
	if err = syncDirectory(stageRoot); err != nil {
		return result, err
	}
	verification, err := VerifyRecoveryBackupPackage(stageRoot)
	if err != nil {
		return result, err
	}
	packagePath := filepath.Join(outputRoot, "polis-recovery-"+generationID)
	if _, err = os.Lstat(packagePath); err == nil {
		return result, fmt.Errorf("%w: generated package path already exists", ErrRecoveryBackupInvalid)
	} else if !os.IsNotExist(err) {
		return result, err
	}
	if err = os.Rename(stageRoot, packagePath); err != nil {
		return result, err
	}
	stageOwned = false
	if err = syncDirectory(outputRoot); err != nil {
		return result, err
	}
	return CreateRecoveryBackupResult{PackagePath: packagePath, Verification: verification}, nil
}

type recoveryDatabaseIdentity struct {
	DatabaseName    string
	DatabaseOwner   string
	RuntimeRoleName string
	PostgresMajor   int
	SchemaVersion   int64
	Extensions      []RecoveryBackupExtension
}

type recoveryBackupWindow struct {
	pool     *pgxpool.Pool
	lease    *pgxpool.Conn
	Database recoveryDatabaseIdentity
}

func (window *recoveryBackupWindow) Close() error {
	if window == nil {
		return nil
	}
	var unlockErr error
	if window.lease != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		unlockErr = window.lease.QueryRow(ctx, "SELECT pg_advisory_unlock(714209831)").Scan(&unlocked)
		window.lease.Release()
	}
	if window.pool != nil {
		window.pool.Close()
	}
	return unlockErr
}

func acquireRecoveryBackupWindow(ctx context.Context, dsn, runtimeRoleName string) (*recoveryBackupWindow, error) {
	if strings.TrimSpace(dsn) == "" || !core.ValidID(runtimeRoleName) {
		return nil, fmt.Errorf("%w: explicit database DSN and runtime role are required", ErrRecoveryBackupInvalid)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to recovery database: %w", err)
	}
	lease, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to reserve recovery maintenance connection: %w", err)
	}
	var locked bool
	if err = lease.QueryRow(ctx, "SELECT pg_try_advisory_lock(714209831)").Scan(&locked); err != nil {
		lease.Release()
		pool.Close()
		return nil, err
	}
	if !locked {
		lease.Release()
		pool.Close()
		return nil, fmt.Errorf("%w: another Polis control-plane instance owns the database", ErrRecoveryBackupMaintenanceBusy)
	}
	window := &recoveryBackupWindow{pool: pool, lease: lease, Database: recoveryDatabaseIdentity{RuntimeRoleName: runtimeRoleName, Extensions: []RecoveryBackupExtension{}}}
	var versionNum int
	if err = lease.QueryRow(ctx, `SELECT current_database(), current_setting('server_version_num')::integer, max(version_id)
FROM goose_db_version WHERE is_applied`).Scan(&window.Database.DatabaseName, &versionNum, &window.Database.SchemaVersion); err != nil {
		_ = window.Close()
		return nil, fmt.Errorf("failed to inspect recovery database identity: %w", err)
	}
	window.Database.PostgresMajor = versionNum / 10000
	if window.Database.DatabaseName == "" || window.Database.PostgresMajor < 12 || window.Database.SchemaVersion <= 0 {
		_ = window.Close()
		return nil, fmt.Errorf("%w: database identity or migration schema is invalid", ErrRecoveryBackupInvalid)
	}
	if err = lease.QueryRow(ctx, `SELECT owner.rolname FROM pg_database database
JOIN pg_roles owner ON owner.oid=database.datdba WHERE database.datname=current_database()`).Scan(&window.Database.DatabaseOwner); err != nil {
		_ = window.Close()
		return nil, err
	}
	var runtimeRoleExists bool
	if err = lease.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", runtimeRoleName).Scan(&runtimeRoleExists); err != nil {
		_ = window.Close()
		return nil, err
	}
	if !runtimeRoleExists {
		_ = window.Close()
		return nil, fmt.Errorf("%w: backup runtime role %q is absent", ErrRecoveryBackupInvalid, runtimeRoleName)
	}
	rows, err := lease.Query(ctx, `SELECT extension.extname,extension.extversion,namespace.nspname
FROM pg_extension extension JOIN pg_namespace namespace ON namespace.oid=extension.extnamespace ORDER BY extension.extname`)
	if err != nil {
		_ = window.Close()
		return nil, err
	}
	for rows.Next() {
		var extension RecoveryBackupExtension
		if err = rows.Scan(&extension.Name, &extension.Version, &extension.Schema); err != nil {
			rows.Close()
			_ = window.Close()
			return nil, err
		}
		window.Database.Extensions = append(window.Database.Extensions, extension)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		_ = window.Close()
		return nil, err
	}
	rows.Close()
	if err = verifyRecoveryBackupQuiescence(ctx, lease); err != nil {
		_ = window.Close()
		return nil, err
	}
	return window, nil
}

func verifyRecoveryBackupQuiescence(ctx context.Context, connection *pgxpool.Conn) error {
	checks := []struct {
		query  string
		reason string
	}{
		{query: "SELECT EXISTS(SELECT 1 FROM worker_sessions WHERE state<>'stopped')", reason: "WorkerSessions are not stopped"},
		{query: `SELECT EXISTS(SELECT 1 FROM job_runs run LEFT JOIN LATERAL (SELECT state FROM job_run_events event WHERE event.company_id=run.company_id AND event.job_id=run.job_id ORDER BY event_seq DESC LIMIT 1) latest ON true WHERE COALESCE(latest.state,'accepted') IN ('accepted','starting','running'))`, reason: "a JobRun is still active"},
		{query: `SELECT EXISTS(SELECT 1 FROM environment_preparation_runs run LEFT JOIN LATERAL (SELECT state FROM environment_preparation_events event WHERE event.company_id=run.company_id AND event.run_id=run.run_id ORDER BY event_seq DESC LIMIT 1) latest ON true WHERE COALESCE(latest.state,'accepted') IN ('accepted','starting','running'))`, reason: "an environment preparation is still active"},
		{query: "SELECT EXISTS(SELECT 1 FROM mission_inputs WHERE state='uploading')", reason: "a MissionInput upload is still in progress"},
		{query: "SELECT EXISTS(SELECT 1 FROM notification_deliveries WHERE state='sending')", reason: "an external notification send is still in progress"},
		{query: `SELECT EXISTS(SELECT 1 FROM service_endpoint_events endpoint WHERE endpoint.event_seq=(SELECT max(latest.event_seq) FROM service_endpoint_events latest WHERE latest.company_id=endpoint.company_id AND latest.job_id=endpoint.job_id) AND endpoint.readiness<>'revoked' AND endpoint.lease_expires_at>clock_timestamp())`, reason: "a service endpoint lease is still active"},
	}
	for _, check := range checks {
		var active bool
		if err := connection.QueryRow(ctx, check.query).Scan(&active); err != nil {
			return fmt.Errorf("failed to check backup maintenance boundary: %w", err)
		}
		if active {
			return fmt.Errorf("%w: %s", ErrRecoveryBackupMaintenanceBusy, check.reason)
		}
	}
	return nil
}

func copyVerifiedCAS(sourceRoot, destinationRoot string) ([]RecoveryBackupCASEntry, int64, error) {
	entries := make([]RecoveryBackupCASEntry, 0, 128)
	if err := os.Mkdir(destinationRoot, 0o700); err != nil {
		return nil, 0, err
	}
	companies, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	for _, companyEntry := range companies {
		if !companyEntry.IsDir() || companyEntry.Type()&os.ModeSymlink != 0 || !core.ValidID(companyEntry.Name()) {
			return nil, 0, fmt.Errorf("%w: CAS contains an unsafe company directory", ErrRecoveryBackupInvalid)
		}
		companyPath := filepath.Join(sourceRoot, companyEntry.Name())
		companyInfo, statErr := os.Lstat(companyPath)
		if statErr != nil || companyInfo.Mode()&os.ModeSymlink != 0 || !companyInfo.IsDir() {
			return nil, 0, fmt.Errorf("%w: CAS company path changed during backup", ErrRecoveryBackupInvalid)
		}
		if err = os.Mkdir(filepath.Join(destinationRoot, companyEntry.Name()), 0o700); err != nil {
			return nil, 0, err
		}
		blobs, readErr := os.ReadDir(companyPath)
		if readErr != nil {
			return nil, 0, readErr
		}
		for _, blobEntry := range blobs {
			if blobEntry.IsDir() || blobEntry.Type()&os.ModeSymlink != 0 || !validRecoveryDigest(blobEntry.Name()) {
				return nil, 0, fmt.Errorf("%w: CAS contains an unknown file or nested directory", ErrRecoveryBackupInvalid)
			}
			sourcePath := filepath.Join(companyPath, blobEntry.Name())
			destinationPath := filepath.Join(destinationRoot, companyEntry.Name(), blobEntry.Name())
			digest, size, copyErr := copyAndHashRegularFile(sourcePath, destinationPath)
			if copyErr != nil || digest != blobEntry.Name() {
				return nil, 0, fmt.Errorf("%w: CAS blob copy or digest verification failed", ErrRecoveryBackupInvalid)
			}
			entries = append(entries, RecoveryBackupCASEntry{CompanyID: companyEntry.Name(), SHA256: digest, ByteSize: size})
			if total > int64(^uint64(0)>>1)-size {
				return nil, 0, fmt.Errorf("%w: CAS size total overflows", ErrRecoveryBackupInvalid)
			}
			total += size
		}
		if err = syncDirectory(filepath.Join(destinationRoot, companyEntry.Name())); err != nil {
			return nil, 0, err
		}
	}
	canonicalRecoveryCASEntries(entries)
	if err = syncDirectory(destinationRoot); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func copyAndHashRegularFile(sourcePath, destinationPath string) (string, int64, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", 0, ErrRecoveryBackupInvalid
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(destination, io.TeeReader(source, hash))
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return "", size, copyErr
	}
	if syncErr != nil {
		return "", size, syncErr
	}
	if closeErr != nil {
		return "", size, closeErr
	}
	finalInfo, err := os.Lstat(sourcePath)
	if err != nil || finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() || finalInfo.Size() != info.Size() || finalInfo.ModTime() != info.ModTime() || size != info.Size() {
		return "", size, ErrRecoveryBackupInvalid
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func validateExistingDirectory(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("path must be an existing non-symlink directory")
	}
	return filepath.EvalSymlinks(abs)
}

func pathsOverlap(left, right string) bool {
	leftRelative, leftErr := filepath.Rel(left, right)
	rightRelative, rightErr := filepath.Rel(right, left)
	return leftErr == nil && (leftRelative == "." || (leftRelative != ".." && !strings.HasPrefix(leftRelative, ".."+string(filepath.Separator)))) || rightErr == nil && (rightRelative == "." || (rightRelative != ".." && !strings.HasPrefix(rightRelative, ".."+string(filepath.Separator))))
}

func writeSyncedFile(path string, content []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}

func newRecoveryGenerationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func resolvePostgresUtility(path, name string) (string, error) {
	if path == "" {
		resolved, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("%s executable is unavailable", name)
		}
		path = resolved
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s must be a regular executable", ErrRecoveryBackupInvalid, name)
	}
	return resolved, nil
}

var postgresUtilityVersionPattern = regexp.MustCompile(`(?m)PostgreSQL\)\s+([0-9]+)(?:\.[0-9]+)?`)

func postgresUtilityMajor(ctx context.Context, path string) (int, error) {
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to query PostgreSQL utility version: %w", err)
	}
	match := postgresUtilityVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("%w: PostgreSQL utility version output is unrecognized", ErrRecoveryBackupInvalid)
	}
	var major int
	if _, err = fmt.Sscanf(string(match[1]), "%d", &major); err != nil || major < 12 {
		return 0, fmt.Errorf("%w: PostgreSQL utility major version is unsupported", ErrRecoveryBackupInvalid)
	}
	return major, nil
}
