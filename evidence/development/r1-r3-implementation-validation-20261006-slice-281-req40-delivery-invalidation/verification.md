# Slice281 verification: REQ-40 DeliveryManifest invalidation/withdrawal

Date: 2026-10-06

## Scope

- Added owner/CSRF Workbench route `POST /companies/{company}/artifacts/{artifact}/delivery/invalidate`.
- Added append-only Kernel revision command for `invalidated` and `withdrawn` states.
- Bound stale/current revision, Company/Mission/Task/Artifact identity, canonical manifest digest and idempotent request handling.
- Preserved Artifact bytes and reset the new non-deliverable revision's feedback state to `not_requested` while persisting the owner actor/reason.

## Verification

- `bash scripts/go.sh fmt ./internal/control ./internal/kernel ./internal/workbench` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestValidateProductDeliveryManifestInvalidationCommand` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestInvalidateProductDeliveryManifestRejectsMalformedCommandBeforeDatabaseAccess` — passed.
- `bash scripts/go.sh test ./internal/control -run TestInvalidateDurableDeliveryManifestRejectsMalformedRequestBeforeRuntime` — passed.
- `npm test -- --run src/domain/workbench-validation.test.ts -t "durable delivery invalidation receipt"` — passed.
- `npm run build` — passed.
- `git diff --check` — passed.

The full `workbench-validation` file still has one pre-existing mission-change validation failure unrelated to this slice. No PostgreSQL migration/runtime, Worker, provider, browser, external account or frozen scenario ran.
