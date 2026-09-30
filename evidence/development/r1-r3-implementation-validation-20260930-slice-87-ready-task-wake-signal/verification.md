# Slice 87 — ready Task wake generation

Date: 2026-09-30  
Status: local software verification passed  
Database schema: 62

## Implemented

- Schema 62 adds a Task trigger for inserts and transitions into `ready` while the owning Mission is active or paused.
- Active-Mission ready work advances the owner schedule generation and sets eligible schedules to `wake_pending`; paused-Mission work advances the generation but preserves `paused` and `mission_paused`.
- Task rows remain authoritative; the trigger sends only a latency `pg_notify` hint. It does not create a second Task queue.
- A forward-migration backfill covers existing ready Tasks in active/paused Missions once per owner, preserving pause/quota barriers.
- Sleep reconciliation checks ready Tasks in active Missions; Mission resume checks ready Tasks and restores their owners to `wake_pending`.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on disposable PostgreSQL 18. The script migrates Schema 60→62, seeds existing ready Tasks in active and paused Missions and peer obligation states, and verifies backfill. It also verifies Mission-start planning Task wake, product Task preparation wake, pause-time Task creation, resume wake, peer signal idempotence, eight concurrent signal/sleep interleavings and FYI suppression.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -c 'export GOOS=windows GOARCH=amd64; bash scripts/go.sh build ./cmd/...'` — passed.
- `scripts/test-migration-hash-manifest.ps1` — passed.
- `rtk git diff --check` — passed.
- Independent read-only code review — passed with no Critical, Important or Minor findings.

No model, provider, QQ, MCP, GitHub, business account or production database was used.

## Limits

Routine events, LISTEN/reconnect/startup scans, due-time fairness, quota release, automatic Worker admission, Workbench schedule projection and host qualification remain open under REQ-13.
