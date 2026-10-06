# Slice302 REQ-38 BrowserRun evidence binding

Date: 2026-10-07

## Scope

Added a pure operation-evidence manifest validator and integrated it into the
BrowserRun read path. A `succeeded` BrowserRun now requires an evidence-manifest
SHA-256 matching the canonicalized stored evidence JSON, a same
Company/Mission/Task/Run scope, and 1–16 unique Artifact references whose real
Artifact rows match scope/digest and are ready with candidate/passed verdicts.
Invalid or cross-scope success evidence fails closed; blocked BrowserRun
records remain explicit and unchanged.

This adds evidence integrity only. It does not open Playwright, browser,
credential, network, Provider or external-account execution.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./internal/kernel -run OperationEvidence -count=1`
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk git diff --check`

No browser process, network request, migration/runtime database, Worker,
Provider, external account or frozen scenario was used.

## Remaining boundary

Controlled browser/search/fetch execution and source retrieval remain blocked
until the separately required isolated host, identity, egress and qualification
evidence exist.
