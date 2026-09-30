# Slice 83 — Restored-generation verification

Date: 2026-09-30  
Worktree: `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`  
Database schema: 60; no new migration.

## Behavior

- `VerifyRestoredRecoveryGeneration` re-verifies the backup package manifest and CAS inventory before checking a target.
- The target database restore comment and adjacent CAS marker must match the package generation ID and manifest SHA-256.
- The target database schema version must match the package. The target CAS tree must contain exactly the manifest entries with matching paths, sizes and hashes.
- The API only reads the target database and files. `polis recovery-generation-verify PACKAGE_DIRECTORY BLOB_ROOT` reads its target DSN from `POLIS_GENERATION_DSN`; it never places the DSN in argv. Connection and query errors are sanitized.
- Verification does not stop source writers, switch Desktop's active generation, or claim a maintenance window. Those checks belong to the cutover coordinator.

## Verification results

- Test-first PostgreSQL round-trip assertion for a valid restored generation and rejection of a different package — passed in the dedicated integration below.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/r2-recovery-backup-postgres-test.sh` — passed with a dedicated temporary PostgreSQL 18 cluster through Schema 60; output ended `R2_RECOVERY_BACKUP_RESTORE=PASSED`. The script exercises CLI backup, restore, generation verification, mismatch refusal and restored Workbench readback, then cleans only its bounded temporary resources.
- `GOOS=windows GOARCH=amd64` command build for `./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64` cross-compilation of the `cmd/polis` recovery test binary — passed.
- `rtk git diff --check` — passed.
- Independent read-only review found no Critical, Important or Minor findings.

No production database, real provider, QQ, MCP, GitHub account or external service was used. The verifier records readiness evidence only; Desktop cutover and rollback remain to be implemented.
