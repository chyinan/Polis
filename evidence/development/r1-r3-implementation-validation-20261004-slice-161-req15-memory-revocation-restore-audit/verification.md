# Slice 161 — REQ-15 / FT-41 old-backup memory revocation audit

Date: 2026-10-04

## Scope

Source audit whether restoring a previously captured Desktop generation can make a revoked memory record readable again. No runtime behavior or frozen scenario was executed.

## Findings

- `desktop/src-tauri/src/supervisor.rs:1473-1477` passes `POLIS_MEMORY_REVOCATION_ROOT=<desktop-data-root>/memory-revocations` to each backend process.
- `desktop/src-tauri/src/generation_store.rs:110-120` resolves non-legacy CAS under `generations/<generation-id>/cas`, separate from the stable tombstone directory.
- `internal/kernel/kernel.go:165-179` initializes and loads revocation tombstones before database recovery, then reapplies them after recovery and before startup reconciliation or returning the Kernel.
- `internal/kernel/memory_revocation_overlay.go:124-180,303-319,447-489` validates and loads external immutable overlay files, exposes the in-memory overlay check, and reconciles each company's entries into the recovered database. Overlay records contain identifiers, hashes, revision, reason code and timestamp, not memory text.
- `internal/kernel/memory.go:1340-1390` persists an external tombstone before the SQL transaction commits, so a failed database commit retains the in-process barrier and the next `Kernel.Open` can replay it. Memory read paths consult the overlay through `memoryRecordRevokedTX` (for example `internal/kernel/memory.go:230,331,494,1421`).

For a restore performed by the same Desktop data root, the source indicates that a prior database generation cannot remove its separately stored tombstone; startup replays the tombstone before exposing the recovered Kernel. This is logical access denial only. The audit does not show physical erasure of old database or CAS bytes.

## Verification and limits

- Source locations were inspected with `sed`/`grep`.
- `git diff --check` is the only validation run for this documentation-only slice.
- No tests, database restore, Desktop/Tauri boot, browser flow, WorkerSession action or frozen FT-41 scenario was run.
- FT-41 remains `not_run`; REQ-15 remains partial pending runtime restore qualification and its other acceptance evidence.
- Worker activity remains unperformed; no database-confirmed active WorkerSession was available or used.
