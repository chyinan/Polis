# Slice 252 — REQ-24 receipt replay authorization guard

## Change

When the advisory request lock is unavailable, the receipt replay fast path now opens a fresh read transaction and runs the same runtime lease, Company lock, binding, Employee epoch, and session-state guard as the ordinary write path before reading the receipt. A stale or ended WorkerSession can no longer retrieve a previously committed receipt through this contention path. Matching receipts still require the original request fingerprint.

This closes a local authorization gap only. It does not qualify the real employee-operation/provider surface or establish an active WorkerSession.

## Verification

- `go build ./...` passed on integrated source commit `8c370ee` (published implementation commit `bb4e212f83c55c63e6bc33f29b3b64a91942cc24`).
- `git diff --check` passed.
- All six REQ-24 scenario rows remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, database operation, Worker/provider action, or frozen scenario ran.
