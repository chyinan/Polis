# Slice 124 — protected ProblemKey closing reserves

Date: 2026-10-03

## Change

Schema 85 adds an append-only revision ledger for ProblemKey closing reserves. A revision binds the protected amount to the expected shared cap, cap revision, previous reserve revision and current usage by closing-class Tasks. The database trigger locks the ProblemKey budget row and checks the revision chain, finite cap, current balance and closeout-usage snapshot. Reserve changes cannot be updated, deleted or truncated.

The fixed closing classes are Kernel-created `review` and `peer_review` Tasks. The Workbench may set a reserve before the first WorkerSession initializes a pending ProblemKey. The first admission then requires a finite cap at least as large as the configured reserve; the Kernel and an independent database trigger both enforce this. For finite budgets, ordinary Tasks can consume only the remaining shared balance above the outstanding reserve. Closing-class Tasks can consume the full remaining balance; accepted calls by those Tasks burn down the reserve created by the current policy revision. The local-owner Workbench command can set or release a reserve with a required reason and confirmation, expected cap/revisions and a unique request ID. A zero reserve is the default. Existing Task limits remain in force.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `npm run build` — passed. Vite emitted its existing large-chunk advisory; the build completed successfully.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 85 migration hashes passed.
- `git diff --check` — passed.
- Tests were not added or run. PostgreSQL migration/runtime and budget command execution were not run. No provider or Mission execution was started.

## Remaining limits

This reserves admitted protocol tool calls only. It does not count hidden CLI retries, tokens or money; compose Mission/Company/Provider ceilings; persist rejected admission-route explanations; or define exhaustion closeout/recovery policy. Closing privilege is limited to the two fixed Kernel-created Task kinds and is never selected by caller input.
