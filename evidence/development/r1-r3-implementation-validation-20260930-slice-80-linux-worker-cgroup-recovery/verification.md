# Slice 80 verification — Linux WorkerSession cgroup v2 containment and recovery

Date: 2026-09-30

## Implemented boundary

- Linux real Provider Workers require the configured delegated cgroup v2 manager. A missing manager prevents readiness and start.
- Control creates a stable opaque `polis-worker-<digest>` leaf and binds `linux_worker_cgroup_v2@1` plus the exact leaf ID before process spawn. The existing `process_containment` observation is preserved; the cgroup binding is appended as `process_containment_bound`. No schema migration was added.
- Runner launches atomically with `UseCgroupFD`. A Worker stop requires `cgroup.kill` and a bounded poll until `cgroup.events` reports recursive `populated 0`; only then is the leaf removed and the process stop proof returned.
- Startup cleanup still reclaims Node JobRun cgroups and preserves Worker leaves for database reconciliation. A bound cgroup profile may reconcile an absent leaf idempotently. A legacy `linux_process_group@1` placeholder may be upgraded only if its exact deterministic Worker cgroup leaf exists and cleanup succeeds; an absent leaf remains unresolved.
- Empty unreferenced Worker leaves are removable. Populated unknown Worker leaves fail closed before the Linux control listener opens. The cgroup manager's root lease remains held while the service runs.
- Recovery appends cgroup-bound host evidence and does not replay Worker start, provider turns, Task state or Mission state.
- Runner receives the actual configured cgroup root from the created Worker leaf. The bwrap validator accepts only the exact ordered Worker argument profile produced by `BuildNativeWorkerLaunch`, including a single writable home bind whose source matches `ProcessLaunchSpec.RuntimeHome`; additional or reordered mount arguments are rejected. Direct, nested, configured-root and symlink-alias cgroup control-file binds are covered.

## Verification

- `rtk bash scripts/go.sh test ./... -count=1` — passed across all Go packages.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — passed.
- `rtk bash scripts/r2-linux-worker-recovery-postgres-test.sh` — passed against a dedicated disposable PostgreSQL 18 cluster through Schema 60. It verifies the prelaunch containment upgrade, unresolved legacy process-group sessions, and unchanged Task/Mission state.
- Environment/Runner tests cover deterministic opaque leaf names, strict cgroup.events parsing, stop delegation, bad-leaf cleanup, known-leaf preservation, empty-orphan disposition, and fail-closed populated-orphan handling.
- The symlink-alias bind regression failed against the path-prefix-only validator and passed after the closed Worker bwrap argument profile was enforced.
- Final read-only code review found no remaining Critical, Important or Minor issues in the Linux Worker cgroup containment/recovery slice.
- No migration was added; migration hash manifest is unchanged.

## Qualification boundary

No delegated cgroup v2 host, real Linux Node project, real Provider Worker, model turn, QQ send, external MCP, GitHub account or business database was used. Host qualification must verify that the Worker cannot write `cgroup.procs` or otherwise move itself outside its owned cgroup, along with real cgroup kill/recursive population behavior. Linux host, persistent target-project workspace restoration and multi-day Worker operation remain open.
