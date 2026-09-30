# Slice 48: fixed stdio MCP transport foundation

Date: 2026-09-27
Schema: 40 (no migration added)

## Result

`internal/mcptransport.StdioClient` implements the pinned MCP `2026-07-28` stdio subset over a caller-owned `io.ReadWriteCloser`. It performs `server/discover`, `tools/list`, and `tools/call`; includes protocol/client metadata on every request; correlates JSON-RPC IDs; and bounds wire messages, schemas, arguments, and text results. It rejects legacy fallback, missing discovery cache metadata, unsupported capability shapes, unknown request IDs, invalid UTF-8, open/nested input schemas, non-text tool content, malformed content/text fields, and tool-schema digest drift. Numeric bounds and numeric enum equality use exact rational comparison, so values beyond float64 precision cannot round through a limit or mismatch solely by JSON number spelling. Each call refreshes the tool list and checks the supplied approved schema digest before dispatch. On context cancellation after a complete request write, it sends `notifications/cancelled` for the original request ID before closing the stream; if request delivery is incomplete it closes without sending a cancellation for an unconfirmed request.

This package is a transport component only. It does not spawn a server, verify the child binary, persist server/tool-schema qualification, connect to EmployeeTools or change capability `runtime_unqualified` projections. The process owner must supply a sandbox-contained stream and must close, stop, and wait for the server process.

## Verification

- `scripts/go.sh test ./internal/mcptransport -count=1` passed.
- `scripts/go.sh test -race ./internal/mcptransport -count=1` passed after the final code change.
- `scripts/go.sh test ./...` passed after the final code changes. The Kernel package completed in 42.934 seconds; all listed packages passed.
- `scripts/go.sh build ./cmd/...` passed for the Linux host.
- Windows amd64 cross-builds passed for `./cmd/polis` and `./cmd/polisd`, with outputs written under `/tmp`.
- `git diff --check` passed.

The wire contract follows the [official MCP 2026-07-28 stdio specification](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/transports/stdio.mdx) and [transport overview](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/transports/index.mdx). Tests use only a local `net.Pipe` fixture. No MCP command or endpoint, real model, QQ, GitHub account, production database, or Windows sandbox policy was used.

## Review

The read-only review identified numeric precision, cancellation notification, malformed result-content, and numeric enum equality issues. They were fixed with regression tests: exact `big.Rat` bounds and enum comparison, request-ID-matched `notifications/cancelled` before stream close, and required non-null content/text fields while allowing explicitly empty text. The final re-review confirmed all findings fixed and reported no new issues.

## Remaining R1–R3 work

Process launch containment and owner-verified stop, server binary identity and persistent tool-schema qualification, drift revocation, Worker dispatch, R2 multi-day/cutover/upgrade rollback and Linux host qualification, plus independent real-domain R3 evidence and global qualification remain open.
