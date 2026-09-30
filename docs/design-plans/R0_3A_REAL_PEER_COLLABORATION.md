# R0.3A Real Peer Collaboration — deterministic qualification design

> Status: local design for deterministic/FakeWorker/PostgreSQL qualification only.
> No real model allowance is authorized or consumed by this slice.

## Fixture and causal dependency

The disposable fixture models a backend API and frontend client:

- Contract v1: `GET /items` returns `{items}`.
- Backend owns the API workspace and proposes ContractRevision v2:
  `GET /items?cursor=...` returns `{items,next_cursor}`.
- Frontend starts from v1. Its deterministic verifier fails unless the accepted v2 revision is observed, acknowledged, applied to the frontend workspace, and included in the final integration candidate.
- The negative control keeps the same base inputs and blocks the backend→frontend collaboration message; frontend remains on v1 and the integration verifier must fail.

The frontend cannot pass by guessing v2: its final candidate records the applied contract revision and the verifier requires that revision plus the backend artifact digest.

## Minimal persisted model

Add only the fields needed for this experiment:

- task kinds `peer_backend` and `peer_frontend`;
- task revision and workspace owner metadata;
- contract revisions with immutable content/digest and accepted state;
- message delivery/observation/application state and sender/recipient;
- obligation states `pending`, `observed`, `applied`, `fulfilled` with evidence references;
- a work signal linked to the actionable message.

All mutations use the existing company guard and `TXWrite` receipt/idempotency path. `collab.send`, `collab.observe`, `collab.ack`, `collab.apply`, and `obligation.resolve` are explicit kernel operations; acknowledgement never fulfills an obligation.

## Execution flow

```text
backend task
  → contract.propose / accepted ContractRevision v2
  → collab.send to frontend task
  → persisted message + WorkSignal + pending Obligation
  → frontend observes and acknowledges
  → frontend applies v2 to its own workspace revision
  → obligation.resolve with applied workspace evidence
  → freeze backend/frontend candidates + integration base + verifier revision
  → deterministic positive/negative verification
  → FakeReviewer checks frozen revisions
```

Planner is only involved in fixture setup. No runtime send path goes through `emp-planning`.

## Isolation and handover

Backend and frontend tasks receive independent writable workspaces. A task-specific workspace CAS guard rejects cross-owner writes and old epochs. The frontend handover test stops only the frontend session after observation but before application, creates a neutral bundle containing the pending obligation, message cursor, contract revision and workspace digest, then starts the same EmployeeId with a new epoch. The successor applies v2 and resolves the obligation. The stopped writer is rejected.

## Deterministic qualification

The PG/FakeWorker suite must cover:

- duplicate send and same-key/different-body conflict;
- persisted message survives a simulated crash boundary;
- delivered/observed/applied/fulfilled are distinct;
- FYI creates no obligation or work signal;
- actionable collaboration creates exactly one obligation and signal;
- old v1 frontend fails integration verification;
- v2 frontend succeeds only with the accepted revision and backend digest;
- stale/superseded v1 cannot apply or resolve;
- cross-workspace write and old epoch are rejected;
- Planner is absent from the persisted sender/recipient path;
- frontend handover preserves the pending obligation and contract revision;
- frozen integration artifact contains both candidate revisions and verifier revision.

No real model, QQ, MCP, browser, UI, multi-company execution, or E-ORG comparison is part of R0.3A.
