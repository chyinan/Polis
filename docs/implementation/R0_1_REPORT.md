# R0.1 result — 2026-09-08

**Overall: inconclusive.** The real model transport worked, but the single-worker execution failed before any Polis tool callback. The real handover and behavioral acceptance were not run. The packaging fault is repaired and the repaired native callback path passes a deterministic local-provider test; no real model was called after the repair.

## Actual attempt and authorization

Owner authorized GPT-5.6 Sol Medium → GPT-5.6 Sol High, same harness/model with different reasoning profiles. Limits: at most 3 turns per profile, 6 overall, concurrency 1, 10 minutes, no unknown-outcome retries or automatic increase.

The one attempt ran from 05:33:22.102 to 05:34:51.482 UTC (89.38 seconds), using **3 Medium turns, 0 High turns**. Native model responses completed, but each requested tool failed inside Codex because `/codex-code-mode-host` was absent from the namespace. The Go bridge received zero `item/tool/call` callbacks. PostgreSQL independently confirms zero accepted tool receipts, zero checkpoints and zero artifacts, with the task/obligation retained and the native session stopped with a process receipt.

The last native cumulative usage observation reports 130,846 total tokens: 129,538 input (98,432 cached), 1,308 output; 373 reasoning tokens are a subset of output. These are native counters, not dollars. Fourteen usage updates must not be summed as independent totals. Three app-server turns can contain several provider requests; a turn limit is not a three-request spending guarantee.

The original `real/result.json` is preserved, including its erroneous default `single_worker=not_run`. `assessment.json` corrects that interpretation to **attempted and failed**, and the reporter implementation was fixed. Nothing in the failed record was rewritten as a pass.

## Implementation

- `db/migrations/00002_workers.sql`: persistent sessions/attempt bindings, write-grant states, process attachment, workspace revisions, program checks, checkpoints and historical late observations. Migration is explicit; existing R0 migration remains immutable.
- `internal/kernel/worker_state.go`, `employee_ops.go`: control-bound identity and stable native-call receipts; `work.current`, context/workspace reads, bounded CAS replacement, evidence-backed checkpoint and fixed artifact submission. Model arguments cannot supply employee/scope/epoch/task identity.
- `internal/kernel/recovery.go`: unresolved real workers become `reconcile_required`; fake-only automatic recovery cannot grant a replacement writer permission.
- `internal/codex/`: bounded stdio JSON-RPC, pinned thread/profile settings, dynamic-tool schemas, correlation checks, persisted turn allowance and no-retry transport failure handling. Native resume is unused.
- `internal/runner/`: Linux PID/mount namespace supervision, process-bound stop receipts, read-only native fixture access, mediated writes, and isolated frozen Go verification.
- `internal/fixture/`: signed-zero behavior plus adjacent optional-unit modification. Acceptance checks actual compiled behavior, not matching summary words.
- `internal/probe/`, `cmd/polis-probe/`, `scripts/r01-real.sh`: single-worker and forced-checkpoint handover driver under one allowance. The consumed allowance cannot be reset by simply restarting the script.

## Versions and fingerprints

Both installed Windows CLI and the downloaded Linux runtime are **Codex 0.151.0**. Their experimental app-server schemas were generated independently and are byte-identical. Linux binaries come from the integrity-checked `@openai/codex@0.151.0-linux-x64` package; no token extraction or credential copying was used. The unmodified native CLI accessed the authorized auth file through a read-only mount at its normal auth.json location.

- Schema bundle SHA-256: `6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455`.
- Native Codex binary SHA-256: `9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a`.
- Capability/configuration digest used in the failed real run: `4fd3254378d087f0f5503cdc89e569293ceb595d30adc0d27b73a4dd90a43285`.
- Repaired bundle/helper/configuration/tool fingerprints: `evidence/development/r0.1/real/capability-after-fix.json`. This is **offline evidence only**, not a real-model pass.

Control/test stack remains Go 1.26.8, PostgreSQL 18.6, pgx 5.11.0, goose 3.28.0 and sqlc 1.31.1. Bubblewrap 0.6.1 and the existing WSL Linux 6.6.87.2 kernel provide the measured local namespaces. General hostile-worker containment and all provider egress guarantees are not claimed.

## Results

| Layer | Result | Evidence |
|---|---|---|
| R0 regression, real PG, build, vet | passed | `r0.1/go-tests*.txt`, test script |
| Race regression including new lifecycle/behavior cases | passed | `r0.1/go-tests-race.txt` |
| Native initialization / two profile thread configurations without inference | passed | native protocol test and inspect traces |
| Repaired native dynamic callback through a local scripted Responses provider | passed | `TestNativeDynamicCallbackWithScriptedProvider`; no real model/account |
| Restore/stop/epoch/pause/identity deny-and-allow tests | passed | worker and lifecycle tests |
| Behavioral handover using scripted employee operations and frozen checker | passed | `TestEmployeeOpsBehavioralHandover` |
| Real single-worker execution | failed | model ran, native helper missing; no accepted work |
| Real forced-disconnection handover / High profile execution | not_run | primary allowance exhausted before partial work existed |
| Real behavioral handover acceptance | not_run | no real candidate or checkpoint |
| QQ, external MCP, feedback, UI, cross-provider work | not_run | outside authorized slice |

## Problems found and fixed

1. Native handshake/schema validation did not exercise tool execution. The package requires the sibling code-mode host, even though the original attempt disabled a code-mode feature toggle. The repaired launcher checks, mounts and fingerprints it. A native capability-unavailable warning is now a hard failure, and the new local scripted-provider test exercises a real app-server callback without spending inference allowance.
2. A candidate could previously exit in `init` before frozen tests ran. AST restrictions now reject lifecycle hooks, non-permitted imports, globals and compiler directives; JSON test output must show each frozen test ran and passed. The malicious early-exit case is retained as a regression.
3. Mission pause initially did not fence new worker writes/activation. Both write checks and activation transactions now require an active mission. Paused refusal and resumed legitimate operation are tested.
4. WSL could not reach the provider or the Windows loopback proxy, and Windows-executable interop inside WSL was unavailable. A temporary loopback CONNECT bridge over wsl.exe pipes reused the existing Windows proxy, allowing only the three provider/auth hosts. It did not decrypt TLS, inspect credentials, change WSL/system proxy settings or open a Windows network listener.
5. Final regression review found that the legacy fake dispatcher could claim the new `compat` task kind and that the arithmetic verifier did not explicitly reject another contract. Fake claims are now limited to their original kinds and arithmetic verification checks its contract. This prevents a legacy command from accidentally accepting a real-worker candidate under the wrong rules.
6. Unexpected tool/storage errors are now explicitly classified as `OUTCOME_UNKNOWN` and terminate the native turn instead of inviting a model retry. Tool results are recorded before forced termination, including checkpoint-boundary results. Expired contexts are rejected before sending a native request, and independent verification shares the probe deadline.

The single read-only reviewer confirmed fixes 2 and 3. Native packaging failure 1 was discovered by the real experiment, reproduced by new offline regressions and fixed without further real calls. Native stopped-session evidence and the unresolved task are retained; no “done” message was treated as acceptance.

## Reproduce

From PowerShell, in the prepared workspace:

```powershell
rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/start-test-pg.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/test.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/Programs/Polis && POLIS_TEST_RACE=1 bash scripts/test.sh'
rtk proxy .tools/doccheck/Scripts/python -X utf8 scripts/check-r01-schema.py
```

The actual real invocation was `rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/r01-real.sh`. It now refuses because the allowance already exists. Do not delete or reset that evidence to force a rerun.

The remaining ticket is a **newly authorized bounded retest of the repaired integration**, suggested maximum 2 Medium + 1 High turn, concurrency 1, 10 minutes, preserving this failed attempt and its consumption. No other integration should be added first. Successful scripted behavior is insufficient evidence that a real replacement model retained the constraint.

The exact prepared retest command and explicit new run label are in `REAL_MODEL_PROBE.md`; neither that new allowance nor its execution has been authorized. Temporary proxy processes were stopped at handoff. The dedicated failed-probe database is retained for inspection, with a read-only JSON snapshot; the PostgreSQL service is stopped after verification.
