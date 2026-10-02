# Slice 113 — memory CAS retention pins

Date: 2026-10-02

## Changes

- Schema 78 adds the immutable `memory_cas_retention_pins` ledger and backfills memory revision sources, all correction evidence sources (including rejected requests), MissionInput/artifact dependency targets, and exact Task workspace digests recorded by revalidation.
- Kernel memory writes insert each applicable pin atomically with the owning revision, proposal, dependency, or revalidation event. Repeated exact ownership is idempotent; a conflicting pin for the same reference is treated as an integrity error.
- Kernel startup now requires Schema 78 or newer. The previous hard-coded Schema 73 gate made Kernel-backed features unavailable once the database advanced to Schema 77.
- The repository has no production CAS collector. Pin insertion uses the same Company-row lock as `TXWrite`; a future collector must also serialize on that lock and account for CAS-first in-flight writers before checking all owners and deleting bytes. Collector integration and runtime race proof remain open.

## Verification

- `go build ./cmd/...` — passed.
- `npm run build` — passed; Vite reports the existing 754.66 kB minified JS chunk exceeds its 500 kB advisory threshold.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations` — passed, including Schema 78.
- `git diff --check` — passed.
- Tests and PostgreSQL migration/runtime execution — not run.

## Scope and remaining work

No model, provider, account, host, or production action was used. This adds durable references; it does not claim that CAS collection is safe yet. Next: implement a reference-aware CAS collector integrated with all CAS owners and CAS-first writes; then implement deletion/revocation overlays applied after restoring older backups. Memory correction authoring/review in Workbench, provider successor admission, and frozen-scenario qualification also remain open.
