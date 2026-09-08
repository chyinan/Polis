# R0 progress

- [x] Confirm empty designated workspace and preserve naming/kickoff instructions.
- [x] Verify source archive SHA-256 and CRC; preserve archive and extract immutable specification.
- [x] Inspect validator source; 172 checksums and 489 static checks passed.
- [x] Read current slice contracts and release applicability; no blocking semantic conflict found.
- [x] Inspect installed Codex 0.151.0 help and generate protocol schema without model turns.
- [x] Prepare dedicated PostgreSQL 18 and Go toolchain; record exact versions.
- [x] Write failing tests for the authorized vertical slice.
- [x] Implement and verify transactions, scope, evidence, recovery and idle behavior.
- [x] Record raw results, limits and one next slice; stop at stage boundary.

Current work is the initial local R0 implementation; see `git log -1` for its checkpoint after commit. No remote exists or was pushed.

Temporary test/demo databases were dropped by their scripts and the dedicated PostgreSQL instance was stopped at handoff. The workspace ext4 image remains mounted for caches and retained demo artifacts. Restart instructions are in README.md.

## Actual verification

- Archive CRC + SHA-256 and all 172 baseline checksums: passed.
- Design static checks: 489 passed; reports outside the immutable baseline.
- `bash scripts/test.sh`: Go tests on real PG 18.6 passed; `go vet` and both CLI builds passed.
- `POLIS_TEST_RACE=1 bash scripts/test.sh`: same tests with Go race detector passed.
- `bash scripts/demo.sh`: actual `polis`/`polisd` processes completed the small workflow. Two fake claims, two messages, one fulfilled responsibility, one fixed artifact with independent `passed` verdict; subsequent run did zero work.
- Process-crash test kills real child controller processes after committed request and after committed artifact, without `Close` or a final worker summary, then recovers and finishes.
- Read-only reviewer found two control lifecycle issues; both reproduced, fixed and re-reviewed. See `REVIEW.md` and the preserved red test log.

Failures encountered and resolved: missing bs4, direct Go proxy timeout in WSL, D: mount permissions rejected by initdb, a test missing its foreign-company fixture, missing staging provenance/evidence-file recheck, and the two review findings. No assertion was removed or converted to skip to produce a passing integration run. Without a dedicated DSN, a plain Go test deliberately skips PG cases; the full test script supplies it and uses unique databases.

Evidence: `evidence/development/go-tests.txt`, `go-tests-race.txt`, `demo-pending.json`, `demo-complete.json`, `demo-idle.json`, `design-static-checks.json`, pinned native Codex schemas, and version manifests. The original 232 design scenarios remain `not_run`; `SLICE_TESTS.json` records only bounded variants.

## Explicit limits

This is trusted compiled fake code, not real AI organization intelligence. No runtime inference, paid provider, QQ, external MCP, business account or project access occurred. The independent code reviewer is part of this development session, not a product-worker experiment.

Not executed/implemented: full FT/PP parent scenarios, RLS bypass qualification, generic scheduling/fairness, real money-budget accounting, full owner/login authorization, OS stale-writer isolation, host power-loss or database backup/restore, arbitrary workspace tools, downloads/GC, full mission closing/terminal transitions, UI and long-term value experiments. Events form a durable local outbox but external delivery/acknowledgement is not implemented. A mission intentionally remains active and occupies its slot after its two tasks are accepted.

Most dangerous remaining assumption: the tested control-plane fencing can be combined with real harness tools that may retain OS write access after control connection loss. Before opening real execution, verify one bounded execution combination's stop/isolation receipts; an epoch check or released PG lock is insufficient.

## One next ticket

Implement a pinned Codex 0.151.0 dynamic-tool binding for this employee-operation slice using scripted protocol fixtures, and prepare a concrete bounded two-profile handover/old-tool-stop probe. Obtain separate model-profile/data/budget authorization before running it; do this before expanding the UI. See `REAL_MODEL_PROBE.md`. R1/R2/R3 are not automatically started.
