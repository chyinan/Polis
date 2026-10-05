# Slice 270 — REQ-36 Worker environment status

## Implemented

- Added `polis_environment_status` to a distinct fake-only product tool surface, `polis-product-tool-surface@14`; the fixed product @4 registry and real-provider authorization remain unchanged.
- Added an opt-in `POLIS_OFFLINE_ENVIRONMENT_STATUS_ENABLED=1` adapter path requiring `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`.
- The Kernel checks the exact bound WorkerSession and Task, then reads only environment revisions belonging to that Task's Mission in one repeatable-read, read-only transaction. Results are capped at 50 revisions and include persisted policy, executor qualification, and preparation state.
- Policy manifests are reparsed internally and their canonical digest/profile are checked before status is reported; invalid stored policy is downgraded rather than presented as approved.
- The response omits project/source bytes, workspace files, preparation logs, executor fingerprints, and qualification evidence references. Tool-call authorization treats the operation as read-only; the normal `checkSession(..., false)` state fence and Company/Mission budget gates remain in force.

## Verification

- `go build ./cmd/polis` — passed.
- Tool-surface metadata was calculated from the registered tool definitions: 8 tools, manifest SHA-256 `b59ff6acd757764e66c73e1b5e638c3ff1a64a06f44491c919e017f91c48e0af`, schema 2,282 bytes, schema SHA-256 `513160783bc7effd340097ee55519f6a993a3eb57e3ebccd70bf7884afca5270`.
- No tests were added or run. No database query/write, migration, WorkerSession, provider turn, browser operation, or frozen scenario ran.
- The recorded local database remains Schema 108; the latest migration source remains Schema 110.

## Remaining REQ-36 work

Worker `environment.ensure`, binding-aware preparation attribution/idempotency, DependencyChange proposal/acceptance, and an owner-defined dependency/license/script/size/network envelope remain open. Native Node/npm, WFP, dedicated-host and clean-VM qualification also remain open. REQ-36 stays partial; all frozen scenarios remain `not_run`.
