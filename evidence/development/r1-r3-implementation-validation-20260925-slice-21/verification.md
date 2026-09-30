# Slice 21 — JobRun authorization and stop-race hardening

Date: 2026-09-25  
Scope: address static review findings for tokenless desktop auth, Task-kind admission, stop during process launch, retrying a verified stop after `outcome_unknown`, and bounded JobRun listing.

## Implemented

- All Task JobRun list/start/stop/log endpoints require the desktop session token even when the local listener is tokenless and loopback-only.
- Kernel JobRun admission independently restricts execution to a `working` `compat` Task owned by `emp-backend` with its active matching WorkerSession; the UI condition is no longer the only enforcement.
- Control registers a cancellable JobRun handle before process launch. Stop can cancel a launch that has not returned its handle yet. Unconfirmed stop leaves the active handle available for retry; a later verified process-tree stop transitions `outcome_unknown` to `cancelled` without replaying the command.
- The task page offers retry-stop for unresolved jobs and shows `exited` as successful only with exit code 0. The JobRun query is capped at the same 300 records accepted by the frontend validator.

## Verification

- Dedicated Schema 26 PostgreSQL `TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution` passed. It now covers start replay, normal stop, stop during blocked launch, unconfirmed stop followed by verified retry, denial of a non-compat Task, CAS log readback, and restart conversion to `outcome_unknown`.
- `scripts/go.sh test ./internal/control`, `./internal/desktop`, `./internal/environment`, and `./internal/workbench` passed; dedicated auth tests and the full control race test passed.
- Full offline `scripts/go.sh test ./... -count=1` passed. `scripts/go.sh test -race ./internal/control -count=1` passed.
- Windows amd64 Control test-binary cross-compile, Windows and Linux `scripts/go.sh build ./cmd/...` passed.
- Frontend `npm test -- --run` passed (60 tests); typecheck, lint and production build passed. Vite emitted the >500 kB chunk advisory.
- Static final review rechecked tokenless auth, compat-only Task admission, pre-launch cancellation, retained-handle stop retry, exit-code status tone and the 300-row cap; Critical/Important/Minor findings: zero.
- No native Windows AppContainer JobRun, npm install, project script, WFP filter or loopback change ran.
