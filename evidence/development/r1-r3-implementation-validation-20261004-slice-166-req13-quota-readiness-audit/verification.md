# Slice 166 verification — REQ-13 quota readiness/recovery boundary audit

Date: 2026-10-04

## Source findings

- `db/migrations/00061_employee_schedule_wake_protocol.sql` allows `waiting_quota` as a persisted EmployeeSchedule state.
- `internal/core/employee_schedule.go` preserves that state when actionable work arrives (`RecordEmployeeWorkSignal`) and when sleep reconciliation runs (`ReconcileEmployeeSleep`). `ResumeEmployeeSchedule` only transitions a paused schedule; it does not clear quota state.
- `internal/kernel/worker_state.go` locks the EmployeeSchedule in the WorkerSession admission transaction and rejects both `paused` and `waiting_quota` before reserving a WorkerSession.
- `internal/kernel/employee_schedule.go` selects automatic candidates only when the persisted schedule is `wake_pending` and rejects any non-stopped session for that Employee.
- `internal/control/automatic_worker_dispatch.go` starts only through the opt-in zero-egress Fake @7 readiness interface. It makes one attempt per 30-second interval and advances a process-local Company-ID cursor; it does not provide shared/global slots or cross-instance cursor state.
- A repository-wide search of non-test Go code found no production writer that sets or releases `waiting_quota`. The only non-test Go references to `provider_quota_exhausted` are notification reason labels. No authoritative provider quota readiness reader or account-level recovery signal is connected to EmployeeSchedule.

The current barrier is fail-closed. Because there is no trusted indication that provider quota is available again, automatically releasing `waiting_quota` would invent readiness. No recovery writer was added. Provider quota recovery, shared fairness and global slot accounting remain open under REQ-13.

## Validation

- Source review only; no code or schema changed.
- `git diff --check` — passed.

## Not run

No tests were run. No WorkerSession was created, started, or stopped. No provider/API/account traffic ran. No live database mutation was made. The local development database remains at Schema 100 with no Company, installation owner, or WorkerSession.
