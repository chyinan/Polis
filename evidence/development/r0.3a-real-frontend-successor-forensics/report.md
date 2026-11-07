# R0.3A revised Frontend successor forensic report

This derived report audits the frozen revised Frontend run. It does not
rewrite its raw `result.json`, session records, or any historical evidence.
Source inputs are the run's two `protocol.jsonl` files, session records,
handover bundles, and old-writer rejection receipt.

## Frozen classification

```ini
frontend_initial = PASSED_TO_HANDOVER_BOUNDARY
successor_started = true
successor_business_completion = FAILED
real_frontend_handover = FAILED
real_peer_collaboration = FAILED
High = 0
artifact_state = NOT_CREATED
```

The raw result remains `status=inconclusive` with its original zeroed
counters. The causal result above is derived from the authoritative session
receipts. Both Medium sessions reached provider turn start, completed, and
were stopped with confirmed Windows process proofs. No third turn was run.

## Successor authoritative timeline

Input digests are SHA-256 of compact JSON tool arguments reconstructed from the
frozen protocol. `returned` means a response without a write receipt. The
workspace column is persisted revision before→after; unchanged reads are
shown explicitly.

| # | tool | request identity | input digest | result / receipt | workspace |
|---:|---|---|---|---|---:|
| 1 | `work_current` | `exec-929b5df9-08f2-401b-ba4a-c8621f40eaf5` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 2→2 |
| 2 | `collab_inbox` | `exec-b370291c-f034-47c2-abd7-9c7389fafa03` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 2→2 |
| 3 | `workspace_read` | `exec-0f7540bb-8b7b-471c-a397-c07d32f6968b` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 2→2 |
| 4 | `contract_read` | `exec-155c240d-8663-4bc5-8d4f-069b76959af1` | `530104af826e2551e790ffddc62c75c076b5e673611ec0fb9102b8fd17d2c5b0` | returned | 2→2 |
| 5 | `workspace_replace` | `exec-878e0930-d9c8-4501-9c5a-54ba2b81bb65` | `0e40971b738615ce83600dbe203a998411cfb6f8fbf267acad524a0e1fa4fc37` | persisted / `fd84e8c0b3e5d11e384241b3ade01fba9ac16804f0d2811f563f53204a3fe775` | 2→3 |
| 6 | `workspace_check` | `exec-7bd5b714-ee95-4455-bb0f-e76b86c8c02a` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `7c740df1ddef49c0ecedcd358ad313ae` | 3→3 |
| 7 | `workspace_read` | `exec-51f363ff-822e-4c90-bc7b-b91e4ca1028e` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 3→3 |
| 8 | `workspace_replace` | `exec-837034f2-dd57-4ea2-8ecf-c92456227814` | `a1ca3d9ed2b746f97aca1e9674e57da820ffe2e7150be4aa972335ae58b7d348` | persisted / `59a3934a1ad9c26fd03e9a9189b7b42a01058866a4e856a54b797fa2acd6383e` | 3→4 |
| 9 | `workspace_check` | `exec-b1bfaaa6-46cb-472c-8c8f-8ddcb9583bee` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `70d628872f550a21181fc0a1f45d8ad9` | 4→4 |
| 10 | `context_read` | `exec-6e97c3de-7b29-42c9-9199-4b0277a76ad6` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 4→4 |
| 11 | `workspace_replace` | `exec-c2938813-8180-4c5e-a7d4-40354ef46077` | `fe98c5e9a0a015421f2250a6228ea3171c185be13ff5152b6d116fc440eb7b64` | persisted / `19e90cd5d547181a0ff37fab9005beaabc92d21fab92fd17fd11ae0b3854c48c` | 4→5 |
| 12 | `workspace_check` | `exec-27882673-ec2c-42d5-a9a4-6f7776e3cecf` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `7a6039804c4b870e7d28f44209593f3c` | 5→5 |
| 13 | `collab_inbox` | `exec-08ce5ea7-a0a6-4c4c-aa93-5c97000c578e` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | returned | 5→5 |
| 14 | `workspace_replace` | `exec-6724b4f1-c06e-40cf-8ffc-1a195b188227` | `4c42f9e4c302ec0b567c7a2f01ce67563327395d4e88d465edeff3d725e3d4fc` | persisted / `fb6b7239ebca3def4bdadf950629deeaa267801f6147a0b62a8417c609a6b2f3` | 5→6 |
| 15 | `workspace_check` | `exec-3e073636-d7a1-4905-ba45-c3ac5d14f243` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `6669900120eaa5290ee49aeb47b4bf4f` | 6→6 |
| 16 | `workspace_replace` | `exec-eeef82a2-028e-4570-9944-c0b8a9869a3e` | `ac2f28063dbc8c8a54158e9d19346fcb27ef2fd9b09746c54f29e9b413d18249` | persisted / `c996bd1c50793360a5844f72bd09e72325d7f4b8c6c313588bcf206faaadd5d2` | 6→7 |
| 17 | `workspace_check` | `exec-33ae5743-510f-498a-ba18-17dd76115bac` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `8dafd5d1ca43bff937095ec43b8bd767` | 7→7 |
| 18 | `workspace_replace` | `exec-7637a81d-268d-4849-b039-168e7ac5d309` | `6ea308bee2e476c1b61a5ee473b08e4918745c8a2e9629db332b72f9fdc96fa1` | persisted / `d981c83635b3aaff440d8d1f3fc254243a9ba550aacebdb5a00ad1fb1ef52f37` | 7→8 |
| 19 | `workspace_check` | `exec-f05e99c4-dfe6-44b6-bbef-7ad46c5eec59` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `5bfe7049723132a28be3a9d85e15e920` | 8→8 |
| 20 | `workspace_replace` | `exec-8f9ae147-b2c9-4d8f-9e9e-4b5ba6a6b7b6` | `4df6c1b790a9ca802f1d764eb771569595bb8cf4aacdbea38306ad019232bed4` | persisted / `0f56bf5d8b498c3cdeb8eb91709ed519d914389cef9e9ebaf945e0a4cda92098` | 8→9 |
| 21 | `workspace_check` | `exec-744eaa38-5889-4050-81d1-0aa3b89fd149` | `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a` | failed / `685bbf33b9aeb84001c8336ec929b278` | 9→9 |
| 22 | `work_checkpoint` | `exec-d68155ca-96a1-4506-a43c-1dcfc67a66b2` | `3f00f909a05c62edf4445a57bc6c75757faec6e2c59dd8eab9cb1f4a35b9c6fe` | `POLICY_DENIED`, no receipt | 9→9 |
| 23 | `work_checkpoint` | `exec-438ea2e0-6032-49f1-9e38-0086b60d16d3` | `6aaf3d6cedc114771220667be351a6a7e6e23dff5672ad669c6cc847371e47dc` | persisted / `d375968b57e6c23cf899f977b262fe55` | 9→9 |

The initial session had 10 tool events and advanced workspace revision 1→2.
Its mutation, apply, check and progress receipts were
`aae3482ff7442125965ac24a884b1b5527a0d7a880f6d0c33d3f7b4c5e42a9e1`,
`7d027b5a8fd2267a68b364e47f4635ca`,
`f28c1113240ca9f73b67832fe83ea2ba`, and
`ac7919e43360fbe4395490751840ec4e`.

No contract proposal or acceptance occurred in the successor. The current
contract was rev3 (`4f57c71061c10ab5a78f632d8db59321`) before and after every
successor call; it is the contract referenced by all seven checker receipts.
The reconstructed per-call elapsed times, in ordinal order, were
`19,23,18,18,23,26,17,30,22,23,23,182,18,24,22,27,24,24,21,23,21,17,26 ms`.

## Workspace-check analysis

All seven checks returned the same public structure:

```yaml
acceptance_checker_revision: peer-semantic-checker@2
passed: false
criteria:
  - criterion_id: candidate_syntax
    public_reason_code: candidate_syntax_invalid
    actionable_summary: provide a compilable frontend candidate with the response consumer function
  - criterion_id: response_shape_compatibility
    public_reason_code: response_shape_mismatch
    actionable_summary: consume all fields required by the accepted contract response schema
relevant_contract_revision: 4f57c71061c10ab5a78f632d8db59321
relevant_workspace_revision: 3..9
```

| check | revision | receipt | next model action | addressed the failure? |
|---:|---:|---|---|---|
| 1 | 3 | `7c740df1ddef49c0ecedcd358ad313ae` | `workspace_read` | no, inspection only |
| 2 | 4 | `70d628872f550a21181fc0a1f45d8ad9` | `context_read` | no criterion-specific correction shown |
| 3 | 5 | `7a6039804c4b870e7d28f44209593f3c` | `collab_inbox` | no, repeated context exploration |
| 4 | 6 | `6669900120eaa5290ee49aeb47b4bf4f` | `workspace_replace` | new candidate attempted; result unchanged |
| 5 | 7 | `8dafd5d1ca43bff937095ec43b8bd767` | `workspace_replace` | new candidate attempted; result unchanged |
| 6 | 8 | `5bfe7049723132a28be3a9d85e15e920` | `workspace_replace` | new candidate attempted; result unchanged |
| 7 | 9 | `685bbf33b9aeb84001c8336ec929b278` | `work_checkpoint` | recorded failure; no qualified state |

The model saw the failure category, current ContractRevision and current
workspace revision. It did not see a syntax location, field-level expected
response shape, or actual-vs-expected mismatch. The evidence therefore
classifies this as **MIXED / FEEDBACK_TOO_COARSE**: it was structured and
directional, but not diagnostic enough to explain why changed candidates kept
failing. The model made real workspace changes, so this is not a pure
no-action failure; it became repeated exploration without demonstrated
criterion convergence.

## Checkpoint and handover audit

The first successor `work_checkpoint` was rejected with:

```yaml
reason_code: evidence_ref_invalid
actionable_summary: each evidence_refs[] value must be a persisted receipt for this worker and workspace.
expected_reference_types: [evidence_refs[] (receipt_id)]
current_contract_revision: 4f57c71061c10ab5a78f632d8db59321
current_workspace_revision: 9
error: POLICY_DENIED
```

The immediately following request persisted progress checkpoint
`d375968b57e6c23cf899f977b262fe55`. This rejection was actionable and
corrected; it did not cause the business failure. A progress checkpoint does
not resolve the obligation, qualify the candidate, or authorize an artifact.

The successor handover contained the original Obligation/Message
`7d027b5a8fd2267a68b364e47f4635ca`, ContractRevision rev3
`4f57c71061c10ab5a78f632d8db59321`, workspace revision 2 and digest
`aae3482ff7442125965ac24a884b1b5527a0d7a880f6d0c33d3f7b4c5e42a9e1`, the
initial progress checkpoint, same `EmployeeID=emp-frontend`, and pending
work. `handover_context_quality = PASS`; the successor was not forced to
reconstruct missing authoritative state.

## Termination and budget

The successor emitted a final answer and a native `turn/completed` event. It
made 23 formal tool calls, below the configured 48-call allowance, and was
then stopped by the driver with confirmed proof
`windows-process-handle:39820:waited`. Thus:

```text
model natural completion → turn/completed → driver stop → confirmed stop proof
```

This was not a model deadline, reconnect/no-output termination,
provider/runtime terminal failure, hidden lower call limit, or budget
exhaustion. The observed successor budget was 23 used / 25 remaining.

## Side effects and finalizer defect

Receipts establish two Medium sessions and two provider egresses, both with
terminal process reconciliation. The workspace reached revision 9 and the
progress checkpoint persisted. No artifact was created and no obligation was
resolved. The old-writer apply/checkpoint/resolve probes were all
`STALE_EPOCH`; they are negative-control evidence, not external side effects.

After the successor returned with an empty artifact ID, the old driver looked
up that empty ID, got `no rows in result set`, and wrote a result that said
`frontend_successor=not_started`, `medium_started=0`, and `provider_egress=0`.
That is a post-business finalizer/reporting defect, not the successor failure
root cause. The driver now aggregates session records first, classifies an
empty artifact as `NOT_CREATED`, skips the lookup, and preserves the real
session/provider state.

Future result evidence records these identities directly instead of inferring
them from allocation order:

```yaml
source_runtime_incarnation: observed before recovery advances runtime_control
runtime_incarnation: current kernel runtime
frontend_initial_incarnation: initial worker binding
frontend_successor_incarnation: successor worker binding
frontend_initial_epoch: initial worker binding
frontend_successor_epoch: successor worker binding
transport_state: derived from turn completion and stop confirmation
```

## Causal classification

```ini
transport/runtime = PASS
authorization = PASS
PG/blob/process infrastructure = PASS
handover_context_quality = PASS
checkpoint_path = PASS after one correctable evidence-ref rejection
artifact_path = NOT_CREATED because no qualified candidate existed
model_behavior = FAILED_TO_CONVERGE / repeated candidate exploration
tool_feedback_quality = MIXED / FEEDBACK_TOO_COARSE
budget_boundary = NOT_CAUSAL; 23/48 and natural completion
successor_business_result = FAILED
```

The historical raw result remains unchanged. Because feedback quality is an
independent finding, a new provider sample is not currently eligible:

```ini
eligible_for_revised_frontend_run = NO
required_before_new_sample = clarify public semantic mismatch reporting,
                              then requalify any provider-visible surface
```

No model, Frontend, successor, Reviewer, database run, or historical raw
evidence rewrite was performed by this audit.
