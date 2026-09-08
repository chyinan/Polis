# Polis

Last verified: 2026-09-08.

Current stage: R0.1, fixed Codex worker integration and same-harness profile handover. Read `docs/implementation/R0_1.md`, `CODEX_START_v0.4.5_2026-09-08.md`, `BRANDING.md`, then `spec/design-v0.4.5/00_START_HERE.md`. Draft 0.4.5 ARCHITECTURE.md is semantic authority; chapter 46 and RELEASE_SCOPE.json determine release scope.

Use Go / PostgreSQL 18 / pgx / sqlc / goose. New concrete names are Polis, `polis`, `polisd`; internal packages stay neutral. Preserve the original design snapshot and archive. Development evidence belongs in `evidence/development/`.

One principal code writer. No real employee model turns, QQ sends, external MCP or business accounts without separate authorization. Dependency installation is authorized. Test databases and files are dedicated temporary resources; never use business databases or global cleanup. No push, publication or automatic expansion into full R1/R2/R3.

R0.1 authorization: GPT-5.6 Sol Medium then Sol High, one harness, concurrency 1, at most 3 turns per profile / 6 total, 10 minutes total real-probe wall time, no automatic retry of unknown outcomes or increased allowance. Only disposable fixture data; no QQ/MCP/GitHub. Complete offline verification before consuming this allowance. Preserve attempt counters and evidence across failure; do not reset the budget by restarting a probe.

That real attempt ended inconclusive after 3 Medium turns and 0 High turns. The primary-profile allowance is exhausted. Preserve `evidence/development/r0.1/real/allowance.json`; no further real probe, new run label or reset is authorized until the owner explicitly renews the allowance. Offline tests and inspection remain allowed.

Commands (WSL Linux, from this workspace): `bash scripts/go.sh test ./...`, `bash scripts/go.sh build ./cmd/...`. Integration entry and actual results are recorded in `docs/implementation/PROGRESS.md`; unexecuted tests must remain unexecuted.

`internal/` holds pure validation and I/O modules; `db/` holds migrations and sqlc queries; `scripts/` holds bounded local test tooling. Continue from `docs/implementation/PROGRESS.md` and `NEXT_SLICE.md` after restart.
