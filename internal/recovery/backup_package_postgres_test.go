// pattern: Imperative Shell
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/intake"
	"polis/internal/kernel"
)

func TestRecoveryBackupPackagePostgresRoundTrip(t *testing.T) {
	sourceRuntimeDSN := os.Getenv("POLIS_RECOVERY_TEST_SOURCE_DSN")
	sourceBackupDSN := os.Getenv("POLIS_RECOVERY_TEST_BACKUP_DSN")
	targetDSN := os.Getenv("POLIS_RECOVERY_TEST_TARGET_DSN")
	targetRuntimeDSN := os.Getenv("POLIS_RECOVERY_TEST_TARGET_RUNTIME_DSN")
	blobRoot := os.Getenv("POLIS_RECOVERY_TEST_BLOB_ROOT")
	pgDumpPath := os.Getenv("POLIS_PG_DUMP_PATH")
	pgRestorePath := os.Getenv("POLIS_PG_RESTORE_PATH")
	if sourceRuntimeDSN == "" || sourceBackupDSN == "" || targetDSN == "" || targetRuntimeDSN == "" || blobRoot == "" || pgDumpPath == "" || pgRestorePath == "" {
		t.Skip("dedicated temporary PostgreSQL backup/restore environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := os.MkdirAll(blobRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	k, err := kernel.Open(ctx, sourceRuntimeDSN, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CreateRecoveryBackupPackage(ctx, CreateRecoveryBackupOptions{DatabaseDSN: sourceBackupDSN, BlobRoot: blobRoot, OutputRoot: t.TempDir(), PGDumpPath: pgDumpPath, RuntimeRoleName: "polis_runtime"}); !errors.Is(err, ErrRecoveryBackupMaintenanceBusy) {
		k.Close()
		t.Fatalf("backup window overlapping the live Polis controller error=%v", err)
	}
	companyID := fmt.Sprintf("recovery-backup-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Recovery package fixture", "preserve a CAS-backed input", "recovery-backup-mission-"+companyID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	content := []byte("R2 backup and restore source fixture")
	prepared, stored, err := intake.PrepareMissionInput("recovery-source.md", "text/markdown", content)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	input, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "recovery-backup-input-"+companyID, prepared, stored)
	k.Close()
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err := pgxpool.New(ctx, sourceBackupDSN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, "UPDATE mission_inputs SET state='uploading' WHERE company_id=$1 AND input_id=$2 AND revision=$3", companyID, input.InputID, input.Revision); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	if _, err = CreateRecoveryBackupPackage(ctx, CreateRecoveryBackupOptions{DatabaseDSN: sourceBackupDSN, BlobRoot: blobRoot, OutputRoot: t.TempDir(), PGDumpPath: pgDumpPath, RuntimeRoleName: "polis_runtime"}); !errors.Is(err, ErrRecoveryBackupMaintenanceBusy) {
		adminPool.Close()
		t.Fatalf("backup window with an uploading MissionInput error=%v", err)
	}
	if _, err = adminPool.Exec(ctx, "UPDATE mission_inputs SET state=$4 WHERE company_id=$1 AND input_id=$2 AND revision=$3", companyID, input.InputID, input.Revision, input.State); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	adminPool.Close()

	outputRoot := t.TempDir()
	backup, err := CreateRecoveryBackupPackage(ctx, CreateRecoveryBackupOptions{DatabaseDSN: sourceBackupDSN, BlobRoot: blobRoot, OutputRoot: outputRoot, PGDumpPath: pgDumpPath, RuntimeRoleName: "polis_runtime"})
	if err != nil {
		t.Fatal(err)
	}
	if backup.Verification.Status != "PASSED" || backup.Verification.SchemaVersion < 40 || backup.Verification.VerifiedBlobCount != 1 ||
		backup.Verification.DatabaseOwner == "" || backup.Verification.RuntimeRoleName != "polis_runtime" || len(backup.Verification.Extensions) == 0 {
		t.Fatalf("backup verification=%+v", backup.Verification)
	}

	restoredBlobRoot := filepath.Join(t.TempDir(), "restored-blobs")
	firstRestore, err := RestoreRecoveryBackupPackage(ctx, RestoreRecoveryBackupOptions{
		PackageRoot: backup.PackagePath, TargetDatabaseDSN: targetDSN, TargetBlobRoot: restoredBlobRoot,
		RuntimeRoleName: "polis_runtime", PGRestorePath: pgRestorePath,
	})
	if err != nil || firstRestore.Status != "RESTORED" || firstRestore.DatabaseName == "" || firstRestore.RestoredBlobCount != 1 {
		t.Fatalf("restore result=(%+v,%v)", firstRestore, err)
	}
	verifiedGeneration, err := VerifyRestoredRecoveryGeneration(ctx, backup.PackagePath, targetDSN, restoredBlobRoot)
	if err != nil || verifiedGeneration.Status != "PASSED" || verifiedGeneration.GenerationID != backup.Verification.GenerationID ||
		verifiedGeneration.ManifestSHA256 != backup.Verification.ManifestSHA256 || verifiedGeneration.DatabaseName != firstRestore.DatabaseName ||
		verifiedGeneration.SchemaVersion != backup.Verification.SchemaVersion || verifiedGeneration.VerifiedBlobCount != backup.Verification.VerifiedBlobCount {
		t.Fatalf("restored generation verification=(%+v,%v)", verifiedGeneration, err)
	}
	secretDSN := "postgres://recovery-user:recovery-password@127.0.0.1:1/missing"
	if _, err = VerifyRestoredRecoveryGeneration(ctx, backup.PackagePath, secretDSN, restoredBlobRoot); err == nil || strings.Contains(err.Error(), "recovery-password") {
		t.Fatalf("unavailable target database error=%v, want failure without credential disclosure", err)
	}
	inputPath := filepath.Join(restoredBlobRoot, companyID, input.ContentDigest)
	restoredContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(restoredContent)
	if hex.EncodeToString(digest[:]) != input.ContentDigest || string(restoredContent) != string(content) {
		t.Fatalf("restored CAS content digest=%s bytes=%q", hex.EncodeToString(digest[:]), restoredContent)
	}
	restoredSnapshot, err := kernel.ReadSnapshot(ctx, targetRuntimeDSN, companyID, mission.ID)
	if err != nil || restoredSnapshot.MissionState != "draft" {
		t.Fatalf("restored database snapshot=(%+v,%v)", restoredSnapshot, err)
	}
	secondRestore, err := RestoreRecoveryBackupPackage(ctx, RestoreRecoveryBackupOptions{
		PackageRoot: backup.PackagePath, TargetDatabaseDSN: targetDSN, TargetBlobRoot: restoredBlobRoot,
		RuntimeRoleName: "polis_runtime", PGRestorePath: pgRestorePath,
	})
	if err != nil || secondRestore.Status != "ALREADY_RESTORED" {
		t.Fatalf("idempotent restore result=(%+v,%v)", secondRestore, err)
	}
	otherBackup, err := CreateRecoveryBackupPackage(ctx, CreateRecoveryBackupOptions{DatabaseDSN: sourceBackupDSN, BlobRoot: blobRoot, OutputRoot: outputRoot, PGDumpPath: pgDumpPath, RuntimeRoleName: "polis_runtime"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyRestoredRecoveryGeneration(ctx, otherBackup.PackagePath, targetDSN, restoredBlobRoot); err == nil {
		t.Fatal("generation verifier accepted database/CAS markers from a different backup package")
	}
	if _, err = RestoreRecoveryBackupPackage(ctx, RestoreRecoveryBackupOptions{
		PackageRoot: otherBackup.PackagePath, TargetDatabaseDSN: targetDSN, TargetBlobRoot: restoredBlobRoot,
		RuntimeRoleName: "polis_runtime", PGRestorePath: pgRestorePath,
	}); err == nil {
		t.Fatal("restore from a different package overwrote a previously restored target")
	}
}
