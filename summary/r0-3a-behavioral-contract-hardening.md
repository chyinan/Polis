# R0.3A behavioral contract hardening

## Status

`backend_behavioral_contract_hardening=PASSED` and
`eligible_for_revised_backend_run=YES`. This phase consumed zero Medium, zero
High, zero provider egress, created no allowance, and started no employee,
Frontend, Reviewer, or continuation-5 run.

Continuation-4 remains sealed as `backend_real_execution=FAILED`, with its raw
protocol, result, and postmortem unchanged.

## Checker feedback contract

Peer candidate acceptance now evaluates public semantic compatibility with the
currently accepted contract endpoint and response schema. It no longer tests
fixture marker strings. The worker-visible report contains `criterion_id`,
`passed`, `public_reason_code`, `actionable_summary`,
`relevant_contract_revision`, and `relevant_workspace_revision`. Hidden oracle
implementation and private test details remain outside the tool response.

## Checkpoint contract

Checkpoints have explicit `progress` and `qualified` kinds. Progress checkpoints
may bind failed workspace-check evidence and incomplete work while recording
workspace digest/revision, accepted contract revision, pending message and
obligation state, session/epoch, and next action. A progress checkpoint cannot
serve as qualified evidence or authorize artifact submission. Artifact submit
requires a qualified checkpoint for the current workspace revision. The
metadata is carried through the existing neutral handover bundle.

## Tool-call budget contract

Business allowance/auth bindings now carry an explicit positive
`tool_call_limit`, bounded by the runtime safety cap of 256. Worker sessions
persist the limit and usage, and `work_current`, `context_read`, and handover
context expose limit/used/remaining. `TurnWithOptions` enforces the explicit
business limit; the historical hidden 32-call business boundary is removed.
Generic callers without a business limit use only the separately documented
runtime safety cap.

The next Backend run must pre-register one concrete limit and bind that exact
value in the allowance, authorization, worker session, and turn options. This
phase intentionally selected no new experimental number.

## Contract supersession contract

Actionable peer messages and obligations remain bound to
`contract_revision_id`. Accepting a higher revision atomically marks active
lower-revision messages, obligations, and peer signals as `superseded`.
Stale apply/ack/resolve paths are denied, while an explicit replacement message
bound to the new accepted revision creates the current obligation. Repeated
replacement with the same idempotency key is unchanged and idempotent.

## Continuation-4 regression

The frozen continuation-4 protocol/result are read as a regression fixture. It
still reproduces generic checker feedback, checkpoint denial, hidden tool-call
exhaustion, and rev3-message/rev4-acceptance drift. New tests demonstrate the
replacement rules for the same classes of state without changing the historic
`FAILED` result.

Token accounting records protocol bytes, repeated workspace payload bytes,
repeated tool-result bytes, accumulated serialized conversation bytes, and
cached versus uncached input. It does not alter prompt or context serialization.

## Verification

- `go test -count=1 ./...` — PASS on fresh schema-4 disposable PostgreSQL
- `go test -race -count=1 ./...` — PASS on fresh schema-4 disposable PostgreSQL
- `go vet ./...` — PASS
- `go build ./cmd/...` — PASS
- Windows native cross-build — PASS
- PowerShell syntax parse — PASS
- `git diff --check` — PASS
