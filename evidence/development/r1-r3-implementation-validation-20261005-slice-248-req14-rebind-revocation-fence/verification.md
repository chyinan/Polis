# Slice 248 — REQ-14 capability rebind after revocation

## Change

Employee capability rebinding after a revoke now runs a fail-closed guard inside the company-locked bind transaction. If the historical revoke inventory is incomplete, rebinding requires an exact matching acknowledged_unresolved installation-owner review. That review records unresolved history; it does not mark the revoke quiesced or prove that an old session stopped. All current WorkerSessions for the Employee must be stopped before rebinding, and MCP rebinding additionally requires no dispatching call for that Employee/capability. Global capability revocations retain their global decision scope when the per-Employee revoke event is resolved.

The code does not stop Workers, write owner reviews, or create host-backed stop/restart proof. Runtime qualification remains open.

## Verification

- go build ./... passed on integrated source commit e0d93a7 (published implementation commit 4ab1cd4fd01f7149a5e406b0c561367702b89906).
- git diff --check HEAD^ HEAD passed.
- All 13 REQ-14 scenario rows remain partial/not_run; all 232 frozen scenarios remain not_run.
- No tests, database operation, Worker/provider action, owner review, or frozen scenario ran.
