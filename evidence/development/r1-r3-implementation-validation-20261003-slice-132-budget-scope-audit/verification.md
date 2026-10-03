# Slice 132 — Company and ProviderAccount budget identity audit

Date: 2026-10-03

## Design and source findings

- `spec/design-v0.4.5/ARCHITECTURE.md` §14.1 allows separately dimensioned Company and ProviderAccount caps, requires one uniquely identified consumption fact projected to each applicable scope, and says a child cap is not automatically parent allocation.
- §14.2 and §34.1/34.5 reject presenting partially observed CLI/service retries or token/cost data as an exact USD hard limit. `internal/codex/client.go` exposes token usage observations, while `internal/control/real_provider_worker.go` persists them in terminal Worker observations; that is not a billing settlement or reserved/unknown liability ledger.
- Company has a stable primary key (`companies.id`). `Kernel.guardWithSessionMode` obtains `dbgen.LockCompany` before callback-level locks. Admitted Worker protocol calls are idempotently keyed from WorkerSession ID and provider call ID in `internal/kernel/employee_ops.go` and `peer_employee_ops.go`; the call is charged in the same `TXWrite` and projected to Task/ProblemKey/Mission counters. `tasks.task_tool_calls_used` is the available conservative backfill source for a Company protocol-tool-call counter.
- `worker_sessions.profile` identifies a model/effort profile, not a ProviderAccount. General `ExecutionProfile` and `SessionStartOptions` do not carry an account ID. Codex auth identity fingerprinting exists, but `CodexRuntime` captures it only for the specific LIVE_2 diagnostic auth-source pin; it is not bound into normal Worker admission/session state. It also does not settle provider spend.

## Decision

Proceed with a separately configured Company cap measured only in admitted protocol tool calls. Exclude model requests without tool calls, hidden provider CLI/service retries, token totals, raw provider egress and USD from that claim. Keep ProviderAccount financial caps unimplemented until the account identity is bound to general WorkerSessions and the runtime has explicit reserved, settled and unknown liability semantics.

## Verification

| Check | Result |
| --- | --- |
| Frozen architecture and retry contract source review | PASS; sections cited above |
| Company lock/idempotency/usage source review | PASS; `internal/kernel/kernel.go`, `internal/kernel/employee_ops.go`, `internal/kernel/peer_employee_ops.go`, and Schema 81/89 |
| Provider identity/token observation source review | PASS; `internal/provider/codex_runtime.go`, `internal/provider/runtime.go`, `internal/codex/client.go`, `internal/codex/auth_fingerprint.go`, `internal/control/real_provider_worker.go` |
| Tests/build/database/provider execution | Not run; this slice is a read-only design/source audit |

No frozen catalog, source code, database schema or runtime configuration was changed in this audit.
