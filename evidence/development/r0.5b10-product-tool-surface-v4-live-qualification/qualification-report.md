# R0.5B10 — Product tool surface v4 live qualification

## Result

`product_surface_v4_live_L2 = INCONCLUSIVE`.

The single authorized diagnostic attempt was consumed and is terminal. No retry, second reservation, High turn, successor, business Mission, LIVE_3, or multi-Agent E2E was started.

## Exact current surface

The current production product registration is `polis-product-tool-surface@4`, derived from `provider.ProductToolSurface()` / `codex.ProductEmployeeTools()` and used by the `polis serve` real-worker path and `RealProviderWorkerAdapter`.

- tool count: `7`
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- aggregate schema bytes: `2206`
- aggregate schema digest: `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- B9 frozen identity: `MATCH`
- post-canary freshness: `MATCH`

Tools in exact order:

1. `polis_work_current` — current authorized Task, validation binding, workspace metadata, handover and budget.
2. `polis_context_read` — approved context and persisted facts/decisions.
3. `polis_workspace_read` — current workspace content, digest and revision.
4. `polis_workspace_replace` — full workspace mutation with expected digest/revision and conflict feedback.
5. `polis_workspace_check` — validation against the Task-bound public acceptance contract; `task_submit` is the explicit delivery decision.
6. `polis_work_checkpoint` — progress/qualified checkpoint capability; final qualification is deterministically created by `task_submit`.
7. `polis_task_submit` — explicit post-PASS submission; the control plane creates the qualified checkpoint, publishes the Artifact and moves the Task to `candidate`.

The complete exact descriptions and JSON schemas are frozen in `exact-product-surface.json`. No stale @3 manual `artifact_submit` flow is exposed.

## Offline L2

`product_surface_v4_offline_L2 = PASSED` before provider access. The disposable `polis_r0_r05b10_*` PostgreSQL run passed all 18 gates, including registration/schema/manifest, dispatcher and authorization binding, TaskValidationBinding, stale validation, workspace CAS and fencing, deterministic B9 delivery transaction, duplicate/replay safety, checkpoint/Artifact publication, post-publication fencing, adapter compatibility and the frozen LIVE_2 counterfactual. Provider traffic was `0` and business mutations were `0`.

## Execution identity and authorization

- model / effort: `gpt-5.6-luna / medium`
- provider runtime bound in authorization: Windows-native `0.155.0-alpha.9.2`
- launch envelope: `windows_native_direct`, `r03a-transport-policy@1`
- exact-surface execution fingerprint: `d931a5faf01f72f23ad91fd16f812a5e2b240b967f57b7fe9ab1b718db76bf61`
- diagnostic purpose: `PRODUCT_SURFACE_V4_LIVE_QUALIFICATION`
- diagnostic reservation / attempt / provider egress limits: `1 / 1 / 1`
- Medium / High / retry / business allowance: `1 / 0 / 0 / 0`

`product_provider_v4_L2_fingerprint = NOT_CREATED` because the turn did not reach a valid provider L2 pass boundary.

## Single live diagnostic attempt

| Measure | Result |
| --- | --- |
| reservation / provider egress | `1 / 0` |
| process | started, PID `79300` |
| initialize | response received, then version validation failed |
| thread/start / turn/start | not started |
| first output / latency | empty / `0 ms` |
| tokens | `0` |
| tool calls / reconnects | `0 / 0` |
| terminal / clean stop | `initialize_failed / PASS` |
| duplicate reservation | `DENIED` |
| business side effects | Company `0`, Mission `0`, Task `0`, WorkerSession `0`, successor `0`, mutations `0` |

The provider protocol reported user agent `polis/0.154.0-alpha.6.2`, while the authorization was bound to expected version `0.155.0-alpha.9.2`; `initialize` therefore failed closed. The provider process was stopped cleanly and the credential snapshot was removed. The available controlled runtime scan found only the 0.154 binary; no compatible 0.155 binary was available for a retry.

The consumed first runner build also exposed a source-level path defect: its default diagnostic root still pointed at the historical B8 runtime directory. The B10 source was corrected to use the dedicated `r0.5b10-product-tool-surface-v4` root after the terminal attempt; this correction is recorded for a future separately authorized run and did not trigger or justify another provider request.

## Status

- `@3`: `HISTORICAL / NOT_CURRENT`
- `@4`: `CURRENT`, exact surface and offline L2 passed
- `product_provider_surface_status`: `NOT_QUALIFIED`
- `eligible_for_future_product_sample`: `NO`
- business `ProductExactSurfaceExecutionFingerprint` / `ProductProviderL2Fingerprint`: unchanged; no unqualified v4 live result was promoted into business authorization constants.

Evidence files include `product-execution-manifest.json`, `diagnostic-authorization.json`, `diagnostic-reservation.json`, `provider-transport-terminal.json`, `live-canary-result.json`, `surface-after.json`, `exact-product-surface.json`, `offline-l2-final.json`, and the provider `protocol.jsonl`.

## Post-canary verification

- targeted surface/registration, B9 delivery, and RealProviderWorkerAdapter tests: `PASS`
- `go test ./...`: `PASS`
- `go test -race ./...`: `PASS`
- `go vet ./...`: `PASS`
- Linux and Windows `go build ./cmd/...`: `PASS`
- frontend tests: `19/19 PASS`; typecheck, lint and build: `PASS`
- disposable PostgreSQL/CAS delivery, fencing, adapter and Workbench projection: `PASS`; provider traffic `0`
- Bash syntax, Python syntax and `git diff --check`: `PASS` (line-ending warnings only)
