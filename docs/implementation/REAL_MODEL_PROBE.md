# R0.1 real probe — consumed allowance and proposed retest

The original authorization was 3 Medium + 3 High turns maximum, concurrency 1, 10 minutes. The attempt consumed **3 Medium, 0 High**, ending inconclusive due to a missing code-mode host. The fixture was not modified and no checkpoint/artifact was accepted. See `R0_1_REPORT.md` and `evidence/development/r0.1/assessment.json`.

Do not delete, overwrite or reset `real/allowance.json`. Do not use a new label to evade that authorization. A new real run requires an explicit owner grant. The proposed smallest complete retest is **2 Medium + 1 High**, 10 minutes, concurrency 1, no unknown-outcome retry. These limits are configurable downward and covered by a regression test.

After that authorization only, start the dedicated test PG using README instructions. Start `rtk proxy python -u scripts/r01-proxy-bridge.py` in a separate Windows terminal; it reuses the existing local proxy without TLS interception or system changes. Then run this command from the workspace in WSL:

```sh
POLIS_R01_RUN_LABEL=r01-retest-1 \
POLIS_R01_MEDIUM_LIMIT=2 \
POLIS_R01_HIGH_LIMIT=1 \
bash scripts/r01-real.sh
```

The script creates a dedicated database, runs no-inference native inspection first, then records a new allowance before any `turn/start`. Evidence goes to `evidence/development/r0.1/r01-retest-1/`. The original failed record is untouched. The prepared command has **not** been run or authorized in this session.

The profiles remain `gpt-5.6-sol` with Medium and High effort, both under pinned Codex 0.151.0. Only disposable formatting source, the neutral bundle and fixture test results may reach the provider. Native credentials stay in the normal CLI auth path through a read-only mount; Polis does not extract or copy token material. Usage is reported as observed native counters, not dollars. No QQ, external MCP, GitHub feedback, cross-provider run or extra real employee is included.
