# Slice 51 verification: controlled stdio MCP process owner

Date: 2026-09-27

## Scope

Added the `internal/mcpowner` process lifecycle around the pinned stdio transport. This is a local foundation only; it does not add persistence, capability approval/employee binding lookup, schema-drift events, or Worker dispatch.

## Results

- `rtk proxy bash scripts/go.sh test -race -count=1 ./internal/mcpowner ./internal/mcptransport` — PASS. The owner suite uses a local `net.Pipe` MCP pseudo-server and covers server/schema pinning, approved dispatch, missing approval denial, schema-drift stop, unresolved-stop retry, unsafe launch-spec denial, public Start isolation enforcement, changed-command digest rejection, unpinned entrypoint rejection, incomplete package-manifest rejection, and unexpected process exit. The transport contract suite also passes under the race detector.
- `rtk proxy bash scripts/go.sh test -count=1 ./internal/runner` — PASS.
- `GOOS=windows GOARCH=amd64 rtk proxy bash scripts/go.sh test -c -o /tmp/polis-mcpowner.test.exe ./internal/mcpowner` — PASS (test binary was written to a dedicated temporary path and removed).
- `gofmt` — PASS for all changed Go files.

## Review and safety boundary

The read-only review found that a caller could inject a launcher that bypassed isolation, and that declared file pins did not prove command arguments or the complete package tree were pinned. The public API now accepts only a concrete `runner.AppContainerSandbox`; injected launchers are package-private for tests. Launch validates command/package digests, requires the entry point and path-like arguments in the exact manifest, walks the package for unexpected files/links, locks the package root and directories, rechecks the inventory, then holds write-denying leases on every file while re-hashing it. The reviewer confirmed the launcher-injection and manifest findings are closed.

Further review found that a simple relative config name could be interpreted from a workspace CWD, then that Windows drive-relative paths could bypass the package check. The owner now requires the working directory to equal the pinned package root and rejects Windows drive-relative arguments in bare, equals, and quoted forms. Colon-bearing option values are treated as paths, preventing NTFS alternate data-stream bypasses. Regression tests cover each case. Final read-only review confirmed zero remaining findings across the launcher, package inventory, and argument-path checks.

No actual AppContainer or WFP setup, MCP command, remote endpoint, database qualification write, real employee Worker turn, QQ send, or external service was run. No database migration was added; the development schema remains 40. Compile and pseudo-server evidence does not qualify Windows isolation or an external MCP server.
