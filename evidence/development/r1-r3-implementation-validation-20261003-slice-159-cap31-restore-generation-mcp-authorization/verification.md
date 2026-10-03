# Slice 159 — CAP-31 recovery-generation MCP authorization gate

Date: 2026-10-03

## Change

`internal/recovery/backup_restore.go` now holds the existing control-plane PostgreSQL session advisory lock (`714209831`, shared with `Kernel.Open`) across target inspection, CAS installation, `pg_restore`, authorization finalization and runtime grants. A Kernel cannot start while the restored generation is staged.

An empty restore target is tagged with a generation-specific staging marker before `pg_restore`. The restore remains one database transaction. A matching staging marker plus the package schema version permits a verified retry after the database restore committed but finalization did not. Finalization appends immutable `revoked` records for each currently bound MCP Employee capability and corresponding `capability.employee.revoke` company event rows in a transaction. Event and request IDs are deterministic from the recovery generation and source binding event. The complete marker is written after that transaction; if the comment write fails, the staging marker remains and retry sees the revocations already applied. A completed marker records that the gate was applied. Backups predating Schema 18 have no Employee capability binding table to revoke.

The resulting policy is fail-closed: restored Employee MCP grants do not survive as active bindings. Any later use needs a new explicit binding and a fresh database-confirmed active WorkerSession. This change does not reset the distinct global catalog approval/qualification records.

## Verification

- `gofmt -w internal/recovery/backup_restore.go` — passed.
- `go build ./internal/recovery ./cmd/polis` — passed.
- `git diff --check` — passed.
- Tests were not run under the session's no-tests convention.
- No database/CAS restore, live WorkerSession, MCP endpoint or frozen CAP scenario was run. CAP-31 remains `not_run`; this evidence is source/build verification only.

## Files

- `internal/recovery/backup_restore.go`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
- `docs/implementation/CLOUD_CODEX_HANDOFF.md`
- `AGENTS.md`
