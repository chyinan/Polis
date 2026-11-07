# R0.3A H1 Review Disagreement Adjudication

This is a new, offline-only adjudication record. It does not modify, recompute, or overwrite H1 raw evidence, the candidates, the public contract, or any business state. No Medium/High turn, provider call, retry, or candidate mutation was performed.

## Frozen anchors

The following values were read from the frozen H1 package/result and independently checked against the retained candidate bytes and final frontend state:

| Anchor | Frozen value | Check |
| --- | --- | --- |
| ProblemKey | `r03a-real-peer-collaboration-v1` | exact |
| final ContractRevision | rev3, `4f57c71061c10ab5a78f632d8db59321`, state `accepted` | exact |
| public endpoint | `GET /items?cursor={cursor}&limit={limit}` | exact |
| public schema | `{"items":["id","name"],"next_cursor":"string|null"}` | exact |
| public contract digest | `7d7325dcc9052a361e4da8691cf8ea1ae83ba25a79282faca376372343fda9be` | exact |
| Message / Obligation | `7d027b5a8fd2267a68b364e47f4635ca` / same ID | exact |
| Backend artifact / digest | `5bf77f15ee867b857f116faad0312c4f` / `5b0bdad7236e0e80ba20bbb34b291c0451ce1e67388cb2a6a3ca711adaa4bac2` | retained CAS bytes hash to digest |
| Frontend artifact / digest | `a0191e5ce29a56eb347b421df39fb86e` / `16f3c32b05a6a57edaf38d42b4c7ebf706be593c09b836ea70097dc2a0af768f` | retained CAS bytes hash to digest |
| H1 subject digest | `46fc53b6c15f91dbfe0915badee92869aca72dbfed9c170f0baedd52739e918a` | exact H1 preflight/result value |
| H1 review package file | `review-package.json`, SHA-256 `18ee02fc240242a0a7e6cc68b58eb0a9874980c75205ff33632840310c512310` | frozen file |
| H1 formal review record file | `review-record.json`, SHA-256 `534602462ffffcb99acea30c7383080b3b53544a853204c6a3fb4a786a035054` | frozen file |
| H1 protocol file | SHA-256 `50ecaf0ed81afcce63f8824e15c1948c4a8c3a77fc543a13a3b86db654bf3405` | frozen file |

The H1 package does not persist a standalone integration-candidate ID or digest. The independently derived digest of the frozen integration input tuple (contract/artifact IDs and digests, `r03a-verifier@1`, positive flag) is `15dca95680197740758e2c74f6832efc4f0d8925c66f30c11fbc63ef38c44245`; it is explicitly derived, not presented as a historical stored digest.

H1 historical result remains unchanged: `reviewer_verdict=failed`, `hidden_verifier_result=passed`, `review_isolation_result=passed`, `reviewer_quality=FAIL_DISAGREEMENT`, `r0_3a_composite=FAILED`, `high_started=1`, `provider_egress=1`.

## Reviewer finding decomposition

The final formal record contains one finding: `The final candidates fail the declared paginated Backend-to-Frontend integration.` The earlier substantive submissions in the same frozen protocol expand that finding. It decomposes as follows:

| finding_id | Exact public requirement | Reviewer claimed behavior | Evidence ref | Candidate/source location | Severity |
| --- | --- | --- | --- | --- | --- |
| H1-F1-BACKEND | The accepted rev3 endpoint accepts `cursor` and `limit`, and its response has public `items` and `next_cursor` fields with the declared schema. | `FetchItems` ignores both parameters and returns the literal string `items,next_cursor`, not a structured paginated response. | Final accepted record cites rev3 `4f57c71061c10ab5a78f632d8db59321`; earlier attempts also cited the frozen Backend artifact/digest. | Backend CAS artifact `5bf77f15ee867b857f116faad0312c4f`, frozen bytes: `FetchItems` body, assignments `_ = cursor`, `_ = limit`, and `return "items,next_cursor"`. | high / acceptance-blocking |
| H1-F1-FRONTEND | The public response shape is the input available to the Frontend candidate; the candidate must consume the public response fields for the integrated result. | `ConsumeItems` ignores `body` and returns a fixed descriptive string; it does not consume item values. | Same rev3 and Frontend artifact refs in the protocol; final formal record summarizes this under the integration finding. | Frontend CAS artifact `a0191e5ce29a56eb347b421df39fb86e`, frozen bytes: `ConsumeItems`, `_ = body`, fixed return. | high / acceptance-blocking |
| H1-F1-ALGORITHM | Cursor propagation and null-termination behavior would require an explicit public algorithmic rule in addition to the stored rev3 endpoint/schema. | Reviewer also claimed no pagination advancement or null termination. | Reviewer’s candidate-content evidence; no separate algorithmic clause is persisted in the H1 Contract object. | Frontend comment is not treated as contract authority. | unresolved by this subclaim |

## Public-contract black-box adjudication

The adjudicator used only the frozen rev3 endpoint/schema and direct observable calls represented by the frozen candidate functions. It did not read the H1 verdict or hidden-verifier result to determine outcomes. No extra pagination rule was added.

| Case | Concrete input | Observed Backend behavior | Observed Frontend behavior | Expected public behavior | Result |
| --- | --- | --- | --- | --- | --- |
| request cursor binding | `cursor=""`, `limit=2`, then `cursor="2"`, `limit=2` | Same `"items,next_cursor"` output for both; source explicitly discards both arguments. | n/a | Request must be bound to the declared endpoint inputs; changing the public cursor/limit must not be silently discarded. | FAIL |
| response shape | Backend output from either input | A string literal, with no structured `items` field or structured `next_cursor` value. | `ConsumeItems` returns a string literal rather than consuming a structured response. | Public response has `items` and `next_cursor` fields according to rev3. | FAIL |
| `next_cursor` presence/type | Backend output for `cursor="2", limit=2` | The substring `next_cursor` is present only as text; no value with type `string|null` is produced. | No response field is read from `body`. | A `next_cursor` field with public type `string|null`. | FAIL |
| Backend/Frontend exchange | Body `{items:[{id:"i1",name:"n1"}],next_cursor:"2"}` and body `{items:[],next_cursor:null}` | Backend candidate provides neither structured body. | Both bodies produce the same fixed descriptive string. | Frontend must receive and interpret the public response shape for the integrated candidate. | FAIL |
| cursor propagation | successive cursor values | No observable cursor-dependent output or request construction. | No body-dependent cursor operation. | The frozen ContractRevision does not state a separate “feed `next_cursor` into the next request” algorithm. | `CONTRACT_AMBIGUITY` for this subclaim |
| termination | `next_cursor=null` | No executable termination signal is produced by Backend. | No body-dependent termination operation is observable. | Rev3 declares `string|null`, but the persisted contract does not separately state the client action for `null`. | `CONTRACT_AMBIGUITY` for this subclaim |

The first four cases are sufficient to adjudicate the reviewer’s core finding: both frozen candidates fail the declared public response/integration behavior. The two algorithmic subclaims remain explicitly marked ambiguous rather than being used to invent requirements. Therefore the reviewer finding is valid even though not every descriptive subclaim is independently contract-resolvable.

## Hidden-verifier coverage audit

The effective H1 hidden verifier is `h1HiddenVerify`/`validateH1FrozenSubject` in `internal/probe/h1_review_core.go`, which delegates candidate checks to `fixture.CheckPeerBackendCandidate` and `fixture.CheckPeerFrontendCandidate` in `internal/fixture/peer_contract.go`. The relevant current source files have hashes `87de06a009adcf2235c311843be6c1bc68a8a707768333b43cd9d1f5f96b530d` and `d8d93fa9ab6d025a8cf53ca51f2dd3ca1e834a98a290e8689267f21ca323eabc`; their derived source-set digest is `1902153fbc96deaf4db5e1e046ce23e2988bccbb5e38aebd815f8cf5a599d386`. H1 did not persist an explicit hidden-verifier revision identifier; that absence is recorded as `UNVERSIONED_IN_H1_EVIDENCE`, not silently replaced with a fabricated historical version.

What the verifier actually tests:

- rev3 is accepted, IDs and lifecycle anchors are non-empty/consistent, and session/epoch flags are present;
- the Backend has an `Endpoint` string matching the endpoint and a `FetchItems` string return containing tokens parsed as `items` and `next_cursor`;
- the Frontend has a `ConsumeItems` string return containing tokens parsed as `items` and `next_cursor`;
- positive/negative integration and old-writer booleans are trusted as supplied subject facts.

What it does not test:

- it never executes `FetchItems` with different cursor/limit inputs;
- it never executes `ConsumeItems` against different response bodies;
- it does not require a structured runtime value, a cursor-dependent result, or a termination transition;
- it does not recompute the artifact digest against the supplied content before checking it;
- it does not independently recompute the positive integration behavior; it accepts the frozen boolean and reuses the same static candidate checker.

Coverage classification for H1-F1:

| Claim | Classification | Reason |
| --- | --- | --- |
| Backend ignores cursor/limit | `NOT_COVERED` | no invocation or data-flow behavior test |
| Backend response shape is not structured | `NOT_COVERED` | marker tokens are treated as fields |
| Frontend ignores body | `NOT_COVERED` | body is not varied/executed |
| Frontend pagination advancement/termination | `NOT_COVERED` / `UNKNOWN` | no public algorithm test; no explicit persisted algorithm rule |
| final rev3/artifact IDs selected | `PARTIALLY_COVERED` | IDs/revision are checked, but artifact bytes are not digest-recomputed |
| stale cache/wrong artifact/wrong contract revision | `UNKNOWN` for independent detection | no cache is evident in the package, but the verifier has no independent content/digest provenance check |

Thus `hidden_verifier_result=passed` was a false positive for the disputed behavioral claim. It passed a static shape/marker approximation, not the public behavior.

## Review-package integrity

The package supplied the correct final rev3, Message/Obligation ID, Backend and Frontend artifact IDs/digests, candidate contents, session IDs/incarnations, policy revisions, and positive/negative integration facts. The retained CAS bytes independently hash to the package’s Backend and Frontend digests. No stale candidate, missing source file, truncated contract, or path/reference mismatch was found for the disputed claim.

The package limitation “limited to frozen read-only tool outputs” was real, but the candidate contents needed for this finding were present. The missing standalone integration-candidate digest and the absence of a persisted hidden-verifier revision reduce evidence provenance quality; they did not prevent this adjudication and were not facts on which the Reviewer’s finding depended. `review_package_integrity_for_disputed_finding=PASS`; `integration_digest_persistence=INSUFFICIENT/DERIVED_ONLY`.

## `review_submit` ×11 audit

The 11 calls are ordered by the frozen `item/tool/call` receipts. `finding_digest` is the SHA-256 of the submitted arguments’ compact JSON and is an audit aid, not a historical receipt ID.

| # | Accepted | Result | Finding digest | Substantive conclusion |
| ---: | --- | --- | --- | --- |
| 1 | no | `POLICY_DENIED` | `ec5ac70314c569ba445f2173b5c385399aa9c4c7aa63761618080fce2529a73f` | failed; detailed Backend/Frontend finding |
| 2 | no | `POLICY_DENIED` | `df702773201f9a101f461c4d4ac02841749298ca59c23665c7a4b63187bbf075` | same failed finding |
| 3 | no | `POLICY_DENIED` | `fb83c3d348f7b9bb2746a23b68f10107824fb6d72afc12dfe27bbf6598bde36b` | same failed finding |
| 4 | no | `POLICY_DENIED` | `a6de9a45d4271f3710e7efdddadb6ed1fd96b3aa17cd895143a2e17d3f9b2015` | same failed finding |
| 5 | no | `POLICY_DENIED` | `cd6de8f3444efc14dabd906d14b741d8698d51f6e566e5efe49890394df81ac0` | same failed finding |
| 6 | no | `MALFORMED_INPUT` | `985c3b5a5eec8024da5e347e43f9c773b0fb9c4973bad9716de66ac6ebbf7cc3` | same failed finding; empty evidence |
| 7 | no | `POLICY_DENIED` | `fa9863008528768a2e13991302dbd8e23c51d2767dff1c32971ca0f17ce4275a` | same failed finding |
| 8 | no | `POLICY_DENIED` | `a1f76dcef39c62a34ea99abeecc505b1db3c8ee6132838fee8cb459b1b3d4d1c` | same failed finding |
| 9 | no | `POLICY_DENIED` | `ee08f135cea05152e738fa8adb7e38d492717b32c9d208d67d1f623ac8f0b2b0` | same failed finding |
| 10 | no | `POLICY_DENIED` | `4ef1efcbcdff0e460f7c0904b9c092b1c37a00d38607008c57ef711c76c8aac4` | generic failed finding |
| 11 | yes | accepted | `c876de12479b1910238025af070f690b88ee1523cc3865af08917c2c82cd33d0` | generic failed finding |

The final accepted receipt is `review-20260913T123920.807156200Z`. Calls 1–10 are protocol/evidence-reference friction (nine policy denials and one malformed empty-evidence request). The substantive verdict did not change from failed to passed or inconclusive; the final finding became shorter, but not substantively opposite. This is `REVIEW_PROTOCOL_QUALITY_FINDING=reference-friction-only`, not a basis to overturn the Reviewer verdict.

## Integration-verifier coverage

The frozen revised-v2 run records `positive_integration=true` and `negative_control_passed_as_failure=true`. The integration verifier checks accepted-contract/artifact/lifecycle bindings, artifact presence, obligation state, and the same static candidate checker. The negative control demonstrates rejection of the v1 fixture. It does not execute cursor/limit behavior, response parsing, cursor propagation, or termination. Therefore integration coverage is `PASS_FOR_BINDING_AND_NEGATIVE_DISCRIMINATION`, `NOT_COVERED_FOR_RUNTIME_PAGINATION_BEHAVIOR`.

## Final adjudication

```ini
disagreement_classification = REVIEWER_FINDING_VALID
adjudicated_subject_result = FAIL
reviewer_quality = VALID_FINDING_FOR_DISPUTED_CLAIM
hidden_verifier_quality = FAIL_FALSE_POSITIVE
H1_historical_composite = FAILED (unchanged)
post_hoc_acceptance_adjudication = FAILED
eligible_for_new_independent_review = NO
```

The decisive basis is the public-contract black-box result: the frozen Backend ignores public request inputs and returns no structured public response, and the Frontend does not consume a structured response. The hidden verifier’s pass is explained by its marker-based static checker and insufficient behavioral coverage. The H1 package was adequate for the disputed finding; its missing standalone integration digest and unversioned hidden-verifier identity are recorded as provenance limitations, not used to relabel the case as a package defect.

No candidate, contract, hidden verifier, raw evidence, business state, or historical result was changed. No model was called.

## Verification

Full Go tests without PG, race, vet, Linux build, Windows cross-build and `git diff --check` passed. The dedicated PostgreSQL deterministic run reached the database but stopped on the existing `TestPeerHandoverBoundarySnapshotReadsStoppedInitialSession` failure at `TXPeerSend` with `POLICY_DENIED`; its disposable database was dropped and PG was stopped. This independent local regression was not used to choose the H1 classification and was not repaired because this stage forbids repair.
