# Slice284 verification: REQ-37 BorrowerLease product surface and recovery hardening

Date: 2026-10-06

## Scope

- Added fake-only product surface `polis-product-tool-surface@17` with `jobs_borrow`, `jobs_touch` and `jobs_release`; existing @4 and @15 surfaces remain unchanged.
- Bound acquire, touch and release to the current exact Task and WorkerSession. Owner session state and the exact service generation must remain active/ready and unexpired.
- Closed PostgreSQL row streams before appending revoke events, made recovery/session-stop/generation revocation durable, returned replayed revoke counts from the stored receipt, capped idle grace by endpoint expiry, and added the database same-Task check.
- No service process, native host, Provider turn, browser, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/kernel -run TestValidateServiceBorrowerLeaseScopeRequiresSameMissionDistinctTaskAndExactSessions -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestServiceBorrowerLeaseExpiryUsesEndpointAndPolicyBounds -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestServiceBorrowerLeaseExpiresAtTTLOrIdleBoundary -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestEmployeeToolsBorrowerLeaseRequiresTheIsolatedWritableSurface -count=1` — passed.
- `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeBorrowerLeaseSurfaceExtendsReadOnlyJobsWithBoundedLeaseCalls -count=1` — passed.
- `bash scripts/go.sh test ./internal/provider -run TestProductBorrowerLeaseSurfaceIsFakeOnlyAndBounded -count=1` — passed.
- `bash scripts/go.sh test ./internal/control -run TestBorrowerLeaseFakeSurfacePassesAdapterReadinessWithoutRealProvider -count=1` — passed.
- `bash scripts/go.sh test ./db -run TestEmbeddedMigrationManifestMatchesRepository -count=1` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.

Database migration runtime, native service/restart qualification, Playwright/Chromium, real Provider and frozen-scenario execution remain unrun.
