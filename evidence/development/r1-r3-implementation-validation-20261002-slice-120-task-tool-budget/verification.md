# Slice 120 — cumulative Task tool-call budget

Date: 2026-10-02

## Change

Schema 81 adds a nullable cumulative tool-call limit and a durable usage count to each Task. A null limit means no WorkerSession has fixed the Task envelope yet; the first admitted WorkerSession fixes it to that session's authorized limit. `0` retains the existing unlimited meaning. Existing bounded WorkerSessions are backfilled by summing their limits and usage; if any historical session for a Task was unlimited, the migrated Task remains unlimited.

Kernel admission refuses a new WorkerSession when a bounded Task budget is exhausted. Every accepted employee tool call increments the session and Task counters atomically. The effective remaining allowance is the lesser of the session and Task remaining amounts and is included in Handover. The real product-provider adapter clamps the protocol tool-call limit to that remaining Task amount and records its start snapshot and effective turn limit with terminal usage.

This protects the same Task across reviewed successor sessions. It does not compose a ProblemKey across Tasks, set Mission/Company/Provider budgets, reserve closing capacity, measure money or tokens, account for hidden CLI retries, or close a Task with a stable `blocked_no_progress` outcome. It does not turn an unlimited `0` limit into a cap.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 81 migrations passed.
- `git diff --check` — passed.
- Tests were not added or run. PostgreSQL migration/runtime and provider execution were not run.

## Files

- `db/migrations/00081_task_tool_call_budget.sql`
- `db/migration_hashes.sha256`
- `internal/kernel/worker_state.go`
- `internal/kernel/worker.go`
- `internal/kernel/kernel.go`
- `internal/control/real_provider_worker.go`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
- `AGENTS.md`
