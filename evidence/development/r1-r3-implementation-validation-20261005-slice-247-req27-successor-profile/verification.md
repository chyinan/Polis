# Slice 247 — REQ-27/32 product-provider successor profile continuity

## Change

Product-provider successor readiness now reads the most recent prior WorkerSession persisted model profile and requires the requested successor profile to match exactly. The check runs during successor admission and again during authorization validation immediately before reservation. First-session admission remains unchanged. A stopped prior session may still be followed by a same-profile successor after the existing owner-reviewed memory revalidation gate. Cross-profile product successors fail closed until an owner-approved model-transition contract exists.

This does not change budgets, thresholds, owner decisions, handover records, schema, provider account scope, or make cross-model handover qualified.

## Verification

- go build ./... passed on the integrated implementation source (local source commit 50bc3825bb5a969327dcd0e23ebb438d2c2d35b5; published equivalent implementation commit 97db192c9659582e2e5e72e05da21f0617fb3b6e).
- git diff --check HEAD^ HEAD passed.
- The REQ-27 and REQ-32 scenario rows remain partial/not_run; all 232 frozen scenarios remain not_run.
- No tests, database operation, Worker/provider action, owner approval, or frozen scenario ran.
