# R0.15 MCP `tools.call` gate — evidence (pending DB run)

This slice is stdio-first and deny-by-default. **No external MCP/OAuth/network egress, QQ, or provider turn is authorized by this README.**

## Verified (2026-11-07)

- `go test ./internal/kernel -run "TestToolCall|TestValidateToolCall"` — PASS
- `go build ./...` — PASS
- `go vet ./internal/kernel ./internal/control ./internal/workbench ./internal/core` — PASS
- `TestMCPToolCallGateAndExecution` — **PASS** on disposable temp PostgreSQL 18/schema 16 (`polis_r0_*`, redacted DSN, bundled `.tools/pg`, local fake stdio MCP only, no external egress). See `disposable-pg-integration.md`.

## Disposable PostgreSQL integration (requires explicit local setup)

Use a **dedicated temporary** database with the `polis_r0_` prefix; never use business databases.

```bash
# WSL/Linux example shape (adapt DSN/host/path to your disposable instance)
POLIS_TEST_DSN='postgres://user@host:5432/polis_r0_toolcall_test?sslmode=disable' \
  bash scripts/go.sh test ./internal/kernel -run TestMCPToolCallGateAndExecution -count=1
```

The test builds `internal/kernel/testdata/toolcallmcp` (local fake stdio MCP, deterministic, no network), creates a temporary blob root, applies migrations, and exercises: deny without call qualification (`CALL_NOT_QUALIFIED`), explicit qualification approval, successful local call, idempotent replay, and `DESCRIPTOR_DIGEST_MISMATCH`.

Record actual command output, DSN redaction, database name (must be disposable), and git revision under this directory when run.
