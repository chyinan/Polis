# R1–R3 implementation validation — Slice 14

Date: 2026-09-24

## Scope

- Require a random per-lease Basic credential on the loopback CONNECT broker and inject the credentialed URL only into the isolated child environment.
- Expose the raw registry endpoint to launch-spec callers without exposing the credential; clear the endpoint and return the outer sandbox policy to `deny_all` after close.
- Serialize `Launch`, `Close` and endpoint reads. Preserve Windows loopback SID entries under a cross-process Polis mutex and roll back a committed SID update when returned-buffer cleanup fails.
- No Control/JobRun integration, real registry install, WFP system filter, loopback system setting change, model call, QQ send, or external provider call is included in this slice.

## Verification

- Red: the lease-auth test rejected the previous unauthenticated broker behavior; missing credentials were accepted and the credentialed URL was not yet valid.
- Red: `TestRegistryProxyEndpointIsEmptyAfterClose` returned `127.0.0.1:43123` after close, as expected before the fix.
- Green: `rtk bash scripts/go.sh test -race ./internal/runner -count=1` — PASS (`polis/internal/runner`, 1.570s).
- Green: Windows amd64 cross-build of `./internal/runner` test binary — PASS.
- Green: native Windows full runner package — PASS. This includes deny-all AppContainer tests and a named-mutex synchronization test; it does not invoke the registry-only constructor.
- Read-only security review: no Critical, Important, or Minor findings after two follow-ups. Reviewers did not run tests or call external/system network-policy APIs.
- Windows WFP test coverage uses an injected API to inspect filter plans and cleanup; actual WFP arbitration and rights remain unqualified. No WFP filters were installed and no loopback SID list was changed during verification.

## Remaining boundary

The registry-only constructor is not connected to EnvironmentPreparation or JobRun. Windows registry egress has not been independently qualified on a clean VM. The named mutex coordinates Polis instances that share that mutex; arbitrary programs modifying the system loopback SID list remain outside that coordination boundary.
