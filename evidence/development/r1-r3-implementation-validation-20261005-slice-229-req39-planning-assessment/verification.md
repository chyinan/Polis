# Slice 229 — REQ-39 bounded Planning impact assessment

## Change

Added Schema 107's append-only Planning-assessment store and database trigger; Schema 108 safely extends its open-request guard to allow a new receipt after a considered request's Mission is resumed. Each assessment is immutable, size-bounded, hash-checked by the Kernel, capped at 20 revisions per request, and tied to the exact change request, current requirements/inputs/tasks/artifacts/takeover-snapshot basis, active `emp-planning` WorkerSession, Task and epoch. The database independently rejects inserts unless that session is active, provider-bound, current-epoch, attached to a working Planning-owned Task in the request Mission, and the request is open.

The Planning-only `mission_change_request_read` and `mission_change_impact_assess` tools are in the exact fake-only product surface @12. They return only the one open request in the bound Mission and accept no caller-supplied employee/session identity. A complete disjoint Task classification and bounded summary, risk, questions and controls are required. High-risk or uncertain assessments cannot be considered while prior results remain deliverable. Owner review must submit the exact current assessment digest; changed bases or assessment receipts fail closed. The Workbench shows assessment freshness, risk, Task groups, questions, controls and Worker provenance.

The @12 surface has 25 tools, manifest digest `cd9f4f852cd5674b1e0660868cb5af628a0186a0f137e8e1778e8fd6809c326d`, aggregate schema digest `0be7f09a90c96ed73596617207ed1bac7575c6b8fd56cfe086a5880352c6b28a`, and 6740 schema bytes. It is opt-in with `POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED=1`, requires `POLIS_WORKER_MODE=real` and fake transport, and cannot be combined with another offline extension. Real-provider @4 authorization remains unchanged.

## Verification

- `go build ./...`: passed.
- `npm run build`: passed (`tsc -b` and Vite production build); Vite emitted the existing large-chunk advisory.
- Rebuilt `.runtime/bin/polis`; `.runtime/dev-termux.sh migrate` applied Schema 108 successfully.
- PostgreSQL reports Goose version 108, 108 migration-evidence rows, zero Companies, zero WorkerSessions and zero active WorkerSessions.
- `(cd db/migrations && sha256sum -c ../migration_hashes.sha256)`: all 108 migration entries passed.
- Backend health after restart: ready at `127.0.0.1:8080/healthz`; Vite remains on loopback at `127.0.0.1:4173`.
- `git diff --check`: passed.
- No test, formal change request, Planning assessment, Worker/provider turn or frozen scenario ran. All frozen scenario states remain `not_run`.

## Disposition

This closes the local implementation seam identified in Slice 228. It does not qualify model behavior, owner review usability, restart recovery or any of the eight REQ-39 frozen scenarios. The database currently has no active Planning WorkerSession, so the new Worker path was not exercised. REQ-39 remains `partial` / `not_run` pending session-backed qualification and frozen scenarios.
