# R0.6 C checkpoint — Real-time Workbench updates

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added company-scoped SSE at the existing `/api/workbench/companies/:companyId/stream` route.
- SSE uses authoritative `company_seq` IDs, `Last-Event-ID`/cursor recovery, and `activity` event framing.
- PostgreSQL `LISTEN polis_company_events` is fed by the same transaction that commits an event; the server replays committed rows after the cursor before waiting for new notifications.
- Added response flush/cache headers and client-disconnect cancellation handling.
- Frontend now enables live updates by default in RealWorkbenchApi, preserves native EventSource reconnect behavior, and ignores duplicate/out-of-order company sequences before invalidating scoped queries.
- Added a distinct presentation mapping for `mission.draft` lifecycle events.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/workbench ./internal/control ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- `TestHandlerStreamsScopedActivityWithSequenceID` — passed; verified `id: <company_seq>` and `event: activity` framing.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-sse-pg` — migrations through schema 9 applied; `TestPostgresStreamReplaysCompanySequenceEvents` passed using real LISTEN/replay; cluster stopped and removed.

## Known boundaries

- The browser remains query-authority driven; SSE only invalidates scoped queries and never becomes a second state store.
- Cross-company streams are rejected by route scope and notification payload filtering.
