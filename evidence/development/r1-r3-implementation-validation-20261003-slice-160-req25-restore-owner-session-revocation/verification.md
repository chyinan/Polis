# Slice 160 — REQ-25 restored installation-owner session revocation

Date: 2026-10-03

## Finding

The recovery package excludes external credential files, but the restored PostgreSQL database includes installation-owner session token digests. `installation_owner_sessions` (Schema 97) treats those rows as bearer-session authorization until their `revoked_at` is set. Restoring an older database without a new restore-generation fence could therefore make a token from that backup valid again. Frozen FT-77 includes old-token behavior after database restore.

## Change

`internal/recovery/backup_restore.go` now acquires the control-plane advisory lock (`714209831`) and the installation-owner auth advisory lock (`714209843`) on one pinned PostgreSQL connection and holds both through completion-marker write. This prevents Kernel startup and serializes owner login/bootstrap mutations with recovery finalization.

For restored databases at Schema 97 or later, finalization revokes every still-unrevoked row in `installation_owner_sessions` and inserts one immutable `session_revoked` row in `installation_owner_auth_events` per token digest. Both operations share a transaction with Slice159's MCP Employee-binding revocations. Raw owner tokens are never stored or emitted. The owner password remains in place; a legitimate owner can log in again after recovery.

The generation marker now requires both `mcpBindingsRevoked` and `ownerSessionsRevoked`. A marker written by Slice159 is treated as unfinished and upgraded through the staged, idempotent path. `VerifyRestoredRecoveryGeneration` also requires both flags. Backups before Schema97 have no installation-owner session table and skip that revocation step.

## Verification

- `gofmt -w internal/recovery/backup_restore.go` — passed.
- `go build ./internal/recovery ./cmd/polis` — passed.
- `git diff --check` — passed.
- Tests were not run under the session's no-tests convention.
- No database/CAS restore, owner login/browser request, WorkerSession, endpoint or frozen scenario was run. FT-77 and CAP-31 remain `not_run`; REQ-25 remains open pending other gaps and qualification.

## Files

- `internal/recovery/backup_restore.go`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
- `docs/implementation/CLOUD_CODEX_HANDOFF.md`
- `AGENTS.md`
