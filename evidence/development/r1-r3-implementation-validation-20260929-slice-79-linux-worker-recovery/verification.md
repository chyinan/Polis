# Slice 79 verification — Linux WorkerSession process-group recovery

Date: 2026-09-29

## Implemented boundary

- Linux `Process.Stop` only returns a stop proof after the process leader exits and a bounded `/proc` scan confirms the associated `linux_process_group@1` group is empty. Remaining members or procfs read/parse/size errors keep the stop unresolved and ownership retained.
- Startup reconciliation reads persisted process-containment metadata and `reconcile_required` WorkerSessions. Only a positive persisted PID with the exact Linux process-group profile is eligible. It appends a host-reconciliation observation and stop receipt after the group is empty.
- The recovery path does not restart a Worker, replay a provider action, change Task state, or reactivate a Mission. It adds no migration.
- The profile confirms only process-group emptiness. It does not qualify escaped/detached descendant behavior, the delegated Linux cgroup host, persistent project-workspace restoration, or an actual multi-day Linux Worker deployment.

## Verification

- RED: runner tests first demonstrated that stop could return success while a child remained in its process group; the Linux stop path now leaves that case unresolved.
- `rtk bash -c '.tools/go/bin/gofmt -w internal/runner/process_linux.go && bash scripts/go.sh test ./internal/runner ./internal/provider -run "TestReconcileLinux|TestStoppedCodexProcessRemovesCredentialSnapshot|TestFakeRuntimeExercisesProductProtocolWithoutProvider" -count=1'` — passed.
- `rtk bash scripts/r2-linux-worker-recovery-postgres-test.sh` — passed using a disposable PostgreSQL 18 cluster through Schema 60. It observes the fake WorkerSession process as unresolved while live, reconciles after its process group stops, and verifies Task remains `working` and Mission remains `active`.
- `rtk bash scripts/go.sh test ./... -count=1` — passed across all Go packages.
- Linux and Windows amd64 command builds, `scripts/test-migration-hash-manifest.ps1`, and `rtk git diff --check` — passed.
- No actual Linux Node project, model turn, provider, QQ, MCP, GitHub account, cgroup host, or business database was used.

## Remaining Slice 79 blocker

The kernel process-group probe closes the scan-versus-fork race, but independent review confirmed it cannot detect a descendant that changes session/process group with `setsid`. The current WorkerSession stopped transition is not a complete process-tree proof. The next Linux recovery change must bind the WorkerSession to an owned cgroup v2 and prove that exact cgroup is empty or was killed during startup reconciliation. Do not count Linux WorkerSession restart recovery as complete before that proof is implemented and tested.
