# Slice 251 — REQ-15 memory revocation overlay durability retry

## Change

When a matching revocation overlay already exists on disk, persistence now syncs its parent directory before caching it in memory. This applies to both the initial idempotency path and a link conflict retry. For a newly linked final name, the in-memory fast path is populated only after the parent-directory sync check succeeds (or the existing platform-specific unsupported case is allowed). A failed durability check therefore leaves a later retry to re-read and revalidate the file.

This preserves the overlay's fail-closed role when a later SQL transaction fails. It does not qualify restore behavior on PostgreSQL/Desktop hosts or the session-bound correction and revalidation paths.

## Verification

- `go build ./...` passed on integrated source commit `aa2784a` (published implementation commit `d03df66f3235e2ea8c3c6b6b1966331297009748`).
- `git diff --check` passed.
- All six REQ-15 scenario rows remain `partial/not_run`; all 232 frozen scenario rows remain `not_run`.
- No tests, database operation, Worker/provider action, memory correction, restore, or frozen scenario ran.
