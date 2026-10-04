# Slice 170 verification — REQ-23 owner closeout decisions

Date: 2026-10-04

## Changes

- Added an owner closeout command for `ended_not_met` and evidence-backed `succeeded` through Control and the Workbench HTTP route.
- Added Workbench controls for the rationale, outcome, and exact success Artifact IDs. Interrupted closeout can be resumed from the persisted request ID and original intent; legacy cancellation resumes through the existing cancellation command.
- Added the closeout request ID to the Workbench projection for durable retry.
- Kernel admission rejects success before entering `closing` unless the referenced Artifacts are ready and independently passed and all Tasks/Obligations are settled. Finalization continues to recheck those conditions.
- No schema migration.

## Verification

- `go build ./cmd/polis ./internal/control ./internal/workbench ./internal/kernel` — passed.
- `npm run build` in `frontend/` — passed. Vite emitted the existing advisory for a JavaScript chunk larger than 500 kB.
- `git diff --check` — passed.
- Tests, database-backed closeout execution, Worker/provider activity, restart qualification and frozen scenarios were not run.

REQ-23 remains partial pending restart and frozen-scenario qualification. Source/build verification does not change the execution status of those scenarios.
