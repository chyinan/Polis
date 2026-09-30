# Slice 57 verification: Linux/Node cgroup restart recovery

Date: 2026-09-28. Database schema: 44. No migration was added.

## Implemented

- The Linux cgroup manager accepts only direct children named `polis-node-` plus 24 lowercase hex characters. It validates all child names and links before changing any group.
- For each recognized group left by a previous `polis` process, startup writes `1` to `cgroup.kill`, waits for `cgroup.procs` to become empty, then removes the group. Cleanup errors, unknown child names and symlinks fail startup.
- `Kernel.HasUnrestoredLinuxNodeHostWork` detects active or `outcome_unknown` JobRuns and environment preparations bound to `linux-node-npm@1`.
- Linux `polis` requires the delegated cgroup root when unresolved Linux Node work exists. If the root is configured, startup reaps prior groups before opening the Workbench; Control then records active prior work as `outcome_unknown`. It never resets the Task or replays the command.

## Verification

- `rtk bash scripts/go.sh test ./internal/environment -run TestLinuxNodeCgroupRestartCleanup -count=1` — PASS. Temporary-directory fixtures cover recognized names, unknown directories and linked children. They do not emulate a kernel cgroup filesystem.
- `rtk bash scripts/go.sh test ./cmd/polis ./internal/environment -run TestLinuxNode -count=1` — PASS.
- `rtk bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS at Schema 44. It verifies Linux profile-work detection for running/terminal JobRuns and preparations alongside the existing one-use cross-backend handover tests.
- `rtk bash scripts/go.sh test ./... -count=1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — PASS.

## Qualification limits

- No delegated cgroup v2 host, `cgroup.kill`, bwrap namespace, Node/npm install, project script, real Worker/model, QQ, MCP, GitHub account or production data was used.
- Linux rootfs, cgroup parent quotas, separate ext-family/XFS workspace volume, reboot cleanup and clean-VM recovery still need host qualification. R3 profiles remain `not_run` pending real domain evidence.
