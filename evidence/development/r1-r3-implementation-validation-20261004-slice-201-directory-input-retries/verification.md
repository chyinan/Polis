# Slice 201 — directory MissionInput exact retry

## Scope

After a directory upload response is lost, keep the exact selected browser `File` objects, destination input ID, and request ID in the Workbench component state. Offer an explicit retry that resends this same payload, and clear it only after a successful receipt. The operator may explicitly discard the pending retry after checking refreshed MissionInput history.

## Verification

- `cd frontend && npm run build` — passed (`tsc -b` and Vite production build).
- `git diff --check` — passed.
- Vite reported its existing large-chunk warning (850.90 kB minified JavaScript); build succeeded.
- No automated tests were run.
- No MissionInput upload or other API command was submitted.
- No Worker/provider, host operation, frozen scenario, or schema change occurred.

## Limits

The pending browser `File` objects survive only while this page component remains mounted. If the page is reloaded or unmounted before reconciliation, the operator must inspect refreshed input history before starting a new upload. Durable recovery across browser restarts is not implemented here.
