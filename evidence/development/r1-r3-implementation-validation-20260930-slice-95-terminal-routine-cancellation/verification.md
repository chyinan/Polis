# Slice 95 — Terminal Mission Routine cleanup and upgrade backfill

Date: 2026-09-30

## Change

Schema 69 handles future terminal Mission transitions and remains pinned. Schema 70 performs a forward-only backfill of existing pending Routine occurrences attached to Missions already in `cancelled` or `succeeded`. Paused Missions and their pending occurrences are preserved.

The employee-schedule PostgreSQL script upgrades to Schema 68, seeds both kinds of terminal Mission with a pending occurrence, then applies Schema 69 and 70. The new check reported both rows as `pending` before the Schema 70 update and both as `cancelled` afterward.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed; `R1_EMPLOYEE_SCHEDULE_WAKE_PROTOCOL=PASSED`.
- `rtk bash scripts/r2-cross-backend-handover-postgres-test.sh` — passed; includes the dedicated migration execution ledger test through Schema 70.
- `rtk powershell.exe -NoProfile -File scripts/test-migration-hash-manifest.ps1` — passed; the 69 existing migration pins were unchanged and Schema 70 was added as a forward migration.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -lc "GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/..."` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk git diff --check` — passed.

No real Worker, model, QQ send, MCP endpoint, GitHub account, or business database was used.
