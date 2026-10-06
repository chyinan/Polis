# Slice301 REQ-02 TaskRevision Workbench projection

Date: 2026-10-07

## Scope

Added an optional Company/Mission/Task-scoped Workbench projection for the
latest durable semantic TaskRevision and its latest owner decision. The read
store fails closed on malformed hashes, Task owner/kind mismatches, invalid
qualification/reason/decision states or cross-scope rows; pre-Schema122
responses remain compatible and Schema122 without Schema123 reports
`proposed`. The frontend validates the projection as read-only and displays
the semantic type, decision and qualification without adding approval or
execution controls.

Also aligned two stale frontend API fixtures with current contracts; the full
frontend test suite is green.

## Verification

Passed:

- `npm test -- --run` — 146 tests
- `npm run build`
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk git diff --check`

No migration was applied, and no database runtime, Worker/provider, browser,
external account or frozen scenario was used.

## Remaining boundary

The projection is observational only. Owner decisions, qualification evidence
and execution admission remain separately gated; the Workbench does not expose
an approval action in this slice.
