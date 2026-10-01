# Slice 102 verification — bounded automatic product Worker admission

Date: 2026-10-01

## Change

Schema remains 72. The dispatcher is disabled by default and requires both `POLIS_AUTO_WORKER_DISPATCH_ENABLED=1` and `POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED=1`. Startup accepts the combination only for the exact zero-egress Fake @7 tool surface over the fake transport. Every cycle examines at most one eligible `compat/emp-backend` Task from an active Company and Mission, and calls the existing Worker adapter. The first selection uses the oldest eligible schedule; after each attempt an in-process Company-ID cursor advances so one repeated failure cannot monopolize the process. Candidate selection requires `wake_pending`, a Task workspace and validation binding, no prior WorkerSession for the Task, and no non-stopped session for the Employee. The dispatch loop shares the Mission lifecycle lock; WorkerSession reservation still locks the EmployeeSchedule and denies `paused` / `waiting_quota`.

Routine `compute` Tasks, real-provider dispatch, global slot accounting, cross-instance fairness, Task retries after WorkerSession creation, and automatic quota recovery are outside this slice. There is no authoritative provider quota readiness source; `waiting_quota` remains blocked. No real model/provider turn or external action ran.

## Verification

- `gofmt` completed for the changed Go files.
- `go test ./internal/control ./cmd/polis` passed on Termux Go 1.27.1 (`android/arm64`). This includes the Fake @7 startup/readiness gate tests.
- `go test ./internal/kernel ./internal/control ./cmd/polis` compiled the packages, but the Kernel package run terminated with `SIGSYS` in the existing `TestUnixParentDirectorySyncUnsupportedSentinelRemainsFatal` test when Termux seccomp denied `fchmodat2`. The Control and command packages passed. The new PostgreSQL-backed candidate ordering/barrier test was not run because this host could not complete the Kernel suite and the repository's PostgreSQL harness targets the bundled Linux amd64 PostgreSQL under `.tools/pg`, which is absent here.
- `go build ./cmd/...` passed for the host target `android/arm64`.
- `GOOS=windows GOARCH=amd64 go build ./cmd/...` passed.
- `git diff --check` passed.
- `rtk` is unavailable in this Termux environment, so commands were run directly.

The local server was not reconfigured to enable automatic dispatch; both flags remain opt-in and unset by default.
