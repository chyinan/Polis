# R0.3A Pagination V3 Backend non-convergence forensics

This is a derived offline audit. It does not modify the frozen V3 Backend raw evidence, candidate, `r03a-pagination-contract@3`, or `r03a-pagination-behavior@1`.

## Subject and authoritative conclusion

- Raw run: `evidence/development/r0.3a-pagination-v3-real-backend-v2/`
- Session: `e96c09670af9b98c8fb14a236c8f340f`
- Parent ProblemKey: `r03a-real-peer-collaboration-v1`
- Result: `backend_real_execution = FAILED`
- Medium/provider: `1 / 1`
- Business budget: `48 used / 0 remaining`
- Transport: `turn/completed = true`, stop proof confirmed, reconnects `0`
- Contract proposal/accept: one proposal and one acceptance; no later proposal or acceptance
- Direct collaboration: Message/Obligation `e3d00e326a00607b737d044198d14bc0` bound to accepted revision `5c38ede92a02d6e2dee7dc8de09ee6c5`
- Progress checkpoint: `96dde5812333ba83492b9d5c86d96726` persisted; qualified checkpoint and artifact were not created

The causal classification is `MIXED`. The model demonstrably failed to converge within the authorized 48-call boundary, but the binding it was expected to discover was not stated in public ContractRevision@3 and the checker feedback did not expose the expected or observed public binding. This is not evidence for `ACTIONABLE_FEEDBACK_MODEL_NONCONVERGENCE` alone.

## Public entrypoint audit

ContractRevision@3 publicly defines HTTP request/response semantics: `cursor` is `string|null`, `limit` is an integer, the response contains structured `items` and required `next_cursor`, and a string cursor is propagated while null terminates pagination. It does not define a source-level function name, package, parameter signature, return type, import policy, or permitted declaration set.

The current checker/runtime implementation separately assumes a source-level entrypoint named `FetchItems` in package `backend`, generates calls with a concrete binding, and rejects imports or unsupported declarations. Those assumptions are not represented as a readable public ContractRevision field. The actual tool feedback only exposed `OUTCOME_UNKNOWN` plus one of three bounded details; it did not state expected binding or actual observed binding.

Therefore:

| Question | Finding |
| --- | --- |
| Is the public entrypoint explicit in ContractRevision@3? | `NO` |
| Does checker feedback state the expected public binding? | `NO` |
| Does checker feedback state the actual observed binding? | `NO` |
| Could the model deterministically select the required binding from public contract plus feedback? | `NO` |
| Was the model exploring alternatives rather than converging? | `YES` |

The model did have the initial workspace stub and could observe its own mutations, but that does not substitute for a public binding contract. The failure is consequently both a model non-convergence observation and a contract/feedback quality finding.

## Workspace-check timeline

Failed checks had no returned/persisted check receipt; the preceding successful `workspace_replace` receipt/digest is the authoritative workspace state identity. Each result below was visible to the model.

| Tool ordinal | Request identity | Workspace revision / digest | Candidate binding | Public code / visible feedback | Model next action | Next mutation / result |
| ---: | --- | ---: | --- | --- | --- | --- |
| 7 | `exec-043a9008-3093-42c0-890d-1dee78589d66` | 2 / `d9963f52...` | `FetchItems(request ItemsRequest) (ItemsResponse, error)` | `OUTCOME_UNKNOWN`: only declared package without imports | `workspace_read` | rev3, same typed binding with `candidateError`; no confirmed resolution |
| 10 | `exec-ac5f48a6-4902-455d-8c73-a0ca463eb59c` | 3 / `5edce039...` | same typed binding | `OUTCOME_UNKNOWN`: unsupported declaration | `workspace_read` | rev4, typed binding returning nil error; no confirmed resolution |
| 13 | `exec-0f49f689-bd2c-4604-bb14-2cc2913ebf3f` | 4 / `10e866d3...` | same typed binding | `OUTCOME_UNKNOWN`: unsupported declaration | `workspace_read` | rev5, `(*string,int) → map[string]interface{}` |
| 16 | `exec-9b1915f6-6e4d-473c-820e-2b88756df540` | 5 / `c462cf68...` | `(*string,int) → map[string]interface{}` | `OUTCOME_UNKNOWN`: function binding mismatch | `workspace_read` | rev6, adds error return |
| 19 | `exec-cdb132fe-806d-4dfa-8253-158c639395b2` | 6 / `7bfca873...` | `(*string,int) → (map[string]interface{},error)` | same binding mismatch | `workspace_read` | rev7, changes cursor to `string` |
| 22 | `exec-e453034d-7827-4830-b553-58a057992b19` | 7 / `b9a9a4d2...` | `(string,int) → map[string]interface{}` | same binding mismatch | `workspace_read` | rev8, returns `([]map[string]string,*string)` |
| 25 | `exec-0106d535-15f9-476f-971d-eb43d89f5408` | 8 / `6950d407...` | `(*string,int) → ([]map[string]string,*string)` | same binding mismatch | `workspace_read` | rev9, `(string,int) → (map[string]interface{},error)` |
| 28 | `exec-c4aeb986-88d1-448a-a0e6-dab16d131005` | 9 / `90e0fb26...` | `(string,int) → (map[string]interface{},error)` | same binding mismatch | `workspace_read` | rev10, map request input |
| 31 | `exec-e89fcaa2-2c17-4d74-9d37-9b5f713484b6` | 10 / `ff90942b...` | `FetchItems(request map[string]interface{}) map[string]interface{}` | same binding mismatch | `workspace_read` | rev11, named slice/string-pointer return |
| 34 | `exec-127bda91-3cad-4b6a-b1fe-02d8749f960d` | 11 / `0ebff5c6...` | `(*string,int) → (items []map[string]string,nextCursor *string)` | same binding mismatch | `workspace_replace` | rev12, `interface{}` cursor |
| 36 | `exec-25ba2c0c-bc15-4a78-8613-4f9c51c10e24` | 12 / `9532f1dc...` | `(interface{},int) → map[string]interface{}` | same binding mismatch | `workspace_replace` | rev13, `(*string,int) → string` |
| 38 | `exec-25e29bd5-2d58-458a-b8b4-c97089a0ca90` | 13 / `f8c35166...` | `(*string,int) → string` | same binding mismatch | `workspace_replace` | rev14, adds `FetchPage` and leaves `FetchItems()` |
| 40 | `exec-ddcd5a9d-52f2-4f82-9cd8-e3b803989846` | 14 / `5ef5e13e...` | `FetchItems() string` | same binding mismatch | `workspace_replace` | rev15, `(string,int) → string` |
| 42 | `exec-61e4d496-c5b2-4239-bf72-50f43919e928` | 15 / `70e117df...` | `(string,int) → string` | same binding mismatch | `workspace_replace` | rev16, `(*string,int) → (string,error)` |
| 44 | `exec-2282bbc2-afcf-48f5-a877-63a68c430f54` | 16 / `7763acc8...` | `(*string,int) → (string,error)` | same binding mismatch | `work_checkpoint` | no further mutation; budget then used by checkpoint attempts |

The digest suffixes above are abbreviated only for readability; the full values are in the frozen protocol/receipt evidence. Every mutation after the first failure changed the candidate declaration or signature, but no subsequent check passed. The first three iterations partially changed imports/declarations; the remaining iterations were alternative-binding exploration without a public expected signature to target.

## Budget distribution

| Category | Count | Formal tools |
| --- | ---: | --- |
| Reads | 11 | `work_current` 1, `workspace_read` 10 |
| Workspace writes | 15 | `workspace_replace` 15 |
| Workspace checks | 15 | `workspace_check` 15, all failed |
| Contract proposal/accept | 2 | `contract_propose` 1, `contract_accept` 1 |
| Collaboration | 1 | direct `collab_send` 1 |
| Checkpoints | 4 | `work_checkpoint` 4; 3 rejected, 1 progress persisted |
| Other | 0 | none |
| Total | **48** | exact business limit |

There was no contract churn: the single accepted revision remained rev2 throughout. The dominant loop was workspace mutation → check failure → read or another mutation. Once the model had exhausted feedback-directed alternatives, it attempted progress checkpointing rather than artifact submission. Artifact submission was never attempted.

## Token and payload attribution

From the frozen native usage record:

- total: `1,134,782`
- input: `1,122,406`
- cached input: `1,058,816`
- uncached input: `63,590`
- output: `12,376`
- reasoning: `4,252`
- native usage updates: `49`

From the frozen protocol JSONL (`437,404` bytes):

- completed dynamic-tool result text: `21,482` bytes
- workspace-read/check result text: `17,116` bytes
- workspace-replace source payloads: `15,729` bytes
- repeated workspace-read content bytes: `0` by exact-content comparison; each read followed a changed workspace
- repeated dynamic-tool result text: `1,804` bytes by exact-content comparison

The protocol log is not the provider's complete serialized prompt/context. The evidence proves a large accumulated input and a high cached fraction, but does not expose a reliable byte-level split between repeated conversation serialization, tool-result replay, and other app-server context. No stronger attribution is claimed, and no prompt/context optimization was performed.

## Final classification

| Dimension | Finding |
| --- | --- |
| transport/runtime | `PASS` |
| authorization and allowance | `PASS` for the authorized binding |
| PG/migration/runtime connection | `PASS` before the turn |
| public contract clarity for source binding | `FAIL / ambiguous` |
| checker feedback quality | `FAIL / too coarse for binding convergence` |
| model behavior | `FAILED_TO_CONVERGE` within 48 calls |
| contract churn | `NONE` |
| artifact/checkpoint finalization | `FAIL`; progress only |
| final classification | `MIXED` |

## Eligibility and proposal

```ini
model_behavior = FAILED_TO_CONVERGE
checker_quality = NOT_PASS_FOR_PUBLIC_BINDING
eligible_for_contract_hardening = YES
eligible_for_new_model_run = NO
```

Pure-local hardening proposal only; it was not implemented in this audit:

1. Add a public, source-level entrypoint/binding section to a future contract revision or fixture API that does not disclose hidden verifier internals.
2. Make checker output distinguish public binding/schema rejection from behavioral failure, and report bounded expected/actual public binding details.
3. Preserve the 48-call business limit as an explicit experimental boundary; do not increase it for this failure.
4. Require a fresh business run after qualification if a revised contract/checker becomes provider-visible. Do not synthesize a new starting state from this cleaned disposable DB.

No model, provider, Backend retry, Frontend, High, candidate repair, or historical evidence rewrite occurred during this audit.
