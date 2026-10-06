# REQ-40 Delivery continuation

## Verified current state

- Cloud `origin/main` is Slice 272 at commit `91b1520`; the next pointer in `docs/implementation/NEXT_SLICE.md` is to produce a complete `ready` Manifest from immutable source/build/instruction/limitation/license evidence and route `changes_requested` by Mission lifecycle.
- Existing Schema 111 stores append-only `delivery_manifest_revisions` and `delivery_user_dispositions`; product publication creates only `assembling` revision 1.
- Existing owner disposition command only accepts an exact `ready` Manifest and does not route changes.
- Existing formal Mission change requests accept only active/paused Missions; terminal Missions require a separate Company backlog record.

## Implemented direction

- `internal/kernel/product_delivery_manifest_core.go` contains pure validation/canonicalization for five evidence sections and the ready Manifest. It preserves `feedback=not_requested` and computes the SHA-256 over canonical JSON.
- `internal/kernel/product_delivery_manifest_completion.go` is the database shell. It locks the current assembling revision, verifies the Artifact and qualification, binds source-input/validation-contract references, appends a ready revision, and creates its initial not-requested disposition.
- `internal/kernel/product_delivery_disposition.go` routes only `changes_requested` atomically: active/paused creates a formal change request; terminal Mission states append `delivery_feedback_backlog_events`; accepted dispositions have no route side effect; closing/draft/unknown states fail closed.
- `internal/control` and `internal/workbench/http.go` expose the installation-owner/CSRF-protected completion command. The existing frontend already displays ready/disposition state; no automatic acceptance or external side effect is added.

## Verification notes

- TDD RED/GREEN passed for pure Manifest construction, completion-command validation, malformed completion rejection, and lifecycle routing.
- Schema 113 adds database-level guards so backlog inserts require a terminal Mission and an installation-owner `changes_requested` disposition with matching reason; canonical JSON disables Go HTML escaping to match frontend `JSON.stringify`.
- Full baseline before this work had pre-existing failures in `internal/control` (9), `internal/kernel` (1), and `internal/provider` (1); these are recorded in the task progress file and are outside the REQ-40 changes.
- PostgreSQL-backed completion/routing tests require the dedicated disposable DSN and have not yet been run.
