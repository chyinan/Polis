# Slice 126 — authorized Task budget recovery

Schema 87 adds `task_tool_call_allocations`, an append-only local-owner ledger for raising an exhausted finite Task tool-call cap. Each revision stores the old/resulting cap, authorized increment, request ID, reason, and the ProblemKey allocation and closing-reserve revisions used for the decision. The database validates revision continuity, Task eligibility, the current shared ProblemKey budget and reserve, and allows a Task cap update only when it matches the newest ledger row. Initial `NULL` to finite Task-limit initialization remains supported.

Kernel performs the confirmed allocation in one transaction: it locks the Task and ProblemKey budget, rejects Tasks below their cap, completed/cancelled Tasks and any live WorkerSession, compares the UI's Task/ProblemKey/reserve snapshots, checks sufficient ProblemKey calls after preserving any ordinary-Task closing reserve, writes the allocation revision, and updates the Task cap. The runtime's existing admission, provider turn budget and handover paths consume the updated Task cap; ProblemKey remains the shared enforcement ceiling. The Workbench lists up to 20 Task budgets under each visible ProblemKey and provides an allocation form with amount, reason and explicit confirmation. Request handling is no-store and requires the existing local-owner Workbench authorization.

Verification completed:

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `npm run build` — passed; Vite reported the existing advisory that the main bundle exceeds 500 kB (778.03 kB).
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 87 migration hashes passed.
- `git diff --check` — passed.

Tests and PostgreSQL migration/runtime were not run. The schema gate now requires version 87. No provider calls or production actions were performed.
