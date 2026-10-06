# Slice272 — REQ-40 explicit UserDisposition command

## Implemented

- Added an installation-owner authenticated and CSRF-protected command for explicit `accepted` and `changes_requested` dispositions. The command binds the Company, Artifact/Delivery ID, current ready manifest revision, and current disposition revision; appends the next immutable disposition revision; and uses the existing company-scoped idempotent write receipt.
- The Kernel verifies the canonical Manifest digest and exact Artifact/Mission/Task scope, requires the Artifact to remain `ready` with internal verdict `passed`, rechecks its persisted qualification/checkpoint evidence and formal change-request block, and rejects stale expected revisions. The actor is set server-side to `installation-owner`.
- The Workbench GET projection also rejects a `ready` Manifest if its Artifact no longer has internal verdict `passed`, so it does not show an actionable disposition form for that inconsistent state.
- Workbench adds the explicit decision form, strict receipt validation, bounded UTF-8 reason input, and exact refresh of the matching durable delivery query after success. Retries preserve the request ID. Preview, ZIP download, notification, internal validation, and Mission state do not change the UserDisposition.

## Deliberate boundaries

- Only `accepted` and `changes_requested` can be written. `awaiting_feedback` is not offered because finite feedback deadline and expiry behavior are not defined by an owner policy.
- The command accepts decisions only for a `ready` manifest and a still-qualified Artifact. The current publication path produces only `assembling` manifests, so these controls are visible as unavailable until a separate complete-manifest path can produce a qualified `ready` revision.
- This slice records `changes_requested`; it does not create a revision Task, route feedback to a Company backlog for a terminal Mission, invalidate the Artifact, or reopen any Mission. Mission closeout does not depend on this disposition. REQ-40 remains partial.

## Validation and runtime boundary

- `go build ./...` passed.
- `npm run build` passed; Vite emitted the existing large-chunk advisory.
- `git diff --check` passed.
- No tests were added or run. No migration was added or applied; source schema remains 111. No database read/write, Worker, provider, browser, or frozen scenario ran. The recorded local runtime remains Schema108. All 232 frozen scenarios remain `not_run`.
