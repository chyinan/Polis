# Slice 66 — R3 global domain qualification path

Date: 2026-09-29  
Schema: 50

## Changes

- Added a strict `r3-domain-profile-qualification@1` aggregate report. It must use one exact content/research profile revision and cover every required area with one or more record ID, evidence digest and substantive assessment ID references.
- Added append-only migrations 49–50 for company/profile qualification and revocation events. Migration 49's previously pinned hash was preserved. Migration 50 adds a monotonic event sequence and serializes decisions on the company row.
- Kernel reads the report from a same-company CAS MissionInput, checks its digest and report size, then revalidates each cited submission, accepted reference review, overall substantive outcome and area-specific accepted decision before recording an owner decision. Revocation requires a current qualification. Failed or cross-company references do not create an event.
- The Workbench shows the latest company-scoped profile decision, generates a bounded aggregate JSON report from accepted case reviews, accepts an owner qualification/revocation command, and always shows execution disabled because no R3 runtime exists.
- Bounded CAS reads now accept an explicit caller limit up to 8 MiB while the legacy reader remains at its prior smaller default. The qualification-report path requests only its 64 KiB cap and rejects oversized CAS files before reading.

## Verification

- Red/green CAS bound test: `TestReadBlobBoundedAllowsAnExplicitLimitAboveLegacyContentSize` failed before the explicit bounded reader allowed the requested limit, then passed. `TestReadBlobBoundedRejectsOversizedCASBeforeRead` also passed.
- `bash scripts/r3-domain-evidence-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 50. It exercised content/research case evidence review, owner qualification, revocation, company isolation, report/case digest verification, profile projection and disabled execution.
- `bash scripts/r1-capability-source-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 50.
- `bash scripts/r2-cross-backend-handover-postgres-test.sh` — passed, including migration evidence and cross-backend handover checks at Schema 50.
- `bash scripts/r2-recovery-backup-postgres-test.sh` — passed at Schema 50.
- `bash scripts/go.sh test ./... -count=1` — passed. Database-backed tests without `POLIS_TEST_DSN` skipped under their test contract; the R1/R2/R3 disposable suites ran the relevant database cases.
- Linux amd64 and Windows amd64 `cmd/...` builds — passed. The Windows result is a cross-build; no native Windows APIs were exercised in this slice.
- Frontend Vitest: 113/113; TypeScript build check and ESLint with zero warnings passed. Vite production build passed with the existing large-chunk advisory.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 50 migration pins passed. `rtk git diff --check` passed.

## Qualification boundary

All aggregate qualification evidence used in tests is synthetic and stored only in disposable databases. The active project has no real content-operations or research evidence, so both profiles remain `not_run`. An owner-qualified evidence profile still cannot execute: execution remains disabled until a domain runtime is implemented and separately qualified. No real model, QQ send, MCP/GitHub account, or production database was used.
