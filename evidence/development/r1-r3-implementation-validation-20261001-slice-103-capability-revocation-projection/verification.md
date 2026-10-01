# Slice 103 verification — capability revocation status projection

Date: 2026-10-01  
Host: Termux, Android/arm64  
Schema: 72; no migration

## Changes

- The company capability catalog now derives current global and employee-scoped Skill/MCP revocation status from durable governance rows and binding events.
- The projection reports accepted/effective-for-new-dispatch separately from quiescence, and inventories sessions with recorded Skill loads or MCP intents plus MCP call outcomes.
- Quiescence requires every inventoried session to be persisted as `stopped` and no matching MCP intent to remain `dispatching`. `reconcile_required` counts as live; `outcome_unknown` remains in the inventory.
- The Workbench capability page displays the projection. Detailed status is capped at 64 revocations and 64 sessions/calls per revocation; aggregate counts use all matching rows and truncation is explicit.
- The projection is read-only and rebuilt in the catalog's repeatable-read transaction. It does not use process-local state or add a migration.

## Verification

- `go build ./cmd/...` — passed for the local Android/arm64 host.
- `npm run build` from `frontend/` — passed (`tsc -b` and Vite production build). Vite reported the existing-size class warning for the 740.88 kB minified JavaScript chunk.
- `git diff --check` — passed after the final documentation/code edits.
- Tests were not run. The SQL projection and its lifecycle semantics have therefore not been exercised against PostgreSQL in this slice.

No provider, MCP endpoint, QQ, GitHub account, production, or real Worker action was run. No persistent development database was modified.

## Limits and next step

This projection reports currently effective revocations and sessions with a durable Skill-load event or MCP intent. It does not snapshot bound-but-unused WorkerSessions at the revocation linearization point, stop Workers, or list superseded historical revocations. The next REQ-14 slice should capture the exact affected session set transactionally, then add idempotent Worker-stop and restart-reconciliation behavior before exposing a management action.
