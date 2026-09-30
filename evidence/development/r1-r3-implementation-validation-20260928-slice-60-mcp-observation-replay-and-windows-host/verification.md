# Slice 60 verification — MCP observation replay safety and native Windows probes

Date: 2026-09-28  
Schema: 45  
Scope: code-review fixes for Slice 59; local native Windows runner probes; no real MCP or project execution.

## Verification results

- `rtk run "bash scripts/go.sh test ./... -count=1"` — PASS.
- Linux and Windows amd64 `cmd/...` cross-builds — PASS.
- Frontend Vitest — 104 tests passed across 12 files.
- Frontend TypeScript project build and ESLint with zero warnings — PASS.
- Vite production build — PASS in Slice 59; frontend sources were unchanged in Slice 60.
- `rtk run "bash scripts/r1-capability-source-postgres-test.sh"` — PASS, disposable PostgreSQL 18 through Schema 45. Covers observation reservation replay denial, stale package approval denial, server-lock conflict before CAS writes, workspace-close revocation, startup stale-workspace revocation and session-close retry.
- `rtk run "bash scripts/r2-cross-backend-handover-postgres-test.sh"` — PASS, disposable PostgreSQL 18 through Schema 45; `R2_CROSS_BACKEND_HANDOVER=PASSED`.
- `rtk run "bash scripts/r2-recovery-backup-postgres-test.sh"` — PASS, disposable PostgreSQL 18 through Schema 45; `R2_RECOVERY_BACKUP_RESTORE=PASSED`.
- `rtk run "bash scripts/r3-domain-evidence-postgres-test.sh"` — PASS, disposable PostgreSQL 18 through Schema 45.
- `TestControlledMCPWorkerHoldsPackageFenceThroughToolCompletion` — PASS; the package lock remains held over reservation, authorization, process start, tool call and result persistence.
- `TestControlledMCPWorkerDoesNotReserveIntentWhenPackageImportOwnsFence` — PASS; lock contention is rejected before a dispatch intent is recorded.
- `TestServiceClosesSharedSandboxOnlyAfterObserverCleanup` — PASS; a failed observer cleanup retains both handles, and the shared sandbox closes only after the retry succeeds.
- `TestCloseCommandServiceWithRetryRetriesOneTransientFailure` and `TestCloseCommandServiceWithRetryPreservesBothFailures` — PASS.
- `TestStdioMCPObservationReservationPreventsRequestReplay` now exercises startup reconciliation: leftover `reserved` observations become `outcome_unknown`, and the request remains non-replayable.

## Native Windows host probes

A Windows amd64 runner test binary was cross-compiled into a dedicated temporary path and run natively. These tests used disposable fixtures only:

- `TestWindowsAppContainerStopsTimedOutProcessTree` — PASS.
- `TestWindowsAppContainerDeniesOutsideFilesAndLoopback` — PASS; outside read/write and loopback were denied; protected toolchain bytes remained unchanged.
- `TestWindowsNodeSnapshotMaterializesInsideFreshAppContainerProfile` — PASS; the verified project snapshot stayed under a newly created real AppContainer profile.
- `TestWindowsJobObjectStopsDescendantsWhenRootExits` — PASS.
- `TestWindowsReconcileNamedJobTerminatesItsActiveTree` — PASS.
- `TestWindowsWorkerProcessUsesSessionNamedJobForRestartReconciliation` — PASS.
- `TestWFPStructuresMatchWindowsSDK64BitLayout` and `TestRegistryWFP*` injected-API tests — PASS. No WFP filter was installed.
- The registry-only npm install test was skipped; no dependency install or Node project script ran.

The temporary test executable was removed after the probes. These checks do not qualify WFP arbitration/elevation, registry egress, actual Node/npm project preparation, packaged Desktop install/update, or clean-VM recovery.

## MCP call and shutdown fencing

The Worker obtains the same per-server PostgreSQL advisory lock used by package import before reserving a tool-call intent and holds it until the result is persisted or the call is classified as unresolved. Kernel dispatch admission and runtime authorization also take the transaction-scoped form of this lock when called without the Worker-held lease. Startup reconciliation converts any leftover `reserved` intent to `outcome_unknown` without replay. Shutdown uses three phases for the shared-sandbox adapter: quiesce WorkerSessions, close the runtime observer and clean its package tree, then close the sandbox. `serveWorkbench` retries a failed `Service.Close` once; unresolved first and second errors are preserved together.

## Scope boundaries

`POLIS_MCP_RUNTIME_OBSERVATION_ENABLED` remained disabled. No MCP executable/endpoint, tool call, model, QQ, GitHub, or production database was used. R3 content-operations and research profiles remain `not_run`; the database suite verifies workflow contracts only, not real domain outcomes.
