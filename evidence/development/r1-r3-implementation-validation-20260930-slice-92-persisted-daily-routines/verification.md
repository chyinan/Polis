# Slice 92 — persisted daily Routine occurrences

Date: 2026-09-30  
Schema: 63 (forward-only migration)

## Scope

Schema 63 stores daily Routine definitions, request-idempotent materialization receipts, and unique occurrences keyed by Routine and logical day. Kernel materialization locks the Routine row and atomically persists its bounded plan, skip/coalesced ranges, occurrence rows, and cursor. Pending occurrence insertion advances the assigned Employee's durable work generation and emits a non-sensitive PostgreSQL wake hint. Schedule reconciliation considers pending occurrences only while their Mission is active. Mission pause preserves occurrence rows and the pause barrier; explicit resume restores pending routine work. `next_due_at` reflects the earliest active daily Routine deadline for an Employee.

This slice does not add a background timer or LISTEN/reconnect loop, fairness/quota admission, Routine-to-Task delivery, automatic Worker admission, or a Workbench Routine configuration page.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on a disposable PostgreSQL 18 cluster; migration from Schema 60 to Schema 63, employee wake, pause/resume, idempotent Routine materialization, Workbench schedule readback.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 rtk bash scripts/go.sh build ./cmd/...` — passed as a cross-build.
- `rtk powershell.exe -NoProfile -File scripts/test-migration-hash-manifest.ps1` — passed.
- `rtk git diff --check` — passed.
- Independent read-only review — no Critical, Important, or Minor findings.

No model, Worker, QQ, MCP, GitHub, or production account action ran. REQ-13 remains partial.
