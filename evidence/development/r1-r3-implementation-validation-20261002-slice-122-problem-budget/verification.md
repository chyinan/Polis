# Slice 122 — shared ProblemKey tool-call budget

Date: 2026-10-02

## Change

Schema 83 adds one durable tool-call limit and usage counter for each immutable ProblemKey. Existing lineages are backfilled from the sum of Task limits already fixed by WorkerSessions; if any historical Task is unlimited (`0`), the ProblemKey remains unlimited. A new ProblemKey's first WorkerSession fixes its cap from that Task's explicit limit. Tasks created later inherit the same ProblemKey and cannot expand the cap. Database guards prevent later allocation changes, usage decreases, row deletion, and table truncation.

WorkerSession admission now rejects an exhausted ProblemKey. Every accepted tool call locks and charges the WorkerSession, Task, and ProblemKey counters in one transaction. Handover reports the shared limit and remaining amount; the provider adapter's existing turn clamp now uses the minimum remaining amount across all three envelopes. This is tool-call accounting only.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 83 migrations passed.
- `git diff --check` — passed.
- Tests were not added or run. PostgreSQL migration/runtime, provider execution and live cost measurement were not run.

## Remaining limits

`0` retains the existing unlimited meaning. This slice does not account for hidden CLI retries, token/money usage, Mission/Company/Provider budgets, additional authorized allocations or revisions, rejected-route/policy-revision records, closing reserves, or stable exhaustion closeout/recovery. New allowances must come from an explicit authorization record; no default numeric allowance was introduced.
