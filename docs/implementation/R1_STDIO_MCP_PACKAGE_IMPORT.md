# R1 stdio MCP package import

Updated: 2026-09-28. Slice 55 / Schema 44. Import stores source bytes only; runtime observation is a separate, unimplemented operation.

## Package profile

The Workbench accepts a ZIP with `mcp-package.json` at its root. The versioned JSON manifest declares the display name, expected MCP server identity/version, a package-relative command, entry point and fixed arguments. Every other ZIP file becomes part of the immutable package manifest.

```json
{
  "schemaVersion": "polis-controlled-stdio-mcp@1",
  "name": "fixture-mcp",
  "serverName": "fixture-server",
  "serverVersion": "1.0.0",
  "command": "runtime/node.exe",
  "entryPoint": "server/main.mjs",
  "args": ["server/main.mjs"]
}
```

The importer rejects unknown or duplicate JSON keys, invalid UTF-8, absolute or escaping paths, duplicate ZIP entries, symlinks and special entries, empty files, missing command/entry-point files, over 64 ZIP entries, paths over the shared 1,024 UTF-8-byte limit, or more than the shared intake expanded-size limit. The canonical digest sorts package paths and binds each path, media type, byte size and SHA-256, so ZIP order does not change package identity. The browser validator uses the same byte limit.

## Storage and governance

A first import creates an `unverified` stdio definition and one package revision in a single company transaction. Importing another revision for an existing definition requires the same display name, command and arguments; changing those fields creates a new immutable definition. The package manifest is stored in Schema 44 and each file is written to that company's CAS. Kernel reads the CAS bytes back and re-verifies the canonical digest before committing the revision.

The importer derives the descriptor digest on the server. The multipart request accepts only an optional server ID, revision, request ID and ZIP; it rejects caller-supplied digest fields. Skill and MCP CAS-first imports use one PostgreSQL session to acquire the company/request-ID lock, then the company/package-revision lock, before writing CAS bytes. Every other `TXWrite` tries the same request-ID key inside its transaction and returns a conflict without waiting when a source import owns it. The import marks its pre-held lock in an unexported context value so its own receipt transaction does not try to reacquire the lock. This coordinates package imports with metadata commands and avoids holding a main-pool connection while waiting. A committed matching request replays its receipt; reusing an ID with different input conflicts before writing unique package blobs.

Import creates a candidate only. It does not launch the local command, observe tools, approve a capability, bind an Employee or enable Worker calls. Metadata approval, explicit runtime observation/approval and Employee version binding remain separate gates.

## Verification and limits

Parser tests cover all package-file order permutations, canonical digest stability, exact CAS-byte verification, invalid UTF-8, duplicate JSON keys, unknown fields, path traversal, the shared 1,024-byte path boundary, missing command/entry point, duplicate archive entries and readable display names. Kernel integration verifies company isolation, append-only history triggers, request replay, same-revision contention, cross-target and cross-capability request-ID conflicts before CAS writes, and uncoordinated `TXWrite` rejection without pool waits. Workbench tests cover multipart field allowlisting, session-token gating, upload bounds and multipart spill-file cleanup on rejected requests; frontend tests verify same-origin token propagation, path-limit parity and the returned package manifest before rendering.

The package is not executable through Workbench yet. The next R1 step is a separately labeled runtime-observation action that materializes these CAS bytes into a deny-all AppContainer, starts the local process once, records the clean-stop observation and observed tool schema, then returns to manual runtime approval. No MCP command or endpoint was started in Slice 55. Actual Windows AppContainer/WFP and clean-VM qualification remain unrun.
