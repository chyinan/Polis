# Polis

Last verified: 2026-09-08.

Current stage: R0.2, fixed Codex worker integration and same-harness Luna handover probe. Read `docs/implementation/R0_2_REPORT.md`, `docs/implementation/R0_1.md`, `CODEX_START_v0.4.5_2026-09-08.md`, `BRANDING.md`, then `spec/design-v0.4.5/00_START_HERE.md`. Draft 0.4.5 ARCHITECTURE.md is semantic authority; chapter 46 and RELEASE_SCOPE.json determine release scope.

Use Go / PostgreSQL 18 / pgx / sqlc / goose. New concrete names are Polis, `polis`, `polisd`; internal packages stay neutral. Preserve the original design snapshot and archive. Development evidence belongs in `evidence/development/`.

One principal code writer. No real employee model turns, QQ sends, external MCP or business accounts without separate authorization. Dependency installation is authorized. Test databases and files are dedicated temporary resources; never use business databases or global cleanup. No push, publication or automatic expansion into full R1/R2/R3.

R0.1 authorization is historical and retained. R0.2 authorization: GPT-5.6 Luna Medium + Medium + optional High, one harness, concurrency 1, exactly at most 2 Medium and 1 High turn, 10 minutes total, no automatic retry of unknown outcomes or increased allowance. Only disposable fixture data; no QQ/MCP/GitHub. Complete offline verification before consuming this allowance. Preserve attempt counters and evidence across failure; do not reset the budget by restarting a probe.

The R0.1 attempt ended inconclusive after 3 Medium turns and 0 High turns; preserve `evidence/development/r0.1/real/allowance.json`. R0.2 consumed 2 Medium turns and 0 High turns; its 10-minute window expired before the optional High could be reserved. Preserve `evidence/development/r0.2/luna-1/allowance.json` and do not reset it. A future High-only retry requires a new explicit authorization. Offline tests and inspection remain allowed.

Commands (WSL Linux, from this workspace): `bash scripts/go.sh test ./...`, `bash scripts/go.sh build ./cmd/...`. Integration entry and actual results are recorded in `docs/implementation/PROGRESS.md`; unexecuted tests must remain unexecuted.

`internal/` holds pure validation and I/O modules; `db/` holds migrations and sqlc queries; `scripts/` holds bounded local test tooling. Continue from `docs/implementation/PROGRESS.md` and `NEXT_SLICE.md` after restart.
