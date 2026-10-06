# Slice282 verification: REQ-36 DependencyChange proposal/approval

Date: 2026-10-06

## Scope

- Added Schema115 append-only DependencyChange proposal and decision-event tables.
- Added fixed-envelope validation: exact semver versions, bounded package count, explicit registry hosts and rationale.
- Added Kernel proposal and owner decision methods with Company/Mission/base-environment scope and canonical SHA-256 proposal bytes.
- Added Control service methods for proposal and decision; approval has no npm, lockfile, host or environment-revision side effect.

## Verification

- `bash scripts/go.sh fmt ./internal/control ./internal/kernel ./internal/environment` — passed.
- `bash scripts/go.sh test ./internal/environment -run TestValidateDependencyChangeProposalKeepsTheApprovedEnvelope` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestDependencyChangeCommandsRejectMalformedInputBeforeDatabaseAccess` — passed.
- `bash scripts/go.sh test ./internal/control -run TestDependencyChangeServiceRejectsMalformedRequestBeforeRuntime` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.
- `git diff --check` — passed.

No PostgreSQL migration/runtime, npm, Worker, native executor, provider, browser, external account or frozen scenario ran. The approved event remains an intent gate; actual lockfile generation still requires a qualified executor.
