# Slice 91 — bounded daily Routine planner

Date: 2026-09-30  
Schema: 62 (no migration)

## Scope

Implemented the pure calculation core for a single daily Routine schedule. It creates stable keys from the Routine ID and local logical day, supports `skip`, `coalesce_latest`, and bounded `catch_up`, and caps catch-up at ten generated occurrences. Time-zone behavior is explicit: host-local `Local` is rejected, DST folds select the earlier UTC instant, and nonexistent wall times move to the first valid local minute on the same logical day.

No Routine persistence, occurrence uniqueness ledger, wake signal, Task creation, listener/reconnect loop, fairness/quota scheduling, or Worker admission was added. REQ-13 remains partial.

## Verification

- `rtk bash scripts/go.sh test ./internal/core -run TestPlanDailyRoutineOccurrences -count=1` — passed.
- Independent read-only review of `internal/core/routine_schedule.go` and `internal/core/routine_schedule_test.go` — no Critical, Important, or Minor findings; prior DST-gap and host-local-time findings are resolved.
- No schema migration, external service, model, Worker, or account was used.

This evidence covers only the daily planner calculation. It does not qualify a persisted Routine execution path or REQ-13 as complete.
