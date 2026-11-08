# R0.15 disposable PostgreSQL integration run (post receipt-detail/pattern regression)

- date: 2026-11-08T12:21:36Z
- git: 647a9a33 (workspace with receipt-detail/redaction + RE2 pattern support)
- database: polis_r0_15_toolcall_1794140407_315 (disposable, polis_r0_ prefix)
- postgres/server_version_num|schema: 180006|16
- dsn: [redacted socket DSN, user=polis_runtime, db=polis_r0_15_toolcall_1794140407_315, port=55487]
- external_egress: none (local fake stdio MCP only)
- exit: 0

## Test output
```text
ok  	polis/internal/kernel	64.680s
```
