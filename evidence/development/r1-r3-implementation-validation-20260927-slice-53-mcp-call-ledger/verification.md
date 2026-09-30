# Slice 53 verification: durable one-shot stdio MCP call ledger

Date: 2026-09-27. Development schema: 43.

## Implemented

- Schema 43 stores immutable MCP call intents bound to one WorkerSession, Employee, capability, runtime qualification, provider call ID, tool name/schema digest, and argument digest. Arguments are not stored in the ledger.
- Append-only call events encode `dispatching`, `completed`, or `outcome_unknown`. PostgreSQL triggers verify the active company-side Employee binding, descriptor version/approval, metadata approval, runtime qualification, session identity, and tool/schema relationship before accepting a dispatch intent. Snapshot, intent, and event histories reject update/delete/truncate.
- Kernel intent reservation internally checks current dispatch authorization and returns only the immutable call record, never a reusable runtime-authorization value. Its post-commit read confirms the exact call remains dispatching and matches the same WorkerSession/Employee/capability/runtime/schema/arguments. Result completion rechecks exact session and Employee ownership and is idempotent for identical completed results. Unknown calls cannot be replayed or completed late.
- Session reconciliation accepts only `stopped` or `reconcile_required` WorkerSessions; active and merely `stopping` sessions are refused so process cleanup must be confirmed before marking pending calls unknown.
- The persisted MCP result is bounded text carrying `untrusted_stdio_mcp_text`, stored as the exact bounded JSON bytes (`bytea`) so PostgreSQL JSONB whitespace expansion cannot exceed the Kernel's persistence limit. The database trigger parses those bytes and checks the tool name, schema digest and content boundary. Provider arguments are hashed but not retained. No Worker call path invokes these APIs yet.
- The Workbench runtime-approval API now removes URL-scoped `companyId` from the strict-decoded request body.
- R3 evidence script now first qualifies at Schema 39 for its Schema 38 rollback-guard test, then upgrades to current Schema 43 before domain workflow integration checks. Migration evidence tests now assert all 43 versions.

## Verification

- `rtk npm test -- src/data/real-workbench-api.test.ts` — PASS, 31 tests. The changed expectation first failed because the request body included `companyId`; after the fix all 31 passed.
- `rtk proxy bash scripts/go.sh test ./internal/kernel -run TestValidateStdioMCPToolCall -count=1` — PASS for exact dispatch-row and Employee/session-owner validation. The test was first written against missing helpers (RED), then passed after implementation.
- `rtk proxy bash scripts/go.sh test ./internal/kernel -run TestBeginMCPCallReturnsIntentState -count=1` — PASS; reservation result contains call state only and does not expose a reusable authorization. The reflection regression first failed with the old result type.
- `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` — PASS on a temporary PostgreSQL 18 database at Schema 43 with race detection. Covers current approval/binding, duplicate intent rejection, completed result persistence/idempotence and stable intent `createdAt`, exact Employee/session owner checks, immutable ledger protections, unknown outcome only after confirmed WorkerSession stop, active/stopping sweep refusal, refusal to replay/complete unknown calls, refusal after schema drift, and a 5,800-empty-block result whose compact JSON fits the Kernel bound while JSONB rendering would exceed it. The synthetic positive authorization rows are fixture-only.
- `rtk proxy bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS; reports `R2_CROSS_BACKEND_HANDOVER=PASSED`, including the Schema 43 migration ledger.
- `rtk proxy bash scripts/r3-domain-evidence-postgres-test.sh` — PASS on a temporary PostgreSQL 18 database at Schema 43.
- `rtk npm run typecheck` and `rtk npm run lint` — PASS.
- `rtk proxy bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk powershell -NoProfile -File scripts/test-migration-hash-manifest.ps1` — PASS; updater rejects pinned edits and invalid version/layout changes.
- `rtk git diff --check` — PASS.
- Migration hash updater verified 42 prior migration pins and added the forward Schema 43 hash after the migration content was finalized.

The `createdAt` and active/stopping sweep regressions were observed failing before their fixes. The large empty-block result regression first failed the JSONB size check; storing validated JSON bytes verbatim made it pass. No external or model activity was used during RED runs.

## Review

The final read-only Slice 53 review returned zero findings. It confirmed that reservation returns no reusable authorization, the byte-preserving bounded result fixes the JSONB expansion mismatch, and exact Employee/session completion checks plus the stopped/reconcile-required sweep fence are present. The reviewer ran no tests or external actions.

## Limits

No MCP process, endpoint, Windows AppContainer/WFP setup, Worker model, QQ, GitHub, or production service ran. Schema 43 provides a tested persistence boundary, not a completed Worker dispatch integration. Per-session `mcpowner.Owner` start/stop and real Windows host qualification remain open. The Worker path must reauthorize and atomically consume the intent while holding a per-session lifecycle gate, then prevent stop/reconciliation until the owner call is settled; this API intentionally returns no reusable authorization. The plan remains incomplete.
