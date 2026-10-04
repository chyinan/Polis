# Slice 174 — REQ-39 takeover snapshot retry recovery

## Finding

`TXSubmitTaskTakeoverSnapshot` read the current lease projection and rejected every state except `granted` before calling `TXWrite`. The first successful submission atomically persisted both a `returned` lease event and its command receipt. If the response was lost, an exact retry therefore conflicted before the receipt's idempotency check could run.

## Changes

- A snapshot retry against a non-granted lease now routes through the ordinary `TXWrite` operation/fingerprint guard before any CAS write. The exact request returns its stored receipt and lease projection. A changed payload, unrelated request ID or released lease still conflicts.
- First submissions still persist the snapshot only after the frozen digest/revision, Mission, writer-stop and lease state are rechecked in the locked transaction.
- Workbench holds the exact pending snapshot payload and request ID in component state and offers a retry button using that same command. Snapshot mutations refresh the lease and change-request projections on both success and failure, so an ambiguous HTTP response is reconciled against persisted state.

## Verification

- `go build ./...` — passed.
- `cd frontend && npm run build` — passed. Vite reports the existing JavaScript chunk above 500 kB.
- `git diff --check` — passed.

No tests were run. No Worker, provider or frozen scenario was executed. No schema migration or database data change was needed.

## Remaining REQ-39 work

This repairs exact retry after a committed handback; it does not complete the full safe-change workflow or qualify the frozen change/takeover scenarios. The current Workbench retry payload survives only while the panel remains mounted. Persisted lease state remains authoritative after reload.
