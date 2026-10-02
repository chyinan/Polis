# Slice 123 — authorized ProblemKey budget allocations

Date: 2026-10-02

## Change

Schema 84 adds an append-only allocation history for finite ProblemKey tool-call caps. Each row records the authorization label, request ID, expected prior cap, resulting cap, added calls, revision, reason and timestamp. Kernel requires an explicit positive amount, a non-empty reason, confirmation, and the current expected cap/revision. A row lock serializes commands; a database insert trigger independently checks that the revision and cap transition continue the existing chain. Allocation records cannot be updated, deleted or truncated.

The Workbench exposes a no-store budget projection and a local-owner allocation form with stale-revision feedback. Both budget routes require the desktop session token, and each mutation uses a bounded unique request ID. New allocations extend the shared ProblemKey cap; they do not reset or rewrite the immutable Task envelopes. No allocation was executed during implementation.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `npm run build` — passed. Vite emitted its existing large-chunk advisory; the build completed successfully.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — all 84 migrations passed.
- `git diff --check` — passed.
- Tests were not added or run. PostgreSQL migration/runtime, allocation command execution, provider execution and live cost measurement were not run.

## Remaining limits

This authorizes only additional tool calls for a ProblemKey. It does not raise an existing Task's fixed envelope, create/dispatch a WorkerSession, or start a Mission. Mission/Company/Provider caps, protected closing reserves/task classes, rejected-route/policy-revision records, hidden retries, token/money accounting and stable exhaustion closeout/recovery remain open.
