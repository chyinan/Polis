# Slice 20 — controlled batch JobRun lifecycle

Date: 2026-09-25  
Scope: connect the persisted JobRun contract to the prepared Windows/Node AppContainer; expose company-scoped start, stop and log readback through the Workbench API and task page.

## Implemented

- Start admits only a `compat` Task with an active matching WorkerSession, a current immutable Windows/Node source snapshot, and an in-memory prepared deny-all environment. The current executable surface is a bounded relative `.js`/`.cjs`/`.mjs` batch script with bounded argv. Service and controlled-input Jobs stay unavailable until their separate readiness/interaction contracts are qualified.
- The Control path persists `accepted`, `starting`, `running` and terminal JobRun events. Request replay reads the existing state and never launches the same JobRun twice. Stop requires the retained process handle and a process-tree termination proof; unconfirmed outcomes stay `outcome_unknown` and are not replayed.
- Bounded stdout/stderr are redacted, stored as a company-scoped CAS log manifest, and referenced by the immutable JobRun event digest. The Workbench reads log bytes through a company/job-scoped API. The task page offers start only when Task/session/environment gates pass and displays stop/log actions.
- Startup reconciliation turns prior-process `accepted`/`starting`/`running` JobRuns into `outcome_unknown` before the listener opens.

## Verification

- On the dedicated temporary Schema 26 PostgreSQL database `polis_r0_envprep_20260925_verify`, `TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution` passed. A fake process verified one launch under replay, terminal state persistence, stop/cancel, CAS log-manifest digest/readback, and restart conversion of a seeded running JobRun to `outcome_unknown`.
- `scripts/go.sh test ./internal/control -count=1`, `scripts/go.sh test ./internal/workbench -count=1`, and `scripts/go.sh test -race ./internal/control -count=1` passed.
- The full offline `scripts/go.sh test ./... -count=1` suite passed with PostgreSQL DSN unset; dedicated PostgreSQL integration was run separately.
- Windows amd64 Control test binary and command packages cross-compiled.
- Frontend `npm test -- --run` passed (60 tests), `npm run typecheck`, `npm run lint`, and `npm run build` passed. Vite emitted its existing >500 kB bundle advisory.
- No native Windows JobRun launched, no project script or npm command ran, no WFP/loopback state changed, and no model, external registry, QQ, MCP, GitHub account, or production service was used.

## Still open

- Default runtime configuration remains disabled and this environment profile is not qualified on a clean Windows VM. Prepared AppContainers exist in memory only.
- Service Jobs/health probes, independent browser verification, restart restoration, clean-VM install/uninstall, signing, startup, updates and rollback remain open. R2 Linux/Node, Streamable HTTP MCP, cross-backend/multi-day support and R3 content/research domain evidence remain unqualified.
