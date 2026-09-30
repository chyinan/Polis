# Slice 41 — version-bound Task takeover lease

Date: 2026-09-26

## Implemented

- Added an immutable Schema 37 Task takeover lease, one-active-lease-per-Task slot, append-only grant/return/release events, frozen requirements/workspace digests, and a guarded down migration.
- Added Kernel grant/return/release and read paths. Grant and return require a paused Mission and stopped writers. Return rechecks the immutable requirement base and exact workspace digest/revision, stores the submitted text as a MissionInput, records a bounded diff/effort summary, and does not mutate the old Task workspace.
- Connected the flow to Mission change requests so successor input mapping preserves `human_takeover` origin and source Task identity.
- Added session-token-gated Workbench HTTP routes and a Mission page panel that loads the exact frozen `workspace.txt`, edits a bounded snapshot, returns it, or releases the lease. The panel reports that a formal Mission change request is still required.
- Added strict frontend DTO validation, including latest-event/lease-state consistency, and RealWorkbenchApi request/response checks.
- Required the desktop session token for Task workspace content reads in tokenless loopback mode, as well as for all four takeover endpoints.

## Verification

- `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` — passed. Temporary PostgreSQL migrated through Schema 37; populated legacy/36/37 upgrade and downgrade guard tests passed; Kernel takeover and Mission change tests passed under `-race`; Workbench HTTP route/dispatch tests passed. The script removed only its uniquely named temporary database and cluster.
- `rtk proxy npm test` — passed, 12 files and 92 tests.
- `rtk proxy npm run typecheck` — passed.
- `rtk proxy npm run lint` — passed.
- `rtk proxy npm run build` — passed. Vite reports the existing single-bundle size warning (610.01 kB minified).
- `rtk proxy bash scripts/go.sh test ./internal/desktop -run TestTaskTakeoverEndpointsAlwaysRequireSessionToken -count=1` — passed.
- `rtk proxy bash scripts/go.sh test ./internal/desktop -run TestTaskWorkspaceContentAlwaysRequiresSessionToken -count=1` — passed.

No real model, external provider, QQ, MCP, GitHub account, project JobRun, or production service was used. This slice does not qualify a general filesystem editor or an R1 live employee workflow.
