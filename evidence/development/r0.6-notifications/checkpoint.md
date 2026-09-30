# R0.6 H checkpoint — Notifications baseline

> Date: 2026-09-21  
> Status: PASSED local/deterministic baseline; external route unqualified

## Delivered

- Added durable notification route, intent and delivery schema with enabled/disabled/status separation.
- Added event-to-intent persistence for terminal/provider failure/operator/artifact event categories; notification persistence failure is isolated from the event commit path.
- Added deterministic `local` adapter and a webhook adapter boundary; local test delivery records an explicit delivered result without external network activity.
- Added Notifications settings/readback/test UI and API routes with redacted route state.
- No QQ send, provider call or external account was used.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-h-pg` — schema 11 applied; `TestPostgresLocalNotificationDeliveryIsReadable` passed; cluster stopped and removed.

## Known boundaries

- External QQ/webhook delivery remains disabled/unqualified until separately authorized and tested.
- Delivery failure isolation and local readback are implemented; no claim is made about remote platform acceptance.
