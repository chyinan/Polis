# Slice 28 verification — Linux cgroup v2 CPU, memory and PID limits

Date: 2026-09-25

## Changes covered

- `linux-node-toolchain@4` binds the delegated cgroup root path and fixed resource profile. Per install and project batch JobRun, the Linux executor creates a private cgroup v2 child and assigns the bwrap process at clone time with `UseCgroupFD`/`CLONE_INTO_CGROUP`.
- Defaults are `cpu.max=200000 100000` (2 CPU cores), `memory.max=2147483648` (2 GiB), `memory.swap.max=0`, `memory.oom.group=1`, and `pids.max=256`.
- Startup requires a pre-existing empty cgroup v2 subtree with the CPU, memory, and PID controllers available and enabled. It verifies parent ceilings of at most 4 CPU cores, 4 GiB memory, no swap, 512 PIDs, group-scoped OOM, and 32 cgroup children, bounding aggregate JobRun concurrency/resources even across different workspaces. Startup refuses existing child cgroups without deleting them. It does not mount cgroupfs or change host controller settings.
- Cleanup fences processes remaining in the owned cgroup. Preparation and JobRun outcomes remain unknown and their workspace/process ownership stays retained if the group cannot be emptied and removed.
- This change does not enforce an aggregate disk-space limit. That and actual delegated-host qualification remain required before enabling the Linux profile for project work.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/environment ./internal/control ./internal/runner -run TestLinux -count=1` | PASS. Resource defaults/validation, regular-filesystem rejection, existing-child preservation, launch cgroup attributes, fake executor placement and cleanup-retention paths passed. |
| `bash scripts/go.sh test ./internal/control -run TestLinuxNodeFailedProcessCreationRetainsCgroupForStopRetry -count=1` | PASS. Cgroup cleanup failure during spawn rollback retains the group and can be reconciled only after cleanup succeeds. |
| `bash scripts/go.sh test ./internal/control -run TestProjectJobCgroupCleanupFailureOverridesCancellation -count=1` | PASS. An unconfirmed cgroup cleanup overrides a concurrent timeout/cancellation and remains `outcome_unknown`. |
| `bash scripts/go.sh test ./internal/control -run TestLinuxNodeExecutorCloseRetainsWorkspaceWhenPendingCgroupIsUnconfirmed -count=1` | PASS. Close retains the workspace and cgroup ownership when resource cleanup cannot be confirmed. |
| `bash scripts/go.sh test ./internal/control -run TestLinuxNodeJobRefusesWorkspaceWithUnconfirmedPendingCgroup -count=1` | PASS. A workspace with an unconfirmed spawn-rollback cgroup refuses another JobRun before process launch. |
| `bash scripts/go.sh test ./internal/control -run TestLinuxNodeJob -count=1` | PASS. Includes a concurrent launch regression proving the workspace reservation allows exactly one process launch. |
| `bash scripts/go.sh test -race ./internal/control -run TestLinuxNodeJobAtomicallyReservesWorkspaceDuringLaunch -count=25` | PASS. Repeated the concurrent workspace admission case under race detection. |
| `bash scripts/go.sh test -race ./internal/environment ./internal/control ./internal/runner -run TestLinux -count=1` | PASS. |
| `bash scripts/go.sh test ./... -count=1` | PASS. Database integration cases without a dedicated DSN were skipped. |
| `bash scripts/go.sh build ./cmd/...` | PASS for Linux. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| `git diff --check` | PASS after final documentation updates. |

No cgroup filesystem was created or modified during verification. No bubblewrap process, npm command, project script, real model, QQ, external MCP/GitHub, or business account was used. The Linux executor remains unqualified. A hard aggregate workspace disk quota, effective cgroup/namespace qualification, service probes, restart recovery, and clean-VM evidence remain open.
