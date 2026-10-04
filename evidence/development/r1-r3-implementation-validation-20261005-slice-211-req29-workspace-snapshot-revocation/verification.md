# Slice 211 — REQ-29 draft workspace snapshot revocation

## Change

Schema 104 adds an immutable, source-owner WorkerSession/epoch-bound receipt for revoking one ready `draft_not_accepted` workspace snapshot. Receipt creation and the Artifact state transition to `revoked` are atomic. Manifest and file reads require a ready snapshot; file reads hold the Artifact lock while validating and returning CAS content so revocation cannot race delivery. The CAS content and any earlier read result are retained; this slice does not add reclamation or claim to recall content already read.

The separately opt-in fake-only @11 product surface exposes the revocation operation and leaves @10 unchanged. Real-provider authorization remains pinned to @4. Workbench deliverable views exclude workspace snapshot Artifacts, and startup recovery skips revoked snapshots. REQ-29 remains partial: host mounts, company/group shared roots, retention/GC policy and CAP-01–06 qualification are still open.

## Verification

- `go build ./...` — passed.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` — all 104 migration entries passed.
- `git diff --check` — passed.

No tests were run. Migration 104 was not applied. No WorkerSession was created or used, no provider activity occurred, and no frozen scenario was executed or reclassified.
