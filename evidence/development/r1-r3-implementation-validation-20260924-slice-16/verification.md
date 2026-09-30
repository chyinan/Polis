# R1–R3 implementation validation — Slice 16

Date: 2026-09-24

## Scope

- Block all new registry-only AppContainer launches as soon as revocation is requested, including permit-delete errors, active-process refusal and loopback cleanup failure. Keep revocation retryable.
- Bind the configured Node executable and npm CLI bytes to a canonical `windows-node-toolchain@1` digest; stage them from separate administrator-selected source paths under exclusive `.polis-toolchain`, and verify the staged digest on every snapshot launch.
- Require a valid toolchain digest when registering/loading a project environment revision and when constructing a registry-only install snapshot.
- Load a company-scoped project execution snapshot from the exact MissionInput CAS revision after rechecking policy, package/lock metadata and source digest.
- No Control/JobRun execution, real Node/npm package install, WFP system filter, loopback configuration change, or external network request is included.

## Verification

- Linux `go test -race ./internal/runner -count=1` — PASS (`1.754s`); includes revoke-failure launch denial regression.
- Linux `go test ./internal/environment -count=1` — PASS (`0.795s`); includes digest binding, separate staging, project-tree toolchain rejection, tampered-byte rejection, and symlink containment checks.
- Linux `go test ./internal/kernel -count=1` — PASS (`49.193s`); includes the verified CAS-backed `ProjectEnvironmentExecutionSnapshot` readback.
- Windows amd64 runner and environment test binaries cross-compiled successfully.
- Native Windows full runner package — PASS; native Windows full environment package — PASS. Registry-only OS setup is not invoked by these tests.
- Linux command build (`go build ./cmd/...`) — PASS; `git diff --check` — PASS.
- No WFP filter was installed, no system loopback SID list was changed, no actual toolchain was staged, and no npm command or external network request ran.

## Remaining boundary

The secure helper paths are not wired into the Control EnvironmentPreparation and JobRun lifecycle. Real WFP rights/arbitration and clean-VM recovery remain unqualified. An operator still has to select and qualify the trusted toolchain source/digest; no real Node/npm bundle or install was executed.
