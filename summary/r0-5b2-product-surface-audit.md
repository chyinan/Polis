# R0.5B2 Product Provider Surface Audit

## Exact bytes

The current production registry path is `provider.ProductToolSurface() -> codex.Tools()`. It contains 7 ordered tools and recomputes to manifest digest `730b9b2aba9de7ca015e800b1a597de37a22bd0c299dc7ad10e022c8399388d9`, matching the B2 required digest. Manifest size is 2718 bytes. Applying the T21-style aggregate method independently to the ordered `inputSchema` array gives 1318 schema bytes and digest `12008e917b4b5a71dc5fa70e8c2388450205bc08fce1bddc84bae3742e155717`.

`ToolSurfaceFromTools` initially aliased `AggregateSchemaDigest` to the full manifest digest. B2 added independent schema digest/byte computation and makes the real adapter readiness reject missing aggregate schema metadata. This metadata correction does not change provider-visible tool bytes; the golden manifest test still passes.

## Semantics finding

The formal source-level `EmployeeTools.call` dispatcher handles all 7 registered names, and the R0.5B1 fake/local product vertical slice exercised the same manifest. However, current provider-visible descriptions for `workspace_read` and `workspace_replace` hardcode `formatter.go` and formatting-only source restrictions; `workspace_check` is described as running frozen task checks; and `artifact_submit` refers to a fixed-file candidate. The architecture's employee-ops/workspace contract describes authorized workspace resources and general product work, not that fixed formatter task.

The current `cmd/polis serve` real path also constructs `OfflineProductCheckRunner`; its `Check` passes for any non-empty content in phase `single`, not a qualified product acceptance policy. It is an offline bridge substitute, not sufficient business acceptance semantics.

Therefore `product_tool_surface_semantics = NOT_CURRENT` for this live product L2 request. The semantic gate stops before exact-surface execution identity, diagnostic authorization, process start, or live canary. Do not change the provider-visible surface in this qualification run, and do not continue to R0.5B business smoke.

## Result/evidence

`r0_5b2_product_provider_surface_live_qualification = INCONCLUSIVE`
`product_provider_surface_live_L2 = NOT_STARTED`
`product_provider_surface_status = NOT_QUALIFIED`
`eligible_for_r0_5b_real_provider_smoke = NO`
`provider_reservation = 0`
`provider_egress = 0`

Evidence is frozen under `evidence/development/r0.5b2-product-provider-surface-live-qualification/`. R0.5A, R0.5B1 and R0.5B raw evidence remain unchanged.
