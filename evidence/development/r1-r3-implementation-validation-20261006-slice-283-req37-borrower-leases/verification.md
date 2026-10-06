# Slice283 verification: REQ-37 durable BorrowerLease lifecycle

Date: 2026-10-06

## Scope

- Added Schema116 immutable BorrowerLease identities and append-only lifecycle events.
- Added same-Mission, distinct Task, exact owner/borrower WorkerSession checks.
- Added 15-minute maximum TTL, 2-minute idle grace, endpoint-expiry bound, release/touch and service-generation/endpoint revoke paths.
- No service process, arbitrary command, native host, Provider, browser, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh fmt ./internal/kernel ./internal/environment` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestValidateServiceBorrowerLeaseScopeRequiresSameMissionDistinctTaskAndExactSessions` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestServiceBorrowerLeaseExpiryUsesEndpointAndPolicyBounds` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestServiceBorrowerLeaseExpiresAtTTLOrIdleBoundary` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.
- Migration hash remains covered by the Schema116 manifest.

Database runtime/integration tests were not run because the recorded runtime database is not authorized for migration. Native service/process and restart qualification remain open.
