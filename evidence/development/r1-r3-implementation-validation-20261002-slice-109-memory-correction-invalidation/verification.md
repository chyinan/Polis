# Slice 109 verification — REQ-15 correction and dependency invalidation

Date: 2026-10-02

## Implemented

- Schema 75 stores immutable correction requests and one independent approval/rejection event per request.
- A correction request pins the currently verified base revision, corrected content digest, and exact source revision/digest.
- Only a different fixed Planning/Review employee can decide. The proposer and original memory author cannot decide; `emp-review` is denied for restricted records.
- Approval rechecks source usability and digest, creates a verified successor revision, marks the base superseded, and emits one invalidation event per explicit base-revision dependency in the same transaction.
- Ordinary dependencies become `needs_revalidation`; high/critical dependencies become `frozen`. Rejection leaves memory and dependency state unchanged.

## Verification performed

- `gofmt -w internal/kernel/memory.go` — passed.
- `go build ./cmd/...` — passed (exit 0).
- `git diff --check` — passed.
- Migration hashes: Schema 74 `fc26f2fc5a34e73ca85ca09dc6d88a84295e5792f61ba2857caace4188175790`; Schema 75 `007c8fdca6d72f1967a4a2990f0f5f8f9bc11930b4d2a8ed9534fe8d2bd2fd6a`.

## Not performed

- No tests were added or run.
- No PostgreSQL server was started and Schema 74/75 were not executed against a database.
- No model/provider, external account, endpoint, or production action was used.

This stage does not yet mark active Tasks dirty or enforce the frozen state at Worker/tool/acceptance gates. Handover revalidation, retention pins, and restored-backup deletion propagation remain open.
