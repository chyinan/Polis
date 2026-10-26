# Slice 330 — REQ-02 historical fixed-role reconciliation

Date: 2026-10-27
Result: `OWNER_REVIEWED_FIXED_ROLE_REPAIR_IMPLEMENTED`; qualification remains `unverified` and execution remains disabled.

## Implemented

- Company summaries expose a read-only list of fixed Employee IDs whose persisted `role_name` differs from the repository's canonical fixed mapping. The SHA-256 binds Company ID and the exact ordered current/expected assignment pairs; roster ordering does not affect it.
- An installation-owner-only Workbench command requires CSRF, explicit rationale and the preview digest. The Kernel recomputes the current snapshot inside the write transaction and refuses stale snapshots, no-op repair, inactive Companies, and any active Mission, non-stopped WorkerSession, or accepted/starting/running/outcome-unknown JobRun.
- A transaction advisory lock serializes the repair against Mission activation, WorkerSession admission, ProjectJob session creation, canonical team confirmation, organization role edits, probe/peer Mission creation, and Company archive. It avoids taking the Company row lock before subsystem budget/task locks.
- The command changes only the mismatching `role_name` values to the canonical fixed roles and appends a durable audit event with before/after digests, mismatch list, request ID and owner rationale. It does not alter RoleRevision qualification, confirm the matrix, or enable role execution.
- The Group company directory shows exact current-to-fixed differences and requires explicit owner review before enabling repair. Same-snapshot/reason retries reuse the request ID after an uncertain response.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Frontend TypeScript project build/typecheck, ESLint, Vite build and Vitest: PASS — `frontend-typecheck.log`, `frontend-lint.log`, `frontend-build.log`, `frontend-tests.log` (19 files, 169 tests).
- Migration hash-manifest regression test: PASS — `migration-manifest-test.log`; no schema migration was needed.
- Disposable PostgreSQL 18.6 / Schema 136 integration: PASS — exact owner snapshot repair, changed-snapshot conflict, audit event, qualification remains unverified, no-op rejection, active-Mission denial and unauthenticated HTTP denial. Evidence: `postgres-runtime/20261026T171421Z-388/`.
- All database writes were confined to the harness's disposable, Unix-socket-only database. No Worker, Provider, external network, or production database operation ran.

## Qualification boundary

This closes the historical source-level drift-repair gap only. It is not evidence that any role is qualified or executable, and no R1–R3 qualification or frozen scenario status was changed.
