# Slice300 REQ-02 TaskRevision owner-decision lifecycle

Date: 2026-10-07

## Scope

Added a pure monotonic decision state machine and Schema123 append-only owner
decision ledger for semantic TaskRevision context. The allowed transitions are
`proposed -> approved|rejected` and `approved -> revoked`; rejected/revoked
revisions cannot be resurrected and `qualified` is deliberately not a decision
state. The Kernel owner command checks the exact TaskRevision binding,
qualification and current state before appending `local-owner` decision
events. The Workbench command is installation-owner authenticated and CSRF
protected.

Decisions do not alter `qualification=unverified`, `requires_human=true`,
Worker admission or Provider dispatch. This is an approval ledger foundation,
not a runtime qualification or execution grant.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./db ./spec`
- `rtk bash scripts/go.sh test ./internal/control ./internal/workbench`
- `rtk bash scripts/go.sh test ./...`
- `rtk git diff --check`

No migration was applied, no Worker/provider/browser/external account was
used, and no frozen scenario was executed. The lifecycle state machine has
unit coverage; Kernel/database concurrency and live CSRF integration remain
unexecuted because they require the runtime database/HTTP fixture.

## Remaining boundary

Decision events now exist in source, but qualification evidence and any future
execution admission policy remain separately gated and unimplemented.
