# Slice 94 — bounded automatic due Routine materialization

Date: 2026-09-30  
Schema: 68 (forward-only migrations)

## Scope

The live schedule loop now materializes daily Routine occurrences when their persisted due instant has passed. Each pass backfills at most 32 missing deadlines and materializes at most 96 due Routines. Per-Routine work is additionally limited by `MaxDailyRoutineCatchUp=10`. The due timestamp is resolved using the explicit IANA timezone and deterministic DST policy. A Routine row lock, request receipt, unique logical-day occurrence key, and one PostgreSQL transaction preserve idempotency.

Schema 68 stores `scheduling_active`, synchronized with Company and Mission state by database triggers. Partial due, backfill, and per-employee next-due indexes exclude inactive schedules. Paused, cancelled, and archived Routines retain their logical cursor/deadline but do not occupy active due indexes. A cancelled Mission's pending occurrence cannot reawaken an Employee.

Materialization only creates durable occurrence records and updates the employee schedule. It does not create a Task, WorkerSession, model request, or external side effect.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 68; covers migration 60→68, automatic due materialization, no duplicate across repeated cycles, pause/resume, cancellation barrier, company-scoped notifications, reconnect, periodic scan, and Workbench readback.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 rtk bash scripts/go.sh build ./cmd/...` — passed as a cross-build.
- `rtk powershell.exe -NoProfile -File scripts/test-migration-hash-manifest.ps1` — passed.
- `rtk git diff --check` — passed.
- Independent read-only review — no Critical, Important, or Minor findings.

No Worker/provider, QQ, MCP, GitHub, or production account action ran. REQ-13 remains partial: fairness/quota admission, Routine-to-Task delivery, and automatic Worker admission are open.
