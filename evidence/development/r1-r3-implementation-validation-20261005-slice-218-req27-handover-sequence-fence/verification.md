# Slice 218 — REQ-27 handover sequence fence

Date: 2026-10-05

## Change

`Kernel.Handover` reads its base Task/workspace/checkpoint/memory/budget context in a read-only repeatable-read transaction. `EmployeeTools.call` may then append direct-message targets, shared Artifact read history, and bound MCP tool sets using separate read APIs. A Company write between those reads could previously make one `work_current` response combine different Company versions.

The adapter now checks that the Company sequence still matches the sequence captured by the base Handover after composing all enabled projections. It rebuilds the complete read up to three times. If concurrent writes keep advancing the sequence, it returns a clear `CONFLICT` result asking the Worker to read current work again, without returning the mixed projection.

Source review also confirmed that the Worker binding, Employee epoch, runtime incarnation, and execution mode are checked on Handover reads; writes remain restricted to an active session in an active Mission. No Worker was started.

## Verification

- `go build ./...`: passed.
- `git diff --check`: passed.
- Parsed `R1_R3_TRACEABILITY_DISPOSITION.json` and verified all eight REQ-27 entries link this evidence.
- The crosswalk still contains 232 scenarios and 17 open software requirements; 112 dispositions remain `partial`, 120 remain `implemented`, and all 232 execution states remain `not_run`.
- No tests, database operation, Worker/provider action, or frozen scenario ran.

## Status

REQ-27 remains partial. This closes the mixed-sequence read gap in Worker Handover assembly. The eight mapped scenarios remain unexecuted; E-HANDOVER still requires the owner-approved behavioral protocol, thresholds/budget, and real model-transition evidence.
