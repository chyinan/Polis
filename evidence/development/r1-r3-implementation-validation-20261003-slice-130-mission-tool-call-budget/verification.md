# Slice 130 — Mission protocol tool-call budget

Date: 2026-10-03

## Result

Schema 89 adds a nullable explicit Mission cap, cumulative Mission usage and allocation revision. Existing usage is backfilled by summing the durable per-Task tool-call counters; existing Missions retain a null cap until an owner configures one. An append-only local-owner ledger permits initial configuration and cap increases only. Immutable denial records preserve the Mission cap/usage/revision snapshot for rejected Worker admission and tool-call routes.

New Workbench Mission creation requires an explicitly supplied finite protocol tool-call ceiling. Settings lists Mission budgets and permits a local owner to configure legacy Missions or increase the total cap with confirmation, reason and expected-revision checks. Kernel Worker admission and accepted tool-call charging lock Mission before WorkerSession/Task and ProblemKey. One accepted protocol tool call increments all applicable counters atomically. Worker Handover reports Mission usage and remaining calls, and the provider turn cap uses the minimum remaining allowance.

The cap counts admitted protocol tool calls. It does not claim complete provider-request, hidden CLI/service retry, token, or USD accounting. Mission closing reserve inside the total cap, Company/Provider limits and exact FT-42–45 qualification remain open.

## Validation

- `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis` — passed.
- `npm run build` — passed; the existing Vite advisory reports a minified JavaScript chunk over 500 kB.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` — all 89 migration pins passed.
- `git diff --check` — passed.
- Tests were not run. PostgreSQL migration/runtime and provider execution were not run.
