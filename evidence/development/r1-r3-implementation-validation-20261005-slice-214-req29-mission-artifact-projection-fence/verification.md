# Slice 214 — REQ-29 Mission Artifact projection fence

## Change

The shared sqlc `ListArtifacts` query now includes only `artifact_kind='deliverable'`. This is used by `snapshotMissionTX`, which supplies both recovery snapshots and peer-state Mission projections. Those projections do not carry an Artifact-kind field, so excluding workspace snapshots prevents a draft tree snapshot from being presented as a delivery Artifact. `db/queries.sql` and the generated `internal/dbgen/queries.sql.go` are synchronized.

## Verification

- `go build ./...` — passed.
- `git diff --check` — passed.

No tests were run. No database operation or migration ran. No Worker/provider activity or frozen scenario ran.
