# Slice303 REQ-38 ResearchOperation evidence binding

Date: 2026-10-07

## Scope

ResearchOperation `succeeded` results now require an evidence envelope with a
canonicalized manifest SHA-256, same Company/Mission/Task/Operation scope and
real ready candidate/passed Artifact rows whose digests match the manifest.
The same shared validator used by BrowserRun enforces unique approved evidence
kinds and fail-closed artifact lookup. Existing `unavailable` search/fetch
results remain unchanged.

This adds result-evidence integrity only; it does not implement search, HTTP,
browser, credential or Provider execution.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./internal/kernel -run OperationEvidence -count=1`
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk git diff --check`

No migration/runtime database, Worker/provider/browser process, external account
or frozen scenario was used.

## Remaining boundary

Controlled search/fetch retrieval, source registration and execution
qualification remain separately gated.
