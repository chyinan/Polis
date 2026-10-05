# Slice 225 — REQ-13 fairness coverage reconciliation

## Change

Reconciled current REQ-13 coverage with Schema 102/Slice 173. Candidate selection locks a persisted shared Company cursor and creates an expiring per-Task claim in the same transaction, superseding the older process-local cursor findings in Slices 162/166. The dispatcher remains opt-in and restricted to Fake @7. No owner-backed global slot cap or authoritative provider quota readiness/recovery source is present, so `waiting_quota` remains fail-closed.

Added this source evidence to each of the nine REQ-13 scenario rows in the implementation crosswalk. The row count remains 232; disposition counts remain {"implemented":120,"partial":112,"planned":0,"excluded":0}; all 232 execution states remain `not_run`. All nine mapped rows remain `partial` / `not_run`.

## Verification

- Parsed the crosswalk JSON and checked all nine REQ-13 rows cite this evidence.
- Confirmed the 232 scenario records and all execution states remain unchanged at `not_run`.
- `git diff --check`: passed.
- No tests, database operation, Worker/provider activity, migration or frozen scenario ran.

## Status

REQ-13 remains partial. Shared candidate rotation/claims and bounded opt-in Fake dispatch are implemented; an owner-backed global concurrency limit, authoritative quota-readiness/recovery and production dispatch remain open.
