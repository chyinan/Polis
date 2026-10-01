# Slice 108 verification — REQ-15 memory record/dependency foundation

Date: 2026-10-01

## Implemented

- Schema 74 adds append-only, company-scoped memory record, revision, review-state, dependency, and dependency-invalidation event tables.
- Kernel APIs create a first proposed memory revision, perform an independent fixed Planning/Review decision, pin verified revisions to exact target revisions, and read revisions under scope/sensitivity/state checks.
- MissionInput and Artifact sources are validated against current digest and usable state. `observed_at` is derived from the immutable source timestamp when the caller omits it.
- REQ-15 traceability now records FT-37 and FT-40 as partial; correction/replacement workflows, invalidation propagation, dirty/freeze/revalidation, retention pins, backup deletion overlays, and Handover integration remain open.

## Verification performed

- `gofmt -w internal/kernel/memory.go` — passed.
- `go build ./cmd/...` — passed (exit 0).
- `git diff --check` — passed.
- Migration hash recorded in `db/migration_hashes.sha256`: `fc26f2fc5a34e73ca85ca09dc6d88a84295e5792f61ba2857caace4188175790`.

## Not performed

- No tests were added or run.
- No PostgreSQL server was started and Schema 74 was not executed against a database.
- No model/provider, external account, endpoint, or production action was used.

The migration and database query behavior therefore still require PostgreSQL runtime verification in a later authorized local development stage.
