# Slice 56 verification: Worker input delivery and receipt locking

Date: 2026-09-28. Database schema: 44. No migration was added.

## Implemented

- Extended `scripts/r1-capability-source-postgres-test.sh` to exercise the frozen-input path through the fake Worker turn for text, PNG/JPEG, directory, fixed-commit Git and PDF sources. Tests inspect the actual prompt or typed image bytes received by the fake provider session, then verify manifest digest, selected files and zero provider egress in the final delivery receipt.
- Fixed the Slice 55 review findings: multipart spill files are removed before file-count rejection; invalid UTF-8 manifests are rejected before JSON decoding; server and browser validate package paths against the same 1,024 UTF-8-byte limit; and the immutable-history test updates a real Schema 44 revision row and checks the trigger error.
- Skill and MCP CAS-first imports share a company/request-ID session lock, acquired before the package revision lock on one PostgreSQL connection. The revision key remains compatible with metadata registration.
- Ordinary `TXWrite` calls use `pg_try_advisory_xact_lock` on the shared receipt key. A conflict rolls back immediately, releases the main-pool connection, and checks for a committed matching receipt before returning conflict. Source imports mark their already-held lock with a private context value so their own transaction does not reacquire it.
- Added integration coverage for cross-Skill/MCP request-ID reuse and sixteen simultaneous metadata writes under an import-held request lock. The metadata writes return conflicts without waiting; a new write succeeds after release, and a committed matching receipt still replays while the source lock is held.

## Verification

- `rtk bash scripts/r1-capability-source-postgres-test.sh` — PASS at Schema 44. Includes capability import/approval, MCP package CAS/immutability/replay, request-lock contention, Worker input delivery, Control and Workbench integration tests.
- `rtk bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS at Schema 44.
- `rtk bash scripts/r2-recovery-backup-postgres-test.sh` — PASS at Schema 44 (`R2_RECOVERY_BACKUP_RESTORE=PASSED`).
- `rtk bash scripts/r3-domain-evidence-postgres-test.sh` — PASS at Schema 44.
- `rtk bash scripts/go.sh test ./... -count=1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — PASS.
- `rtk npm run test -- --run` — PASS, 102 frontend tests. `rtk npm run typecheck`, `rtk npm run lint`, and `rtk npm run build` — PASS. Vite retains the existing large-chunk advisory.
- Final read-only review of the shared request key, transaction try-lock and pre-held-lock context path found no remaining findings.

## Qualification limits

- All Worker input delivery checks use the deterministic fake runtime. No real employee model turn ran.
- No MCP process or endpoint, QQ send, GitHub account, WFP policy change, production database or deployment was used.
- Windows AppContainer/WFP and clean-VM installation/update remain unqualified. Linux/Node host isolation and restart recovery remain open.
- R3 content-operations and research profiles remain `not_run`; the scripts validate evidence contracts, not real domain quality or organization benefit.
