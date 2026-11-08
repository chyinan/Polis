# R0.15 disposable PostgreSQL integration run (post frontend-detail/local-ref regression)

- date: 2026-11-08T12:30:03Z
- git: dd37428a (workspace with frontend read-only receipt detail + local ref/defs/RE2 pattern subset)
- database: disposable polis_r0_15_toolcall_* (name not captured due evidence-script quoting; temp PG root created/stopped/removed)
- postgres/schema: PG18 bundled .tools/pg, schema 16 (migrate applied in run)
- dsn: [redacted socket DSN, user=polis_runtime, disposable polis_r0_ DB, port=55487]
- external_egress: none (local fake stdio MCP only)
- exit: 0

## Test output
``text
ok  	polis/internal/kernel	60.765s
``
