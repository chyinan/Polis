# R0.5B16 reservation-to-initialize bridge hardening

## Frozen LIVE_4 boundary

The immutable LIVE_4 record is `evidence/development/r0.5b-real-provider-product-smoke-live-4/live-4-result.json`. It proves business authorization, reservation, WorkerSession, process creation and PID attachment, with the child alive before initialize. It does not prove initialize payload construction or write: those are `NOT_DETERMINABLE_FROM_FROZEN_EVIDENCE` / `NOT_OBSERVED`. Initialize ACK, thread/start, turn/start and provider egress were not observed; provider egress and turn count were zero.

The derived, non-mutating boundary record is `evidence/development/r0.5b16-reservation-to-initialize-bridge-hardening/preflight-v6/live4-boundary.json`.

## Root cause and remediation

`CodexRuntime.Start` always wrapped sessions in `productSurfaceDiagnosticSession`, including business reservation-bearing starts. That wrapper compares the incoming transport policy with the empty diagnostic authorization identity and rejects before calling the protocol client. The process therefore existed, but initialize request write never happened.

The fix makes the wrapper conditional on `CodexRuntimeConfig.DiagnosticOnly`. Business sessions now retain the normal `codexSession`; diagnostic-only sessions retain the diagnostic wrapper. Reservation ownership is held by `providerWorker` and closed exactly once during cleanup or pre-initialize start failure.

## Evidence and tests

- `preflight-v6/r0-5b16-result.json`: PASSED; 3/3 fresh reservation-bearing business cycles.
- Each cycle: process/PID, initialize request/write/ACK, thread/start, clean stop and process exit PASS.
- Reservation created/closed `3/3`, active `0`, provider egress `0`, turn/start `0`, orphan processes `0`.
- Protocol lifecycle phases: `initialize_prepare_started`, `initialize_payload_ready`, `initialize_write_started`, `initialize_write_completed`, `initialize_ack_received`.
- Durable controller evidence: `provider.runtime.lifecycle` events and bounded `provider_lifecycle` observations; no credentials, prompts, raw stderr or unrestricted environment.
- B14 identity comparison: surface/runtime/launch/execution unchanged; `QUALIFIED_REUSABLE`; new live product smoke remains eligibility-only.

## Validation

Fresh verification passed: `go test ./...`, `go test -race ./...`, `go vet ./...`, Linux/Windows command builds, frontend tests (20/20), typecheck, lint, build, targeted PostgreSQL control tests, Bash/PowerShell syntax and `git diff --check`.

The dedicated B16 PostgreSQL server was stopped after evidence capture; its `pg_ctl status` reports no server running. LIVE_4 evidence and all prior B15/B14 evidence remain unchanged.
