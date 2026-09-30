# T21C offline gate: NOT_ELIGIBLE_FOR_LIVE

The canonical manifest is generated from `codex.PeerBackendTools()` in
`TestT21CRevisedCanonicalSurface`. `revised-manifest.json` contains all eleven
tools, descriptions, ordered per-tool schema digests/bytes, declared bindings,
aggregate digests, and a proposed execution manifest. Its fingerprint is not
a qualification certificate.

Exact JSON semantic differences from frozen T21B are in `semantic-diff.json`:

- work_current description: exposes tool-call budget.
- context_read description: exposes tool-call budget.
- work_checkpoint description: progress versus qualified semantics.
- work_checkpoint inputSchema: adds required kind, next_action, failed_checks;
  limits summary to 512 characters. Other existing fields remain unchanged.

No tool names/order or declared binding metadata changed. Structured checker
feedback and supersession are handler response/policy changes, not additional
input-schema changes. Their absence from the schema diff does not qualify their
runtime behavior. Historical binding labels for work_current/context_read say
Handover while the actual adapter calls PeerWorkCurrent; no new binding identity
was invented to conceal that existing metadata discrepancy.

The PostgreSQL policy test failed: a qualified checkpoint bound to revision 2
remained sufficient for artifact_submit after accepting incompatible revision 3
with an additional response field. The artifact was persisted as ready/candidate.
See `policy-binding-failure.json` for the observed database identities and state.
This is a current policy-binding defect; it is not a provider or transport error.

Phase A is incomplete/failed. Full schema validation, remaining response-contract
checks and fake-native callbacks were not completed after this blocker. Phase B
was not started; no diagnostic or business allowance was created. Historical
T21B and continuation-4/5 records were not changed. No production fix or Backend
execution was performed in T21C.
