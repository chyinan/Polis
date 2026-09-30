# Slice 98 — Routine occurrence-to-Task delivery

Date: 2026-09-30

## Change

Forward-only Schema 71 adds a bounded per-Routine task instruction, an immutable occurrence instruction snapshot, and a unique occurrence-to-Task link. New Routines reject empty, noncanonical, invalid UTF-8, or oversized task instructions. Existing instruction-less Routines and their pending occurrences are preserved as `needs_instruction`; supplying an instruction converts each blocked occurrence into one linked Task in the same transaction. Due occurrences with an instruction create one ready `compute` Task, bind the Mission input manifest, store the instruction/provenance plan and initial workspace, and link the occurrence atomically. `Task.Plan` is included in Worker Handover. Schema 72 adds consistency checks for linked Tasks and instruction snapshots. Task completion closes the occurrence; terminal Mission transitions cancel blocked and still-ready Routine work.

Paused Missions may retain the ready Task, but the existing Mission and EmployeeSchedule barriers reject Worker admission. The generic Worker reservation path can create the local session for `compute` Tasks; the product-provider authorization path still rejects them. No scheduler starts a Worker, no real model/provider was called, and no external account was used. Routine create/repair is not yet exposed through Workbench.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed against a dedicated temporary PostgreSQL 18 cluster; migrated Schema 60→72 and removed the database/cluster on exit.
- The dedicated suite seeds an instruction-less pending Routine under an active Mission at Schema 68, upgrades it through Schema 72, verifies `needs_instruction` with no Task/link/snapshot, then repairs it once. It also covers missing-instruction rejection, one linked Task per due occurrence, all frozen plan fields in Worker Handover, Mission input-manifest candidate digest, persisted workspace digest, Product Provider denial with no WorkerSession, paused admission denial, terminal cancellation/completion, reconnect/materialization, and Workbench schedule readback.
- A disposable-database trigger injects failure at occurrence insertion after the Task, input manifest, and workspace writes have begun; the test verifies the materialization receipt, Task, input manifest, workspace row, occurrence, cursor, and employee generation all roll back.
- `rtk bash scripts/go.sh test ./...` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — passed.
- Migration hash manifest updater added only forward Schema 71 and 72 pins; prior migration hashes were preserved.
- `rtk git diff --check` and the migration hash manifest regression script — passed.
- Independent read-only re-review confirmed all findings fixed and returned zero remaining findings.

Remaining REQ-13 work is fairness/quota readiness and automatic Worker admission. Routine authoring and legacy instruction repair still need authenticated Workbench routes and UI. The profile remains local/test-verified only.
