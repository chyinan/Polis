# Slice 133 — Company protocol tool-call budget

Date: 2026-10-03

## Delivered

- Schema 91 backfills Company usage from durable `tasks.task_tool_calls_used`, leaves the owner cap pending, and adds append-only configure/increase allocations plus immutable Worker-admission/tool-call rejection evidence.
- Kernel checks Company before Mission and lower scopes. Each accepted admitted protocol tool call increments Company, Mission, WorkerSession, Task and ProblemKey counters within the same idempotent transaction. Handover/provider turn limits use the minimum effective remaining allowance.
- Workbench exposes a no-store Company budget view and a confirmed, reasoned configure/increase operation. The Settings budget panel clearly labels the unit and records recent Company-level rejections.
- ProviderAccount financial limits remain unimplemented: general WorkerSessions do not bind an account identity, and no settled/reserved/unknown liability model is available.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis` | PASS |
| `npm run build` from `frontend/` | PASS; Vite reports the existing large-bundle advisory (806.01 kB minified JS) |
| `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` | PASS; migrations 00001–00091 |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution/live cost measurement | Not run |

## Operational boundary

Every existing Company starts with a pending cap after Schema 91. Worker admission and protocol tool calls remain blocked until a local owner configures a finite Company limit in Settings. This cap counts admitted protocol tool calls only; it does not bound model requests without tools, hidden CLI retries, tokens, raw provider egress or USD spend. Exact FT-42–45 qualification remains `not_run`.
