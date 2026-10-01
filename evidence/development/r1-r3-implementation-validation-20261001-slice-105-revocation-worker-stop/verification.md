# Slice 105 verification — capability revocation Worker stop and restart sweep

Date: 2026-10-01  
Host: Termux, Android/arm64  
Schema: 73 (unchanged)

## Changes

- Added a bounded Kernel keyset query over immutable Schema 73 revocation snapshots. Non-stopped sessions are stop candidates; stopped sessions remain candidates only while an MCP tool call is still `dispatching`.
- Added an immediate and 15-second Control reconciliation loop. It receives wake hints after successful global capability revocation and employee capability unbind commands, advances a process-local keyset cursor, and shares the Mission lifecycle lock with starts/cancellations.
- Added exact-session stop support to deterministic and real-provider Worker adapters. In-memory sessions use their owned process/provider cleanup path; missing in-memory ownership delegates to existing host process-tree reconciliation with the configured Linux cgroup manager where available.
- After a confirmed stop, pending MCP tool calls are recorded as `outcome_unknown`. Stopped sessions with a failed MCP status write remain queryable and retry on later passes.
- No database migration was added. Current pre-Schema-73 revocations retain the read projection fallback but have no revoke-time snapshots for this stop coordinator.

## Verification

- `go build ./cmd/...` — passed for the local Android/arm64 host.
- `git diff --check` — passed after the final edits.
- Tests and PostgreSQL query/runtime behavior were not run. The new keyset query, host reconciliation retry, and MCP status recovery have not been exercised against PostgreSQL in this slice.

The coordinator was not started and no Worker was started or stopped. No provider, MCP endpoint, QQ, GitHub account, or production action was used. No persistent development database was modified.

## Next step

Continue the approved R1–R3 handoff. If any pre-Schema-73 active revocation needs quiescence enforcement, add a bounded fallback queue from the legacy durable use ledgers; otherwise move to the next open lifecycle requirements in the finite coverage list. Preserve the rule that revocation is effective for new dispatch at commit and quiescence is reported only after confirmed session stop and resolution of dispatching MCP calls.
