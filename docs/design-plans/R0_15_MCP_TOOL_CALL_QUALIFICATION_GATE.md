# R0.15 — MCP Tool-Call Qualification Gate (PLAN ONLY)

> Status: planning/design. **No implementation that starts MCP processes, performs network/OAuth egress, executes tool calls, runs Skill scripts, sends QQ, or starts provider turns is authorized by this document.** Any such work requires a separate bounded slice with explicit authorization.

## 1. Objective and boundary

Define the deny-by-default gate for a future `tools.call` capability over already-registered MCP server definitions. R0.8–R0.14 established the read-only inspection path at schema 15: catalog registration, employee binding, local stdio probe receipt, `skills.load`/reference/summary, and `tools.list`/summary/`tools.describe` over frozen cached descriptors. R0.14 returns `callable=false` and `toolsCalled=0`; R0.10 probe and R0.14 descriptor reads must not be treated as permission to call tools.

This slice is **planning only**. It produces the contract, policy model, and offline verification matrix for a later implementation slice. It does not change `RELEASE_SCOPE.json`, does not relax safety constraints, and does not authorize live MCP calls.

## 2. Current facts (schema 15, as of R0.14)

- **Catalog/binding**: MCP definitions and employee capability bindings are Company-scoped; binding approval does not imply execution or connection permission (R0.9).
- **Probe receipt**: `mcp_server_definitions.qualification` may contain `probeStatus`, `toolNames`, `toolsCalled`, and (post-R0.14) `toolDescriptors` with per-tool digests. R0.10 local probe used `@modelcontextprotocol/server-memory@2.0.1`; probe process was not network-sandboxed (evidence recorded). A passed probe moves definition to `candidate`, not to callable.
- **Read path**: `visibleToolPage` / `visibleToolDescriptor` require approved binding, bound server, non-revoked/non-unverified definition, `probeStatus=passed`, `toolsCalled==0` in the receipt for the read-only contract, and matching cached descriptor body for describe. Descriptor returns `Callable:false`, `ToolsCalled:0`.
- **Interfaces**: workbench HTTP exposes `capabilities.tools.list` and `capabilities.tools.describe` (company-scoped). Kernel/control service methods back them.
- **Constraints**: no external MCP/business accounts/QQ/provider turns without separate authorization; dedicated disposable resources for tests; no push/publication expansion.

## 3. Proposed gate contract (design target)

A future `tools.call` request MUST be denied by default. A call may proceed only when every required precondition passes; failure returns a structured, non-mutating denial reason (no silent fallback to a different tool/server).

### 3.1 Required preconditions

| Check | Intent |
| --- | --- |
| Identity/scope | Valid company, employee, task/session context as defined by the call surface; caller is authorized for the binding scope. |
| Binding | Approved `EmployeeCapabilityBinding` explicitly includes the target MCP server definition/revision context; draft/revoked/disabled bindings deny. |
| Definition state | Target definition is not revoked/unverified; descriptor/probe evidence matches the bound server and tool handle (`serverId:toolName`). |
| Cached descriptor integrity | Input schema/descriptor digest matches frozen receipt; malformed or integrity-mismatched descriptors deny. |
| Call qualification | Separate explicit call-authorization/qualification state exists (distinct from read-only probe pass and descriptor cache). **Design decision required:** model as binding qualification transition, per-server/per-tool policy revision, or explicit operator approval record with digest. Must not be satisfied by R0.10 probe alone. |
| Runtime policy | MCPBridge boundary is explicit: command/transport allowlist, env/secret handling, working directory, network/egress policy, timeout, stdout/stderr capture limits, and cancellation. No inherited ambient secrets; no arbitrary shell expansion from model input. |
| Authorization | Operator/human authorization recorded for the environment/class of call where required by policy (especially non-local, stateful, or externally effectful tools). |
| Audit | Request binds company/employee/binding/definition/tool/descriptor digest/argument digest, policy revision, and idempotency key; response records outcome, bounded result/error, timing, and egress/process facts. |

### 3.2 Request/response sketch (non-final)

```text
Request
  company_id, employee_id, binding_id?, task_id/session_id/epoch as applicable
  handle = serverId:toolName
  descriptor_digest (must equal cached descriptor digest)
  arguments (JSON object, validated against cached input schema)
  idempotency_key
  authorization_ref / policy_revision (design choice)

Response
  receipt_id, outcome (denied|completed|failed|timeout|cancelled|unknown)
  denial_reason_code? (structured: BINDING_NOT_APPROVED, DEFINITION_REVOKED,
    DESCRIPTOR_DIGEST_MISMATCH, CALL_NOT_QUALIFIED, SCHEMA_INVALID,
    AUTHORIZATION_REQUIRED, POLICY_DENIED, MCP_PROCESS_FAILED, ...)
  bounded result/error payload, result_digest
  observed tools_called/process/network facts (bounded)
  audit references
```

Exact field names/status codes are deferred to the implementation slice; the plan requires stable machine-readable denial reasons and no hidden success on policy failure.

### 3.3 Safety properties

- Deny-by-default; no implicit upgrade from read/list/describe/load/probe.
- Fail closed on missing/ambiguous qualification, digest mismatch, schema mismatch, revoked state, or authorization absence.
- Idempotency for retried calls where tool semantics allow; unknown-outcome handling must not fabricate success (align with existing core/kernel integrity patterns).
- Bounded input/output capture; redact secrets from receipts/logs.
- No `scripts.run` or Skill script execution implied by `tools.call`; those remain separate capability paths.

## 4. Implementation sequence (future, each step separately authorized as needed)

1. **Policy model & schema decision**: choose how call qualification is represented (catalog/binding qualification state vs explicit approval record), additive migration only if required, with `migrateVersion` gate preserved. Keep read-only behavior unchanged.
2. **Pure validation**: argument validation against cached JSON schema, digest/handle checks, policy preconditions, denial reason mapping; unit tests with no process spawn.
3. **MCPBridge execution boundary**: controlled process/transport adapter, strict env/argv construction, timeout/cancel, bounded I/O, and receipt capture. Local/offline fake MCP server fixtures for deterministic tests.
4. **Kernel/control/workbench surface**: `tools.call` API only after gate passes; company-scoped authorization; no UI auto-execution.
5. **Qualification & evidence**: disposable PostgreSQL tests; optional separately authorized local canary only after offline gates pass. No external/production MCP, OAuth, QQ, or provider egress unless explicitly authorized.

## 5. Verification plan (offline-first)

- Unit: handle parsing, digest mismatch, schema validation, binding/definition state matrix, deny-by-default when call qualification absent.
- Integration (disposable `polis_r0_` DB): approved+qualified happy path with fake/local MCP; revoked binding/definition, stale descriptor digest, missing authorization, schema-invalid args, idempotent replay, process timeout/failure receipts.
- Regression: R0.13 `tools.list` / R0.14 `tools.describe` remain read-only and unchanged; `Callable` stays false on read surfaces unless a deliberate, reviewed contract change is made.
- Standard gates before claiming completion: targeted tests, `bash scripts/go.sh test ./...` / race as appropriate, `go vet`, build; evidence under `evidence/development/r0.15-*` only if/when a future implementation slice runs.

## 6. Definition of done (for the future implementation slice)

- `tools.call` is unavailable unless all gate preconditions pass; probe/descriptor/read/load never authorize execution.
- Denials are structured and auditable; receipts do not leak secrets.
- Cached descriptor digest and schema are enforced at call time.
- Fake/local MCP deterministic path passes with zero external egress.
- Any real MCP qualification/canary is separately authorized, evidenced, and bounded; absent that authorization, result remains planning/unqualified for live calls.
- `NEXT_SLICE.md`, `AGENTS.md` (if stage changes), implementation report, and evidence paths updated consistently.

## 7. Open decisions

- **Call qualification representation**: additive state on binding/definition qualification, or a dedicated approval/receipt table.
- **Schema 16 trigger**: only if persistent call-qualification/audit structures require migration; avoid migration churn if file/receipt-only design suffices.
- **MCP transport scope**: stdio-first (matching R0.10 probe) vs. explicit separate slice for Streamable HTTP/remote endpoints.
- **Effect classification**: read-only/idempotent/effectful tool metadata handling and whether effectful calls require stronger authorization.
- **Relation to product worker tool surfaces**: employee capability MCP calls vs. existing product/provider tool registries are different surfaces; do not conflate without an explicit design decision.
