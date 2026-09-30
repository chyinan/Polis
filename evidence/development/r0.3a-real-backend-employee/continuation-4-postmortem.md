# R0.3A continuation-4 Backend failure postmortem

> This is an additive forensic finding. The raw continuation-4 evidence is unchanged.

## Final classification

- `backend_real_execution`: `FAILED`
- `transport/runtime`: PASS for the Windows-native process/envelope and safe stop; the turn had no provider error/reconnect, but no `turn/completed` was observed.
- `authorization`: PASS; exact fingerprint, auth identity/revision, employee, ProblemKey, purpose, model/profile/effort and limits were accepted.
- `PG/blob/workspace infrastructure`: PASS for runtime and persisted workspace writes. The checker rejected content by its business predicate; no filesystem or DB I/O error occurred during the turn.
- `contract collaboration`: PARTIAL/FAIL. Revision 2 was accepted and read back as accepted; a later message used Revision 3, then Revision 4 was accepted without a replacement message.
- `candidate implementation`: FAIL. Every completed backend `workspace_check` returned `passed=false`; no candidate artifact was frozen.
- `checkpoint path`: `INSUFFICIENT_EVIDENCE`.
- `artifact path`: `EXPECTED_POLICY_DENIAL` because no checkpoint existed.
- `model behavior`: failed to converge and continued changing the contract/workspace after non-actionable failed-check feedback.
- `tool feedback quality`: insufficiently actionable; the formal result exposed only `passed=false` and `peer candidate checks failed`, not the missing `contract-v2` predicate.
- `budget boundary`: questionable implementation default; `32` was enforced in `transport_turn.go` but was not present in the allowance/tool manifest or model-visible contract.

## Authoritative run facts

- Run: `evidence/development/r0.3a-real-backend-employee/continuation-4/`
- Session: `8bfd303beafe6fefd25337a2bd1ef77d`
- Employee: `emp-backend`
- ProblemKey: `r03a-real-peer-collaboration-v1`
- Execution fingerprint: `f3f4e0f8dc64ed373bea7716237ad14932cf98c887acead7e6b5d193216b90af`
- Allowance: one created and consumed Medium reservation; High remained zero.
- Provider egress: one.
- Session elapsed: `261723 ms`; stop proof: `windows-process-handle:14636:waited`.
- Native usage updates: `29`; last observed total `465971`, input `461018`, cached input `422912`, output `4953`, reasoning `2383`.
- Protocol: `454` JSONL lines, `211232` bytes; `33` tool-call requests, `32` processed tool results, and the 33rd request had no tool result because the limit fired first.
- No additional model call, Frontend, successor, or Reviewer started.

## Reconstructed tool timeline

`call_id` is the native request identity. Digests are the relevant content/schema/body digests when the request supplied content. Workspace revisions are marked observed only when returned by a read; successful CAS writes otherwise provide only a receipt/content digest, not a post-write revision.

| Ord | Tool / call_id | Elapsed ms | Relevant input | Result / receipt or state | Workspace / contract observation |
|---:|---|---:|---|---|---|
| 1 | `work_current` / `exec-09a8f052-b63a-4b5a-825d-6cdb9980d6f7` | 6.9 | — | success | workspace rev 1; v1 accepted |
| 2 | `context_read` / `exec-d0c16f39-ab79-42fa-813e-511ea64dd967` | 4.2 | — | success | same task/context |
| 3 | `workspace_read` / `exec-e9cf37fc-0601-4510-a384-8c197c91923d` | 3.7 | — | success | rev 1, digest `d8a78697…d89efa` |
| 4 | `contract_propose` / `exec-d60f1a53-072e-47e2-87a0-2fe24b9df3c5` | 11.7 | schema `30950a26…274cdb` | proposed `01a07db5…303b84b0`, rev 2/base 1 | v2 proposal persisted |
| 5 | `contract_accept` / `exec-9af381ba-839c-4fbe-bbd2-6dc14232571e` | 6.8 | revision `01a07db5…303b84b0` | success | later read confirms v2 accepted |
| 6 | `workspace_replace` / `exec-fce42427-eab1-46dd-bece-f1c0f8c22f89` | 11.4 | expected `d8a78697…d89efa`; content `2a35bb75…f78b9` | persisted receipt `2a35bb75…f78b9` | rev after not returned |
| 7 | `workspace_check` / `exec-31877c32-5a3f-445f-9e8a-771e537192ee` | 10.4 | workspace digest `2a35bb75…f78b9` | persisted check `65817b9f…a67a`; `passed=false` | next read (ord 8) observes rev 2 |
| 8 | `workspace_read` / `exec-62ee12fd-9e8e-42d5-8fa5-2ec9827bf934` | 4.3 | — | success | rev 2, digest `2a35bb75…f78b9` |
| 9 | `contract_read` / `exec-06fa2f43-34d5-425b-a8ed-c01823b977d6` | 3.7 | v2 id | accepted, rev 2 | authoritative v2 acceptance readback |
| 10 | `workspace_replace` / `exec-b7b92a56-5f9c-4660-8620-3fee25c56857` | 10.6 | expected `2a35bb75…f78b9`; content `cde9d969…d5db` | persisted receipt `cde9d969…d5db` | rev after not returned |
| 11 | `workspace_check` / `exec-443cca74-51c9-4ced-bb86-c6de3a26ce02` | 9.5 | digest `cde9d969…d5db` | persisted `fdc67faf…8113`; `passed=false` | next action ord 12 replace |
| 12 | `workspace_replace` / `exec-342c2760-f993-4f07-8462-35afb99d8f31` | 10.7 | expected `cde9d969…d5db`; content `837a6e49…792e` | persisted receipt `837a6e49…792e` | no `contract-v2` marker |
| 13 | `workspace_check` / `exec-4f7ba8eb-088c-4237-82a0-bfe7fd4b0a5d` | 9.5 | digest `837a6e49…792e` | persisted `b5cd3a0c…e0`; `passed=false` | next action ord 14 replace |
| 14 | `workspace_replace` / `exec-e662800e-73ba-47cc-aca7-7e4e06c38b1a` | 10.1 | expected `837a6e49…792e`; content `5a4a7440…f93c` | persisted receipt `5a4a7440…f93c` | no `contract-v2` marker |
| 15 | `workspace_check` / `exec-47d729a3-8b4b-4989-86e0-90148080d36a` | 9.4 | digest `5a4a7440…f93c` | persisted `216f19a5…56c4`; `passed=false` | next action ord 16 replace |
| 16 | `workspace_replace` / `exec-7968e9d3-f87b-4d85-9cc4-9b59af4eb12f` | 10.0 | expected `5a4a7440…f93c`; content `5703eb4e…7799` | persisted receipt `5703eb4e…7799` | no `contract-v2` marker |
| 17 | `workspace_check` / `exec-032cf877-65ae-4e67-a418-c7e3e1228fd5` | 9.6 | digest `5703eb4e…7799` | persisted `cd8f5457…87f2a`; `passed=false` | next action ord 18 replace |
| 18 | `workspace_replace` / `exec-9759b0cc-c383-4f5b-9e1e-9d97063aabfb` | 13.0 | expected `5703eb4e…7799`; content `c962c7d1…f2c` | persisted receipt `c962c7d1…f2c` | no `contract-v2` marker |
| 19 | `workspace_check` / `exec-0fd72a9d-8b9c-4dc9-b6cd-14d9ad44de35` | 9.6 | digest `c962c7d1…f2c` | persisted `e051f164…5d32`; `passed=false` | next action ord 20 proposes rev 3 |
| 20 | `contract_propose` / `exec-57bbc00a-b461-45b5-8e9c-41ce49a6184d` | 9.6 | schema `c3fa2f30…0d4e` | proposed `0580220b…69d1`, rev 3/base 2 | contract churn, not checker repair |
| 21 | `contract_accept` / `exec-001e3a0d-901f-439f-a1c9-175e95549b6c` | 6.4 | rev 3 id | success | no later readback |
| 22 | `workspace_replace` / `exec-be86a9c4-60eb-4895-838b-b6f78ff1d020` | 10.6 | expected `c962c7d1…f2c`; content `c4ca823a…238e` | persisted receipt `c4ca823a…238e` | ord 27 later observes rev 8 |
| 23 | `workspace_check` / `exec-9ac7f5f8-1d60-4c50-aa59-6ed04b69ec4d` | 8.9 | digest `c4ca823a…238e` | persisted `b94df7b0…8879`; `passed=false` | checkpoint evidence was this failed check |
| 24 | `work_checkpoint` / `exec-66a83603-aaa4-41ae-85c8-1dbd06c0b43f` | 9.0 | evidence `b94df7b0…8879` | `POLICY_DENIED` | not malformed; failed evidence predicate |
| 25 | `collab_send` / `exec-02e164f2-a50a-46da-bfab-a0e17059de32` | 13.4 | to task `5abe8610…c040`; rev 3; actionable; body `4de46a93…8ae2` | Message `d16b5843…9246`, Obligation same id, `delivery_state=persisted` | direct to `emp-frontend`; logical PG side effect |
| 26 | `artifact_submit` / `exec-e1537bc2-9aa7-48b0-80ce-b956d71885a8` | 8.0 | — | `POLICY_DENIED` | no checkpoint existed |
| 27 | `work_current` / `exec-322230ac-c3d3-4e2a-b8a9-a57469e443fa` | 5.2 | — | success | rev 8, digest `c4ca823a…238e`; v1 superseded |
| 28 | `workspace_replace` / `exec-73f8426d-0ed5-4c9a-b4aa-d3b4b93bff05` | 10.0 | expected `c4ca823a…238e`; content `212cb8fa…56c4` | persisted receipt `212cb8fa…56c4` | rev after not returned |
| 29 | `workspace_check` / `exec-d3b8df20-d380-49a2-8485-89d3334a1e17` | 9.7 | digest `212cb8fa…56c4` | persisted `ece7d7c1…a15`; `passed=false` | next action ord 30 proposes rev 4 |
| 30 | `contract_propose` / `exec-d7830459-d6a6-484e-8b84-d46d209789b1` | 10.1 | schema `63a92946…586d` | proposed `d56e82de…e42c`, rev 4/base 3 | later acceptance changes effective contract |
| 31 | `contract_accept` / `exec-003eace1-9885-4b8a-88c5-fb3f404d2fad` | 8.0 | rev 4 id | success | no final readback |
| 32 | `workspace_replace` / `exec-594b62de-27c7-4beb-acf1-a102dd6db1e1` | 10.1 | expected `212cb8fa…56c4`; content `1aaa0c13…aad2` | persisted receipt `1aaa0c13…aad2` | final post-write revision not read |
| 33 | `workspace_check` / `exec-7958b490-2fae-45b0-985d-aa1a9041d1bf` | not returned | — | no tool result; `tool-call limit exceeded` fired before handler | no business side effect from this request confirmed |

## Workspace-check analysis

The eight completed checks were global ordinals 7, 11, 13, 15, 17, 19, 23 and 29. Each formal result was the same: `passed=false`, output `peer candidate checks failed`, and a persisted `worker_checks` receipt. The current kernel predicate for a `peer_backend` is `containsPeer(workspace.Content, "contract-v2")`; the returned content digests and protocol arguments show that none of the nine submitted workspace contents contained that marker. The final ninth check was never executed because the 33rd call hit the limit.

The missing-marker reason was not visible to the model. It saw a boolean and generic output, then read/replaced the workspace repeatedly. The next actions were not exact repeats, but none inserted the required marker; two later actions proposed new contract revisions rather than repairing the failed candidate. This is an invalid repair/coordination loop driven by opaque feedback, not a transport retry.

| Check | Global ord | Workspace revision | Formal result | Next model action | Addressed the actual predicate? |
|---:|---:|---|---|---|---|
| 1 | 7 | rev 2 confirmed by ord 8 | persisted, `passed=false`, receipt `65817b9f…a67a` | ord 8 `workspace_read` | No marker |
| 2 | 11 | post-write revision not directly read | persisted, `passed=false`, `fdc67faf…8113` | ord 12 `workspace_replace` | No marker |
| 3 | 13 | post-write revision not directly read | persisted, `passed=false`, `b5cd3a0c…e0` | ord 14 `workspace_replace` | No marker |
| 4 | 15 | post-write revision not directly read | persisted, `passed=false`, `216f19a5…56c4` | ord 16 `workspace_replace` | No marker |
| 5 | 17 | post-write revision not directly read | persisted, `passed=false`, `cd8f5457…87f2a` | ord 18 `workspace_replace` | No marker |
| 6 | 19 | rev 8 later confirmed at ord 27 after ord 22 | persisted, `passed=false`, `e051f164…5d32` | ord 20 `contract_propose` | No marker; changed contract |
| 7 | 23 | rev 8 observed at ord 27 | persisted, `passed=false`, `b94df7b0…8879` | ord 24 `work_checkpoint` | No; checkpoint denied |
| 8 | 29 | post-write revision not directly read | persisted, `passed=false`, `ece7d7c1…a15` | ord 30 `contract_propose` | No marker; changed contract |
| 9 | 33 | not executed | no result | none | unknown; limit stopped dispatch |

## Checkpoint audit

The `work_checkpoint` request had the required summary/facts/decisions/rejected/evidence fields, so this was not `MALFORMED_INPUT`. `TXCheckpoint` reads the current workspace digest and requires every evidence id to belong to the same session/current digest and to a `worker_checks` row with `passed=true`. Evidence receipt `b94df7b0…8879` existed for the current digest but had `passed=false`, so the kernel returned `core.Denied`, serialized as `POLICY_DENIED`.

Classification: `INSUFFICIENT_EVIDENCE`. This predicate does not require the whole task to be complete; it requires a successful candidate check. Draft C-EMPLOYEE-OPS describes checkpoint as structured progress with completed evidence, while the employee tool description explicitly says it uses successful peer-check evidence. Therefore the denial is consistent with the current contract, but exposes a potential usability gap: there is no separate failure/partial checkpoint state for recording a known failed candidate without implying acceptance.

## Artifact audit

`artifact_submit` was a distinct `POLICY_DENIED`. The Backend session was non-initial, but the kernel first loaded the handover and rejected `len(h.Checkpoints)==0`. Because the checkpoint was denied, artifact submission had no eligible checkpoint evidence. Classification: `EXPECTED_POLICY_DENIAL`; this is not the same predicate as the checkpoint denial and does not show an artifact-store failure.

## Contract / message / obligation audit

1. Initial v1 was observed accepted.
2. `contract_propose` ord 4 created `01a07db5d1d815e1366689fc303b84b0`, revision 2/base 1. Ord 5 accepted it; ord 9 read it back as `state=accepted`, revision 2.
3. Ord 20 proposed revision 3, id `0580220bd9831aeb8b41d2c1268d69d1`; ord 21 acceptance returned success but was not read back.
4. Ord 25 `collab_send` referenced revision 3, directly targeted the frontend task, and returned Message `d16b5843f57f1680dea15335ca39d246`, same-id actionable Obligation, and `delivery_state=persisted`. At creation, the obligation path inserts state `pending`; final DB state is `unknown_after_cleanup`.
5. Ord 30 proposed revision 4, id `d56e82def48cbba4d8624f832d79e42c`; ord 31 acceptance returned success but was not read back. Since acceptance supersedes the prior accepted revision in the kernel, the message is potentially stale relative to revision 4. No replacement `collab_send` occurred. A future frontend wake would consume a message referencing revision 3 while the latest accepted revision may be 4.

The message/obligation are confirmed logical business side effects from the tool result, not external-world side effects. The disposable DB was dropped, so final message/obligation/contract rows are not reconstructed or claimed.

## Tool-call budget audit

The limit is a local per-`Client.Turn` counter at `internal/codex/transport_turn.go:183-187`: increment, then reject when `calls > 32`. It is not in the T21B tool manifest, continuation allowance, or model-visible developer contract; the model could not know the remaining count in advance. It is local to this turn, not shared with diagnostic calls. The 33rd request was observed in protocol but did not reach the Polis handler.

Classification: `budget_exhaustion=implementation_boundary`, `budget_boundary_quality=questionable`. This was not an allowance expansion or retry, and the limit must not be raised automatically. The business failure was already established by repeated failed checks; the hidden cap additionally prevented final reconciliation.

## Token/context audit

The protocol proves cumulative context growth rather than one isolated oversized workspace read: 33 tool-call requests, 170 agent-message deltas, 59 item-started events, 58 item-completed events, 29 usage updates, 29 rate-limit updates, and repeated dynamic-tool arguments/results. `workspace_read` returned full workspace content twice; every `workspace_replace` content appeared in request/event/result records; every check returned a receipt and failed report. The app-server conversation therefore accumulated prior tool calls/results and workspace snapshots. The raw protocol contains 29219 bytes of workspace-replace-related records and 211232 bytes overall, while the last usage reports 461018 input tokens and 422912 cached input tokens. Exact token attribution among provider serialization, cached context and event duplication is not recoverable from this evidence.

No prompt/context optimization or allowance change was performed.

## Eligibility for a revised backend run

`eligible_for_revised_backend_run = NO` at this point.

The failure is understood well enough to identify required local decisions/fixes, but none should be applied silently or followed by another model call:

- expose a structured/actionable `workspace_check` failure reason for the missing `contract-v2` predicate;
- decide whether the contract needs a distinct failure/partial checkpoint state, without weakening acceptance evidence;
- define and pre-register any per-turn tool-call budget instead of relying on the hidden `32` default;
- prevent or explicitly reconcile contract revision changes after a message has referenced an earlier effective revision.

The next run would only be justified after those decisions are separately authorized, while keeping the same ProblemKey, Windows-native qualified stack, one Medium/no High/no retry, and no automatic Frontend phase. This postmortem does not authorize a new model call and does not rewrite `backend_real_execution=FAILED`.
