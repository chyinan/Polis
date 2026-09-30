# Slice 52 verification: persisted stdio MCP runtime qualification gate

Date: 2026-09-27. Development schema: 42.

## Implemented

- Schema 41 stores immutable runtime qualification snapshots: descriptor/version, command digest, complete package-manifest digest, host profile, server identity, pinned protocol, tool schema and process launch spec. Initial state is `observed_unqualified`.
- Schema 42 constrains process-spec digests and runtime-event shape, validates observation/approval/drift/revocation transitions, and prevents snapshot/event update, delete and truncate.
- Only the opaque `mcpowner.RuntimeObservation` from public concrete-AppContainer `Start`, after discovery and clean stop with no tool calls, can be recorded. The package-private pseudo-server launcher cannot seal one; zero/fake observations are rejected.
- Runtime approval is a distinct local-owner action after existing metadata approval. Call authorization rechecks the current capability version/decision, Employee binding, latest runtime event, tool-schema digest and tool name under the company write guard. Drift and capability revocation append events that deny later checks.
- The capability catalog exposes safe runtime summaries. The Workbench displays observed/approved/drift/revoked states and labels technically qualified MCP as `runtime_qualified_dispatch_unavailable` because Worker call dispatch is not wired.

## Verification

- `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` — PASS. Uses an isolated temporary PostgreSQL 18 cluster/database, upgrades through Schema 42, and exercises sealed-observation denial, fixture-only authorization sequencing, current employee binding, unknown-tool denial, schema-drift denial, capability revocation and append-only protections.
- `rtk proxy bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS at Schema 42, including migration execution evidence and the existing R2 handover/recovery paths.
- `rtk proxy bash scripts/go.sh test -race -count=1 ./internal/mcpowner ./internal/mcptransport` — PASS.
- Frontend full suite — PASS (97 tests); typecheck, lint and production build — PASS.

## Safety boundary and remaining work

No MCP executable, endpoint, AppContainer/WFP setup, model provider, QQ or GitHub service was used. The positive authorization state-machine branch uses synthetic rows inside the disposable database and is not qualification evidence. The owner-sealed observation path remains unexercised on a Windows host. Worker `mcp_call` dispatch, per-session owner lifecycle, and the durable one-shot intent/result/`outcome_unknown` ledger remain open. No migration rollback or database/global cleanup was run; the temporary test script cleaned only its validated `/tmp/polis-*` resources.
