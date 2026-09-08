# Polis current progress — R0.2

Baseline: `51383e7`; R0.1 implementation checkpoint: `febe1b6`. R0's historical result is preserved in `PROGRESS_R0.md`.

**R0.2 overall is inconclusive.** The repaired integration completed both authorized Luna Medium gates, including real mediated tool calls and same-Employee handover. The optional High behavior review was not started because the persisted ten-minute allowance expired; High consumption remains zero. R0.1's separate failure and allowance are untouched.

- [x] Pin Codex/app-server 0.151.0 and compare Windows/Linux experimental schemas.
- [x] Implement control-owned Employee/attempt/epoch/session binding, read-only restoration and process-bound stop proof.
- [x] Implement seven bounded employee tools, typed input checks, stable receipts, checkpoints, program evidence and neutral bundle.
- [x] Implement a signed-zero compatibility fixture and independent behavior checks.
- [x] Test PG transactions, stale/foreign identities, pause/restore denial and legitimate neighbors, process ownership, fake handover and native callback.
- [x] Run the authorized R0.2 real experiment and preserve its Medium consumption and evidence.
- [x] Repair the review-task uniqueness issue found after the two Medium gates; retain the original failure.
- [ ] Complete the independent High behavior review after renewed authorization; do not reset the expired allowance.

Do not reset `evidence/development/r0.1/real/allowance.json` or `evidence/development/r0.2/luna-1/allowance.json`. R0.2 consumed 2 Medium turns and 0 High turns; the 10-minute window is expired. The later High-only command stopped before reservation. A new High review requires explicit renewed authorization; it is not permission to evade either allowance.

Fresh validation after the R0.2 review-task fix: `bash scripts/test.sh` passed on real PostgreSQL 18.6 with all Go tests, `go vet` and command builds; `POLIS_TEST_RACE=1 bash scripts/test.sh` passed; the native schema validator passed. The R0.2 offline worker lifecycle and scripted behavior tests passed. The two real Medium turns are recorded in `evidence/development/r0.2/luna-1/`.

Post-run review hardening is also tested: recovery binds final candidate content to successful tool receipts/checkpoint evidence; High recovery preserves the original ten-minute deadline; High requires a successful session/digest-bound checkpoint; old-writer rejection distinguishes authorization errors from infrastructure failures.

The immutable R0.2 log was re-read by `TestLoadRecoveredRealEvidenceFixture`; it resolves the successful Medium2 artifact content and checkpoint without trusting a later failed request. The parser's synthetic conflict case is also passed.

No QQ, external MCP, feedback, UI, cross-provider experiment or parallel real employees were introduced. Core R0 invariants remain regression-tested. Full OS/provider security qualification and independent High behavior acceptance remain unproven.
