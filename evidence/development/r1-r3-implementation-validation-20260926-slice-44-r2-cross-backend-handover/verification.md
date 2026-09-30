# Slice 44 verification: R2 cross-backend Task handover

Date: 2026-09-26

## Result

Schema 39 persists immutable, digest-bound cross-backend handovers and binds a consumed record to exactly one JobRun. The Workbench can create, list, inspect, and select a record for the next target-profile JobRun.

The contract is limited to the fixed Windows/Node and Linux/Node profile pair. Creation requires a paused Mission, a working backend Task, all Mission WorkerSessions stopped, a latest known-terminal source JobRun, no active/unknown Mission job or preparation, no upload or service lease, matching immutable project/package/lockfile sources, and a currently approved, qualified, ready target environment. Consumption rereads the frozen workspace/input-manifest versions and source runtime incarnation, requires a distinct target WorkerSession, rechecks current target gates, and is single-use. If a resumed Mission has no active WorkerSession, Kernel creates a `project_job`-mode session in the same transaction as the target JobRun. Mission resume with an unconsumed handover skips provider readiness/start; the project-job session cannot authorize a model turn. Terminal jobs stop the session; unknown outcomes mark it `reconcile_required` and block further Jobs until cancellation is confirmed.

## Verification performed

- `bash scripts/r2-cross-backend-handover-postgres-test.sh` — `R2_CROSS_BACKEND_HANDOVER=PASSED` on a dedicated temporary PostgreSQL 18 cluster. The script migrated 1–39, exercised the Schema 39 empty rollback/up path and persisted-history downgrade guard, then ran the Kernel cross-backend contract test and Workbench PostgreSQL read-model integration. Temporary database, socket, and files were removed by the script trap.
- The same script runs a Control service integration for Pause → create handover → Resume → target JobRun. Its WorkerAdapter deliberately reports provider readiness unavailable; resume bypasses that path while the handover is pending. The job launches through a fake Linux executor, and the generated project-job session is persisted with the target JobRun and stopped at the terminal event.
- Lifecycle coverage also starts a running ProjectJob and verifies that Pause cancels it before marking the Mission paused. An unconfirmed process stop leaves the Mission active and its session `reconcile_required`; an explicit Stop reconciliation records cancellation and closes that session. A later running ProjectJob is cancelled by `CancelMission`, which stops the job and closes its session before the Mission becomes cancelled.
- StartMission holds the lifecycle write lock through WorkerAdapter.Start, preventing concurrent cancellation from committing before the Worker start boundary returns. An append-only outcome receipt records `started`/`outcome_unknown` and the current runtime incarnation. The PostgreSQL script blocks Worker start while cancellation races, verifies failed start cannot be replayed, verifies same-incarnation successful replay after the Worker finishes, and verifies restart replay returns `reconcile_required` with explicit host-side process-containment guidance without starting another Worker. The error states that the Workbench cannot verify or clear this condition. CancelMission also replays through a service with no WorkerAdapter.
- The Kernel integration test uses source and resumed target WorkerSessions with the same Polis process incarnation, matching the production session-creation path. It verifies denial while Mission is active, while a WorkerSession is active, during a MissionInput upload, from an `outcome_unknown` JobRun, for any same-profile retry while that unknown outcome remains unresolved, and toward an unqualified target; it then creates a Windows-to-Linux handover, denies a cross-profile start without the record, persists the handover and generated target session on the JobRun, and denies second consumption.
- `bash scripts/go.sh test ./... -count=1` — passed all Go packages.
- `bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis ./cmd/polisd` — passed.
- `npm test` — 95 tests passed.
- `npm run typecheck`, `npm run lint`, and `npm run build` — passed. Vite reported a bundle-size advisory for the 627.97 kB main JavaScript chunk.
- After adding the lifecycle regression coverage, `bash scripts/go.sh test ./...`, `bash scripts/go.sh build ./cmd/...`, and `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis ./cmd/polisd` — passed.
- The final `bash scripts/r2-cross-backend-handover-postgres-test.sh` run includes the lifecycle race and replay tests and reports `R2_CROSS_BACKEND_HANDOVER=PASSED`.
- Final follow-up after persisting start outcomes and the host-side reconciliation status: `bash scripts/go.sh test ./...`, Linux `bash scripts/go.sh build ./cmd/...`, Windows amd64 `bash scripts/go.sh build ./cmd/polis ./cmd/polisd`, the dedicated PostgreSQL script, and `rtk git diff --check` — passed.

## Limits

The PostgreSQL test inserts fixture qualification and ready events for Windows and Linux targets so the state gates can be exercised offline. The Control integration creates a persisted `project_job` WorkerSession but does not start a model-backed WorkerSession. Fake executors return fixture output; no project command, package installation, Linux `bwrap`, native Windows AppContainer, model, QQ, MCP, GitHub, or business account was run. Linux/Node executor qualification, live cross-host process execution, and multi-day restart/recovery remain open.
