# R0.15 disposable PostgreSQL integration run (stdio + local streamable HTTP MCP)

- date: 2026-11-12T12:41:02Z
- git: 2a33f036 (workspace: local streamable HTTP MCP qualification (localhost-only sandbox) + stdio regression, no external egress/OAuth/provider turns)
- database: polis_r0_15_httpstdio_1794487130_332 (disposable, polis_r0_ prefix)
- postgres/server_version_num|schema: 180006|16
- dsn: [redacted socket DSN, user=polis_runtime, db=polis_r0_15_httpstdio_1794487130_332, port=55499]
- external_egress: none (local fake stdio MCP + local fake HTTP MCP on 127.0.0.1 only; non-local endpoints denied)
- exit: 0

## Test output
```text
ok  	polis/internal/kernel	93.012s
```
