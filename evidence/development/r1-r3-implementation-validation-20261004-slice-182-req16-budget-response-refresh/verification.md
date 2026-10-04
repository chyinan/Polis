# Slice 182 — REQ-16 budget command reconciliation

## Changes

- Company, Mission, ProblemKey and Task budget mutations invalidate their authoritative budget and related Company/activity projections on settle.
- ProblemKey and Task incremental allocations preserve the exact request body and RequestID after an ambiguous response. The form locks that payload for exact receipt retry; the operator can release it for a new confirmed command only after checking the refreshed budget.
- Source review confirms both allocation routes enter Kernel `TXWrite`, which fingerprints the operation/input, returns an exact stored receipt before invoking the new allocation callback, and conflicts on a changed payload under the same request ID.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No Worker/provider action or frozen scenario was executed. No schema/database change was made.

## Remaining boundary

These limits account for admitted protocol tool calls only. They do not establish ProviderAccount liability, observe hidden provider retries, or account for token/money charges. REQ-16 and FT-42–45 remain partial/unqualified.
