# Slice 178 — Workbench setup/config response reconciliation

## Changes

- Company and Mission creation, Company update/archive, runtime-settings changes and operator-instruction submission now invalidate their existing authoritative queries after success or error.
- Error presentations describe the command outcome as unconfirmed and point to company directory, refreshed configuration, or instruction history before another action.

## Verification

- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No Worker/provider action or frozen scenario was executed. No schema/database change was made.

## Remaining boundary

This reconciles UI state after ambiguous responses but does not qualify runtime restart, external provider behavior, or frozen requirement scenarios. Existing R1–R3 host and restart qualification remains open.
