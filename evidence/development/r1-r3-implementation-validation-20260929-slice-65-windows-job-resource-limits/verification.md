# Slice 65 — Windows AppContainer Job Object CPU and memory bounds

Date: 2026-09-29  
Schema: 48 (no migration)

## Changes

- Windows project AppContainer Job Objects now set a 50% hard CPU rate cap, 2 GiB per-process memory cap, 2 GiB aggregate Job memory cap, and 64-process cap using Windows Job Object controls.
- Resource limits are applied before the suspended process tree is assigned/resumed. If Windows refuses either extended Job limits or the CPU-rate control, Job creation fails and no project process starts.
- Generic provider Worker Job Objects continue using their existing kill-on-close and 64-process limits; the new CPU/memory caps are isolated to AppContainer project Jobs.
- Bumped the Windows executor isolation-policy fingerprint to `windows-appcontainer-node-policy@3`, so prior qualifications no longer match the current policy.
- Added bounded pure validation for CPU percentage, memory and process-count settings, including the exact Windows CPU-rate conversion for the default hard cap.

## Verification

- Red test: `bash scripts/go.sh test ./internal/runner -run TestDefaultWindowsJobResourceLimitsAreFinite -count=1` failed to compile because the resource-limit policy did not exist.
- Green tests: `bash scripts/go.sh test ./internal/runner ./internal/environment -count=1` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed. This cross-build compiles the Windows Job Object calls but does not execute them on Windows.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh test -c -o /tmp/polis-runner.test.exe ./internal/runner` — passed. The native-only test binary includes checks that query the created project's CPU/memory limits and verify generic Provider Worker Jobs remain unchanged; those tests were not executed on Windows.
- `bash scripts/r1-capability-source-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 48 after the policy fingerprint bump.
- `rtk git diff --check` — passed.

## Qualification boundary

No native Windows Job Object was created in this environment, so kernel enforcement remains unqualified. The feature does not impose a hard workspace disk-space quota. Windows resource qualification therefore remains unavailable until disk growth is bounded and the resource policy is verified on a native supported Windows host. Fixed-port browser ingress, native Node/npm execution, WFP behavior and clean-VM install/recovery also remain open.
