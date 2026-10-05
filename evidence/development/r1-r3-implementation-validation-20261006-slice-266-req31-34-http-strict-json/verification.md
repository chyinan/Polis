# Slice 266 — REQ-31/34 strict Streamable HTTP JSON-RPC responses

Date: 2026-10-06

## Scope and source finding

This targeted audit used source HEAD `83592318bc49b6f717d68054da7b213a1fbc63af` in the isolated `codex/req02-13-audit-s262` worktree. Slice 256 had added recursive duplicate-key rejection to stdio responses and arguments, while explicitly leaving Streamable HTTP parsing unchanged. The Streamable HTTP response path still decoded JSON-RPC bodies with `json.Unmarshal`; its SSE path also classified messages before that response decode. Duplicate members could therefore be interpreted with Go's last-key-wins behavior in the JSON-RPC envelope or nested result values.

`internal/mcptransport/streamable_http.go` now calls the package's existing recursive `rejectDuplicateJSONKeys` before decoding a matching JSON-RPC response. Both JSON and SSE responses use this validator. It checks the full response recursively, including nested tool-list definitions/schemas and tool results. Tool arguments were already checked by `ValidatePinnedToolArguments`, shared by stdio and Streamable HTTP.

This closes a local parser ambiguity without changing protocol schemas, endpoint policy, or authorization. It does not qualify the Streamable HTTP endpoint, a provider, Worker, Windows host, or any MCP account.

## Focused remaining-work audit

The ledger and current source did not identify an additional self-contained local change for the other requested requirements:

- REQ-14's general ActionIntent/DispatchPermit and shared-write ResourceKey mappings require an owner-approved action and canonical target/account mapping; applicable legacy review and host stop/recovery qualification remain open. Slice 260 already covers controlled MCP calls.
- REQ-15 still needs Desktop/PostgreSQL restore qualification and a current database-confirmed active WorkerSession for session-bound correction/revalidation.
- REQ-16 needs owner-confirmed billing scope/liability, provider accounting evidence, and retry/charge qualification; the implementation does not infer costs from incomplete token telemetry.
- REQ-27 cross-profile handover and behavioral thresholds require an owner-approved contract and authorized model/provider accounts. REQ-32 successor use needs a qualified active session and owner authorization.
- REQ-30 real-provider Skill surface; REQ-31 Windows WFP/recovery and HTTP endpoint/session; REQ-33 exact host/provider capability; and REQ-34 real-provider dispatch remain qualification gates, not code that can be safely asserted from this checkout.

The previous Slice 256 audit remains applicable to Worker stop/restart, memory overlay, billing, profile continuity, Skill loading, and MCP authorization/binding paths. This slice records the narrower Streamable HTTP JSON parser omission it did not change.

## Verification limits

- `go build ./...` passed.
- `git diff --check` passed.
- No tests were added or run.
- No database, Worker, provider, MCP endpoint, account, host qualification, or frozen scenario was accessed or run.
- REQ-31/34 remain partial; all 232 frozen scenario executions remain `not_run`.
