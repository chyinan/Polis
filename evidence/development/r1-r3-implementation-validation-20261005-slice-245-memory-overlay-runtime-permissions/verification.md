# Slice 245 — REQ-15 memory revocation overlay permissions

## Change

Startup loading and runtime persistence now share the same Company-directory permission check for memory revocation overlay files. On Unix, if the directory has group/world permission bits after startup, persistence fails with `core.Denied` before writing new revocation metadata. Symlink/type protections remain in place. This does not qualify restore behavior or perform a session-bound correction/revalidation.

## Verification

- The implementation commit's `go build ./...` passed in its isolated worktree.
- `git diff --check HEAD^ HEAD` passed after integration.
- Static inspection confirms both startup loading and runtime persistence call the same permission validator.
- Traceability remains 17 open requirements; all 232 scenarios remain `not_run`.
- No tests, DB operations, Worker/provider operations, or frozen scenarios ran.
