# Slice 54 verification: controlled MCP Worker path

Date: 2026-09-28. Development schema: 43 (no migration in this slice).

## Implemented

- Added the pinned `polis-product-tool-surface@mcp-v1` surface with one `mcp_call` function. Its manifest digest is `4856eeb48a1b70dd71727223e0877c8a6f68c883616f72dcc747b0c18291f451`; aggregate schema is 2,647 bytes with digest `699ad79b737f03ba5b954ebaef4a0dd8b1d3b90c4df6928d865dbc2b9c63ae14`. The historical 7-tool surface is unchanged.
- The new surface is accepted only by the explicit zero-egress fake runtime. `ValidateRuntimeExecutionAuthorization` rejects it in real-provider mode; the Codex CLI branch also refuses the controlled-MCP surface until separately qualified.
- With this surface, `work_current` reads at most one current stdio MCP tool set for the bound Employee after checking descriptor/version approval, metadata approval, qualification status, host profile and tool-schema digest. Unbound, revoked, stale and drifted capabilities do not appear in Worker context.
- `RealProviderWorkerAdapter` parses a closed `mcp_call` request, reserves a Schema 43 one-shot intent, reauthorizes current binding/runtime/tool, lazily starts or reuses one owner from a concrete `runner.AppContainerSandbox`, calls the fixed stdio transport, and persists the bounded untrusted result. Owner reuse pins the runtime qualification ID, command digest, package-manifest digest and tool schema; a later qualification cannot reuse an old process.
- The adapter shares one owner lease across its WorkerSessions. Calls acquire the lease before reserving an intent; a busy result consumes no intent. Cleanup retains the lease if process-tree stop is unconfirmed and releases it only after Worker stop and owner stop are confirmed.
- A reservation error is treated as potentially committed: the state remains pending and blocks further calls until stopped-session reconciliation records an unknown result. Startup-time and call-time schema drift errors carry the observed digest and append a runtime-disabling event.
- Inbound `item/tool/call` protocol evidence preserves only safe correlation fields. It redacts arguments and drops unknown fields, preventing the MCP argument hash policy from being bypassed by durable protocol logs.
- CLI opt-in requires both `POLIS_CONTROLLED_MCP_TOOL_SURFACE=1` and `POLIS_MCP_APP_CONTAINER_ID`. The default remains off. There is no CLI fixture injection and no startup MCP launch.

## Verification

- `rtk proxy bash scripts/go.sh test ./internal/codex -run TestControlledMCP -count=1` — PASS; legacy tools remain byte-identical and the new closed schema has no command/endpoint/transport selector.
- `rtk proxy bash scripts/go.sh test ./internal/provider -run TestControlledMCP -count=1` and `... -run TestOfflineFakeMCPSessionDispatchesOnlyItsScriptedBoundCall -count=1` — PASS; real-provider authorization rejects the surface, and the fake scripted call is dispatched only after matching `work_current` bound-tool context.
- `rtk proxy bash scripts/go.sh test ./internal/control -run TestControlledMCPWorker -count=1` and `... -run TestLaunchQualificationAdapterAcceptsExplicitOfflineControlledMCPSurface -count=1` — PASS; verify authorization/owner/call/result order, qualification and package pinning, ambiguous reservation reconciliation, call/start schema-drift events, shared-owner contention before intent reservation, stop retry lease retention, and quiesce/owner-stop/unknown-sweep order.
- `rtk proxy bash scripts/go.sh test ./internal/codex -run TestForeignNativeCorrelationNeverReachesKernel -count=1` and `... -run TestInboundProtocolLog -count=1` — PASS; confirms tool-call arguments and unknown fields do not enter `protocol.jsonl`, while safe tool metadata and non-tool frames remain available.
- `rtk proxy bash scripts/go.sh test ./internal/mcpowner -run TestStdioOwnerRetainsFailedStartCleanupForRetry -count=1` — PASS; startup schema mismatch carries the observed digest through unresolved-stop cleanup.
- `rtk proxy bash scripts/go.sh test ./internal/mcptransport -run TestStdioClientRejectsToolSchemaDriftBeforeCalling -count=1` — PASS; drift errors carry the newly observed schema digest used to append a disabling event.
- `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` — PASS at Schema 43 with race detection, including bound Worker MCP context, unbound/revoked context exclusion and the call-ledger regression set.
- `rtk proxy bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS at Schema 43.
- `rtk proxy bash scripts/r2-recovery-backup-postgres-test.sh` — PASS at Schema 43. The integration assertion now accepts schemas at or above 40, the minimum required for migration-execution evidence, instead of freezing the backup check at historical Schema 40.
- `rtk proxy bash scripts/r3-domain-evidence-postgres-test.sh` — PASS at Schema 43; both domain profiles remain `not_run` and execution-disabled.
- `rtk proxy powershell.exe -NoProfile -File scripts/test-migration-hash-manifest.ps1` — PASS.
- Serial Go package tests for Control, Codex, MCP owner/transport and Recovery — PASS. `rtk proxy bash scripts/go.sh build ./cmd/...` — PASS.
- No real MCP process, real model/provider call, QQ, GitHub or production service was used. No Windows AppContainer/WFP host was provisioned or exercised.

## Remaining R1 gaps

- Control/Workbench still lacks a user-facing controlled package import and runtime-observation command, so operators cannot create a qualified ProcessSpec/runtime record through the app.
- Real Codex provider use of `@mcp-v1` remains intentionally denied until an exact-surface/runtime qualification is separately completed.
- The caller must provide the AppContainer identity used when the ProcessSpec was qualified. Windows host qualification and recovery of a detached MCP owner after parent-process crash remain unverified.
- R2 Linux/Node delegated-host restart recovery and host isolation remain unqualified; fake cgroup/filesystem tests are implementation evidence only.
- R3 content-operations and research profiles remain unavailable until each receives real domain-specific quality, recovery, cost and organization-benefit evidence, plus intervention evidence for content operations.
