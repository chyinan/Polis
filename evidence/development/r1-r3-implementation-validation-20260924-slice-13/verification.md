# R1–R3 implementation slice 13 verification

Date: 2026-09-24  
Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`  
Schema: 26 (no migration in this slice)

## Changes

- The Task input delivery boundary now calls a pure verifier after recomputing the canonical context from the frozen manifest and company CAS. Tests reject modified prompt text, file references, input text, payload digest, and manifest digest.
- Workbench labels the summary as distinct input sources that contributed text, while retaining per-file included/excluded details.
- The registry CONNECT broker uses a second bounded queue for overload responses. Valid, header-only CONNECT requests to an approved host in that queue receive 503 after bounded parsing. Queue exhaustion, policy-invalid requests, buffered extra bytes, parser failures, and timeouts close without a status line.
- Broker status remains standalone. No AppContainer/WFP egress filter or Control/JobRun wiring was added; direct socket egress remains possible outside the existing deny-all AppContainer profile.

## Verification

- `scripts/go.sh test -p 1 -count=1 ./...` — passed.
- `scripts/go.sh build ./cmd/...` — passed.
- `scripts/go.sh test ./internal/runner -run TestRegistryTunnelProxy -count=1` — passed.
- Windows amd64 runner test binary cross-compilation — passed.
- Windows native runner package test binary — passed.
- Windows native `TestRegistryTunnelProxy` cases — passed, including malformed/invalid requests, exact header limits, queue saturation, shutdown, and pending-dial cancellation.
- Windows native `TestRegistryTunnelProxyCapsConcurrentTunnels` with `-test.count=100` — passed. Before the fix, the same 100-run reproduction repeatedly failed on the overflow request with `WSAECONNRESET` because the server closed with unread request bytes.
- Frontend `npm test` — 58 tests passed across 11 files; `npm run typecheck` and `npm run lint` passed; `npm run build` passed. Vite emitted a chunk-size advisory for the 537.51 kB JS bundle.
- `git diff --check` — passed.
- Read-only review of Task input delivery: 0 Critical, 0 Important, 0 Minor after the summary-label and forged-context regressions were added.
- Read-only review of the registry broker: no remaining actionable findings after documenting and testing the bounded overload contract.

The Go suite in this slice used local/offline paths. No real model turn, QQ send, GitHub account, external MCP connection, registry request, npm install, or real project process was run. Earlier dedicated Schema 26 upload/Worker/readback evidence remains under slice 12 and is not replaced by this slice.

## Windows egress integration research

Microsoft documents `FWPM_CONDITION_ALE_PACKAGE_ID` at ALE connect layers, WFP add rights and dynamic-session behavior, and requires callers using `NetworkIsolationSetAppContainerConfig` to preserve the existing loopback-enabled SID list. These constraints are recorded for the next implementation slice:

- [Filtering conditions available at each filtering layer](https://learn.microsoft.com/en-us/windows/win32/fwp/filtering-conditions-available-at-each-filtering-layer)
- [WFP access control](https://learn.microsoft.com/en-us/windows/win32/fwp/access-control)
- [NetworkIsolationSetAppContainerConfig](https://learn.microsoft.com/en-us/windows/win32/api/netfw/nf-netfw-networkisolationsetappcontainerconfig)

The required AppContainer-scoped WFP policy and safe Control/JobRun integration remain open; this verification does not qualify registry-only egress.
