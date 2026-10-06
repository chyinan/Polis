# Slice290 verification: REQ-14 generic ActionIntent default-deny foundation

Date: 2026-10-06

## Scope

- Added Schema120 append-only generic ActionIntent records/events with Company/Mission/Task/WorkerSession FKs, canonical ResourceKey grammar, target/input digests and idempotency identity.
- The Kernel records `denied/generic_dispatch_permit_unavailable`; no generic DispatchPermit is issued and no external/shared action can execute through this slice.
- Reusing an idempotency key under a different request ID returns a deterministic conflict; same request ID remains receipt-replayable.
- No external action, Provider, browser, database runtime or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/kernel -run TestNormalizeGenericActionIntentUsesCanonicalBoundedResourceIdentity -count=1` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.
- `git diff --check` — passed.

Generic resource-specific policy/permit issuance and external qualification remain unimplemented and default-denied.
