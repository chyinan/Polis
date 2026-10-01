# REQ-14 capability revocation dispatch map

Updated: 2026-10-01  
Status: mapping, Slice 103 projection, and Slice 104 revoke-time snapshot; Worker stop behavior is not wired.
Schema: 73.

## Contract boundary

`spec/design-v0.4.5/contracts/C-REVOKE.md` separates three facts: a revocation was accepted, new dispatch is blocked, and affected execution is quiesced. A database write can establish the second fact for later calls, but it cannot prove that an already-dispatched external operation stopped. Such operations must remain visible as in-flight or outcome-unknown until isolated and reconciled.

## Current capability paths

| Path | Persisted gate / dispatch point | Behavior after revocation | Gap against C-REVOKE |
|---|---|---|---|
| Global Skill/MCP approval revocation | `Kernel.TXDecideCapability` uses the company-scoped `TXWrite`; it changes capability status, records the decision, appends revoked employee-binding events, and (for MCP) revokes runtime-qualification events in one transaction. | Subsequent authorization reads reject the revoked capability and bindings. The capability catalog now projects accepted/effective/quiesced state for currently active revocations. | No affected-Worker stop or restart reconciliation exists; the projection inventories sessions only when Skill-load/MCP-intent usage is recorded. |
| Per-employee Skill/MCP unbind | `Kernel.TXRevokeEmployeeCapability` appends a `revoked` binding event through the same company-scoped transaction guard. | Subsequent checks for that employee reject the binding. The current employee revoke appears in the capability catalog projection with its recorded uses. | It does not stop an already-running Worker or include bound sessions with no recorded Skill/MCP use. |
| Skill text load | `TXLoadBoundReadOnlySkill` checks the current revision, approval, binding, and qualification on every load. A replay opens a fresh guarded transaction and rechecks before returning text. The use event is durable. | New loads and replayed loads are denied after the revoke commits. The projection inventories sessions from `capability.skill.loaded` events. | Text already returned to a model context cannot be recalled. The revoke does not yet stop the affected session. |
| Controlled MCP tool call | `TXBeginStdioMCPToolCall` re-authorizes in its transaction and commits a unique `dispatching` intent before Control invokes the pinned process/endpoint. The intent binds session, employee, capability, runtime qualification, provider call ID, tool/schema, and argument digest. | A later call is rejected by current capability/binding/runtime-qualification checks. `TXCompleteStdioMCPToolCall` intentionally permits a previously `dispatching` intent to complete, even if revocation happened meanwhile. The projection lists latest intent status and counts dispatching calls. | The committed intent is the practical admission/linearization point, but it is not a distinct expiring DispatchPermit. Revocation does not cancel or stop the active call. |
| MCP Worker stop | Control stops the Worker process first. `stopAfterWorkerStop` then asks Kernel to mark pending calls `outcome_unknown`; Kernel allows this only for `stopped` or `reconcile_required` sessions. | Confirmed stop can isolate the local process and preserve an unknown external result. | Capability revocation is not wired to this stop path. The current non-test call site is normal Worker cleanup; no capability-revocation coordinator or restart sweep for affected calls was found. |
| Automatic product Worker dispatch | Slice102 selects only the fixed `compat/emp-backend` Task and requires exact zero-egress Fake @7. It is disabled by default and cannot select a real-provider runtime. | Capability revocation does not enable a provider call; the Fake surface remains zero-egress. | This protects the current simulation path but does not implement REQ-14 for qualified Skill/MCP or future provider paths. |

## Current ordering guarantees

- A Skill load/replay and the revocation command use the same company transaction guard, so the guarded database read is ordered with the revocation commit.
- An MCP call has a separate preflight authorization, then a second authorization inside `TXBeginStdioMCPToolCall`; the latter and its `dispatching` intent are atomic with respect to the company guard and the MCP server lock.
- The MCP process/HTTP call occurs after that transaction commits. Revocation can block later calls but cannot retract a request already admitted at this point. A returned result may therefore be recorded after revocation; an interrupted call must remain unknown.
- No path in this map justifies reporting `quiesced` from the capability event alone.

## Slice 103–104 projection and snapshot behavior

`Kernel.ListCapabilityCatalog` rebuilds current global and employee-scoped revocation rows in its repeatable-read transaction. Schema 73 adds append-only revoke-time session and MCP-intent snapshot tables. The global approval revoke and employee unbind commands capture affected sessions under the same company guard that serializes Worker creation and capability dispatch. The snapshot includes live sessions for employees bound to the revoked version (or the targeted employee for unbind) and stopped sessions with recorded Skill/MCP use. It freezes the session state and Skill-load count and links exact MCP intent IDs with their initial statuses. The projection reads this snapshot for new revocations; existing current revocations with no snapshot continue to use durable Skill-load/MCP-intent evidence.

The projection reports `revocationAccepted` and `effectiveForNewDispatch` independently from `quiesced`. Quiescence is true only when every inventoried session is `stopped` and no snapshotted MCP intent remains `dispatching`; an `outcome_unknown` intent remains listed and does not block quiescence after its Worker is stopped. `reconcile_required` is still a live session. Output is bounded at 64 revocations and 64 session/call detail rows per revoke; complete aggregate counts and truncation markers remain available. The data survives restart without process-local state.

The projection reports currently effective revocations only. It does not stop Workers or expose superseded historical revoke decisions. The Workbench displays the projection but performs no stop action. Tests and PostgreSQL migration execution were not run for Slices 103–104; Go command and frontend production builds plus `git diff --check` pass. Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-103-capability-revocation-projection/verification.md` and `evidence/development/r1-r3-implementation-validation-20261001-slice-104-revocation-session-snapshot/verification.md`.

## Next implementation boundary

Continue REQ-14 by adding idempotent, restart-reconcilable Worker stop coordination and outcome-unknown retention, then expose a management action only after the stop path is reviewable. Keep provider, QQ, MCP endpoint, and production qualification actions disabled unless separately authorized.

Relevant code: `internal/kernel/capability_governance.go`, `internal/kernel/capability_skill_runtime.go`, `internal/kernel/mcp_runtime_qualification.go`, `internal/kernel/mcp_tool_calls.go`, `internal/control/stdio_mcp_worker.go`, and `internal/control/real_provider_worker.go`.
