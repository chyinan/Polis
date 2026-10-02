# Slice 121 — immutable ProblemKey Task lineage

Date: 2026-10-02

## Change

Schema 82 adds immutable `parent_task_id` and `problem_key` fields to Tasks. A same-Mission parent is enforced by a composite foreign key. An insert trigger assigns root keys as `problem:<task-id>` and copies the parent's key for children; caller-provided keys are overwritten. An update trigger prevents changing either field. The migration backfills only unambiguous product `compat`, planning-created `compute`, and single-artifact `review` relationships, then derives each lineage root key. Composite Peer Review remains a separate root because one parent cannot represent both source Tasks; Routine occurrences remain individual roots.

Worker Handover now reports ProblemKey, related Task count, WorkerSession attempt count, and cumulative admitted Task tool-call usage. Product-provider terminal observations retain ProblemKey. These are durable identity and usage facts only; no shared ProblemKey cap is enforced. The existing cumulative Task envelope is still the only newly enforced budget dimension.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 82 migrations passed.
- `git diff --check` — passed.
- Tests were not added or run. PostgreSQL migration/runtime and provider execution were not run.

## Files

- `db/migrations/00082_task_problem_lineage.sql`
- `db/migration_hashes.sha256`
- `internal/kernel/kernel.go`
- `internal/kernel/mission.go`
- `internal/kernel/collaboration.go`
- `internal/kernel/review_task.go`
- `internal/kernel/worker.go`
- `internal/kernel/worker_state.go`
- `internal/control/real_provider_worker.go`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
- `AGENTS.md`
