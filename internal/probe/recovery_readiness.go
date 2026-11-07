// pattern: Imperative Shell
package probe

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const requiredPostgreSQLClientMajor = 18
const maxRecoveryVersionOutput = 4096

type PostgreSQLClientVersion struct {
	Product            string
	Version            string
	Major              int
	Minor              int
	DistributionSuffix string
	Raw                string
}

type PostgreSQLExecutableReadiness struct {
	ExecutablePath     string `json:"executable_path"`
	RawVersion         string `json:"raw_version"`
	Product            string `json:"product"`
	Version            string `json:"version"`
	Major              int    `json:"major"`
	Minor              int    `json:"minor,omitempty"`
	DistributionSuffix string `json:"distribution_suffix,omitempty"`
	Compatibility      string `json:"compatibility"`
}

type RecoveryReadinessReport struct {
	Status                string                        `json:"status"`
	DumpPath              string                        `json:"dump_path"`
	DumpVersion           string                        `json:"dump_version,omitempty"`
	DumpClient            PostgreSQLExecutableReadiness `json:"pg_dump"`
	RestorePath           string                        `json:"restore_path"`
	RestoreClient         PostgreSQLExecutableReadiness `json:"pg_restore"`
	RecoveryRoot          string                        `json:"recovery_root"`
	PostgresClientMajor   string                        `json:"postgres_client_major,omitempty"`
	PreservationMachinery string                        `json:"preservation_machinery"`
}

var postgresClientVersionPattern = regexp.MustCompile(`^(?:(?:pg_dump|pg_restore) \(PostgreSQL\) |PostgreSQL )([0-9]+(?:\.[0-9]+){0,2})(?: \(([^()\r\n]+)\))?$`)

func parsePostgreSQLClientVersion(raw string) (PostgreSQLClientVersion, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxRecoveryVersionOutput {
		return PostgreSQLClientVersion{}, fmt.Errorf("VERSION_UNPARSEABLE: PostgreSQL client version output is empty or oversized")
	}
	matches := postgresClientVersionPattern.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return PostgreSQLClientVersion{}, fmt.Errorf("VERSION_UNPARSEABLE: unsupported PostgreSQL client version output: %q", trimmed)
	}
	parts := strings.Split(matches[1], ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 1 {
		return PostgreSQLClientVersion{}, fmt.Errorf("VERSION_UNPARSEABLE: invalid PostgreSQL major version: %q", matches[1])
	}
	minor := 0
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return PostgreSQLClientVersion{}, fmt.Errorf("VERSION_UNPARSEABLE: invalid PostgreSQL minor version: %q", matches[1])
		}
	}
	return PostgreSQLClientVersion{Product: "PostgreSQL", Version: matches[1], Major: major, Minor: minor, DistributionSuffix: matches[2], Raw: trimmed}, nil
}

func executableReadiness(path string, version PostgreSQLClientVersion) PostgreSQLExecutableReadiness {
	return PostgreSQLExecutableReadiness{ExecutablePath: path, RawVersion: version.Raw, Product: version.Product, Version: version.Version, Major: version.Major, Minor: version.Minor, DistributionSuffix: version.DistributionSuffix, Compatibility: "PASS"}
}

type boundedVersionBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedVersionBuffer) Write(p []byte) (int, error) {
	remaining := maxRecoveryVersionOutput - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.overflow = true
		_, _ = b.Buffer.Write(p[:remaining])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func validateR03ARecoveryReadinessInputs(cfg R03APaginationV3Config) error {
	if cfg.PostgresDumpPath == "" {
		return fmt.Errorf("RECOVERY_READINESS_FAILED: POLIS_V3_POSTGRES_DUMP is missing")
	}
	if cfg.PostgresSnapshotDSN == "" {
		return fmt.Errorf("RECOVERY_READINESS_FAILED: PostgresSnapshotDSN is missing")
	}
	if cfg.PostgresSnapshotPath == "" {
		return fmt.Errorf("RECOVERY_READINESS_FAILED: PostgresSnapshotPath is missing")
	}
	if cfg.RecoveryRoot == "" || strings.HasPrefix(cfg.RecoveryRoot, "/tmp/") || strings.HasPrefix(cfg.RecoveryRoot, "/mnt/c/") || strings.HasPrefix(cfg.RecoveryRoot, "/mnt/d/") {
		return fmt.Errorf("RECOVERY_READINESS_FAILED: durable recovery root is missing or non-durable: %s", cfg.RecoveryRoot)
	}
	return nil
}

func validateRecoveryExecutable(ctx context.Context, path string) (PostgreSQLClientVersion, error) {
	command := shellQuote(path) + " --version"
	process := exec.CommandContext(ctx, "bash", "-lc", command)
	var stdout, stderr boundedVersionBuffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	if err := process.Run(); err != nil {
		return PostgreSQLClientVersion{}, fmt.Errorf("RECOVERY_READINESS_FAILED: executable is not launchable: %w", err)
	}
	if stdout.overflow || stderr.overflow {
		return PostgreSQLClientVersion{}, fmt.Errorf("RECOVERY_READINESS_FAILED: VERSION_UNPARSEABLE: version output exceeded %d bytes", maxRecoveryVersionOutput)
	}
	version, err := parsePostgreSQLClientVersion(stdout.String())
	if err != nil {
		return PostgreSQLClientVersion{}, fmt.Errorf("RECOVERY_READINESS_FAILED: %w", err)
	}
	if version.Major != requiredPostgreSQLClientMajor {
		return version, fmt.Errorf("RECOVERY_READINESS_FAILED: PostgreSQL client major incompatible: got %d want %d", version.Major, requiredPostgreSQLClientMajor)
	}
	return version, nil
}

func validateRecoveryRestoreExecutable(ctx context.Context, dumpPath string) (PostgreSQLClientVersion, error) {
	restorePath := filepath.ToSlash(strings.TrimSuffix(filepath.ToSlash(dumpPath), "/pg_dump") + "/pg_restore")
	return validateRecoveryExecutable(ctx, restorePath)
}

func recordR03ARecoveryReadiness(ctx context.Context, cfg R03APaginationV3Config) error {
	if err := validateR03ARecoveryReadinessInputs(cfg); err != nil {
		return err
	}
	dumpVersion, err := validateRecoveryExecutable(ctx, cfg.PostgresDumpPath)
	if err != nil {
		return err
	}
	restorePath := strings.TrimSuffix(filepath.ToSlash(cfg.PostgresDumpPath), "/pg_dump") + "/pg_restore"
	restoreVersion, err := validateRecoveryRestoreExecutable(ctx, cfg.PostgresDumpPath)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "recovery-readiness.json"), RecoveryReadinessReport{
		Status:                "PASSED",
		DumpPath:              cfg.PostgresDumpPath,
		DumpVersion:           dumpVersion.Version,
		DumpClient:            executableReadiness(cfg.PostgresDumpPath, dumpVersion),
		RestorePath:           restorePath,
		RestoreClient:         executableReadiness(restorePath, restoreVersion),
		RecoveryRoot:          cfg.RecoveryRoot,
		PostgresClientMajor:   strconv.Itoa(requiredPostgreSQLClientMajor),
		PreservationMachinery: "r0.3a-preserved-postgres-access@1",
	})
}

func RecordR03ARecoveryReadiness(ctx context.Context, cfg R03APaginationV3Config) error {
	return recordR03ARecoveryReadiness(ctx, cfg)
}
