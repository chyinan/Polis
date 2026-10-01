# REQ-14 capability revocation dispatch map

Updated: 2026-10-01  
Status: read-only mapping; no runtime behavior changed.  
Schema: 72.

## Contract boundary

`spec/design-v0.4.5/contracts/C-REVOKE.md` separates three facts: a revocation was accepted, new dispatch is blocked, and affected execution is quiesced. A database write can establish the second fact for later calls, but it cannot prove that an already-dispatched external operation stopped. Such operations must remain visible as in-flight or outcome-unknown until isolated and reconciled.

## Current capability paths

| Path | Persisted gate / dispatch point | Behavior after revocation | Gap against C-REVOKE |
|---|---|---|---|
| Global Skill/MCP approval revocation | `Kernel.TXDecideCapability` uses the company-scoped `TXWrite`; it changes capability status, records the decision, appends revoked employee-binding events, and (for MCP) revokes runtime-qualification events in one transaction. | Subsequent authorization reads reject the revoked capability and bindings. | Receipt status is the generic `revoked`; no accepted/effective/quiesced projection or affected-session summary exists. |
| Per-employee Skill/MCP unbind | `Kernel.TXRevokeEmployeeCapability` appends a `revoked` binding event through the same company-scoped transaction guard. | Subsequent checks for that employee reject the binding. | It does not stop an already-running Worker or report outstanding operations. |
| Skill text load | `TXLoadBoundReadOnlySkill` checks the current revision, approval, binding, and qualification on every load. A replay opens a fresh guarded transaction and rechecks before returning text. The use event is durable. | New loads and replayed loads are denied after the revoke commits. | Text already returned to a model context cannot be recalled. There is no active-session invalidation or quiescence record for that use. |
| Controlled MCP tool call | `TXBeginStdioMCPToolCall` re-authorizes in its transaction and commits a unique `dispatching` intent before Control invokes the pinned process/endpoint. The intent binds session, employee, capability, runtime qualification, provider call ID, tool/schema, and argument digest. | A later call is rejected by current capability/binding/runtime-qualification checks. `TXCompleteStdioMCPToolCall` intentionally permits a previously `dispatching` intent to complete, even if revocation happened meanwhile. | The committed intent is the practical admission/linearization point, but it is not a distinct expiring DispatchPermit and lacks a revocation-linked in-flight ledger. Revocation does not cancel or stop the active call. |
| MCP Worker stop | Control stops the Worker process first. `stopAfterWorkerStop` then asks Kernel to mark pending calls `outcome_unknown`; Kernel allows this only for `stopped` or `reconcile_required` sessions. | Confirmed stop can isolate the local process and preserve an unknown external result. | Capability revocation is not wired to this stop path. The current non-test call site is normal Worker cleanup; no capability-revocation coordinator or restart sweep for affected calls was found. |
| Automatic product Worker dispatch | Slice102 selects only the fixed `compat/emp-backend` Task and requires exact zero-egress Fake @7. It is disabled by default and cannot select a real-provider runtime. | Capability revocation does not enable a provider call; the Fake surface remains zero-egress. | This protects the current simulation path but does not implement REQ-14 for qualified Skill/MCP or future provider paths. |

## Current ordering guarantees

- A Skill load/replay and the revocation command use the same company transaction guard, so the guarded database read is ordered with the revocation commit.
- An MCP call has a separate preflight authorization, then a second authorization inside `TXBeginStdioMCPToolCall`; the latter and its `dispatching` intent are atomic with respect to the company guard and the MCP server lock.
- The MCP process/HTTP call occurs after that transaction commits. Revocation can block later calls but cannot retract a request already admitted at this point. A returned result may therefore be recorded after revocation; an interrupted call must remain unknown.
- No path in this map justifies reporting `quiesced` from the capability event alone.

## Next implementation boundary

Start with a durable revocation-status projection and affected-session inventory for capability-bound Skill/MCP use. The projection must be able to report `effective_for_new_dispatch` while any affected Worker or `dispatching` intent remains, and report `quiesced` only after each affected Worker is confirmed stopped and each in-flight intent is completed or retained as `outcome_unknown`. Make the operation idempotent and restart-reconcilable before wiring the management command to stop Workers. Keep provider, QQ, MCP endpoint, and production qualification actions disabled unless separately authorized.

Relevant code: `internal/kernel/capability_governance.go`, `internal/kernel/capability_skill_runtime.go`, `internal/kernel/mcp_runtime_qualification.go`, `internal/kernel/mcp_tool_calls.go`, `internal/control/stdio_mcp_worker.go`, and `internal/control/real_provider_worker.go`.
