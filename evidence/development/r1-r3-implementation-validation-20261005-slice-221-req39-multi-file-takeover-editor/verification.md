# Slice 221 — REQ-39 lease-bound multi-file editor

The real Workbench API now validates and consumes the lease-bound manifest and per-file reads from Slice 220. `MissionTakeoverPanel` loads every pinned file with a four-request client bound, verifies each response against its manifest entry and revision, and presents a path selector plus editor for the frozen file set. Editing stays local to the panel and never writes into the old Task workspace. The existing single-file `workspace.txt` return remains available; a multi-file lease cannot be returned until the directory-snapshot handback stage is complete.

## Verification

- `npm run build` passed (`tsc -b` and Vite production build).
- `git diff --check` passed.
- Vite reports the existing large-bundle advisory (864.70 kB minified JavaScript).
- No tests were run. No database operation, takeover lease, WorkerSession, provider activity, or frozen scenario was run.

REQ-39 remains partial. This stage reads and edits an existing tree but does not add/remove files or submit a complete multi-file snapshot.
