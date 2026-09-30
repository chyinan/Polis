# Slice 86 — employee sleep/wake protocol

Date: 2026-09-30  
Status: local software verification passed  
Database schema: 61

## Implemented

- Added append-only Schema 61 `employee_schedules`, backfilled from the fixed company roster, current WorkerSession state, existing open peer obligations (`pending`/`observed`/`applied`), and paused Missions.
- New company creation initializes sleeping schedule rows for every roster member.
- Actionable peer messages persist their Obligation and `peer_work_signals`, advance the recipient `work_generation`, and send a non-sensitive `pg_notify` hint in the same transaction. FYI messages do not advance the generation or send a wake hint.
- Signal writers and sleep reconciliation lock the same employee schedule row. Reconciliation checks open peer obligations (`pending`/`observed`/`applied`), live WorkerSessions, and mission pause state before marking an employee sleeping.
- Mission pause records the paused schedule barrier, refuses new peer sends, and resumes paused employees to either `working`, `wake_pending`, or `sleeping` based on current state.
- Preserved historical migrations and updated the pinned forward migration hash manifest.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on a disposable PostgreSQL 18 instance. The script migrated to Schema 60, inserted pending/observed/applied peer-work and paused-Mission fixtures, applied Schema 61, and verified open-work rows backfill to `wake_pending` / paused rows to `paused`. It also verified new-company initialization, idempotent signal replay, eight concurrent signal/reconciliation pairs, FYI suppression, pause denial, observed-obligation wake after Worker stop and resume wake restoration.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -c 'export GOOS=windows GOARCH=amd64; bash scripts/go.sh build ./cmd/...'` — passed.
- `scripts/test-migration-hash-manifest.ps1` — passed.
- `rtk git diff --check` — passed.
- Independent read-only code review: passed with no Critical, Important, or Minor findings.

No model turn, provider, QQ, MCP, GitHub, business account, or production database was used.

## Limits

This closes the durable sleep race for the existing peer-obligation source only. Task and Routine wake sources, a listener/reconnect/startup-scan loop, due-time/fairness and quota handling, automatic Worker admission, Workbench schedule projection, and host/runtime qualification remain open. `NOTIFY` is only a latency hint; this slice does not start a listener or Worker.
