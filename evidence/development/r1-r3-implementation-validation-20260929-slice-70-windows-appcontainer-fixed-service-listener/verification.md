# Slice 70 verification — Windows AppContainer fixed service listener

Date: 2026-09-29  
Schema: 50 (no migration)  
Status: implementation and local offline checks passed; Windows host network qualification remains `not_run`.

## Implemented

- Service JobRuns derive the exact listener address and port from the immutable service health probe. The pure policy accepts only `127.0.0.1` or `::1`, never a wildcard, alternate loopback, mapped address or ephemeral port.
- Windows grants only `internetClientServer` for that launch. Dynamic, AppContainer-SID-scoped WFP rules permit the exact TCP loopback bind/listen/accept path and block other bind/listener/accept paths plus outbound connections.
- The WFP dynamic-session lease remains owned until process-tree cleanup is confirmed. A canceled caller still waits up to the bounded cleanup window before the lease can close; an uncertain process stop or failed WFP close keeps the lease and blocks another launch.
- Windows isolation fingerprint advanced to `windows-appcontainer-node-policy@5`; qualification requires `windows_service_listener_network_boundary` in addition to the existing checks.
- Both service-listener and registry-only filter builders now set and pin the required `FWPM_FILTER0.DisplayData.Name`, as documented by [Microsoft's FWPM_FILTER0 reference](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_filter0).

## Verification

- Added plan tests. The unbounded-target test failed for `127.0.0.2` before the exact-address restriction and passed after the fix.
- Added process lifecycle tests. The canceled-Wait case failed before the wrapper continued to `WaitForTreeCleanup`; it then passed and confirmed the lease closed only after a stop confirmation.
- `rtk bash scripts/go.sh test ./internal/runner ./internal/environment ./internal/control -count=1` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed; database-dependent tests followed their existing DSN skip behavior.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — passed.
- Windows amd64 `internal/runner` test binary cross-compile — passed.
- Native Windows mock-only filter-builder/dynamic-lease and process-lease tests — passed. These use synthetic SID pointers and injected WFP APIs; they do not call `FwpmFilterAdd0` or change host network policy.
- Independent static code review found no outstanding Critical, Important or Minor findings for the Slice70 implementation.
- The service-probe timeout assertion was made stable across its two valid timeout returns: structured `probe_timeout` alone or combined with `context.DeadlineExceeded`. Both remain unhealthy and cannot establish readiness.

## Not run / not qualified

- No WFP filter installation, `CheckNetIsolation`, loopback exemption or other host network change.
- No Node/npm project install/run, AppContainer service JobRun, or packaged Desktop/browser integration.
- No real model, QQ send, external MCP endpoint, GitHub account or production operation.
- Effective WFP arbitration, AppContainer inbound loopback access, bounded-volume enforcement, and clean-VM qualification remain `not_run`; the qualification gate keeps the Windows profile unavailable until independent evidence passes.
