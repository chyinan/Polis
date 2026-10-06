# Slice296 verification: generic ActionIntent Operations projection

Date: 2026-10-07

## Scope

- Added optional Schema120 generic ActionIntent audit projection to Workbench Operations.
- SQL selects only each Intent's latest `denied` event before the bounded 32-row limit, scoped by Company; raw input is never exposed, only resource and digest metadata.
- Frontend validates the requested Company ID, denied state, digests and bounded fields; older schemas omit the optional projection safely.
- No external action, Provider, browser, database runtime, account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/workbench -run TestHandlerServesOperationsProjection -count=1` — passed.
- `frontend/node_modules/.bin/vitest.cmd run src/domain/workbench-validation.test.ts` — 50/50 passed.
- `frontend/node_modules/.bin/tsc.cmd -b --pretty false` — passed.
- `frontend/node_modules/.bin/vite.cmd build` — passed.
- `git diff --check` — passed.
