# R0.5B14 — Product tool surface v4 live qualification attempt 3

## Result

`product_surface_v4_live_L2 = PASSED`.

`product_provider_surface_status = QUALIFIED` and `eligible_for_live_3 = YES`.

This was exactly one diagnostic-only provider attempt. It created no Company, Mission, Task, WorkerSession, successor or business mutation. No LIVE_3, business Mission, High turn, retry, successor or multi-Agent E2E was started. No provider traffic occurred after the one canary turn.

## Exact current binding

- surface: `polis-product-tool-surface@4`
- tool count: `7`
- ordered tool definitions/descriptions/schemas: `exact-product-surface.json`
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- aggregate schema: `2206` bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- runtime version: `0.154.0-alpha.6.2`
- executable SHA256: `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575`
- helper SHA256: `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e`
- launch envelope fingerprint: `dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d`
- exact-surface execution fingerprint: `e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff`
- model / effort: `gpt-5.6-luna` / `medium`
- transport policy: `r03a-transport-policy@1`

Phase A recomputation matched the B13 current authority. Phase B's single native zero-turn preflight passed process creation/PID, initialize, thread/start, exact 7-tool registration and clean stop with reservation=0 and provider_egress=0. Phase C reran the current offline L2 with 18 gates passed, a dedicated temporary PostgreSQL/CAS resource and reservation/egress=0.

## Diagnostic authorization and canary

- authorization ID: `r0.5b14-live-canary-1`
- purpose: `PRODUCT_SURFACE_V4_LIVE_QUALIFICATION_ATTEMPT_3`
- provider session: `r05b14-product-surface-v4-canary`
- diagnostic reservation: `1`
- provider egress: `1`
- High: `0`
- retry: `0`
- business allowance: `0`
- duplicate reservation: `DENIED`

Launch and transport evidence:

- process created: `PASS`
- PID present: `PASS`, PID `71336`
- initialize: `PASS`
- thread/start: `PASS`, exact @4 registration `7`
- turn/start: `1`
- first valid output: `POLIS_PRODUCT_SURFACE_V4_CANARY_OK`
- first-output latency: `5187 ms`
- elapsed: `11148 ms`
- usage: total `11214`, input `11200`, output `14`, reasoning `0`, cached input `0`
- tool calls: `0`
- reconnects: `0`
- terminal: `completed`
- clean stop: `PASS`
- business side effects: Company `0`, Mission `0`, Task `0`, WorkerSession `0`, successor `0`, mutation `0`

The PID/process-created fields are independently reconciled in `launch-evidence-reconciliation.json` from the terminal's process-start timestamp, PID and process identity. The credential snapshot was removed and protocol evidence is valid.

Post-canary surface, runtime binding, launch envelope and exact execution fingerprint were all unchanged. Focused PostgreSQL/CAS integration passed after the canary with provider traffic `0` and business mutations `0`.

## Current provider L2 fingerprint

`product_provider_v4_L2_fingerprint = 59a4ac003fb0f49360382a707f7057edc03e0574ba328d0cf3a59eeab2726781`

The current product provider authorization default is updated to this fingerprint. B10/B12/B13 evidence remains immutable; B12 remains historical `INCONCLUSIVE` with its original `PROCESS_CREATE_FAILED_NO_PID` classification.

## Verification

- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- Linux and Windows command builds: PASS
- frontend test: 19/19 PASS
- frontend typecheck, lint and build: PASS
- B13 launch regressions: PASS
- B9 deterministic delivery tests: PASS
- exact @4 surface tests: PASS
- RealProviderWorkerAdapter tests: PASS
- dedicated offline and post-canary PostgreSQL/CAS integration: PASS
- Bash, PowerShell and Python syntax: PASS
- `git diff --check`: PASS

All raw telemetry, authorization, reservation, offline-L2, preflight, freshness, duplicate-guard and validation logs are retained in this directory.
