# Slice 128 verification — scoped provider retry visibility

Date: 2026-10-03

## Scope

The real Codex provider terminal usage now states which retry signals Polis can observe: app-server `responseStreamDisconnected` errors carrying `willRetry`. `reconnect_attempt_count` counts those events, and `reconnect_recovered` says whether a valid event for the same turn arrived afterward. `retry_observation_scope` records this source. `unobserved_provider_retry_count` is `null` because CLI/service-internal retries are not exposed through this interface. The existing `retry_visibility=limited` remains.

This does not count, infer, or charge hidden retries. No provider execution was performed.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `npm run build` from `frontend/` — passed; Vite reports the existing advisory for a minified bundle above 500 kB.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` — all 88 migration hashes passed.
- `git diff --check` — passed.
- Go tests were not run.
- Provider execution and PostgreSQL runtime were not run.

## Remaining REQ-16 gaps

CLI/service-internal retries, token/money accounting and Mission/Company/Provider budget composition remain open.
