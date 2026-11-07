# R0.6 G checkpoint — Usage / Budget / Runtime Health

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added scoped Operations projection and existing-frontend Operations page.
- Reports persisted tool-call usage/limit with `reported`/`unavailable` quality, active worker count, PostgreSQL/CAS/event-stream status, and last runtime error kind.
- Input/output tokens and elapsed runtime remain explicitly null when the current runtime does not persist authoritative values.
- Operations remains separate from provider settings; it does not infer provider readiness or invent dollar budgets.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/workbench ./internal/control ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-g-pg` — schema 10 applied; `TestPostgresOperationsProjectionPreservesUnavailableTokenSemantics` passed.

## Known boundaries

- Token, dollar and elapsed-runtime accounting remains unavailable until an authoritative persistence source exists.
- Operations does not add a second scheduler or retry path.
