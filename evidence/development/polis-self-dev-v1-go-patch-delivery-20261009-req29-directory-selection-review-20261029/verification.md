# Slice 335 — REQ-29 browser-selected directory review

Date: 2026-10-29
Result: `BROWSER_DIRECTORY_REVIEW_REQUIRED_BEFORE_UPLOAD`.

## Implemented

- Directory selection now creates a local pending preview instead of immediately uploading. The preview lists the selected relative paths and byte sizes; it does not read file contents.
- A pure bounded analyzer checks the 250-file and 7 MiB aggregate limits, per-file validity, relative path safety, Windows reserved names, a consistent selected root, case-insensitive path collisions, and file/parent collisions.
- Common credential/key paths receive a heuristic warning based only on names. The UI explicitly says the hint is incomplete and that selected bytes may later enter Worker context.
- An explicit acknowledgement is required before upload. The selected `File` objects and request ID remain available if the command outcome is uncertain, so retry uses the same batch and identity.
- Directory-size/file-count constants now live with the pure domain analyzer and are re-exported from `workbench-api.ts` for existing callers.
- No server API reads `companies.workspace_root`; this is not a native root binding or host qualification.

## Verification

- `previewMissionDirectorySelection` unit tests: PASS (11 tests), covering valid selection, likely-sensitive names, invalid/reserved paths, root mismatch, duplicate/parent collisions, and count/byte bounds.
- Frontend typecheck: PASS — `frontend-typecheck.log`.
- ESLint: PASS — `frontend-lint.log`.
- Full Vitest suite: PASS — 20 files, 182 tests (`frontend-tests.log`).
- Vite production build: PASS — `frontend-build.log` (existing chunk-size advisory only).
- No Go source, migration, database, Worker, Provider, or external operation was run for this frontend-only increment. No browser E2E was run.

## Boundary

The preview covers only names, relative paths, and sizes exposed by the user's browser directory picker. It is not a content scanner, does not guarantee secret detection, and does not create or imply access to a Company host root. Native root access remains blocked pending durable owner authorization and target-host no-follow/TOCTOU/mount-boundary qualification.
