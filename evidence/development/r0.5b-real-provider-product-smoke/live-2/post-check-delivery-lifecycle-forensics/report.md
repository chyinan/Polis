# R0.5B6 — Post-check delivery lifecycle forensic

## Adjudication

| Field | Result |
|---|---|
| Classification | **MODEL_FAILED_TO_COMPLETE_PUBLIC_DELIVERY_LIFECYCLE** |
| Dominant root cause | Model failed to complete the required lifecycle after acceptance. The checkpoint schema/runtime feedback gap is real but not shown to have blocked the available valid path. |
| Public Task contract quality | **PASS**: the turn prompt says to persist the requested Artifact and checkpoint; the TaskValidationBinding freezes four required-text criteria. Mission title/goal are absent from the `work_current`/`context_read` payloads, and those payloads also expose the unrelated legacy Contract `signed-zero@1`. |
| Product delivery contract quality | **PASS** for the required publication lifecycle. |
| Workspace-check feedback quality | **PASS for validation semantics**: it accurately reports required-text acceptance, but does not state that no checkpoint or Artifact was created or name the next step. The surrounding public tool descriptions supply the lifecycle. |
| Checkpoint contract visibility | **PARTIALLY_PUBLIC** |
| Artifact contract visibility | **FULLY_PUBLIC for the required publication gate and artifact relationship** |
| Product tool semantics quality | **PASS for lifecycle semantics**, with a non-blocking checkpoint schema/diagnostic weakness. |
| Model behavior | **FAILED_TO_COMPLETE_AFTER_ACCEPTANCE** |
| Token/context classification | **NOT_DETERMINABLE** |
| Tool-call reporting classification | **PRODUCT_AGGREGATION_REPORTING_DEFECT** |
| LIVE_2 raw verdict | **FAILED** |
| LIVE_2 adjudicated verdict | **FAILED** |
| Eligible for same-medium retry | **NO** — LIVE_2 is permanently frozen; no retry is authorized. |
| Recommended next action | Offline-only checkpoint contract/schema/error-feedback hardening before any separately authorized future provider sample. Do not retry LIVE_2. |

The raw and adjudicated results are both failed. The derived capability finding is a model failure: call 7 already supplied the exact passing check receipt, and the implementation rejects the empty `rejected` array before the qualified-check policy. That requirement was explicit in the public schema. Filling it and then calling Artifact submit remained possible within the budget. A separate receipt-ID/schema mismatch and generic error feedback are recorded as non-blocking product weaknesses; they do not make the delivery sequence materially ambiguous. No hidden chain-of-thought was inspected or inferred.

## Scope and source boundary

This report derives from the frozen LIVE_2 protocol, the pre-cleanup authoritative Mission/Task/WorkerSession/checkpoint/Artifact state, the terminal record, the public prompt and tool descriptions/schemas, and the implementation of the checkpoint/publication gates and read-model projection. It does not inspect provider reasoning records. The frozen raw run directory was not modified. The trace, public contracts, and token ledger below are derived evidence in this directory.

Primary evidence: [frozen protocol](../run/provider-evidence/f3a7f944f2749750becda51ad9d0e5bb/protocol.jsonl), [pre-cleanup authoritative state](../run/post-turn-pre-cancel-state.json), [terminal summary](../run/provider-terminal-summary.json), [raw result](../run/live-2-result.json), [provider-visible contracts](provider-visible-contracts.json), [derived call trace](tool-call-trace.json), and [token ledger](token-usage-breakdown.json). The pre-cleanup state is used for delivery adjudication; the later Cancel is cleanup evidence and does not alter the raw turn verdict.

## Phase A — Exact ordered 13-call trace

Every call occurred while the `compat` Task was `working`. Workspace revision stayed at 1 through call 3, changed to 2 on call 4, and stayed at 2 thereafter. “Used/remaining” is the tool budget immediately after the call.

| # | Tool / call ID | Argument category | Result category and visible feedback | Workspace revision | Task state | Used / remaining |
|---:|---|---|---|---:|---|---:|
| 1 | `work_current` / `exec-e2618108-94a7-4733-99f4-27eb30493ec9` | Parameterless read | Read: `compat`, owner `emp-backend`, Task `working`; binding `text-acceptance@1`; workspace rev 1; limit 16. Handover also exposed `Contract=signed-zero@1`. | 1 → 1 | working → working | 1 / 15 |
| 2 | `context_read` / `exec-2e22fdb1-8e5f-41e8-a1a7-5ed686850a32` | Parameterless read | Read the same Handover shape, TaskValidationBinding, workspace and budget; same legacy `signed-zero@1` field. | 1 → 1 | working → working | 2 / 14 |
| 3 | `workspace_read` / `exec-8a980b23-c779-4660-9070-95158a2ef860` | Parameterless read | Read current workspace: 148 bytes; zero of four acceptance strings present. | 1 → 1 | working → working | 3 / 13 |
| 4 | `workspace_replace` / `exec-9cd59e5f-5909-4bd7-aadc-65e52c046c59` | CAS workspace-content replace; expected rev 1 and digest; 399 bytes, all four required strings | Persisted; receipt/digest `562b5c53a66e9e5c8e326d8d5d13a55cbf56504ea70ac1b085b5a7a4efd805f2`; rev 2. **Last call before PASS.** | 1 → 2 | working → working | 4 / 12 |
| 5 | `workspace_check` / `exec-0b19e3ae-02ef-4c1f-95ca-90c700facb77` | Parameterless check | PASS, receipt `5d571cd58e6dc80707aa66171a1aff5f`. Exact visible feedback: “All public required-text criteria are present in the current Task workspace.” | 2 → 2 | working → working | 5 / **11** |
| 6 | `work_checkpoint` / `exec-51a2b197-68d5-42b7-8461-ebfbd4c79489` | Qualified checkpoint; arrays supplied, but `rejected` empty; two non-receipt prose refs | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 6 / 10 |
| 7 | `work_checkpoint` / `exec-317fe705-744b-436d-84bd-3d4e58cc9bbb` | Qualified checkpoint; exact PASS receipt ID supplied; `rejected` empty | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 7 / 9 |
| 8 | `work_current` / `exec-951db19d-b213-45ee-924a-12c0ebc124ec` | Parameterless read | Read confirms Task still `working`, rev 2, used 8, remaining 8; no checkpoint receipt. | 2 → 2 | working → working | 8 / 8 |
| 9 | `work_checkpoint` / `exec-cf8e649f-6ca9-4f5b-8ded-ada946678eea` | Progress checkpoint; `decisions`, `rejected`, and `evidence_refs` empty | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 9 / 7 |
| 10 | `work_checkpoint` / `exec-9512573b-e2d2-4932-b5c9-9776f03178ea` | Qualified checkpoint; JSON-schema-valid arrays, but evidence refs were prose/non-ID values | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 10 / 6 |
| 11 | `work_checkpoint` / `exec-c1ac7493-e4c4-4e43-a122-23df45fa3d04` | Qualified checkpoint; all required arrays empty | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 11 / 5 |
| 12 | `artifact_submit` / `exec-bc32d70a-222c-4577-9dbd-30bea2d8934c` | Parameterless submission of the current workspace | Denied; visible error `POLICY_DENIED`; no receipt. No current qualified checkpoint existed. | 2 → 2 | working → working | 12 / 4 |
| 13 | `work_checkpoint` / `exec-f069b8ae-35a8-441c-94e7-59daeba550b8` | Qualified checkpoint; PASS receipt plus workspace-replace receipt, but `rejected` empty | Rejected; visible error `MALFORMED_INPUT`. | 2 → 2 | working → working | 13 / **3** |

Calls after PASS were 6–13 (8 calls). The model had 11 calls remaining at PASS, enough for a valid qualified checkpoint and Artifact submission; three calls remained unused at terminal. The run produced one PASS product check, zero checkpoints, zero Artifacts, and zero Artifact qualifications. WorkerSession/protocol record 13 calls. There was one completed provider turn, one egress, no reconnect/resume evidence, and no runtime/transport interruption.

The final visible model message was: “Artifact persisted and validation passed at workspace revision 2. However, Polis rejected all checkpoint attempts as `MALFORMED_INPUT`, so submission was blocked by the checkpoint tool.” The Artifact-persisted claim is false against authoritative state; this report uses only that terminal user-facing message and does not inspect reasoning.

## Phase B — Public Task completion contract

The actual turn prompt was: “Complete the assigned product task using only the registered Polis tools. Persist the requested artifact and checkpoint, then stop.” The Mission goal was to create one small product integration smoke Artifact. The immutable TaskValidationBinding was revision `text-acceptance@1`, runner `text-contains-all@1`, digest `2bbaa0948f92f21cb0b7ba7c167bacdce61c91e5ee42821dad2177ba1e91a721`, with four required strings: Mission ID, Task ID, acknowledgement that the Artifact came through the Polis real-provider product path, and a one-sentence task summary.

| Question | Finding |
|---|---|
| Was workspace content required? | **Yes.** The Artifact request and the frozen four-string binding required content in the current Task workspace. |
| Was `workspace_check` PASS required? | **Operationally yes for qualification**, because the public checkpoint description requires a passing validation receipt. PASS itself is not one of the four text criteria. |
| Was a qualified checkpoint explicitly required? | **Yes across the public prompt and tool contract.** The prompt requires a checkpoint; the public Artifact description makes a current qualified checkpoint a prerequisite. |
| Was Artifact submission explicitly required? | **Yes semantically.** The prompt requires persisting the requested Artifact, and the only registered product publication tool submits the current workspace as a candidate Artifact. |
| Was a filename/type specified? | **No.** The submit tool accepts no arguments and submits the full current Task workspace as the candidate Artifact. No filename or extension was part of this task. |
| Was “check PASS is not Task completion” stated literally? | **No.** The tool semantics and state model make the distinction derivable: check validates workspace content; a separate submit publishes a candidate Artifact. The PASS response itself does not claim delivery. |

The `work_current` and `context_read` payloads did not include Mission title/goal, but did include the frozen TaskValidationBinding and task role. They also surfaced a legacy Handover `Contract=signed-zero@1` alongside the actual `text-acceptance@1` binding. This is a context inconsistency, not evidence that the acceptance criteria or requested delivery actions were unavailable.

## Phase C — Provider-visible tool semantics

On `polis-product-tool-surface@2`, the public descriptions state:

- `workspace_check`: validates the current authorized Task workspace against the public acceptance contract and returns structured validation outcomes.
- `work_checkpoint`: persists progress or qualified state; a qualified checkpoint requires a passing validation receipt for the current workspace revision and TaskValidationBinding.
- `artifact_submit`: submits the current authorized Task workspace as a candidate Artifact; submission requires a current qualified checkpoint and matching TaskValidationBinding; a Task without validation cannot be qualified or submitted.

These descriptions convey the sequence **check → qualified checkpoint → Artifact submission**. The checkpoint description alone does not call the qualified checkpoint a publication gate, but the Artifact description explicitly makes it mandatory, so the order is derivable. The Artifact tool says “candidate,” which indicates publication is not final acceptance.

## Phase D — Workspace-check PASS response

Exact PASS feedback: “All public required-text criteria are present in the current Task workspace.” It communicates **validation passed**. It does **not** say the Task was delivered, does not explicitly say that no checkpoint or Artifact exists, and does not name the next action. It is nevertheless scope-correct and not misleading: it reports content validation only. The public checkpoint and Artifact descriptions provide the remaining steps.

## Phase E — Checkpoint policy visibility

Formal runtime policy for a qualified product checkpoint requires a product-executable `compat` Task that is still `working`, an existing TaskValidationBinding, and a passing product check receipt bound to the same Task/Mission, worker session/epoch, binding digest, workspace digest/revision, contract and checker revision. `evidence_refs` must contain receipt IDs; every qualified reference must resolve to the current passed `workspace_check` receipt for that session/workspace. The runtime ID syntax is `^[a-zA-Z0-9_-]{1,80}$`. A progress checkpoint has different check-evidence treatment, but still runs the input receipt-ID validation. A progress checkpoint also needs a non-empty `next_action` at runtime.

Public exposure is partial. The description names the passing receipt, current workspace revision and TaskValidationBinding. The schema exposes required fields and array bounds, including `minItems: 1` for `facts`, `decisions`, `rejected`, and `evidence_refs`. But `evidence_refs` is only declared as a generic string array: no ID pattern or explicit rule says “use the exact current `workspace_check` receipt ID only.” The provider-visible tool error was only `MALFORMED_INPUT`; the internal reason/detail was not returned. Five of six checkpoint requests violated visible non-empty-array schema constraints; the remaining schema-valid attempt used prose/non-ID evidence refs and failed the additional receipt rule. **Classification: PARTIALLY_PUBLIC.**

The required non-empty `rejected` array is itself awkward for a checkpoint when nothing was rejected. It is enforced by the runtime before product qualification and was visible in the schema, so its repeated omission is a model-side schema failure. The contract would still benefit from an explicit empty/no-rejections state or a documented sentinel representation.

## Phase F — Artifact policy visibility

The runtime submits the full current workspace content and requires a `working` compatible product Task, a current qualified checkpoint, the matching TaskValidationBinding, and its current passed workspace-check qualification. On successful publication it creates an Artifact with candidate status and transitions the Task from `working` to `candidate`; independent/final acceptance remains separate.

The provider-visible description states the current workspace-to-candidate relationship, the current qualified-checkpoint gate, the matching binding, and the no-validation denial. “Candidate Artifact” signals that publication is not final acceptance. The key delivery prerequisite/order and artifact/workspace relationship were therefore public. **Classification: FULLY_PUBLIC for the lifecycle gate.** The tool description does not enumerate every DB/session guard, but those are implementation fences around the public prerequisite, not a competing delivery sequence.

## Phase G — Budget and opportunity

At PASS, 5 of 16 calls had been used and 11 remained. Eight later calls were made; three remained at terminal. A successful checkpoint plus Artifact submission required only two calls after PASS if valid arguments were supplied. **Delivery budget was sufficient; `DELIVERY_BUDGET_INSUFFICIENT_AFTER_ACCEPTANCE` does not apply.**

## Phase H — Validation versus delivery and read-model consistency

`workspace_check=PASS` proves that required text is present in the current workspace. It does not publish an Artifact or move the Task to `candidate`. Before cleanup, authoritative state showed the Mission `active`, `compat` Task `working`, one passing product check, no checkpoint, no Artifact and no Artifact qualification. The post-turn Cancel later changed Mission/Task states for cleanup; it did not make validation equivalent to delivery.

The Workbench projection/UI keeps Task state, checkpoint, Artifact verdict and final acceptance as distinct fields/indicators; no Artifact appeared in the pre-cleanup projection. The read model therefore preserves `workspace acceptance != Task delivery completion`. Sources: [read-store Task projection](../../../../../internal/workbench/read_store.go#L581-L597), [Workbench evidence boundary](../../../../../frontend/src/pages/CompanyOverviewPage.tsx#L315-L320), and [Task state summary](../../../../../frontend/src/pages/CompanyOverviewPage.tsx#L73-L76).

## Phase I — Token/context forensic

Recorded totals: input 162,336 (cached 124,416), output 2,475 (reasoning-output metric 1,053), last input 15,505, across 12 usage snapshots and a 258,400-token context window. Observable context included seven schemas totaling 1,470 bytes, a 148-byte initial workspace and 399-byte replacement, two repeated Handover results of about 1,274 characters each, and the accumulating tool/request/result history. The snapshots show input usage increasing over the turn. The frozen evidence does not attribute tokens among static instructions, schemas, results/history, workspace text, or provider/app-server overhead. **Classification: NOT_DETERMINABLE.** No component allocation or optimization claim is inferred; this does not alter the business verdict.

## Phase J — Terminal tool-call reporting

The terminal summary records 13 calls from WorkerSession/protocol, while its nested terminal usage object records `tool_calls: 0`; the reconnect counter is absent there. This is an adapter/reporting aggregation defect, not a provider usage-schema limitation: `TurnResult` has `ToolCalls` and `ReconnectAttemptCount`, but the Codex adapter populates the reconnect count and leaves `ToolCalls` at its zero value; the terminal aggregator persists `turn.ToolCalls` and omits reconnect count. Protocol evidence contains one `turn/start`, 13 tool-call events, one `turn/completed`, and no reconnect/resume markers. This reporting defect is independent of delivery and does not change either verdict.

Implementation references: [product tool descriptions/schema](../../../../../internal/codex/tools.go#L45-L47), [checkpoint input schema](../../../../../internal/codex/tools.go#L139-L152), [receipt ID syntax](../../../../../internal/core/validation.go#L42-L44), [checkpoint runtime gate](../../../../../internal/kernel/employee_ops.go#L204-L303), [evidence-ref validation](../../../../../internal/kernel/peer_interaction_contract.go#L125-L143), [Artifact publication and candidate transition](../../../../../internal/kernel/collaboration.go#L55-L165), [TurnResult](../../../../../internal/provider/runtime.go#L89-L96), [provider adapter](../../../../../internal/provider/codex_runtime.go#L384-L388), and [terminal aggregation](../../../../../internal/control/real_provider_worker.go#L163-L164).

## Causal ruling and minimum next hardening

The lifecycle intent and Artifact gate were public, validation feedback was truthful, there was ample budget, and infrastructure completed cleanly. Five of six checkpoint requests violated visible schema constraints. Call 7 contained the exact PASS receipt ID and otherwise had the required qualified-check evidence, but passed an empty `rejected` array; the schema and `EmployeeTools.Call` both require that array to be non-empty ([employee_ops.go:149-155](../../../../../internal/kernel/employee_ops.go#L149-L155)). The runtime returns generic `MALFORMED_INPUT`, but the unmet input requirement was public. The model then made three further checkpoint attempts with visible schema violations, attempted Artifact submission before a checkpoint existed, and ended with no successful checkpoint or Artifact.

One schema-valid request used prose/non-ID evidence refs, exposing a genuine mismatch between the generic-string schema and runtime receipt-ID checks. Because the exact valid PASS receipt had already been returned and was used correctly in call 7, the evidence does not show this mismatch prevented delivery. The unrelated legacy Contract value adds context noise but is not a demonstrated cause. The model failure therefore meets the stated conditions for **MODEL_FAILED_TO_COMPLETE_PUBLIC_DELIVERY_LIFECYCLE**; raw and adjudicated verdicts are both **FAILED**.

Recommended offline hardening before any new separately authorized sample:

1. Describe and encode `evidence_refs` as IDs of current passed `workspace_check` receipts; add the same ID constraint to the provider-visible schema and keep it aligned with runtime validation.
2. Return actionable structured checkpoint failures that identify the field and required correction (including malformed arrays, non-ID refs, stale refs, and non-check receipts).
3. Revisit the mandatory non-empty `rejected` array so “nothing rejected” has an explicit valid representation.
4. Remove or correctly populate the legacy `Contract=signed-zero@1` Handover field for this product Task context. Keep PASS feedback scoped to validation; adding a next-step cue is optional because the delivery order is already available through tool descriptions.

No hardening was implemented in this forensic task. LIVE_2 remains frozen; no provider traffic, retry, successor, or code/runtime/tool-policy change occurred.
