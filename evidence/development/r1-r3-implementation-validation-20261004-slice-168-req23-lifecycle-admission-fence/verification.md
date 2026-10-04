# Slice 168 — REQ-23 Mission lifecycle admission fence

Date: 2026-10-04

## Audit finding

Control stops WorkerSessions and Project JobRuns and checks for outstanding execution before it asks Kernel to pause or cancel the Mission. Those calls used separate transactions. Between the final stop check and the lifecycle write, another admission could still observe an active Mission and commit a WorkerSession, JobRun or service lease.

Kernel `TXWrite` obtains the Company row `FOR UPDATE`, and WorkerSession admissions and lifecycle commands use that same Company guard. The pause/resume and cancellation callbacks previously did not repeat the outstanding-work check while holding it. See [kernel.go](../../../internal/kernel/kernel.go), [worker_state.go](../../../internal/kernel/worker_state.go), [mission.go](../../../internal/kernel/mission.go), and [mission_lifecycle.go](../../../internal/control/mission_lifecycle.go).

## Change

The existing `MissionHasOutstandingJobWork` predicate is now shared between the read path and guarded lifecycle writes. Pause, resume and cancel recheck it inside their Company-serialized transaction. It denies the transition with `reconcile_required` while any non-stopped WorkerSession exists, a JobRun's latest state is accepted/starting/running/outcome_unknown, or a service endpoint remains non-revoked and unexpired. A new admission either commits first and is observed by this check, or waits for the lifecycle transaction and then sees the non-active Mission.

No database migration was required. This closes the lifecycle admission race but does not implement the full terminal closeout contract. The design calls for a `closing` phase and terminal outcomes including `ended_not_met`; those transitions and durable responsibility settlement remain absent. REQ-23 remains partial, and FT-57–60/72 plus PP-03/06 remain `not_run`.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` passed.
- `git diff --check` passed.
- No tests were run or added.
- No WorkerSession or Project JobRun was created, stopped, or dispatched; no provider turn or egress occurred.

