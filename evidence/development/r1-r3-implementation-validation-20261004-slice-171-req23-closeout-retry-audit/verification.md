# Slice 171 verification — REQ-23 closeout retry and terminal-state audit

Date: 2026-10-04

## Findings and change

- Exact closeout retries are keyed to the original request ID and full immutable input fingerprint. Replays return the recorded intent; a different payload using that ID conflicts.
- New closeout requests cannot enter from `closing` or a terminal Mission. Mission start is draft-only, pause/resume check their exact source state, and finalization only changes a Mission from `closing` to the stored outcome.
- Legacy cancellation replay uses its durable `mission.cancel` receipt. The Workbench projection exposes the closeout request ID used for exact resume.
- Updated cancel and closeout query mutations to refresh the Company authority snapshot after success or failure. A host-stop failure that already persisted `closing` now immediately surfaces the stored intent and resume action.

## Verification

- Frontend production build — passed.
- `git diff --check` — passed.
- No tests, Worker/provider actions, database-backed closeout execution, restart exercise or frozen scenario ran.

REQ-23 remains partial pending restart and frozen-scenario qualification; this source audit is not execution evidence.
