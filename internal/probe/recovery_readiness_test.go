// pattern: Functional Core
package probe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func containsReadinessText(value, want string) bool { return strings.Contains(value, want) }

func readinessConfig() R03APaginationV3Config {
	return R03APaginationV3Config{
		Config:               Config{PostgresDumpPath: "/mnt/d/pg_dump", RecoveryRoot: "/home/chyinan/.local/state/polis-recovery/r03a-sample"},
		PostgresSnapshotPath: "/mnt/d/recovery/state.dump",
		PostgresSnapshotDSN:  "host=127.0.0.1 port=55432 dbname=polis_r0_test user=owner",
	}
}

func TestRecoveryReadinessRejectsMissingProductionDumpWiring(t *testing.T) {
	cfg := readinessConfig()
	cfg.PostgresDumpPath = ""
	if err := validateR03ARecoveryReadinessInputs(cfg); err == nil {
		t.Fatal("missing PostgresDumpPath was accepted")
	}
}

func TestRecoveryReadinessRejectsEphemeralOrDrvfsRoot(t *testing.T) {
	for _, root := range []string{"/tmp/polis", "/mnt/c/recovery", "/mnt/d/recovery"} {
		t.Run(root, func(t *testing.T) {
			cfg := readinessConfig()
			cfg.RecoveryRoot = root
			if err := validateR03ARecoveryReadinessInputs(cfg); err == nil {
				t.Fatal("non-durable recovery root was accepted")
			}
		})
	}
}

func TestRecoveryReadinessAcceptsTypedConfiguredInputs(t *testing.T) {
	if err := validateR03ARecoveryReadinessInputs(readinessConfig()); err != nil {
		t.Fatalf("valid typed recovery inputs rejected: %v", err)
	}
}

func TestParsePostgreSQLClientVersion(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantVersion string
		wantMajor   int
		wantMinor   int
		wantSuffix  string
	}{
		{name: "pg_dump patch", raw: "pg_dump (PostgreSQL) 18.6", wantVersion: "18.6", wantMajor: 18, wantMinor: 6},
		{name: "pg_dump Ubuntu suffix", raw: "pg_dump (PostgreSQL) 18.6 (Ubuntu 18.6-1.pgdg22.04+2)", wantVersion: "18.6", wantMajor: 18, wantMinor: 6, wantSuffix: "Ubuntu 18.6-1.pgdg22.04+2"},
		{name: "pg_restore patch", raw: "pg_restore (PostgreSQL) 18.6", wantVersion: "18.6", wantMajor: 18, wantMinor: 6},
		{name: "pg_restore Ubuntu suffix", raw: "pg_restore (PostgreSQL) 18.6 (Ubuntu 18.6-1.pgdg22.04+2)", wantVersion: "18.6", wantMajor: 18, wantMinor: 6, wantSuffix: "Ubuntu 18.6-1.pgdg22.04+2"},
		{name: "bare major", raw: "PostgreSQL 18", wantVersion: "18", wantMajor: 18},
		{name: "bare PostgreSQL suffix", raw: "PostgreSQL 18.6 (Ubuntu 18.6-1.pgdg22.04+2)", wantVersion: "18.6", wantMajor: 18, wantMinor: 6, wantSuffix: "Ubuntu 18.6-1.pgdg22.04+2"},
		{name: "three components", raw: "PostgreSQL 18.1.2", wantVersion: "18.1.2", wantMajor: 18, wantMinor: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePostgreSQLClientVersion(tc.raw)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if got.Product != "PostgreSQL" || got.Version != tc.wantVersion || got.Major != tc.wantMajor || got.Minor != tc.wantMinor || got.DistributionSuffix != tc.wantSuffix {
				t.Fatalf("parsed=%+v", got)
			}
		})
	}
}

func TestParsePostgreSQLClientVersionRejectsMalformedOutput(t *testing.T) {
	for _, raw := range []string{"", "garbage 18.6", "pg_dump 18.6", "pg_dump (SomethingElse) 18.6", "pg_dump (PostgreSQL) foo", "pg_dump (PostgreSQL) 18foo", "pg_dump (PostgreSQL) (Ubuntu 18.6...)", "pg_dump (PostgreSQL) 18.6 (Ubuntu 18.6...", "PostgreSQL eighteen", "PostgreSQL 18.1.2.3"} {
		if _, err := parsePostgreSQLClientVersion(raw); err == nil {
			t.Fatalf("malformed output accepted: %q", raw)
		}
	}
}

func TestRecoveryExecutableVersionAndCompatibility(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		output     string
		exit       string
		wantErr    bool
		wantReason string
	}{
		{name: "major 18 patch passes", output: "pg_dump (PostgreSQL) 18.6", wantReason: "18.6"},
		{name: "major 17 fails compatibility", output: "pg_dump (PostgreSQL) 17.6", wantErr: true, wantReason: "incompatible"},
		{name: "major 19 fails compatibility", output: "pg_dump (PostgreSQL) 19.1", wantErr: true, wantReason: "incompatible"},
		{name: "empty output fails parsing", output: "", wantErr: true, wantReason: "VERSION_UNPARSEABLE"},
		{name: "nonzero exit fails launch", output: "pg_dump (PostgreSQL) 18.6", exit: "exit 7", wantErr: true, wantReason: "not launchable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "pg_dump")
			script := "#!/bin/sh\nprintf '%s\\n' '" + tc.output + "'\n" + tc.exit + "\n"
			if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			_, err := validateRecoveryExecutable(ctx, path)
			if tc.wantErr != (err != nil) {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil && !containsReadinessText(err.Error(), tc.wantReason) {
				t.Fatalf("error=%v missing %q", err, tc.wantReason)
			}
		})
	}
}

func TestRecoveryReadinessUsesInstalledPostgreSQL18Clients(t *testing.T) {
	if os.Getenv("POLIS_RUN_REAL_RECOVERY_READINESS") != "1" {
		t.Skip("set POLIS_RUN_REAL_RECOVERY_READINESS=1 for the local installed-client smoke")
	}
	cfg := readinessConfig()
	cfg.Evidence = t.TempDir()
	cfg.PostgresDumpPath = "/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin/pg_dump"
	cfg.PostgresSnapshotPath = "/home/chyinan/.local/state/polis-recovery/readiness-only/no-source.dump"
	if err := recordR03ARecoveryReadiness(context.Background(), cfg); err != nil {
		t.Fatalf("installed PostgreSQL 18 readiness failed: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Evidence, "recovery-readiness.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report RecoveryReadinessReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.DumpClient.Major != 18 || report.RestoreClient.Major != 18 || report.DumpClient.Compatibility != "PASS" || report.RestoreClient.Compatibility != "PASS" {
		t.Fatalf("unexpected readiness report: %+v", report)
	}
}
