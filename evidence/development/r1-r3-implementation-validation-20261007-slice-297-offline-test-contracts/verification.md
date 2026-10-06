# Slice297 offline test-contract alignment

Date: 2026-10-07

## Scope

This slice aligns stale offline test fixtures with contracts already enforced by
the production code. It does not change production behavior, enable a Provider,
start a Worker, call an MCP endpoint, open a browser, use an external account,
apply a database migration, or execute a frozen scenario.

The updated fixtures cover:

- the bounded `ProtocolToolCallLimit` required by `CreateMissionRequest`;
- the current one-shot MCP dispatch-permit issue/consume path, including owner
  startup ordering and the distinction between pre-permit failure and a pending
  external call;
- the explicit permit callback required by the Streamable HTTP MCP process;
- the Codex reservation prerequisite that identity readiness snapshots exist,
  using disposable unavailable-identity snapshots only.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh test ./internal/control`
- `rtk bash scripts/go.sh test ./internal/provider -run TestCodexRuntimeReservationLifecycleClosesWithoutReleasingEligibility -count=1`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk git diff --check`

The full Go package run completed successfully. No runtime database, real
Provider, WorkerSession, browser, external account, MCP endpoint, or frozen
scenario was used.

## Remaining qualification boundary

The full offline Go suite is clean. The Streamable HTTP process unit tests use
an explicit test permit callback by design; they do not qualify a real endpoint
or network path. The remaining open items are feature and qualification gates documented in the handoff: executable Role/TaskRevision
admission and provider qualification, action-specific generic ResourceKey and
DispatchPermit policy, native executor/service recovery, isolated BrowserRun
and research execution/evidence binding, migration/runtime qualification, and
external-account/provider/scenario evidence.
