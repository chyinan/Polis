# Slice 169 — REQ-23 durable Mission closeout

Date: 2026-10-04

## Changes

- Schema 101 adds `closing` and `ended_not_met`, keeps the Company Mission slot occupied while a Mission is closing, and stores one immutable closeout intent plus its terminal report per Mission.
- Mission cancellation now records `closing` before Control stops project Jobs and Workers. Routine scheduling is disabled and employee schedules are quiesced in that same Company-guarded transaction.
- Finalization rechecks non-stopped WorkerSessions, active/unknown JobRuns, and live service leases in its guarded transaction. Non-success outcomes explicitly decline remaining obligations, cancel unfinished Tasks and settle pending Routine occurrences. The report records the requested outcome, rationale, acceptance references and resulting responsibility totals.
- The existing paused-Mission successor-change transaction now records the old Mission's cancellation closeout, marks handed-over obligations `superseded`, and includes the successor ID in its report instead of bypassing the closeout ledger.
- The Kernel permits only `succeeded`, `ended_not_met`, or `cancelled`. Success requires owner rationale, exact ready Artifacts with independent `passed` review, and fully settled Tasks/Obligations; it is never inferred from Task state alone.
- Workbench reads the durable closeout summary and renders `closing`, `ended_not_met`, and closeout report details.

## Verification

- `go build ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `go build -o .runtime/bin/polis ./cmd/polis` — passed.
- `npm run build` in `frontend/` — passed; existing Vite advisory reports the minified JS chunk exceeds 500 kB.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` — passed through migration 00101.
- `git diff --check` — passed.
- `.runtime/dev-termux.sh migrate` — applied migration 00101 successfully; local database is Schema 101.
- Read-only local database check — Company=0 and WorkerSession=0.

Tests, Worker actions, provider calls and frozen scenarios were not run. The local database has no Company or WorkerSession, so no Mission closeout runtime exercise was performed.

## Remaining REQ-23 work

The closeout Kernel path and cancellation orchestration are implemented, but owner-facing Control/Workbench selection for `ended_not_met` and evidence-backed `succeeded` is not yet wired. Restart recovery and FT-57–60/72 scenario qualification remain open. Late receipts must remain historical and must never reopen a terminal Mission.
