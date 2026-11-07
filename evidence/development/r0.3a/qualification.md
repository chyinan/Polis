# R0.3A Real Peer Collaboration — local qualification

Status: `passed` for the deterministic/FakeWorker/PostgreSQL qualification only.
No Medium, High, reset, QQ, MCP, browser, UI, GitHub, or real employee model call was used.

## Frozen history

- subject revision: `e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c`
- R0.2 artifact id: `57761d55c7b0dbf63668b9f8010de41d`
- R0.1/R0.2/H2 evidence was not rewritten. The H2 `INCONCLUSIVE` result remains historical.
- `evidence/development/r0.1/go-tests.txt` was restored after each full test run.

## Fixture and causal dependency

The disposable fixture starts with API v1 (`GET /items` returning `items`). The backend employee proposes and accepts ContractRevision 2: `GET /items?cursor=...` returning `items` and `next_cursor`. The frontend candidate is accepted only after it consumes that revision and records `applied-contract-revision:2`; it cannot pass by remaining on v1 or by submitting only a guessed final file.

The negative control submits v1 backend/frontend candidates without the v2 collaboration path. The deterministic integration verifier rejects it because revision 2, the v2 backend marker, the frontend application marker, and a fulfilled frontend obligation are missing.

## Persisted lifecycle

`TXPeerSend` persists one message, one actionable Obligation, and one WorkSignal in one `TXWrite` transaction. A same-key replay returns the original receipt; a different body with the same key is a conflict. A non-actionable FYI creates no obligation.

The tested message/obligation/signal progression is:

`persisted → delivered → observed → acknowledged → applied → resolved`

The obligation progression is `pending → observed → applied → fulfilled`; acknowledgement does not fulfill it. The committed send was also read from an independent PostgreSQL session.

## Employees, tasks, workspaces, and handover

`emp-backend` owns the backend task/workspace and the contract proposal. `emp-frontend` owns the frontend task/workspace and must apply the accepted revision before producing its candidate. A backend attempt to apply frontend content is denied.

The handover is injected after frontend delivery/observation/acknowledgement and before application. The same EmployeeId receives a new worker epoch. The old binding's application is rejected with `STALE_EPOCH`; the successor applies v2, submits generation-2 content, and resolves the inherited obligation. No coordinated double-write is permitted.

The persisted message path is direct backend → frontend. The test queries the message table and asserts zero sender/recipient rows involving `emp-planning`.

## Independent final acceptance

`TXFreezePeerIntegration` binds the backend artifact, frontend artifact, accepted contract revision, base revision, and verifier revision. `VerifyPeerIntegration` checks the frozen records, readable artifact blobs, v2 markers, and fulfilled obligation. `PeerReviewIntegration` is a separate deterministic fake-review gate over the frozen candidate state. This local gate is not a real model review.

Future real sessions must receive only frozen contract/task input, candidate content/digest, and controlled provenance. They must not receive the negative-control explanation or verifier source/answers. The local qualification does not consume or expose a hidden verifier to an agent.

## Tests and commands

The following completed with exit code 0:

```text
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/test.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh build ./cmd/...
rtk git diff --check
rtk proxy wsl -d Ubuntu-22.04 -- bash -lc "cd /mnt/d/Programs/Polis && export POLIS_R02_EVIDENCE=/mnt/d/Programs/Polis/evidence/development/r0.2/luna-1 && bash scripts/go.sh test -count=1 ./internal/probe -run 'TestReviewerPreflightAgainstPersistedR02Evidence|TestLoadRecoveredRealEvidenceFixture'"
```

`scripts/test.sh` migrated PostgreSQL 18 through `00003_r03a_peer_collab.sql`, ran the full Go suite, `go vet ./...`, and command builds. The R0.3A-specific passing tests were:

- `TestR03APeerSendCommitVisibleToIndependentSession`
- `TestR03APeerCollaborationPositiveAndNegative`
- `TestR03AFrontendHandoverPreservesPeerObligation`

The existing transaction, stale epoch, process-stop, H3 ordering, reviewer policy, and isolated verifier tests also passed. The test database was disposable and removed by the script trap.

The persisted-R0.2 read-only tests were then run with the project Go wrapper and passed. An earlier direct Go-binary attempt was not counted: it bypassed the wrapper's local dependency environment and failed before test setup while trying to download `pgx`; no source or evidence was changed.

## Hash and allowance preservation

The final local check reported HEAD `e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c`, and `git diff --check` passed. The recorded frozen evidence hashes remained:

```text
f650a6d663452b05cb9e68d8d8001ea1adbde30252273c45e1c39f8c72d5bea2  evidence/development/r0.1/go-tests.txt
82217efcc45881126256a882f6ecaf0c090d270211bc8a4393c7771073ccbec1  evidence/development/r0.2/luna-1/allowance.json
81099acff766cbaedd148b4b7e4040e9f0fb8b841bfaace78f2c46d8559dd2be  evidence/development/r0.2/luna-1/result.json
7e3fb894840e8d3c117ed24c276dd9d7c896cfa8612ca7779ce658db0e8f80ed  evidence/development/r0.2/luna-1/final-assessment.json
4f0fa1185918d8f9e05f6e0a289d7d25d03a4c8032f815c617c54bdaa6d745bc  evidence/development/r0.2/luna-1/handover-bundle.json
95222ac7726ffd82c7da00649c81238e95c79e306bd3bb20802e32acc11805c1  evidence/development/r0.2h2/allowance.json
9432388d7dcd91871c811defec04fb4f5f188edba8958b1bc623fb487d3a3211  evidence/development/r0.2h2/result.json
```

## Proposed future real-model allowance

Only after separate explicit authorization: one serial allowance of up to three Medium turns (backend, frontend, and one frontend successor) plus one optional independent High reviewer, concurrency 1, with a fresh ProblemKey and deadline. No reset or automatic retry; stop on unknown outcome, missing receipt, stale writer, or verifier/infrastructure failure. This is a recommendation, not an authorization consumed by R0.3A.
