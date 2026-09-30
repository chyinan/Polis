# Slice 93 — live employee schedule reconciliation

Date: 2026-09-30  
Schema: 64 (forward-only notification routing migration)

## Scope

`polis serve` starts the employee schedule reconciler after subscribing to `polis_employee_wake` and before starting the HTTP server. Schema 64 carries exact company and employee IDs in the transient hint; the receiver reconciles that scope only. It performs an initial full scan, reconnects with bounded exponential backoff, and repeats a full scan after reconnect. A 30-second periodic scanner advances through at most 128 schedule rows per tick with a keyset cursor. Shutdown cancels and joins both loops. The reconciler only updates persistent schedule projections; it never creates a WorkerSession or starts a Worker.

The notification handler targets the exact company/employee schedule; durable Task, peer-obligation, and Routine occurrence rows remain authoritative if notification delivery is lost.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on a disposable PostgreSQL 18 cluster through Schema 64. Covers initial reconciliation, exact company-scoped notification without cross-company effects, terminating the listener backend and recovering after reconnect, periodic repair with no notification, Routine pause/resume, and Workbench projection.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 rtk bash scripts/go.sh build ./cmd/...` — passed as a cross-build.
- `rtk git diff --check` — passed.
- Independent read-only review — one Minor finding about globally scanning the first page on notification was fixed by routing the hint to company+employee and reconciling only that schedule; no Critical or Important findings remain. The expanded cross-company test passes.

No provider, Worker, QQ, MCP, GitHub, or production account action ran. REQ-13 remains partial: due Routine materialization, fairness/quota, Routine-to-Task delivery, and automatic Worker admission are not implemented.
