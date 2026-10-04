# Slice 208 — REQ-29 logical private Task tree and workspace snapshots

## Implementation

- Added forward-only Schema 103 for private Task workspace roots, active writer session/epoch and root revision; bounded relative UTF-8 file metadata; immutable snapshot file references; and multiple snapshot Artifacts alongside the existing one-deliverable-per-Task constraint.
- Added list, search, read, revision-CAS write/delete, snapshot creation, exact-ID snapshot read and snapshot-file read in `internal/kernel/workspace_tree.go`. The content remains in the existing per-Company CAS; paths are logical relative identifiers and do not grant host-shell or host-mount access.
- Added the distinct opt-in fake-only product Worker surface @10. Real-provider authorization stays pinned to @4, and the simulation marker is explicitly unqualified. No simulation or Worker session was started.
- Kept legacy single-file behavior by seeding `formatter.go` (compat Task) into the logical tree and syncing its CAS digest/revision. Existing validation and product delivery deny tree states other than exactly one `formatter.go` file.
- Updated recovery to verify ready snapshot manifests and their file blobs, retained generic CAS reference detection through database rows, and excluded workspace snapshots from published-deliverable and memory-target paths.
- Updated the REQ-29 audit and current continuation/coverage/handoff pointers. The frozen design and scenario catalogs were not changed; CAP-01–06 remain `not_run`.

## Verification

- `gofmt` on changed Go sources — passed.
- `go build ./...` — passed.
- `sha256sum -c db/migration_hashes.sha256` — passed.
- `git diff --check` — passed.
- No tests were run.
- Schema 103 was not applied to a database. No Worker/provider/model activity, fake runtime simulation, host filesystem operation, notification, external account action or frozen scenario ran.

## Limits

REQ-29 remains partial. The implementation is a CAS-backed logical private Task file tree. It does not implement host filesystem mounts, company shared documents, group public libraries, physical symlink/special-file/TOCTOU enforcement, or an owner-decided snapshot obsolete/revocation policy. CAP-01–06 qualification remains open. The fake-only @10 surface is opt-in and unqualified; real-provider authorization remains @4.
