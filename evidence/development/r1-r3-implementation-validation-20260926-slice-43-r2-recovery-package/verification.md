# Slice 43 — R2 PostgreSQL/CAS recovery package

Date: 2026-09-26

## Implemented

- Added generation-named PostgreSQL/CAS backup packages with a custom-format database dump, source database owner/runtime role/PostgreSQL/schema and installed-extension metadata, sorted CAS inventory and per-blob hashes, a complete manifest and a manifest checksum.
- Backup verifies every source CAS path is a regular, non-symlink company/digest file, copies it under private package permissions, hashes while copying, and publishes by same-filesystem rename only after validating the staged package. CAS is immutable/content-addressed in this implementation; the backup's PostgreSQL snapshot plus later full-CAS capture is a safe superset while there is no CAS garbage collector.
- Added package verification for manifest integrity, strict layout, database hash, CAS manifest hash, exact file inventory and CAS bytes.
- Backup acquires the existing singleton Polis control advisory lock and refuses active WorkerSessions, JobRuns, preparations, uploads, service endpoint leases or in-flight notification sends. The lock remains held through dump and CAS inventory capture.
- Added non-overwriting restore to an already-created empty database on the same PostgreSQL major and a new CAS root. Restore is a single `pg_restore` transaction, re-establishes DML access for the existing runtime role, checks the restored migration version and records generation markers. Repeat restore of the same package is idempotent; a non-empty unmarked database or mismatched package generation is rejected.
- Added `polis recovery-backup`, `polis recovery-backup-verify`, and `polis recovery-backup-restore`. PostgreSQL utility calls use a temporary service file and private temporary pgpass file, so DSN passwords stay out of process arguments and backup artifacts.

## Verification

- `rtk proxy bash scripts/r2-recovery-backup-postgres-test.sh` — printed `R2_RECOVERY_BACKUP_RESTORE=PASSED` from a dedicated disposable PostgreSQL 18 run. It proves backup is denied while a Polis controller owns the advisory lock or a MissionInput is uploading; then it migrates a source database, creates a company/Mission and CAS-backed MissionInput, exercises library backup/restore, verifies database state and blob bytes, and confirms a second restore of the same package is idempotent. It also exercises all three `polis` CLI commands against the temporary source/target databases and rejects overwrite by another package generation.
- `rtk proxy bash scripts/go.sh test ./internal/recovery ./cmd/polis` — passed.
- `rtk proxy bash scripts/go.sh test ./...` — passed for the full Go module.
- `rtk proxy bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis'` — passed.
- Recovery package unit tests cover dump/CAS tamper, extra/unmanifested CAS content, credential exclusion and password separation from the temporary PostgreSQL service file.

## Limits

The package preserves the database and CAS. It does not include provider/QQ/GitHub/MCP credentials, WFP/AppContainer state, OS policy or service configuration. Restore never changes the live service configuration or performs cutover. Cross-backend runtime handover, long-lived observation and migration upgrade/rollback qualification remain open.
