# Slice 114 — reference-aware CAS collection protocol

Date: 2026-10-02

## Changes

- Schema 79 adds expiring write claims keyed by company and SHA-256 digest. All production `putBlob` paths register a claim before publishing bytes. The Routine workspace path, which already runs inside a company-locked transaction, writes its claim and workspace owner in that same transaction.
- CAS-first writers and the collector share a company-scoped PostgreSQL advisory lock. The collector also holds the Company row lock while it checks references and unlinks files, so existing Kernel writes that alter company data cannot race the check.
- `polis cas-collect --company COMPANY_ID` is dry-run by default. `--apply` is required to remove files. It checks candidate digests against every public base table with a `company_id` column by searching serialized row values, then checks active write claims. This broad scan errs toward retention if a digest appears incidentally in text.
- The command processes keyset pages of at most 128 digests (default 32), refuses directories above 4,096 entries, ignores generated `.stage-*` files, and fails closed on symlinks, unexpected names, and I/O/database errors. Kernel startup requires Schema 79 or newer.

## Verification

- `go build ./cmd/...` — passed.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — passed, including Schemas 78 and 79.
- `git diff --check` — passed.
- Tests and PostgreSQL migration/runtime execution — not run.
- The `--apply` command was not run; no CAS file or user data was deleted.

## Limits and remaining work

The collector is restricted to public company-owned database rows plus durable claims. Any future CAS owner stored outside those rows must be brought into this protocol before collection can be considered qualified. The public-row scan can retain extra blobs if their digest appears incidentally. The frozen recovery, deletion-overlay, and Workbench correction-review requirements remain open, and CAS collection has not been qualified on a live PostgreSQL/CAS generation.
