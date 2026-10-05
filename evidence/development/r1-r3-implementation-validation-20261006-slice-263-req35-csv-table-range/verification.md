# Slice 263 verification — REQ-35 bounded CSV table summary and row ranges

Date: 2026-10-06
Source base: `83592318bc49b6f717d68054da7b213a1fbc63af` (Slice 263 source commit)

## Scope

The local CSV path now preserves the exact input bytes and digest while validating UTF-8 and parsing bounded comma-delimited records. The Worker context builder can include a source-revision and manifest-bound table summary containing the header, row/column counts, inferred types, and a capped preview. Summary bytes count against the context limit.

An authenticated backend row-range read checks the active database-bound WorkerSession, current Task manifest, exact source revision, and source digest. It bounds each response by rows and bytes, treats row zero as the header, and writes a metadata-only audit event without cell contents. The provider tool is available only on the isolated fake `polis-product-tool-surface@13`; that profile remains unqualified. The real provider surface remains pinned to `@4` and has no row-range tool. Historical raw-text CSV receipt validation is preserved.

This implements a local CSV foundation, not full REQ-35 qualification. PDF page rasterization and typed image receipts remain open, as do the full R2 format matrix, approved fixtures, provider/runtime qualification, and an authorized delivery to a WorkerSession confirmed active by a current database read.

## Verification

- `go build ./...` passed after integrating the Slice 263 source.
- Migration source hash validation passed for all 110 migrations; Schema 109 and 110 remain unapplied to the local Schema 108 runtime/database.
- `git diff --check` passed.
- No tests were run. No database was accessed or changed; no Worker was started or used; no provider or external endpoint was called; no frozen scenario was executed.
- The traceability ledger retains all 232 scenario rows as `not_run`; REQ-35 remains open and its nine mapped rows remain `partial`.
