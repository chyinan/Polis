# Slice 63 — Startup reconciliation for interrupted project JobRuns

Date: 2026-09-29  
Schema: 47 (no migration)

## Changes

- Added a kernel read for active-company JobRuns whose latest state is `outcome_unknown`, scoped by the fixed Windows or Linux Node profile.
- Startup now enumerates unknown Jobs for profiles supported by the configured executor. On native Windows, it also scans Windows Jobs when Node preparation is disabled.
- Startup routes each candidate through the existing Stop path. The Windows fallback opens the persisted named Job Object, terminates any remaining members, and waits for an empty process tree before recording `cancelled`.
- Service Jobs use the existing persisted endpoint revocation path. If process containment cannot be confirmed, startup returns an error and does not replay the Job.

## Verification

- `bash scripts/go.sh fmt ./internal/kernel ./internal/control` — passed.
- `bash scripts/r1-capability-source-postgres-test.sh` — passed on a disposable PostgreSQL 18 database at Schema 47. It includes a recovery integration test with an injected Windows Job reconciler; the test confirms an unknown Job is stopped and recorded `cancelled`.
- `bash scripts/go.sh test ./... -count=1` — passed. Database-backed tests without `POLIS_TEST_DSN` skipped by their existing contract; the dedicated script above ran the relevant PostgreSQL cases.
- Linux amd64 and Windows amd64 `cmd/...` builds — passed.
- `rtk git diff --check` — passed.

No native Windows Job Object, WFP filter, Node/npm project, real model, QQ, external service or production database was run. The direct named-object fallback compiled for Windows; its host behavior remains unqualified.
