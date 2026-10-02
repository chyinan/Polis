# Slice 125 — ProblemKey budget rejection audit

## Implemented

Schema 86 adds an append-only ledger for Worker admission and tool-call routes denied by the cumulative Task limit, shared ProblemKey limit, WorkerSession limit, or protected closing reserve. Each row captures the Task/session identity, route and reason, usage and cap snapshot, ProblemKey allocation revision, and closing-reserve policy and remaining amount. A database trigger checks the snapshot and validates the rejection reason against the locked state. Rejected first admission does not initialize Task or ProblemKey caps. The Workbench displays the rejection count and latest route, reason, Task and timestamp for each ProblemKey.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` passed.
- `npm run build` passed; Vite emitted the existing advisory for the 772 kB minified JavaScript chunk.
- All 86 pinned migration hashes passed with `sha256sum -c ../migration_hashes.sha256` from `db/migrations`.
- `git diff --check` passed.
- Tests were not added or run. PostgreSQL migration/runtime and Workbench behavior were not run in this environment.

## Remaining REQ-16 work

This slice records policy decisions and denied routes; it does not compose Mission, Company or Provider allowances, account for hidden CLI retries or token/money cost, or define stable exhaustion closeout and recovery. These remain open along with PostgreSQL runtime qualification.
