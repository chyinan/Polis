# R3 deterministic research simulator foundation

> Updated: 2026-09-29, Slice 76 / Schema 55. This is a local deterministic computation with an immutable run ledger and Workbench operation. The research profile remains `not_run` and execution-disabled.

## Contract

`RunResearchSimulation` accepts a frozen `ResearchProtocol`, exact company-authorized MissionInput references for the dataset and method, and their CAS bytes. It supports one declared method, `bootstrap-mean-difference@1`, using strict JSON inputs with control/treatment numeric arrays. Unknown JSON fields, changed source bytes, cross-company or unavailable source revisions, unsupported methods/risk units, invalid numbers, excessive input size, invalid iteration/sample bounds, and risk consumption beyond the declared budget are rejected before output.

Each bootstrap draw uses a fixed SplitMix64 implementation seeded by the protocol seed. The output is canonical JSON carrying protocol revision, dataset/method digest, control definition, seed, iteration/sample size, actual consumed sample-draw units, and deterministic summary statistics; its SHA-256 is returned separately. Repeating the exact sources, method, budget and seed produces byte-identical output.

The algorithm reports estimates only. It does not infer a positive/negative domain outcome, run executable method code, access an external service, or change domain qualification. The existing independent evaluation contract still handles research outcome and evidence; no output from this simulator qualifies `research-simulation@1`.

## Verification

Synthetic unit tests verify exact-source binding, same-seed determinism, budget accounting and over-budget rejection, changed-byte rejection, and unsupported risk-unit rejection. `go test ./internal/domainworkflow` passes. No real research data, external service, model, or domain-quality threshold was used.

## Persistence and Workbench operation

Schema 54 stores each accepted simulation receipt in an append-only company ledger. The row binds exact same-company usable dataset and method MissionInput revisions and content digests, fixed protocol/profile revision, canonical seed, control definition, bounded risk units, output digest and idempotency request ID. A trigger independently checks current input state, media type, byte bounds and exact input digest. Forward migration 55 validates the JSON numeric iteration/sample bounds and the exact risk equation. The Kernel reads exact CAS bytes, validates the deterministic receipt, output digest and sample-draw accounting, then returns database readback. A PostgreSQL session advisory lock serializes request precheck, simulation and persistence; same-payload concurrent retries replay the same run and changed payloads conflict. The authenticated Workbench lists eligible inputs from the current mission, exposes an explicit simulation command, and displays recorded estimates and source/output hashes. Fixture mode denies writes. The UI and ledger continue to show `not_run` and execution disabled. No real research data, model, account, network endpoint, trading or publication was used.

## Remaining R3 work

Keep both profiles `not_run` and execution-disabled until each receives its own real domain quality, recovery, cost and organization evidence and separate runtime qualification. The local content operations path is documented in `docs/implementation/R3_CONTENT_OPERATIONS_FOUNDATION.md`; it is software workflow evidence only.
