# Slice 46 verification: R2 migration execution ledger

Date: 2026-09-27

## Result

Schema 40 records one append-only evidence row for each applied Goose migration version, including the embedded SQL SHA-256, recording build identity, truthful execution result and timestamp. Migrate validates previously recorded rows before applying new migrations, serializes concurrent migration commands with a database advisory lock, and writes missing ledger rows transactionally after Goose succeeds. `observed_preexisting` explicitly marks schema versions applied before their build identity could be recorded. `polis migration-status` returns the ledger; `polis migrate-to VERSION` is forward-only and refuses downgrade.

## Verification performed

- `bash scripts/r2-cross-backend-handover-postgres-test.sh` — `R2_CROSS_BACKEND_HANDOVER=PASSED` on a dedicated temporary PostgreSQL 18 cluster. The script applies through Schema 39, runs the Schema 39 populated/rollback migration test, upgrades with the real CLI to Schema 40, checks the 40-row ledger and idempotent replay, rejects update/delete and runtime-role insert, exercises `migration-status`, and rejects CLI downgrade to 39. Kernel, Control and Workbench PostgreSQL integration tests then pass against Schema 40. Temporary resources are removed by the script trap.
- The ledger integration additionally tampers with a disposable recorded hash, removes the Schema 40 ledger table, and verifies that Migrate rejects both conditions before any further Goose action. A status read using an alternate requested `search_path` still reads the explicitly pinned `public` schema.
- `bash scripts/r2-recovery-backup-postgres-test.sh` — `R2_RECOVERY_BACKUP_RESTORE=PASSED` against Schema 40; the recovery package preserved the ledger as part of the database dump.
- `bash scripts/go.sh test ./...` — passed all Go packages.
- `bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis ./cmd/polisd` — passed.
- `rtk git diff --check` — passed.

## Limits

Rows for pre-Schema 40 versions use the current recording build and `observed_preexisting`; the original applying build is unknown. Failed Goose attempts are not yet persisted. Automatic pre-upgrade backups, desktop DB/CAS cutover, binary rollback, host-confirmed WorkerSession reconciliation and live multi-day observation remain open. No non-test database or external provider/account was used.
