# Slice 336 — REQ-02 RoleRevision fail-closed guard

Date: 2026-10-29
Result: `MUTATED_OR_STALE_ROLE_REVISION_DENIED`.

## Implemented

- `FixedTeamCoverageRoleRevision` exposes public fields and a nested mutable contract. `TaskRevisionFor` and `AdmissionFor` now verify the compiled marker, exact current SHA-256, fixed owner decision, `unverified` qualification, and the exact embedded contract before returning coverage.
- Missing, stale, revoked, or mutated role revisions now return an unavailable TaskRevision with `qualification=unverified` and `requires_human=true`; Admission remains uncovered and human-required.
- The compiler regression verifies revoked decisions and `qualified` rows without a separate qualification process are rejected. The existing monotonic TaskRevision decision tests continue to deny approval resurrection after revocation. No qualification or execution path was added.

## Verification

- TDD red check: the new focused regression failed on stale/revoked qualification leakage and a mutated nested contract being treated as a valid semantic mapping.
- Focused `spec` regressions: PASS.
- Full Go suite: PASS (`go-tests.log`).
- Full Go race suite: PASS (`go-race.log`).
- `go vet ./...`: PASS (`go-vet.log`).
- Linux amd64 and Windows amd64 command builds: PASS (`linux-build.log`, `windows-build.log`).
- `POLIS_TEST_DSN` was unset; PostgreSQL integration tests requiring it were skipped. No database, Worker, Provider, or external operation ran.

## Boundary

RoleRevision/TaskRevision approval remains descriptive. Every current fixed-team revision remains unverified and human-gated; this change does not qualify roles, enable dispatch, or alter provider surfaces.
