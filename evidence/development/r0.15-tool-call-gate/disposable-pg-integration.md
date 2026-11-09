# R0.15 disposable PostgreSQL integration run (task-owner/schema-detail regression)

- date: 2026-11-09T15:05:50Z
- git: 4940282c (workspace with task-owner mismatch precondition, bounded schema violation detail in denial receipts, updated integration assertions)
- database: polis_r0_15_toolcall_1794236663_317 (disposable, polis_r0_ prefix)
- postgres/server_version_num|schema: 180006|16
- dsn: [redacted socket DSN, user=polis_runtime, db=polis_r0_15_toolcall_1794236663_317, port=55487]
- external_egress: none (local fake stdio MCP only)
- exit: 0

## Test output
```text
ok  	polis/internal/kernel	61.049s
```
