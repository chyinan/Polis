# R0.15 disposable PostgreSQL integration run (post-refinement regression)

- date: 2026-11-07T16:26:56Z
- git: dd6f1b60 (workspace with schema-validator/task-scope refinements + integration test)
- database: polis_r0_15_toolcall_1794068819_323 (disposable, polis_r0_ prefix)
- postgres/server_version_num|schema: 180006|16
- dsn: [redacted socket DSN, user=polis_runtime, db=polis_r0_15_toolcall_1794068819_323, port=55487]
- external_egress: none (local fake stdio MCP only; no remote HTTP/OAuth/QQ/provider)
- exit: 0
- note: Regression check after adding ARGUMENTS_SCHEMA_VIOLATION subset validation and TaskID company-scope precondition; earlier passing evidence preserved in git history (dd6f1b60 context).

## Test output
```text
ok  	polis/internal/kernel	58.988s
```
