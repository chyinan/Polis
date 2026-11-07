# R0.5B real provider product smoke preflight audit

Verified 2026-09-16 before any provider reservation or real Employee turn.

## Result

`r0_5b_real_provider_product_smoke = INCONCLUSIVE`.

Provider egress, allowance, reservation, WorkerSession, browser business path and real provider turn all remain zero/not started. R0.5A acceptance evidence was not modified.

## Confirmed path mismatch

- `cmd/polis/main.go` constructs `control.NewDeterministicWorkerAdapter` for `cmd/polis serve`.
- `internal/control/deterministic_worker.go` starts the local deterministic helper process and binds only the deterministic WorkerSession lifecycle; it never calls `codex.New`, `Client.Turn`, provider transport, or business authorization.
- `cmd/polis-r03a-real-backend/main.go` is a separate probe entrypoint, not the product HTTP command path.
- `internal/probe/r03a_t2.go` creates its own Company/Mission peer fixture, binds `r03a-real-peer-collaboration-v1`, starts the peer Backend flow and uses the R0.3A business tool surface. It does not consume the Mission created by the existing frontend command API.
- `internal/probe/business_authorization.go` and `internal/codex/business_authorization.go` enforce exact binding, but there is no product `control.Service` real-adapter implementation that can accept the R0.5B Mission/Task/Employee identity and exact one-turn authorization.

## Qualification context

Historical/current-binary L1/L2 evidence is diagnostic execution-envelope evidence and is retained unchanged. It does not create a product real WorkerAdapter or authorize repurposing the R0.3A peer probe as a frontend Mission smoke. The current product path therefore fails the required pre-provider gate.

## Stop boundary

No browser business command was started, no allowance or provider reservation was created, and no repair-and-continue or retry was attempted. A future R0.5B attempt requires a separately implemented and qualified product-facing real WorkerAdapter plus a fresh exact authorization binding; that is outside this smoke’s safe continuation boundary.
