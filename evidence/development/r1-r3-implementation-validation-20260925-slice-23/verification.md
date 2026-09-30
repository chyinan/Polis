# Slice 23 verification — R2 Streamable HTTP MCP foundation

Date: 2026-09-25

## Changes covered

Schema 28 adds the fixed `streamable_http_mcp_2026_07_28@1` capability qualification profile. `internal/mcptransport` implements bounded request/response handling for the official 2026-07-28 Streamable HTTP profile. The capability catalog now locally verifies the canonical endpoint/profile digest and preserves manual approve/revoke and Employee binding. No remote MCP interaction is part of the local verification action.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/mcptransport -count=1` | PASS: fake HTTP server validates version/metadata headers, JSON and SSE (including priming events and notifications), tool header mapping, ID/media-type/error checks, endpoint canonicalization, and address/URL policy. |
| `bash scripts/go.sh test -race ./internal/mcptransport ./internal/kernel ./internal/control -count=1` | PASS. PostgreSQL-dependent tests without a DSN were skipped in this race run. |
| `POLIS_TEST_DSN='host=/tmp/polis-r2-mcp-20260925/socket port=56437 dbname=polis_r0_r2_mcp_20260925 user=polis_runtime' scripts/go.sh test ./internal/kernel -run TestStreamableHTTPMCPUsesPinned20260728ProfileWithoutNetworkProbe -count=1` | PASS against PostgreSQL 18 / schema 28. The test registers `https://mcp.example.com/v1/mcp`, locally qualifies its descriptor, approves it, and binds it to an Employee without dialing that host; execution remains `runtime_unqualified`. |
| `POLIS_TEST_DSN='host=/tmp/polis-r2-mcp-20260925/socket port=56437 dbname=polis_r0_r2_mcp_20260925 user=polis_runtime' scripts/go.sh test ./internal/workbench -run TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates -count=1` | PASS; Workbench opens on schema 28 and retains its qualification projections. |
| `bash scripts/go.sh test ./... -count=1` | PASS after final code changes. |
| Frontend `npm test`, `npm run typecheck`, `npm run lint`, `npm run build` | PASS after the profile/UI changes; 61 tests. Build reports the existing >500 kB chunk advisory. |
| `git diff --check` | PASS after final code and documentation updates. |

The dedicated PostgreSQL 18 / schema-28 cluster was stopped and its exact temporary directory removed after integration checks.

No external MCP endpoint, OAuth server, model, QQ, GitHub account, registry, production service, or business data was contacted. No runtime MCP call path was enabled. This slice establishes protocol/client and governance foundations, not remote endpoint or Worker execution qualification.
