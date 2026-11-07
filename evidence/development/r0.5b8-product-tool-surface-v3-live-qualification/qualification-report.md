# R0.5B8 product tool surface v3 live qualification

**Status:** `PASSED`

**Current product surface:** `polis-product-tool-surface@3`

**@2 status:** `HISTORICAL`  
**@3 provider status:** `QUALIFIED`  
**Eligible for future product sample:** `YES`

## Exact provider-visible surface

The current `polis serve` product registration through `RealProviderWorkerAdapter` was recomputed from the generic product registry:

- tools: **7**
- manifest: `95d4f2e2b1551096f05c7683786ec09a2d75189f8411fc66ffc759514de32257`
- aggregate schema bytes: **2206**
- aggregate schema digest: `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`

The ordered tools are `polis_work_current`, `polis_context_read`, `polis_workspace_read`, `polis_workspace_replace`, `polis_workspace_check`, `polis_work_checkpoint`, and `polis_artifact_submit`. The provider-visible checkpoint contract exposes `rejected: []` as the sole no-rejection encoding and typed `{receipt_id, proves}` evidence references. Structured public field/reason/expected/actual feedback and the existing TaskValidationBinding, workspace, checkpoint, Artifact, and post-publication fences were retained.

Exact surface evidence: `exact-product-surface.json`, `surface-after.json`, and `exact-product-surface-test.txt`.

## Offline qualification

`product_surface_v3_offline_L2 = PASSED` before provider access. The dedicated PostgreSQL 18 database used the `polis_r0_` namespace and was removed after the run. The offline gate covered registration/schema validity, deterministic manifest, dispatcher and authorization binding, receipt references, checkpoint lifecycle, Artifact policy, workspace/post-publication fencing, corrected LIVE_2 call-7 equivalence, frozen malformed LIVE_2 payload failures, TaskValidationBinding, Workbench projection, and real-adapter compatibility with fake transport.

- diagnostic reservation before canary: **0**
- provider egress before canary: **0**
- business database used: **NO**

Offline evidence: `offline-l2-final.json` and `offline/`.

## Execution identity and authorization

- model / effort: `gpt-5.6-luna / medium`
- provider runtime: Codex `0.154.0-alpha.6.2`
- native launch: Windows direct, unchanged `r03a-transport-policy@1`
- exact-surface execution fingerprint: `f43627632d1abc2c4e60ceac49c9c245e140cce61f7ce9b77552a6b8c192659c`
- historical @2 fingerprint reused: **NO**
- purpose: `PRODUCT_SURFACE_V3_LIVE_QUALIFICATION`
- reservation / provider-attempt / egress limits: `1 / 1 / 1`
- High / retry / business allowance / expected tool calls: `0 / 0 / 0 / 0`

Authorization evidence: `product-execution-manifest.json` and `diagnostic-authorization.json`.

## Single live diagnostic canary

| Measure | Result |
| --- | --- |
| diagnostic reservation / provider egress | `1 / 1` |
| process start → initialize → thread/start → turn/start | recorded in `provider-transport-terminal.json` and `provider/protocol.jsonl` |
| registered tools / manifest | `7 / 95d4f2e2…de32257` |
| first valid output | `POLIS_PRODUCT_SURFACE_V3_CANARY_OK` |
| time to first output | **3089 ms** |
| elapsed reservation → stop | **6692 ms** |
| tokens | **11217 total** (`11203 input`, `14 output`, `0 reasoning`) |
| tool calls / reconnects | `0 / 0` |
| terminal / clean stop | `completed / PASS` |
| credential snapshot removed | `YES` |
| duplicate reservation | `DENIED`; no second process or egress |
| Company / Mission / Task / WorkerSession / successor / business mutations | `0 / 0 / 0 / 0 / 0 / 0` |

The canary is terminal. No further provider traffic was made after it.

## Freshness and final verification

`surface_fresh = true`; pre- and post-canary manifest and schema digest are byte-equivalent. The canary result and provider L2 fingerprint are in `live-canary-result.json` and `product-live-l2-qualification.json`.

`product_provider_L2_fingerprint`:

`1f3a4f3b99247ae1e071ed2309a90dbb2d8ff55c06b39665ece55eafc8b23e6c`

Post-canary verification passed:

- targeted provider/codex, checkpoint/fencing, and RealProviderWorkerAdapter tests;
- `go test ./... -count=1`;
- `go test -race ./... -count=1`;
- `go vet ./...`;
- Linux and Windows amd64 command builds;
- frontend tests: 19/19, typecheck, lint, production build;
- dedicated post-canary PostgreSQL focused lifecycle/read-model/control tests;
- Bash syntax and `git diff --check`.

The first offline preflight failure is preserved separately at `../r0.5b8-product-tool-surface-v3-live-qualification-preflight-failure-1/`; it stopped before provider access because it included historical opt-in control fixtures, with reservation/egress still zero. No LIVE_2 retry, business Mission, High turn, successor, or multi-Agent E2E was started.
