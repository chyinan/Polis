# Slice 268 verification — canonical MCP JSON field names

## Scope

REQ-31/34 local source hardening. This slice rejects case-variant JSON keys when they alias fields on typed MCP/JSON-RPC structs. It applies to stdio response decoding, Streamable HTTP JSON/SSE envelopes and text results, and the controlled `mcp_call` envelope. Arbitrary JSON maps and `json.RawMessage` payloads retain case-sensitive key semantics; tool schemas and application data are not globally case-folded.

The change closes the gap where exact duplicate-key rejection still allowed keys such as `jsonrpc` and `JSONRPC` to map to the same Go struct field. It does not add dispatch surfaces or change the existing capability and one-shot permit authorization checks. Native Windows WFP and endpoint/provider qualification are outside this slice.

The response boundary also requires exactly one top-level `result` or `error`: stdio, Streamable HTTP JSON, and Streamable HTTP SSE reject missing, dual, and explicit `error: null` outcomes, while preserving `result: null` as a present result. The reachable fake runtime `mcp_call` listed-tool check uses the same typed exact-key validator; its envelope contains only scalar fields, and arbitrary tool arguments remain outside the case-variant walk.

## Verification

- `go build ./...` — passed after the response outcome and fake runtime parser follow-up.
- `git diff --check` — passed.
- Tests, database, Worker, provider, MCP endpoint, scenario, migration, and push — not run.

## Files

- `internal/mcptransport/strict_json.go`
- `internal/mcptransport/stdio.go`
- `internal/mcptransport/streamable_http.go`
- `internal/mcptransport/streamable_http_contract.go`
- `internal/control/controlled_mcp_call_contract.go`
