# Slice 45 verification: R2 migration SQL hash guard

Date: 2026-09-26

## Result

`polis migrate` now verifies the exact embedded Goose SQL file set against the committed `db/migration_hashes.sha256` manifest before parsing the DSN or opening a database. It rejects altered contents, missing/extra SQL files, malformed hashes and noncanonical ordering. The PowerShell updater works in the current Windows PowerShell environment and writes deterministic UTF-8 without BOM and LF line endings. Current Goose schema remains 39.

## Verification performed

- `powershell -ExecutionPolicy Bypass -File scripts/update-migration-hash-manifest.ps1` — passed and reported 39 verified existing pins with 0 additions.
- `powershell -ExecutionPolicy Bypass -File scripts/test-migration-hash-manifest.ps1` — `MIGRATION_HASH_UPDATER=PASSED`; the isolated test proves edited pinned SQL, backward/duplicate Goose versions and missing-manifest bootstrap are refused without changing pins, while a new forward migration is added.
- `bash scripts/go.sh test ./db -count=1` — passed. Cases cover the embedded manifest, correct file hashes, changed bytes, missing/extra entries, malformed hash, unsorted entries, and `Migrate` rejecting a tampered manifest before parsing an intentionally invalid DSN.
- `bash scripts/r2-cross-backend-handover-postgres-test.sh` — `R2_CROSS_BACKEND_HANDOVER=PASSED` using a dedicated temporary PostgreSQL 18 cluster. Its `polis migrate` invocation passed the new preflight and applied schemas 1–39. The script removed the temporary database, socket and files.
- `bash scripts/go.sh test ./...` — passed all Go packages.
- `bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis ./cmd/polisd` — passed.
- `rtk git diff --check` — passed.

## Limits

The manifest pins source SQL but does not persist a database-side migration hash, build identity or migration execution outcome. It does not automate a pre-upgrade backup, active desktop DB/CAS cutover or rollback of the installed binary. No non-test database, provider, QQ, MCP, GitHub or production resource was used.
