# Slice 18 — Control preparation wiring and lifecycle hardening

Date: 2026-09-25  
Scope: connect the existing immutable project-environment model to an injected Windows/Node preparation executor; harden stop, cleanup, and restart boundaries. No real project input was installed or executed.

## Implemented

- `Control.EnsureProjectEnvironment` now consumes the Kernel's reverified MissionInput CAS snapshot, persists `starting`/`running` before calling the executor, and stores bounded redacted output/evidence digests with the terminal event. The executor is called only for an accepted run after the persisted policy and executor-qualification gates pass. Missing configuration produces `blocked_unqualified`; replay does not launch a second preparation.
- `cmd/polis serve` installs the Windows executor only when `POLIS_WINDOWS_NODE_PREPARATION_ENABLED=1` and administrator-selected Node/npm paths are valid. The executor's only registry-egress command remains `npm ci --ignore-scripts --no-audit --no-fund`. It revokes registry access before retaining the deny-all AppContainer snapshot for the current service lifetime.
- Registry revocation and profile close now stop the local broker before retrying WFP/profile cleanup. If process-tree stop is unconfirmed, pipe readers are aborted rather than waiting indefinitely for EOF; the pending cleanup owner keeps the process handle and its preparation slot while a monitor retries stop, revoke, and profile cleanup after the process exits. The slot is released only after snapshot cleanup succeeds. Preparation admission is bounded to two concurrent revisions; queued work observes request cancellation or shutdown, and cancellation is checked again after snapshot construction before npm starts.
- Preparation state writes after `starting` use a bounded context independent of the request cancellation. Graceful shutdown invalidates retained profiles as `outcome_unknown`; startup reconciliation invalidates prior accepted/starting/running/ready runs that were not restored. PostgreSQL JSONB policy verification uses canonical digest equality instead of byte-for-byte comparison with the database's reserialized JSON.
- `LaunchNodeProjectScript` validates canonical relative JavaScript paths and bounded arguments before using the pinned Node executable in the deny-all snapshot. No Worker `jobs.*` dispatch or JobRun supervisor uses this primitive yet.

## Verification

- On a dedicated temporary PostgreSQL 18 cluster (`polis_r0_envprep_20260925_final`), the following passed: `TestEnvironmentPreparationExecutorRunsOnlyAfterQualificationAndReplaysReadyState`, `TestEnvironmentLifecycleControlCommandsPersistGatedStatus`, `TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently`, and `TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates`.
- The qualified fake-executor test verified the executor receives the digest-verified source files, preparation reaches `ready`, duplicate ensure does not execute twice, graceful close changes the profile state to `outcome_unknown`, and startup reconciliation changes a later un-restored `ready` run to `outcome_unknown`.
- Offline `internal/control`, `internal/environment`, `internal/codex`, and `internal/provider` package tests passed. `go test -race ./internal/runner` passed, including broker-close-before-WFP-retry and AppContainer close-order tests. Control unit tests cover stream-reader abort, process-handle cleanup monitoring, slot retention through pending cleanup, the two-slot preparation bound, and executor shutdown coordination.
- Linux `go build ./cmd/...` passed. Windows amd64 cross-builds passed for `./cmd/...`, `internal/control` tests and `internal/environment` tests. The first downloads through `proxy.golang.org` timed out; the same builds passed using `goproxy.cn`.
- `git diff --check` passed.

`Kernel.Open` acquires the existing database-scoped advisory lock before `cmd/polis serve` reaches startup environment reconciliation; a second serving process sharing the database exits before it can mark another instance's active preparation unknown. That lock is held by the kernel lease until shutdown.

## Not run / still open

- No native Windows test ran in this slice. No WFP filter or Windows loopback entry was changed, no production Node/npm bundle was staged, and no npm install, Node script, project JobRun, service probe or browser run occurred.
- The AppContainer snapshot is retained only in memory during the running service. Shutdown and startup conservatively mark it unavailable; durable restore is not implemented.
- `jobs.start/status/logs/stop`, process-tree JobRun supervision, durable log readback, service readiness probes, independent browser verification, and the Workbench authenticated download through desktop middleware remain open.
- R2 Streamable HTTP MCP, Linux/Node, cross-backend/multi-day recovery and separate R3 content/research acceptance evidence remain open.
- No real model, QQ send, external MCP, GitHub account, registry request, publication, push or deployment was used.
