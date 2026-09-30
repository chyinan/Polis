# Slice 82 — Migration attempt journal verification

Date: 2026-09-30  
Worktree: `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`  
Database schema: 60; no new migration.

## Implemented boundary

- `migrate` and `migrate-to` require a journal start record to be persisted before database access.
- The journal root is resolved through existing symlink ancestors and rejected when it overlaps the configured CAS root, including direct, ancestor/descendant and symlink-parent aliases. Filesystem roots and unexpected journal entries are refused.
- Start/finish JSON files are exclusive and immutable. They contain attempt ID, target version, embedded migration manifest digest, build identity, timestamp and bounded outcome; they omit the DSN and raw error. Missing finish records report `outcome_unknown`.
- Unix synchronizes event files and containing directories, and re-synchronizes the parent directory whenever the journal root is ensured. Windows synchronizes temporary file content and publishes event files and a newly created root with same-volume `MoveFileEx(MOVEFILE_WRITE_THROUGH)`.
- `scripts/go.sh` sets a workspace-local `GOTMPDIR/migration-attempts` default for development/test commands. A deployed CLI uses `POLIS_MIGRATION_ATTEMPT_ROOT`, or a sibling of `POLIS_BLOB_ROOT`.

## Verification results

- `rtk bash scripts/go.sh test ./cmd/polis -run Migration -count=1` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/r1-service-endpoint-postgres-test.sh` — passed with its dedicated temporary PostgreSQL 18 cluster through Schema 60. It applied migrations via the CLI, verified `migration-attempt-status` reports `applied`, then passed the existing Workbench, environment replay and Task workspace restart checks. The script's temporary database, cluster and files were cleaned by its bounded trap.
- `GOOS=windows GOARCH=amd64` command build for `./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64` cross-compilation of the `cmd/polis` migration test binary — passed.
- `rtk git diff --check` — passed.
- Three rounds of independent read-only review closed all reported findings; the final review found no Critical, Important or Minor issues.

Native Windows execution was unavailable, so Windows durability remains cross-compiled and source-reviewed rather than host-exercised. No real provider, QQ, MCP, GitHub, production database, or business account was accessed. The Windows write-through behavior follows the documented [`MoveFileEx` contract](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexa).
