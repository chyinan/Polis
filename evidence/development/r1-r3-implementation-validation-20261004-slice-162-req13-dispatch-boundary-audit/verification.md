# Slice 162 — REQ-13 automatic dispatch boundary audit

Date: 2026-10-04

## Scope

Source audit of the automatic product Worker dispatcher: its fairness bounds, quota barriers, and the transition from a dispatch candidate to an authorized Worker turn. This slice did not change code or execute a Worker.

## Findings

- `internal/control/automatic_worker_dispatch.go:18-60` requires the opt-in dispatcher and a runtime implementing the Fake-only readiness gate, runs one attempt per 30-second tick, and bounds each attempt to 10 seconds. `internal/control/real_provider_worker.go:212-227` (see `AutomaticProductDispatchReadiness`) requires a business adapter, `fake` mode and the exact offline Fake @7 direct-messaging surface. `cmd/polis/main.go:608-625,769-777` controls startup and rejects automatic dispatch unless offline direct messaging is enabled with `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`.
- `internal/kernel/employee_schedule.go:21-61` queries only active Companies and Missions with a ready, unique `compat/emp-backend` Task, a validation binding and workspace, `wake_pending` schedule, no prior session for that Task and no live session for the Employee. `internal/control/automatic_worker_dispatch.go:63-78` advances a process-local Company cursor after each candidate attempt and holds the in-process Mission lifecycle lock. This limits one process to one admission attempt per cycle and provides a cursor walk within that process.
- `internal/control/real_provider_worker.go:521-562` treats the candidate as a hint and invokes the ordinary Worker startup path. `internal/kernel/worker_state.go:228-239,385-425` performs durable admission in a database write transaction, locks the schedule, rejects `paused`/`waiting_quota` and a non-stopped Employee session, and inserts a WorkerSession in `restoring` state. `internal/control/real_provider_worker.go:699-717` attaches and validates the persisted session, commits it as `active` through `TXActivateWorker`, then starts the run loop. The Worker turn therefore begins only after database-backed active-session admission.
- `internal/core/employee_schedule.go:24-34,41-48` preserves `waiting_quota` across work signals and schedule reconciliation. `internal/kernel/worker_state.go:389-390` rejects admission while that state is set. No production writer that makes a quota decision or automatically releases `waiting_quota` was found in the searched Go paths; existing code preserves the barrier without claiming quota readiness.

## Remaining gaps

Quota readiness and recovery are not authoritative or automated. The Company cursor and the 30-second loop are process-local; there is no shared cursor, global Worker slot accounting, or cross-instance fairness guarantee in this path. Database schedule/session locks protect admission for a given Employee, but do not establish a global concurrency cap across Companies.

## Verification and limits

- Source was inspected with `sed`, `grep` and `nl`; the documentation diff passed `git diff --check`.
- No tests, WorkerSession creation, Worker start/stop, provider call or database mutation was performed. The local Termux database has no Company or WorkerSession.
- REQ-13 remains open. This source audit does not qualify scheduler reconnects, quota recovery, multiple server instances, or FT execution.
