# R0.15 disposable PostgreSQL integration run

- date: 2026-11-07T15:00:09Z
- git: e1478dba (workspace had uncommitted integration-test fix at run time)
- database: polis_r0_15_toolcall_1794063581_306 (disposable, polis_r0_ prefix)
- postgres/server_version_num|schema: 180006|16
- dsn: [redacted socket DSN, user=polis_runtime, db=polis_r0_15_toolcall_1794063581_306, port=55487]
- external_egress: none (local fake stdio MCP only; no remote HTTP/OAuth/QQ/provider)
- exit: 0
- note: Previous attempt (same session, before test fix): failed on employee schema constraint; see git history/local context. Final run below.

## Test output
```text
ok  	polis/internal/kernel	11.146s
```
