# Slice 175 — REQ-39 command state refresh after ambiguous responses

## Changes

- Mission formal-change create, consider, decline and apply mutations now invalidate their persisted request history after either success or error. Apply additionally refreshes the Mission/Company summaries.
- Human takeover lease grant/release mutations now refresh the lease history after either outcome. Snapshot submission already received this behavior in Slice 174.
- Workbench errors for these operations now say the result is unconfirmed and direct the operator to the refreshed request/lease state, since a transport error can occur after the Kernel committed its idempotent receipt.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No Worker/provider operation or frozen scenario was executed. No schema/database change was made.

## Remaining boundary

These refreshes reconcile current persisted state but do not provide persistent client-side retry IDs for every formal-change or lease command. The complete safe-change, patch-import and exclusive handoff workflow and frozen REQ-39 qualification remain open.
