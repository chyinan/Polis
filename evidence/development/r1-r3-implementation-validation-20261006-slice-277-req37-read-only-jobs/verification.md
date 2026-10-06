# Slice277 verification: REQ-37 fake-only read-only JobRun surface

Date: 2026-10-06

## Scope

- Added fake-only product tool surface `polis-product-tool-surface@15`.
- Added read-only `jobs_status` and `jobs_logs` schemas with exact `job_id` input.
- Added Kernel Task/WorkerSession scope checks and log manifest integrity verification.
- Added fake Runtime, authorization, and real-adapter readiness wiring.
- Did not start a Worker, project process, service endpoint, BorrowerLease, real provider, browser, external account, or frozen scenario.

## Verification commands

- `bash scripts/go.sh fmt ./internal/control ./internal/provider ./internal/kernel ./internal/codex` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed (`BUILD_OK`).
- `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeReadOnlyJobsSurfaceAddsOnlyBoundedStatusAndLogs` — passed.
- `bash scripts/go.sh test ./internal/provider -run TestProductReadOnlyJobsSurfaceIsFakeOnlyAndTaskBound` — passed.
- `bash scripts/go.sh test ./internal/control -run TestReadOnlyJobsFakeSurfacePassesAdapterReadinessWithoutRealProvider` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestValidateProductTaskJobReadScopeRequiresCurrentTaskAndWorkerSession` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestReadBlobBoundedDoesNotCreateMissingCompanyDirectory` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestReadVerifiedJobRunLogArtifactAllowsThePersistedLogBound` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestReadVerifiedJobRunLogArtifactRejectsMissingOrTamperedManifest` — passed.

The broader package sweep was also attempted. It exposed pre-existing environment/fixture failures in provider, kernel, and control tests; those failures are not asserted as Slice277 regressions and are recorded in the continuation progress file.

## Boundary

This is implementation evidence only. It does not qualify a real provider, native executor, service consumer lease, restart path, PostgreSQL runtime migration, or REQ-37 frozen scenario.
