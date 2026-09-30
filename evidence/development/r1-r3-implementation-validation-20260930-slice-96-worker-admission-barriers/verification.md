# Slice 96 — Worker admission barriers

Date: 2026-09-30

## Change

`TXNewWorker` now locks the owning `employee_schedules` row inside the same transaction that reserves a WorkerSession. It refuses persisted `paused` and `waiting_quota` states before changing the employee epoch or inserting a WorkerSession. A successful reservation records schedule `admitted` alongside WorkerSession `restoring`; activation moves the schedule to `working`. Reconciliation preserves `admitted` for `restoring`, `validating`, and `activation_pending_environment` sessions and uses `working` for active or stop-unconfirmed sessions. Mission resume applies the same classification. No migration was needed. The schedule row remains the admission barrier; Task and WorkerSession rows remain authoritative for work and execution.

The dedicated PostgreSQL regressions first failed for each missing behavior: the paused and waiting-quota requests returned nil and left WorkerSessions behind; a restoring session's schedule changed from `admitted` to `working` on reconciliation. After the fixes, both barriers return `core.Denied` without changing the schedule or creating a session, reservation atomically leaves the schedule `admitted`, and reconciliation preserves that state until activation.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed at Schema 70; `R1_EMPLOYEE_SCHEDULE_WAKE_PROTOCOL=PASSED`.
- `rtk bash scripts/go.sh test ./internal/core -count=1` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -lc "GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/..."` — passed.
- `rtk git diff --check` — passed.
- Independent read-only review initially found that reconciliation could label a restoring session `working`; the implementation now distinguishes admitted and active sessions. Re-review found no actionable findings and confirmed the company-then-schedule lock order, barrier checks before writes, and atomic session/admitted-state reservation.

This closes the WorkerSession bypass of persisted pause/quota barriers and makes the documented admitted-to-working state observable without prematurely marking restoring sessions as working. It does not implement quota readiness detection/recovery, fairness, Routine-to-Task delivery, or automatic Worker dispatch. No process, real Worker, provider, QQ, MCP endpoint, GitHub account, or production resource was used.
