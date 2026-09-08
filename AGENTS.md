# Polis

Last verified: 2026-09-08.

Current stage: R0, one FakeWorker vertical slice. Read `CODEX_START_v0.4.5_2026-09-08.md`, `BRANDING.md`, then `spec/design-v0.4.5/00_START_HERE.md`. Draft 0.4.5 ARCHITECTURE.md is semantic authority; chapter 46 and RELEASE_SCOPE.json determine release scope.

Use Go / PostgreSQL 18 / pgx / sqlc / goose. New concrete names are Polis, `polis`, `polisd`; internal packages stay neutral. Preserve the original design snapshot and archive. Development evidence belongs in `evidence/development/`.

One principal code writer. No real employee model turns, QQ sends, external MCP or business accounts without separate authorization. Dependency installation is authorized. Test databases and files are dedicated temporary resources; never use business databases or global cleanup. No push, publication or automatic expansion into full R1/R2/R3.

Commands (WSL Linux, from this workspace): `bash scripts/go.sh test ./...`, `bash scripts/go.sh build ./cmd/...`. Integration entry and actual results are recorded in `docs/implementation/PROGRESS.md`; unexecuted tests must remain unexecuted.

`internal/` holds pure validation and I/O modules; `db/` holds migrations and sqlc queries; `scripts/` holds bounded local test tooling. Continue from `docs/implementation/PROGRESS.md` and `NEXT_SLICE.md` after restart.
