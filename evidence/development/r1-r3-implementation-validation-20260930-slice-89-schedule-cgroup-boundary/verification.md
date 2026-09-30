# Slice 89 verification — schedule cancellation and cgroup descendant bound

Date: 2026-09-30  
Schema: 62; no migration

## Changes

- `TXCancelMission` recalculates affected employee schedules in the same transaction after cancelling the Mission and its Tasks.
- Schedule reconciliation treats an open peer obligation as actionable only while its source Mission is active. A stale `paused` schedule is normalized once no Mission remains paused.
- Linux per-job cgroup initialization writes and reads back `cgroup.max.descendants=0`. Missing controls or failed/mismatched writes prevent process start.
- Existing Linux Worker launch checks require `--unshare-cgroup`, hide `/sys` behind tmpfs, reject host cgroup binds, and bind host/root/boot identity into recovery evidence.

## Verification

RED was observed before the schedule fix: `TestCancellingPausedMissionClearsScheduleBarrierForNextMission` reported `state=paused` after cancellation. The core stale-pause test also failed before correction.

Passed after the schedule fix:

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — Schema 60→62 integration passed, including cancellation and next-Mission wake.
- `rtk bash scripts/go.sh test ./internal/core -run TestUnpausedSchedule -count=1`
- `rtk bash scripts/go.sh test ./internal/environment -run TestLinuxNode -count=1`
- `rtk bash scripts/go.sh test ./internal/runner -run TestLinuxWorker -count=1`
- `rtk bash scripts/go.sh test ./internal/environment -run TestLinuxResourceCgroupInitializationWritesAndVerifiesAllControls -count=1`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk bash -c 'GOOS=windows GOARCH=amd64 scripts/go.sh build ./cmd/...'`
- `rtk bash scripts/go.sh test ./... -count=1`
- `rtk git diff --check`

Independent read-only review confirmed the Mission pause-barrier fix and, after an initial Important finding, confirmed that the cgroup descendant bound is applied and read back by the real group initialization path. It found no remaining Critical, Important or Minor findings in the reviewed changes.

## Limits

No real Worker, delegated cgroup host, model, QQ account, external MCP/GitHub endpoint or production service was used. The cgroup namespace/mount profile and recovery identity are code-level controls; they do not substitute for qualification on a delegated Linux cgroup v2 host.
