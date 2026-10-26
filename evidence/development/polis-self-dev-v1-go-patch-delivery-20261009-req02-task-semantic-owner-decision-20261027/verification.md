# Slice 331 — REQ-02 TaskRevision owner decisions

Date: 2026-10-27
Result: `TASKREVISION_OWNER_DECISION_WIRED`; semantic qualification and execution remain unchanged.

## Implemented

- The durable Schema123 TaskSemanticRevision decision handler was unreachable: `parsePath` had no matching task-scoped route. Added `POST /companies/{companyId}/tasks/{taskId}/semantic-revision/decision`; the handler binds Task ID from the URL, ignoring any body-supplied Task ID.
- Owner authentication and CSRF are required in Workbench HTTP and checked again in Control. The real Workbench API sends the exact revision, binding digest, decision, rationale and stable request ID; fixture mode rejects the command.
- Task detail now shows the exact TaskRevision and RoleRevision digests, current owner decision and audit metadata, and exposes only lifecycle-valid controls: approve/reject for proposed; revoke for approved. The UI states that the decision is descriptive only and does not qualify or enable execution. Same-binding/reason retries reuse the request ID.
- The existing Schema123 monotonic decision lifecycle is unchanged: `proposed -> approved|rejected`, followed only by `approved -> revoked`. No decision is consulted as Worker admission or provider execution authority.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Frontend TypeScript project build/typecheck, ESLint, Vite build and Vitest: PASS — `frontend-typecheck.log`, `frontend-lint.log`, `frontend-build.log`, `frontend-tests.log` (19 files, 171 tests).
- Migration hash-manifest regression test: PASS — `migration-manifest-test.log`; no schema migration was needed.
- Disposable PostgreSQL 18.6 / Schema 136 integration: PASS — owner approval and revocation persist append-only, qualification remains `unverified`, human handling remains required, and revoked decisions cannot be resurrected. Evidence: `postgres-runtime/20261026T175924Z-375/`.
- HTTP tests verify the exact route and deny unauthenticated owner decisions. No Worker, Provider, external network, or production database operation ran.

## Qualification boundary

Owner approval/rejection/revocation records review intent only. Role qualification, execution policy and real-provider/host qualification remain open; no frozen scenario status changed.
