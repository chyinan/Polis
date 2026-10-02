# Slice 112 verification — Workbench memory revalidation

Date: 2026-10-02

## Changes

- Added authenticated Workbench reads for durable Task memory-impact state and a no-store, digest-bound revalidation preview, plus an idempotent local-owner confirmation command.
- The preview returns the current approved correction and source references, Task plan/generation, and complete workspace content. Both memory and workspace content are digest-checked; Task plans are capped at 64 KiB, workspace previews at 128 KiB, and Tasks with more than 16 active impacts fail closed.
- Confirmation rechecks the preview context and workspace CAS inside the company-locked transaction, then records the immutable revalidation event and resets an eligible working Task to ready. It never starts a Worker.
- Added a Task page review panel that shows the exact plan, workspace, replacement content, target and remaining impacts. It requires a review acknowledgement and operator reason. Fixture mode stays read-only.
- Updated the Workbench read-store minimum schema to 77 while allowing later additive schemas.

## Verification performed

- `gofmt` on changed Go files: passed.
- `git diff --check`: passed.
- `go build ./cmd/...`: passed (exit 0).
- `npm run build`: passed (exit 0); Vite reports the existing large-bundle advisory.

## Not run

- No Go or frontend tests were run.
- Schema 77 was not executed against PostgreSQL; no database migration/runtime was run.
- No model/provider turn, Worker, external endpoint, or production action was used.

## Remaining work

- Add Workbench authoring/review flows for memory record corrections.
- Add reference-aware memory/source CAS retention pins and concurrent garbage-collection protection.
- Add durable deletion/revocation overlays that remain authoritative after restoring older backups.
- Product-provider Tasks still cannot admit a successor WorkerSession after their first attempt; successor policy remains open.
- FT-37/40/41 and the associated acceptance scenarios remain partial or `not_run`; this slice does not claim REQ-15 completion.
