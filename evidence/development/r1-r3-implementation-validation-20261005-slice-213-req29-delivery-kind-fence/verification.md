# Slice 213 — REQ-29 Artifact delivery kind fence

## Change

The Workbench Artifact delivery-manifest query now requires `artifact_kind='deliverable'`. This matches the existing Artifact overview/detail filters and ensures a logical workspace snapshot cannot be represented as a downloadable deliverable even if a task-validation qualification row is attached later.

## Verification

- `go build ./...` — passed.
- `git diff --check` — passed.

No tests were added or run. No database operation, migration, Worker/provider activity or frozen scenario ran.
