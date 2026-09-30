# R1–R3 implementation validation — slice 11

Date: 2026-09-24

## Scope

Added a Windows native AppContainer runner that creates a unique per-run profile with an empty capability list, stages under the profile's private LocalAppData root, passes only an explicit bounded environment, redirects stdio, starts suspended, assigns the process to a kill-on-close Job Object, then resumes it. It accepts only `networkPolicy=deny_all`; it deliberately rejects the Node/npm profile's `registry_allowlist` policy because no per-registry egress broker/filter exists yet.

Added a preparation helper that composes `MaterializeNodeNPMProjectFiles` with a fresh AppContainer profile. This makes a verified immutable snapshot available inside the profile, but Control and JobRun do not invoke it.

## Verification

- `bash scripts/go.sh test -count=1 ./internal/runner ./internal/environment` — passed.
- `bash scripts/go.sh test -count=1 ./...` — passed serially. Database-backed cases without `POLIS_TEST_DSN` used their explicit skip guards.
- Linux and Windows amd64 builds of `./cmd/...` — passed.
- Windows amd64 `runner.test.exe` and `environment.test.exe` were cross-compiled and executed natively on the local Windows host. Runner tests passed, including AppContainer private workspace read/write, outside temp-file read/write denial, loopback TCP denial, timeout termination proof and existing Job Object descendant cleanup. The environment test passed, including materializing a verified snapshot into a fresh AppContainer profile and checking the stored package/lock hashes.
- `git diff --check` — passed.

## Qualification boundary

This is local native evidence for a deny-all AppContainer profile, not a full executor qualification. It does not implement/qualify registry-only network access, npm installation, dependency-cache isolation, Control preparation/JobRun wiring, service readiness, log artifact readback, clean-VM installation or independent security evaluation. No GitHub/QQ/MCP request, model turn, npm install, production action or deployment was run.
