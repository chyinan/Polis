# Slice 176 — REQ-23 lifecycle response reconciliation

## Changes

- Mission start, pause and resume mutations now invalidate company overview and activity queries in `onSettled`, so server errors and lost responses both trigger an authoritative state refresh. Cancel and closeout already followed this pattern.
- Lifecycle controls distinguish a conflict from an unconfirmed result and direct the operator to use the refreshed Mission state before continuing.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No Worker/provider action or frozen scenario was executed. No schema/database change was made.

## Remaining boundary

The UI refresh closes the stale-state window; it does not qualify pause/resume/start across a real process restart. REQ-23 frozen FT-57–60/72 scenarios remain `not_run`.
