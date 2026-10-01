# Slice 106 verification — legacy active-revocation Worker stop fallback

Date: 2026-10-01

## Scope

The bounded revocation-stop candidate query now combines Schema 73 revoke-time snapshots with Skill-load and MCP-intent session records for currently effective pre-Schema-73 global and employee revocations. The legacy active-revocation rules match the capability catalog projection. Candidate sessions are deduplicated before the existing keyset-paged stop coordinator processes them.

No schema migration was added. Superseded pre-Schema-73 revocations and bound-but-unused legacy sessions still cannot be reconstructed as exact revoke-time snapshots.

## Checks performed

- `gofmt -w internal/kernel/capability_revocation_stop.go` — passed.
- `git diff --check` — passed.
- `go build ./cmd/...` — passed.

## Not performed

Tests were not run. PostgreSQL migration/query execution and actual Worker stop behavior were not run. No provider, MCP endpoint, QQ, GitHub, or production action was used.
