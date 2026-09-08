# R0 startup review — 2026-09-08

Scope is the kickoff's single fake vertical slice, with Polis naming. The source ZIP SHA-256 matches `e4d8593699abe47242663da1b1cbcf4f241511458cd968991d7c4b4e98fe9fd6`; archive CRC, 172 original file checksums and 489 static checks passed. Original specification files were not rewritten. Static checks are not runtime or architecture qualification.

Read the product charter, release/reference matrices, main chapters 01/02/03/18/19/20/25/29/30/31/36/42/43/46, open gates, readiness and FT/PP catalogs. No semantic conflict blocks this bounded slice. Draft schemas and employee-ops registry remain examples; implemented constraints live in `db/` and typed Go methods.

## Measured host and dependencies

Windows 11 AMD64; WSL2 Ubuntu 22.04 with Linux 6.6.87.2-microsoft-standard-WSL2, cgroup v2 (`0::/`). Node 24.15.0, Python 3.14.4, Codex CLI 0.151.0. Neither Go nor PG was on the initial PATH. Docker client 29.4.1 was present, daemon unavailable; Docker was not used or qualified.

Workspace-local Go 1.26.8 Linux amd64, PostgreSQL 18.6 (18.6-1.pgdg22.04+2), sqlc 1.31.1, pgx 5.11.0 and goose 3.28.0 are pinned. Python document dependencies are in `.tools/doccheck`; none are Go runtime dependencies. Go downloads use goproxy.cn because direct proxy.golang.org timed out inside WSL; Go checksum verification remains enabled. The goose module's own test dependency includes SQLite in the download cache; Polis runtime does not import/use SQLite.

WSL's default D: mount failed PostgreSQL's 0700 data-directory check. A 4 GiB sparse ext4 image at `.runtime/linux.ext4`, loop-mounted at `.runtime/linux`, holds only this project's temporary PG data, socket, caches and tests. No existing database was used. PG listens on a private Unix socket only, with fsync and synchronous_commit enabled. This demonstrates process-restart durability on this host, not host power-loss qualification or containment of hostile workers.

The user explicitly authorized autonomous installation of missing dependencies. Go/PG/sqlc are local extractions; gcc/libc development packages in WSL support the Go race detector. No global PG/systemd service was installed.

## Native protocol boundary

Installed CLI help was checked and `codex app-server generate-json-schema --out D:/Programs/Polis/evidence/development/probes/codex-schema` generated the installed native protocol without a model turn. No employee adapter was fabricated from remembered fields.

The [official app-server documentation](https://learn.chatgpt.com/docs/app-server) explicitly distinguishes sandboxed `command/exec` from `thread/shellCommand` and experimental `process/spawn`, both outside the Codex sandbox. These methods are not opened by Polis R0. Generated native schemas are evidence, separate from internal employee operations. Real tool binding, stop/isolation and cross-model behavior remain unverified.

Dependency sources: [Go releases](https://go.dev/dl/), [PostgreSQL Ubuntu packages](https://www.postgresql.org/download/linux/ubuntu/), and module-version provenance in `evidence/development/`. Public dependency retrieval is not business-account access.

## Applied capability limits

The only worker is compiled, in-process scripted code. Trusted local OS management binds opaque company/employee handles; no untrusted process receives DB credentials, and no HTTP worker/admin endpoint is exposed. This is not an implementation of full owner-login or restricted worker authorization. Controller startup refuses superuser runtime DB credentials and incompatible PG/schema versions.

Company guard locking deliberately serializes this small slice. SQL uses composite scope foreign keys and generated scoped reads. No claim of RLS qualification, multi-company capacity, general scheduler fairness, money-budget enforcement or OS stale-writer isolation is made. Fake claim events are exact local execution counts; real inference cost is not estimated.

Artifact files have fixed hashes, bounded size and a durable staging record. There is no download or GC endpoint. Downloads would immediately require R1 variants of WF-27–30; MCP would immediately require CAP-22/23 and applicable unknown-outcome assertions even over stdio. See `SLICE_TESTS.json`; full parent scenarios remain not_run.
