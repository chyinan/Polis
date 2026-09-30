# Slice 81 verification — environment and Task workspace recovery after restart

Date: 2026-09-30

## Contract

- A prepared Windows/Node AppContainer is process-local state. Startup reconciliation marks any prior ready preparation `outcome_unknown`; it never treats a stale ready row as a live process handle.
- Replaying that preparation's original RequestID returns the same unknown run and does not invoke the executor again.
- An explicit new Ensure RequestID may create a fresh preparation only while the immutable source binding, approved policy, and executor qualification still pass. The executor reloads the exact company CAS files and reconstructs the package/lockfile plan.
- The authoritative Worker Task workspace remains a company-scoped PostgreSQL digest/revision pointing into CAS. A fresh WorkerSession after Kernel restart re-reads that same workspace through its Handover, including the exact bytes.
- This path rebuilds from the approved source. It does not resume an interrupted JobRun or restore writes from an unknown process outcome.

## Verification

- `rtk bash scripts/r1-service-endpoint-postgres-test.sh` passed against a dedicated disposable PostgreSQL 18 database through Schema 60. It now includes `TestEnvironmentPreparationExecutorRunsOnlyAfterQualificationAndReplaysReadyState`, which verifies same-RequestID no-replay after restart reconciliation and new-RequestID re-materialization from the same source revision/lockfile.
- The same disposable PostgreSQL run includes `TestTaskWorkspaceCASRehydratesForNewSessionAfterKernelRestart`, which writes a nonempty CAS workspace snapshot, closes and reopens Kernel on the same database/blob root, starts a fresh WorkerSession, and verifies matching digest, revision and content from Handover.
- The test uses a fake executor and immutable fixture CAS files; no Node/npm, model, external registry, provider, QQ, MCP, or business account ran.
- No schema migration was added.

## Remaining boundary

An unknown project JobRun is never resumed. Task workspace state is durable through its PostgreSQL/CAS generation; the product does not recover filesystem writes from an unknown project JobRun. Native Windows/Node clean-VM qualification and R2 multi-day host evidence remain open.
