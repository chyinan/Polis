# Polis current progress — R0.1

Baseline: `51383e7`. R0's historical result is preserved in `PROGRESS_R0.md`.

**R0.1 overall is inconclusive, not completed real integration.** The code/fixture/protocol binding and offline tests are implemented. The one real attempt used 3 Medium turns and 0 High turns, then stopped. Native inference worked, but missing `codex-code-mode-host` prevented every employee tool from reaching Polis. No real checkpoint, artifact or handover was produced.

The packaging defect is fixed. A deterministic local Responses provider now drives the actual pinned app-server through a native dynamic-tool callback, without any real model call. The real attempt was not retried after the fix. Read `R0_1_REPORT.md` and `evidence/development/r0.1/assessment.json` for the distinction between original evidence and post-fix offline results.

- [x] Pin Codex/app-server 0.151.0 and compare Windows/Linux experimental schemas.
- [x] Implement control-owned Employee/attempt/epoch/session binding, read-only restoration and process-bound stop proof.
- [x] Implement seven bounded employee tools, typed input checks, stable receipts, checkpoints, program evidence and neutral bundle.
- [x] Implement a signed-zero compatibility fixture and independent behavior checks.
- [x] Test PG transactions, stale/foreign identities, pause/restore denial and legitimate neighbors, process ownership, fake handover and native callback.
- [x] Run the single authorized real experiment and preserve its failure and consumption.
- [x] Repair discovered verifier, pause and native-package qualification faults; retain regression evidence.
- [ ] Complete single real employee delivery and real profile handover after a newly authorized retest.

Do not reset `evidence/development/r0.1/real/allowance.json`. Its Medium allowance is exhausted. A new explicit run label is supported, but creating a new real run requires renewed owner authorization; it is not permission to evade the existing allowance. Proposed retest: at most 2 Medium + 1 High turn, 10 minutes, concurrency 1, unknown outcomes stop.

Commands: `bash scripts/test.sh`; `POLIS_TEST_RACE=1 bash scripts/test.sh`; `.tools/doccheck/Scripts/python -X utf8 scripts/check-r01-schema.py` from Windows. Exact host startup and commands are in README and `R0_1_REPORT.md`. Current local checkpoint is the latest Git commit for this task; no remote push is authorized.

No QQ, external MCP, feedback, UI, cross-provider experiment or parallel real employees were introduced. Core R0 invariants remain regression-tested. Full OS/provider security qualification, product intelligence and behavioral model handover remain unproven.
