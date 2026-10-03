# Slice 135 — ProviderAccount identity and liability binding audit

Date: 2026-10-03

## Source findings

- `internal/provider/runtime.go` exposes runtime mode, readiness, tool surface, execution profile, reservation, session start and aggregate stats. It has no ProviderAccount identity capability.
- `provider.ExecutionAuthorization` binds the Company, Mission, Task, Employee and WorkerSession plus execution/runtime factors. It has no ProviderAccount ID or identity snapshot.
- `internal/provider/codex_runtime.go` captures and rechecks an auth identity fingerprint for the special LIVE_2 credential path. Those private fields are not exposed to general Worker admission; other runtime purposes do not receive the same account-identity check.
- `worker_sessions` is scoped by Company and binds Employee/Task/session generation and runtime process state. Provider terminal observations retain token usage, tool calls, egress and retry visibility by WorkerSession, but do not bind an account.
- No ProviderAccount registry or durable request reservation, settlement and unknown-liability ledger exists. An identity fingerprint alone would not account for in-flight/unknown spend or hidden provider retries.

## Decision

Do not implement a strict ProviderAccount financial cap from current fields. The next foundation is an explicit versioned optional runtime identity capability with an immutable WorkerSession binding, followed by atomic request reservation, settlement and retained unknown-liability accounting. Sessions whose runtime cannot supply a validated identity remain unbound; admitted protocol tool-call and partial token observations are not account-dollar liability.

## Verification

| Check | Result |
| --- | --- |
| Source inspection of runtime, authorization, WorkerSession and provider-terminal paths | PASS; read-only |
| `git diff --check` | PASS |
| Code/schema changes | None |
| Builds/tests/PostgreSQL/provider execution | Not run |
