# Slice 271 — REQ-40 durable delivery foundation

## Scope

Added the Schema 111 migration source for append-only `delivery_manifest_revisions` and `delivery_user_dispositions`. A newly published product Artifact records delivery revision 1 in the same transaction as its Artifact and validation qualification. The revision binds one Company, Mission, Task and exact Artifact; it starts as `assembling`. Its eight sections identify the Artifact inventory and verification evidence that are present and explicitly mark source/input provenance, environment/build evidence, run instructions, limitations, license/source attribution and feedback as unavailable or not requested.

The separate initial user disposition is revision 1, `not_requested`. The old Artifact ZIP format and download path remain unchanged. Workbench adds a read-only lifecycle endpoint and a Task view for the durable record; the view keeps internal Artifact verification, DeliveryManifest state and UserDisposition separate. Preview, download and notification do not write disposition state.

The Workbench reader uses a read-only repeatable-read transaction, checks the canonical manifest SHA-256 and relational Company/Mission/Task/Artifact/revision bindings, rechecks the current Artifact digest, byte size and state, and preserves the existing formal-change delivery block. Unknown manifest fields and malformed disposition values fail closed.

## Validation

- `go build ./...` passed.
- `npm run build` passed (`tsc -b` and Vite production build). Vite emitted its existing large-chunk advisory.
- All 111 migration hashes passed `sha256sum -c db/migration_hashes.sha256` from `db/migrations/`.
- `git diff --check` passed.
- No tests were added or run. The migration was not applied; no database, Worker, provider, browser, or frozen scenario was used.

## Limits

REQ-40 remains partial. This slice records the first incomplete revision only. It does not assemble source/input, environment/build, instructions, a multi-file package, limitations or license/source evidence; add or invalidate later revisions; withdraw a delivery; expose disposition-changing commands; decide a feedback deadline or expiry action; or make Mission closeout depend on user acceptance. Existing Artifacts from before Schema 111 are not backfilled. The recorded local runtime remains Schema 108, while migration source is Schema 111. All frozen scenario execution states remain `not_run`.
