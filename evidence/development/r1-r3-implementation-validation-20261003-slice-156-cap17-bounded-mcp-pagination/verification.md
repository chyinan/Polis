# Slice 156 — CAP-17 complete bounded MCP tool discovery

## Change

Stdio and Streamable HTTP MCP clients now follow `tools/list` cursors until the catalog is complete. Each page is parsed as a whole; invalid pages or a non-terminating cursor chain do not yield a partial tool catalog. The complete tool set is passed to the existing schema validator, which rejects duplicate tool names, unsupported input schemas and oversized definitions. Existing schema digests continue to bind canonical tool definitions and every MCP call re-reads the full catalog before dispatch.

Both profiles enforce at most 64 pages, 64 tools, 2 KiB per opaque cursor and 256 KiB of aggregate tool definitions. Cursor repetition and page-limit exhaustion fail closed. Stdio discovery has one 30-second deadline for the entire cursor chain; HTTP discovery has one two-minute deadline for the entire chain, in addition to its per-request timeout and one-megabyte response bound. No catalog cache was added, so directory data is not reused across WorkerSessions or employees.

## Verification

- `gofmt` on changed Go files: passed
- `go build ./cmd/polis ./internal/kernel ./internal/control ./internal/provider ./internal/mcptransport ./internal/mcpowner`: passed
- `git diff --check`: passed
- Tests: not run
- Live stdio MCP process: not started
- Live Streamable HTTP endpoint: not contacted
- Frozen CAP-13–19 scenario execution: not run; all remain `not_run`
- Database/schema migration: none

The build and source inspection confirm the bounded pagination path compiles. They do not qualify an external server, native AppContainer/WFP host, real endpoint identity or the frozen acceptance scenarios.
