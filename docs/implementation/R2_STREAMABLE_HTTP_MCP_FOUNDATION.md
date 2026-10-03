# R2 Streamable HTTP MCP

Updated: 2026-10-03, Slice 156. The fixed-version transport, bounded paginated discovery, runtime observation, approval, binding and local Worker dispatch paths are implemented. External endpoints remain unqualified.

The profile pins MCP `2026-07-28`, as described in the [Streamable HTTP transport specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http). It sends per-request protocol metadata and supports request-scoped JSON or SSE responses. It does not perform legacy initialization or session fallback.

## Runtime path

Workbench can register the fixed endpoint descriptor without contacting it. Metadata qualification binds the canonical HTTPS endpoint, name, transport and protocol version. A separate human decision approves or revokes that descriptor.

Runtime observation is available only when `POLIS_MCP_STREAMABLE_HTTP_ENABLED=1`. The authenticated Workbench action rechecks the current descriptor, metadata qualification and approval while holding the per-server advisory lock. It follows the complete `tools/list` cursor chain, validates the fixed tool-schema subset and records an HTTP-profile-bound digest. Discovery is limited to 64 pages, 64 tools, 2 KiB per cursor, 256 KiB of aggregate tool definitions and two minutes total; repeated cursors, incomplete pages or limit violations fail closed without recording partial results. The response is evidence for human review; it does not qualify the server until the owner separately approves the runtime record.

After runtime approval, an Employee binding pins the metadata qualification. The separately versioned MCP v2 provider surface describes both controlled stdio and fixed Streamable HTTP profiles. When egress is enabled, the trusted adapter includes the approved endpoint and tool schema in the Employee's `work_current` context; the flag-off projection omits HTTP tool sets. Worker dispatch rechecks the current approval, binding, runtime event and endpoint under the same per-server lock. It checks the egress flag before creating the call intent. The ledger records one dispatch intent before any tool request; an interrupted or ambiguous call becomes `outcome_unknown` and is never replayed. Before each call, Worker fetches `tools/list` again and compares the digest. A changed schema records drift and blocks the call. Arguments must match the pinned closed-object primitive schema. Only bounded text results are accepted, marked as untrusted, and stored with the intent.

The same egress flag controls observation and Worker calls. With the flag unset, no endpoint request is issued. The Worker path selects the separately pinned MCP v2 surface with `POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2=1`, `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`; the real Codex provider gate still rejects this unqualified surface. A Windows AppContainer is needed only when the same v2 Worker must also expose controlled stdio tool sets. The HTTP observation and runtime-approval routes require the desktop session token, including on tokenless loopback installations.

## Transport limits

- HTTPS only. Proxies and redirects are disabled. TLS verifies the configured hostname.
- Each request resolves the hostname again and rejects private, loopback, link-local, shared, documentation, benchmark and other special-use addresses.
- IPv6 literals and DNS answers must be in `2000::/3`, IANA's current Global Unicast allocation, and must pass the special-use denylist. IPv4-mapped IPv6 and reserved/out-of-allocation ranges are rejected. The list includes `100::/64`, documentation `3fff::/20` and `5f00::/16`. See the [IANA IPv6 address space registry](https://www.iana.org/assignments/ipv6-address-space) and [IPv6 Special-Purpose Address Registry](https://www.iana.org/assignments/iana-ipv6-special-registry).
- JSON-RPC request IDs are checked. JSON and request-scoped SSE are bounded; unsupported status codes and media types, malformed responses, oversized bodies and oversized SSE lines are rejected.
- Tool lists are followed to completion under the page, cursor, tool-count, byte and time limits above. The full canonical list is re-read before each tool call; no partial catalog is approved or exposed.
- Tool results accept text blocks only. Images, resource links, embedded resources, structured content and error results are rejected.
- `x-mcp-header` annotations are limited to static object-property paths, primitive values and bounded safe header names. Unsafe header values use the profile's Base64 sentinel encoding.
- No OAuth, endpoint credentials, subscription/listen streams, legacy fallback, arbitrary transport or marketplace profile is enabled.

## Qualification status and limits

Local fake-server tests cover JSON/SSE framing, metadata and routing headers, response bounds, endpoint policy, schema canonicalization, argument validation and text-only result handling. The disposable PostgreSQL test records a synthetic tools/list fixture and exercises runtime approval, Employee binding, authorization, dispatch intent and completion without network access. Full Go tests, Linux/Windows command builds, frontend tests and the R1/R2/R3 disposable PostgreSQL suites pass at Schema 46. See `evidence/development/r1-r3-implementation-validation-20260928-slice-61-streamable-http-mcp-worker/verification.md`.

No external MCP endpoint was contacted. No source-specific reachability, identity, quality, recovery or cost evidence has been recorded. A server can still change after the final schema read and before it handles `tools/call`; the protocol cannot make that remote interval atomic. Unknown call outcomes therefore remain unresolved instead of being retried. The implementation does not claim arbitrary endpoint compatibility.
