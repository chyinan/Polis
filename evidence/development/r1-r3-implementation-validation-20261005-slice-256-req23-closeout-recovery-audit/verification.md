# Slice 256 — REQ-23 closeout and restart recovery source audit

## Review

A read-only audit of the closeout path found no specific locally repairable fail-open in the reviewed code. Closeout persists the `closing` intent before attempting to stop project Jobs and Workers, and it aborts when a stop is not confirmed. The control and Kernel layers check for non-stopped WorkerSessions, active or outcome-unknown JobRuns, and current service leases; oversized JobRun enumeration fails closed. Worker stop evidence is bound to the exact session, PID, and stop proof. A missing in-memory Worker record after adapter restart does not bypass the final database outstanding-work check, and failed stdio MCP cleanup keeps stop/closeout from completing. Final settlement runs guarded checks again, and successful closure revalidates passed Artifacts and settled Tasks/Obligations. Schema 101 constrains closeout intent mutation and terminal transitions.

Reviewed paths include `internal/control/mission_closeout.go`, `internal/control/mission_lifecycle.go`, `internal/kernel/job_run_read.go`, `internal/kernel/worker_state.go`, `internal/control/real_provider_worker.go`, `internal/control/stdio_mcp_worker.go`, `internal/kernel/mission_closeout.go`, and `db/migrations/00101_mission_closeout_state_machine.sql`.

## Remaining qualification

This source review does not qualify process restart recovery. A Mission can remain in `closing` after interruption and requires retry with the original request identity and payload. Durable recovery across a real restart, representative persisted work, and frozen FT-57–60/72 scenarios remain open; PP-03 and PP-06 also remain `partial/not_run`. All 232 frozen scenarios remain `not_run`.

## Verification limits

- `git diff --check` passed in the audit worktree.
- No source change was made, so no build was run.
- No tests, database query/write, Worker, provider, or scenario operation ran.
