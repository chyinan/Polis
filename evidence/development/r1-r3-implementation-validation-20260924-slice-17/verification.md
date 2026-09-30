# R1–R3 implementation validation — Slice 17

Date: 2026-09-24

## Scope

- Keep the raw `AppContainerSandbox` private behind the environment snapshot API. While registry egress is active, only the fixed plan-derived `npm ci --ignore-scripts --no-audit --no-fund --registry=...` path can launch. Generic project launches require successful permit revocation and then run under deny-all.
- Hold Windows file handles for `.polis-toolchain`, `node.exe` and `npm-cli.js` with read sharing only. Keep those leases until every tracked child process exits/stops, then release them before deleting the AppContainer profile.
- Rename the public lower-level constructor to `NewWindowsNodeNPMInstallAppContainerSandbox`; runner launch validation also enforces the fixed npm-ci argv shape and the selected proxy host allowlist. Retry cleanup closes only outstanding file leases.
- Preserve the exact npm-ci plan, separate toolchain digest, and active permit/default-block lifecycle from Slice 16.
- The ACL-only attempt was rejected after native evidence showed an AppContainer child could still write despite the added ACE. That code was removed. The implemented boundary is the tested Windows file-sharing lease.
- No WFP/loopback system changes, production Node/npm bundle, npm package install, provider, QQ or external MCP activity is included.

## Verification

- Focused native Windows `TestWindowsAppContainerDeniesOutsideFilesAndLoopback` — PASS. The child process ran from a file held open with read-only sharing; its write to a separately locked toolchain fixture failed, and the fixture bytes remained unchanged.
- Runner validation rejects arbitrary npm scripts, enabled lifecycle scripts, non-toolchain executables/CLI paths, noncanonical registry URLs, and registries outside the proxy allowlist.
- `TestCloseReadOnlyLeasesRetriesOnlyOutstandingHandles` verifies a transient close failure does not cause successfully closed handles to fail every subsequent retry.
- Linux `go test -race ./internal/runner -count=1` — PASS (`1.764s`).
- Linux `go test ./internal/environment -count=1` — PASS (`0.812s`), including separate staging, exact digest checks, project-tree source rejection, tampered-byte rejection and symlink checks.
- Linux `go test ./internal/kernel -count=1` — PASS (`49.748s`), including verified execution snapshot readback from immutable CAS.
- Windows amd64 runner and environment test binaries cross-compiled successfully.
- Native Windows full runner test package — PASS; native Windows full environment test package — PASS.
- `go build ./cmd/...` — PASS; `git diff --check` — PASS.
- No WFP filter was installed and no Windows loopback SID entry was changed; WFP logic is tested with an injected API only.

## Remaining boundary

The registry-only preparation and immutable source snapshot are still not connected to Control's EnvironmentPreparation or JobRun lifecycle. The code has not run an actual Node/npm bundle or dependency install. Real WFP rights, effective arbitration and clean-VM recovery are unqualified.
