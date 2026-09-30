# Slice 61 verification — Streamable HTTP MCP runtime and Worker path

Date: 2026-09-28  
Schema: 46  
Scope: local implementation and disposable fixture verification. No remote MCP endpoint, model, QQ account, GitHub account, WFP filter or production database was used.

## Verification results

- `rtk run "bash scripts/go.sh test ./... -count=1"` — PASS.
- `rtk run "bash scripts/go.sh build ./cmd/..."` — PASS for Linux amd64.
- `rtk run "bash -lc 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'"` — PASS for Windows amd64.
- `rtk npm run test --prefix frontend` — PASS, 106 tests across 12 files.
- `rtk npm run build --prefix frontend` — PASS; TypeScript project build and Vite production bundle completed. Vite reports the main bundle is above its 500 kB advisory threshold (645.15 kB before gzip).
- `rtk npm run lint --prefix frontend` — PASS with zero warnings.
- `rtk run "bash scripts/r1-capability-source-postgres-test.sh"` — PASS on disposable PostgreSQL 18 through Schema 46. The HTTP profile test covers metadata approval, synthetic runtime record, runtime approval, version-bound Employee binding, the disabled/enabled `work_current` projection, authorization, one-shot dispatch and completion using fixture-only tool-list data.
- `rtk run "bash scripts/r2-cross-backend-handover-postgres-test.sh"` — PASS; `R2_CROSS_BACKEND_HANDOVER=PASSED`.
- `rtk run "bash scripts/r2-recovery-backup-postgres-test.sh"` — PASS; `R2_RECOVERY_BACKUP_RESTORE=PASSED`.
- `rtk run "bash scripts/r3-domain-evidence-postgres-test.sh"` — PASS on disposable PostgreSQL 18.
- Focused Go tests pass for Streamable HTTP response parsing, deterministic tool-schema digest, schema drift rejection, argument validation, text-only result conversion, Worker context projection, dispatch fencing, default-off egress, the separately pinned transport-neutral provider surface, IPv6 documentation-range rejection and desktop-token route policy.
- `TestBuildWorkerAdapterSelectsOfflineMCPV2WithoutAppContainer` and `TestBuildWorkerAdapterRejectsMixedOrRealMCPV2Surface` — PASS. The v2 manifest is `11deb3d2e8b5038d21c23d61ab2b53af9a267c8bbd0c4828814b597064063346`; v1 remains pinned to `4856eeb48a1b70dd71727223e0877c8a6f68c883616f72dcc747b0c18291f451`.

## Implemented behavior

Schema 46 extends the append-only MCP runtime qualification record and tool-call ledger with transport-specific fields and result boundaries. The token-gated Workbench observation action requires the current endpoint descriptor, metadata qualification and manual approval. It then records one bounded `tools/list` response and its canonical digest. Runtime approval and Employee binding are separate decisions. The separately versioned MCP v2 provider surface describes controlled stdio and fixed Streamable HTTP profiles; the historical stdio-only surface keeps its original fingerprint.

With the v2 surface active, the trusted adapter includes the approved HTTP endpoint and schema in `work_current` only when `POLIS_MCP_STREAMABLE_HTTP_ENABLED=1`. The disposable PostgreSQL test verifies that the disabled projection is empty and the enabled projection carries the approved endpoint and schema into the actual Employee context. The Worker validates current authorization before creating an intent. It refuses Streamable HTTP dispatch when the v2 surface or egress flag is absent. With the gate enabled, it commits one intent, checks the remote schema before each call, validates arguments against the pinned schema and accepts bounded text-only results. A changed schema records drift and blocks the call. Unknown call outcomes are not replayed.

## Limits

The database integration uses synthetic observations and never opens a network connection. The HTTP client tests use local fake servers. IPv6 literals and DNS answers must be within IANA's `2000::/3` Global Unicast allocation; IPv4-mapped IPv6 addresses are denied. The denylist also covers the special-purpose ranges `100::/64`, `3fff::/20` and `5f00::/16` ([IANA IPv6 address space registry](https://www.iana.org/assignments/ipv6-address-space), [IANA IPv6 Special-Purpose Address Registry](https://www.iana.org/assignments/iana-ipv6-special-registry)). No external endpoint or real tool call was run, so no source-specific endpoint, identity, reachability or operational qualification is claimed. The v2 surface remains unqualified for a real provider. OAuth, credential forwarding, subscriptions, pagination, legacy fallback and non-public/private endpoints remain unsupported. R3 content-operations and research profiles remain `not_run`; the R3 suite verifies evidence contracts, not domain outcomes.
