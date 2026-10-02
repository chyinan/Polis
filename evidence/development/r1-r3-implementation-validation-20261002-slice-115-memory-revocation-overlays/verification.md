# Slice 115 — memory revocation overlays

Date: 2026-10-02
Scope: approved R1–R3 handoff, REQ-15, Schema 80.

## Implementation

- Desktop starts the backend with one stable `memory-revocations` path under its data root. Per-generation database and CAS roots remain separate.
- Kernel startup validates the overlay directory and records, loads tombstones before database recovery, mirrors/replays them before schedule reconciliation, and retains file-only tombstones if the matching Company does not exist in the restored generation.
- Revocation files contain only schema, Company/record/operation IDs, source revision and SHA-256, a fixed reason code, and timestamp. New records publish with a no-replace hard link; file contents and, where supported, the containing directory entry are synced before the SQL transaction can commit. Windows follows the existing documented parent-directory-sync-unsupported contract, so this local check does not qualify power-loss durability there. A failed SQL write remains denied in the current Kernel and the file is replayed at the next startup.
- Reads and Handover deny revoked content. New record creation, review, correction, dependency creation, revalidation and Task write/finalization paths check the overlay. Revocation freezes matching dependencies and Task impacts; resumption requires a new dependency. The owner endpoint is explicit-confirmation and binds the request to the latest revision and digest; it refuses any live WorkerSession consuming the record.
- `GET /api/workbench/companies/{company}/memory/{record}/revocation` returns a no-store metadata preview. `POST /api/workbench/companies/{company}/memory/{record}/revoke` writes the tombstone and database mirror. Schema 80 adds the immutable database mirror and `source_revoked` Task-impact cause.

The overlay suppresses use after restoring an older Desktop recovery generation. It does not erase immutable database revisions or CAS bytes, scrub text already copied into Task plans/workspaces/Artifacts, or survive deletion of the stable Desktop data root. It is not a claim of physical or secure erasure.

## Verification

- `go build ./cmd/...` — passed.
- From `db/migrations`, `sha256sum -c ../migration_hashes.sha256` — passed through `00080_memory_revocation_overlays.sql`.
- `git diff --check` — passed.
- `cargo fmt --manifest-path desktop/src-tauri/Cargo.toml -- --check` — passed.
- `cargo check --manifest-path desktop/src-tauri/Cargo.toml --offline` — could not resolve uncached `fs2`; online dependency resolution succeeded.
- Online `cargo check` — not passed in this Termux environment. The toolchain target is `aarch64-linux-android`, so Tauri selected its mobile cfg and Rust reported the desktop-only `tauri::menu`, `tauri::tray`, and single-instance `init` APIs unavailable. The build script also required the platform sidecar and the absent `.tools/pg-windows-runtime` resource; temporary local placeholders were removed. A supported desktop Rust target and the Windows PostgreSQL runtime are not present here.

Tests were not run. Schema 80 was not applied to PostgreSQL, and no Desktop restore, memory revocation, CAS deletion, external provider, or production operation was executed. FT-41 runtime qualification therefore remains `not_run`.
