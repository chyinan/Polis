# R0.3A-T1 Native Provider Reconnect Lifecycle Qualification

Status: `PASSED` for the deterministic native WorkerAdapter lifecycle qualification.

No Medium, High, reset, or real provider call was used in T1. The sealed R0.3A real run remains `INCONCLUSIVE`; its allowance and protocol are unchanged.

## Root cause

The sealed real protocol emitted structured `method: "error"` events whose `params.error.codexErrorInfo.responseStreamDisconnected` field was present and whose `willRetry` field was `true`. `codex.Client.receive` returned those messages as ordinary asynchronous messages. `Client.Turn` had no `error` branch, so it neither entered a reconnect state nor established a reconnect deadline; it continued waiting until the outer ten-minute context. The provider's natural-language `message` was not the source of the new classification.

The sealed real event sequence is preserved in [r03a_t1_disconnect_protocol.jsonl](D:/Programs/Polis/internal/codex/testdata/r03a_t1_disconnect_protocol.jsonl), extracted from the original Backend protocol. It contains the original `turn/started` followed by six reconnect error events in order.

## Lifecycle and time semantics

The adapter now records explicit lifecycle events:

`normal_running → reconnecting → recovered → normal running`

or, on failure:

`reconnecting → reconnect_deadline_exceeded`

Other terminal states are explicit: `explicit_terminal_provider_failure`, `total_turn_deadline_exceeded`, `stop_requested`, `actual_termination_confirmed`, and `outcome_requiring_reconciliation`.

All elapsed deadlines use Go monotonic time carried by `time.Time`; wall-clock timestamps are audit fields only. The configurable timeout fields are:

- start acknowledgement deadline;
- first valid output deadline;
- streaming idle deadline;
- reconnect grace/deadline;
- total turn deadline;
- stop acknowledgement deadline.

The reconnect deadline is capped at the total turn deadline. A stop acknowledgement is not treated as termination: the adapter waits for a correlated terminal `turn/completed` event before reporting `actual_termination_confirmed`.

## Replay and reconciliation

Tool invocations are keyed by the same turn identity and native `callId`. A duplicate event reuses the first tool result and does not call the Polis handler again. A duplicate identity with changed tool or arguments returns `outcome_requiring_reconciliation`. This preserves existing Polis idempotency keys instead of generating a new business action after reconnect.

The stop-during-reconnect fixture verifies `RequestStop → turn/interrupt acknowledgement → terminal completion`; missing terminal confirmation remains reconciliation-required.

Provider errors are classified from structured fields. `responseStreamDisconnected + willRetry=true` is reconnectable; other structured error events are explicit terminal provider failures. Warnings without a structured code are non-terminal compatibility events. `code_mode_unavailable` is the only structured capability warning currently gated. The adapter does not use provider prose such as `Reconnecting...` for transport semantics.

## Deterministic tests

The following all passed:

- actual disconnect fixture is stable and preserves event order;
- bounded no-recovery path stops at reconnect deadline;
- recovered same-turn path completes normally;
- duplicate tool call is delivered once;
- disconnect after a tool call plus replay is delivered once;
- explicit terminal provider failure is not classified as reconnect;
- stop during reconnect requires both acknowledgement and terminal confirmation;
- unstructured warning is non-terminal while structured capability warning is gated.

Exact commands:

```text
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/probe -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test -race ./internal/codex ./internal/kernel ./internal/probe -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh vet ./internal/codex ./internal/kernel ./internal/probe
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh build ./cmd/...
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/test.sh
rtk git diff --check
```

The complete PostgreSQL suite also passed after the transport changes; the tracked R0.1 timing evidence was restored afterward.

## Freeze and next authorization boundary

- R0.3A deterministic qualification remains `PASSED`.
- R0.3A real run remains `INCONCLUSIVE`.
- R0.3A parent ProblemKey and sealed allowance remain unchanged.
- R0.1/R0.2/H2/H3 evidence and hashes remain unchanged.
- T1 made no changes to the peer fixture, ContractRevision, Message, Obligation, Reviewer, or real-run evidence.

The technical condition to request a new real Backend Medium is now met: `R0.3A-T1 native_transport_qualification=PASSED`. This is not an authorization to resume the sealed allowance; a future real turn requires a separate explicit owner decision and a new allowance boundary.
