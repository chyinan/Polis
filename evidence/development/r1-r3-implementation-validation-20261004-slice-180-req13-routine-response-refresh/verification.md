# Slice 180 — REQ-13 Routine command-response reconciliation

## Changes

- Routine creation and legacy task-instruction repair invalidate Routine history, Company/Mission overview and activity after either success or error.
- Existing exact retry request IDs remain in place. Error messages identify the outcome as unconfirmed and direct the operator to check updated Routine and Task state.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No scheduler iteration, Worker/provider action or frozen scenario was executed. No schema/database change was made.

## Remaining boundary

This is UI state reconciliation only. Due occurrence materialization, Worker admission, provider execution and frozen REQ-13 qualification remain governed by their existing gates and are not qualified here.
