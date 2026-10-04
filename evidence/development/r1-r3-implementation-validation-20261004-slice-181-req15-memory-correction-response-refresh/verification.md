# Slice 181 — REQ-15 memory correction command reconciliation

## Changes

- Memory correction proposal and independent review now invalidate the correction queue, Company overview and activity on settle.
- UI errors identify uncertain outcomes and direct operators to the refreshed queue/memory state. Existing request IDs remain available for exact retries.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No correction was created or reviewed, no Worker/provider action occurred, and no frozen scenario ran. No schema/database change was made.

## Remaining boundary

These commands still require a database-confirmed active WorkerSession and the approved independent reviewer role. This change only reconciles Workbench projections; restore-resistant revocation and FT-37/40/41 qualification remain open.
