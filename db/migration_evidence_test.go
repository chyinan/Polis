// pattern: Imperative Shell
package db

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5"
	"os"
	"polis/internal/pgfixture"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationEvidenceLedgerTracksBuildHashesAndAppendOnlyOutcome(t *testing.T) {
	dsn := pgfixture.EmptyDSN(t, "POLIS_MIGRATION_EVIDENCE_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration-evidence database required")
	}
	ctx := context.Background()
	if err := MigrateToVersion(ctx, dsn, 39); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	expectedVersion := currentSourceVersion(t)
	records, err := ReadMigrationExecutionEvidence(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(records)) != expectedVersion {
		t.Fatalf("migration evidence rows=%d, want %d applied migration versions", len(records), expectedVersion)
	}
	for index, record := range records {
		wantVersion := int64(index + 1)
		wantResult := "observed_preexisting"
		if wantVersion >= 40 {
			wantResult = "applied_during_this_run"
		}
		if record.VersionID != wantVersion || len(record.MigrationSHA256) != 64 || record.RecordedByBuildID == "" ||
			record.ExecutionResult != wantResult {
			t.Fatalf("migration evidence row %d = %+v", index, record)
		}
	}
	if records[len(records)-1].VersionID != expectedVersion || records[len(records)-1].ExecutionResult != "applied_during_this_run" {
		t.Fatalf("latest migration evidence = %+v, want current Schema %d execution", records[len(records)-1], expectedVersion)
	}
	searchPathRecords, err := ReadMigrationExecutionEvidence(ctx, dsn+" search_path=pg_catalog")
	if err != nil || !reflect.DeepEqual(records, searchPathRecords) {
		t.Fatalf("migration-status with alternate DSN search_path = %d records err=%v, want pinned public schema", len(searchPathRecords), err)
	}

	if err = Migrate(ctx, dsn); err != nil {
		t.Fatalf("idempotent Migrate: %v", err)
	}
	afterReplay, err := ReadMigrationExecutionEvidence(ctx, dsn)
	if err != nil || !reflect.DeepEqual(records, afterReplay) {
		t.Fatalf("migration evidence changed on repeated Migrate: before=%+v after=%+v err=%v", records, afterReplay, err)
	}

	if err = MigrateToVersion(ctx, dsn, 42); err == nil {
		t.Fatal("MigrateToVersion accepted a downgrade from schema 70")
	}
	version := int64(0)
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.QueryRowContext(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil || version != expectedVersion {
		t.Fatalf("schema after rejected downgrade=%d err=%v, want %d", version, err, expectedVersion)
	}
	if _, err = database.ExecContext(ctx, "UPDATE migration_execution_evidence SET recorded_by_build_id='tampered' WHERE version_id=50"); err == nil {
		t.Fatal("migration evidence UPDATE was accepted")
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM migration_execution_evidence WHERE version_id=50"); err == nil {
		t.Fatal("migration evidence DELETE was accepted")
	}
	runtimeDSN := pgfixture.RuntimeForDatabase(t, os.Getenv("POLIS_TEST_DSN"), dsn)
	if runtimeDSN == "" {
		t.Fatal("POLIS_MIGRATION_EVIDENCE_RUNTIME_DSN is required to verify runtime-role insertion is denied")
	}
	cfg, err := pgx.ParseConfig(runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{cfg.User}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{cfg.User}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	runtimeDatabase, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeDatabase.Close()
	if _, err = runtimeDatabase.ExecContext(ctx, `INSERT INTO migration_execution_evidence(version_id,migration_sha256,recorded_by_build_id,execution_result)
VALUES(45,repeat('a',64),'forged-runtime-build','applied_during_this_run')`); err == nil {
		t.Fatal("runtime role inserted migration execution evidence")
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE migration_execution_evidence DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "UPDATE migration_execution_evidence SET migration_sha256=repeat('f',64) WHERE version_id=50"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE migration_execution_evidence ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, dsn); err == nil {
		t.Fatal("Migrate accepted recorded hash evidence that diverges from the embedded migration")
	}
	if _, err = database.ExecContext(ctx, "DROP TABLE migration_execution_evidence CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, dsn); err == nil || !strings.Contains(err.Error(), "missing the required migration execution evidence table") {
		t.Fatalf("Migrate error after deleting migration evidence ledger=%v, want fail-closed integrity error", err)
	}
}
