# Polis R0.2 directed real retest

R0.2 used one fixed Codex app-server harness and the user-authorized GPT-5.6 Luna profile. The exact allowance was two Medium turns plus one optional High turn, concurrency 1, ten minutes total, with no unknown-outcome retry and no budget expansion.

The result is **INCONCLUSIVE overall** because the first two real execution gates passed, while the optional High behavior review was never started: the ten-minute allowance expired before the final High reservation. `high_turns` remains zero. The previous R0.1 failure, allowance and ProblemKey remain untouched.

## Real turns

Turn 1 used GPT-5.6 Luna Medium for the first real worker milestone. It took 59,024 ms. The model called `context_read`, `workspace_read`, `work_current`, `workspace_replace`, `workspace_check` and `work_checkpoint`. Polis accepted three receipts, including a passed compiled check and checkpoint `f77b1bacd26e3a7c77614f6dc990f0c5`. The process was force-stopped with `process-group:12991:waited`; the control plane recorded `stop_confirmed=true`. The milestone intentionally had no artifact yet so the session could be interrupted before overall completion.

Turn 2 used a new GPT-5.6 Luna Medium session with the same Polis EmployeeId `emp-backend`. It took 67,090 ms. The neutral bundle carried the current Task, one pending Obligation, workspace digest/revision, the signed-zero compatibility contract, facts, decisions, the rejected zero-collapsing approach and the first checkpoint evidence. The second session called `work_current`, `workspace_read`, `context_read`, `workspace_replace`, `workspace_check`, `work_checkpoint` and `artifact_submit`. It accepted four receipts, including checkpoint `b0ee253c776468fe4ac85c7f25ab7276` and candidate artifact `57761d55c7b0dbf63668b9f8010de41d`. The process was force-stopped with `process-group:14980:waited`; `stop_confirmed=true`. The old first binding was tested after the new epoch was issued and its write was rejected (`old_writer=rejected`).

The two Medium turns therefore demonstrate real mediated worker calls, persisted checkpoints, forced interruption, a new native session, same EmployeeId continuity, a neutral bundle and a continuation artifact. The original temporary database was cleaned up by the script after it wrote the immutable evidence. That cleanup exposed a review-task uniqueness bug before High; it is fixed offline by using the distinct `review` task kind. The High-only recovery command can reconstruct the exact Medium2 candidate from the protocol log, but its attempt was refused because the original ten-minute allowance had expired. It consumed no High turn.

## Behavior-level status

The real High behavior acceptance is **NOT RUN**. The real Medium2 model did produce a candidate that passed the deterministic full signed-zero checker in the original driver before the review-task creation failure, but that checker is not a substitute for the authorized independent High review. The offline equivalent remains passed: `TestEmployeeOpsBehavioralHandover` deliberately breaks signed-zero handling and fails the checker, then accepts the valid `RenderWithUnit` neighbor; the checker also rejects test-lifecycle bypasses.

The required High review would use `emp-review` with read-only worker tools, inspect the candidate, execute the full compiled behavior check, checkpoint why `-0` must remain distinct from `0`, explain the rejected zero-collapsing route, and then let the trusted controller perform the final deterministic acceptance. No real High turn was issued because the persisted deadline rejected reservation.

## Versions and fingerprints

Codex CLI/app-server: **0.151.0** on Windows and Linux. Experimental schema bundle fingerprint: `6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455`. The installed Linux native binary fingerprint and repaired helper/config/tool fingerprints are in [capability-after-fix.json](/D:/Programs/Polis/evidence/development/r0.1/real/capability-after-fix.json). R0.2 uses the same pinned app-server package and adds Luna as the model parameter; no cross-provider or native resume path was used.

## Validation

Fresh local validation after the review-task fix:

- `bash scripts/test.sh`: passed on real PostgreSQL 18.6; all Go tests, `go vet`, and all four command builds passed.
- `POLIS_TEST_RACE=1 bash scripts/test.sh`: passed with the Go race detector.
- `bash scripts/go.sh test ./internal/probe ./internal/codex ./internal/kernel`: passed.
- `bash scripts/go.sh test ./internal/runner`: passed.
- `bash scripts/check-r01-schema.py`: passed; 17 native requests/responses and 7 dynamic tool schemas checked, zero model turns.
- R0.2 `--inspect`: passed with Luna profile thread configuration and no turn.
- R0 deterministic PG/failure/restart tests remain passed.

The R0.2 real attempt is recorded in [luna-1/result.json](/D:/Programs/Polis/evidence/development/r0.2/luna-1/result.json), [handover-bundle.json](/D:/Programs/Polis/evidence/development/r0.2/luna-1/handover-bundle.json), both per-session protocol logs, and [allowance.json](/D:/Programs/Polis/evidence/development/r0.2/luna-1/allowance.json). `high-recovery-result.json` records the later pre-reservation refusal; it contains no High protocol log.

No QQ, MCP, GitHub feedback, browser, UI, production repository, production credential or extra real employee was used. Network access was only the temporary TLS CONNECT bridge to the configured OpenAI endpoint; it did not decrypt traffic or log credentials. Observed usage is recorded as native counters where available, never converted to dollars.

The next action requires a new explicit owner authorization for a High-only behavior review or a fresh bounded run. The expired R0.2 allowance cannot be reset, and no later stage starts automatically.

Post-run hardening added before this checkpoint: evidence recovery now accepts only a successful checked content digest followed by a successful checkpoint and matching `artifact_submit` receipt; it does not trust the last tool-call request. High recovery uses the original allowance deadline instead of a new ten-minute window. High acceptance requires a successful checkpoint bound to its session and current workspace digest. Old-writer evidence requires an explicit stale/denied authorization error plus an unchanged successor digest.
