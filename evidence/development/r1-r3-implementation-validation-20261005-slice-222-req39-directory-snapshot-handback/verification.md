# Slice 222 — REQ-39 bounded directory snapshot handback

Tree-bound Task takeover now preserves the complete frozen tree through human return. Grant validates that the pinned files can be represented through the existing bounded directory MissionInput format before creating a lease. The frozen tree is limited to 250 files and 7 MiB of UTF-8 content; the canonical archive remains within the existing 8 MiB MissionInput limit.

The return command binds the exact base manifest digest, validates each relative path and non-empty UTF-8 body, creates a deterministic directory archive, computes added/modified/deleted paths, and persists the archive as an immutable `directory_snapshot` MissionInput. Its append-only lease event records the archive digest/size, diff summary, request provenance and effort value in the same transaction that releases the Task slot. Reusing the request ID with the same content replays the receipt; changing the content conflicts.

Schema 106 raises the receipt byte bound from 4096 to 8388608. Its down migration refuses to truncate receipts above the old bound. The Workbench reads the frozen manifest-bound tree, supports editing, adding and deleting files, enforces the established file/count/byte limits, and keeps the exact pending request payload for retries. No edit writes into the old Task workspace.

## Verification

- `go build ./...` passed.
- `npm run build` passed (`tsc -b` and Vite production build); Vite reports the large-bundle advisory at 870.09 kB minified JavaScript.
- Migration checksums passed with `(cd db/migrations && sha256sum -c ../migration_hashes.sha256)`.
- `git diff --check` passed.
- No tests were run. No migration was applied, no database or takeover lease was used, no WorkerSession/provider activity occurred, and no frozen scenario ran.

REQ-39 remains partial pending successor input delivery, natural-language impact assessment and frozen scenario qualification. The directory handback uses the existing MissionInput archive format and does not create or start a Worker.
