# R0.3A-T2.1 Phase-Aware Reconnect Deadline Qualification

Status: `PASSED`.

No Medium, High, reset, or real provider call was used. The deterministic R0.3A result, first real R0.3A result, T1 result, T2 result, parent ProblemKey, allowances, and sealed evidence remain unchanged.

## Why T2 stopped at 30.004s

T2 entered `responseStreamDisconnected(willRetry=true)` before first valid output. The previous policy always computed `disconnect_start + ReconnectGrace`, with the default grace of 30 seconds, then selected the earliest deadline. That made the reconnect deadline win even though the first-valid-output deadline was later. T2 therefore ended at approximately `30.004149655s` without a tool call, receipt, token usage update, or business side effect. This is a transport-policy result, not proof that the provider could not recover.

## Phase-aware policy

Before first valid output:

```text
effective = min(first_valid_output_deadline,
                total_turn_deadline,
                outer_experiment_or_allowance_deadline)
```

After valid output/tool activity:

```text
effective = min(reconnect_start + post_output_reconnect_grace,
                total_turn_deadline,
                outer_experiment_or_allowance_deadline)
```

The reconnect phase is persisted in lifecycle events as `pre_first_output_reconnecting` or `post_first_output_reconnecting`. The total and outer deadlines remain hard caps. The phase policy is a pure monotonic-time function, so tests use fixed timestamps rather than real 30–180 second sleeps.

## Regression coverage

- T1 original disconnect fixture remains stable and now ends at the first-output deadline in the pre-output phase.
- T2 actual protocol is frozen in `internal/codex/testdata/r03a_t2_disconnect_protocol.jsonl`; the regression proves the old 30-second choice was earlier than the new first-output deadline.
- Pre-output recovery after more than 30 seconds but before first-output deadline is allowed.
- Pre-output no-recovery ends at first-output deadline.
- Post-output recovery within grace succeeds.
- Post-output no-recovery is bounded by post-output grace.
- Total turn and outer experiment deadlines always win over phase grace.
- Stop during both reconnect phases still requires interrupt acknowledgement and terminal completion confirmation.
- T1 duplicate/replay and reconciliation protections remain passing.

## Exact verification

```text
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/probe -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test -race ./internal/codex ./internal/kernel ./internal/probe -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh vet ./internal/codex ./internal/kernel ./internal/probe
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh build ./cmd/...
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/test.sh
rtk git diff --check
```

All commands passed. The tracked R0.1 timing evidence was restored after the full suite.

The technical condition to request one new Backend Medium is met. This qualification does not authorize that turn and does not reopen the sealed T2 allowance.
