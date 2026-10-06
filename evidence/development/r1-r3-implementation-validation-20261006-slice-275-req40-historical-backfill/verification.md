# Slice 275 — REQ-40 historical delivery backfill

Date: 2026-10-06

## Scope

Added a bounded management command and Kernel transaction for historical deliverable Artifacts that predate durable DeliveryManifest rows. It appends only `assembling` revision 1 plus `not_requested` UserDisposition, preserves missing provenance/build/instruction/verification/limitation/license evidence explicitly, and is idempotent by request ID. It does not claim ready, accepted, qualified, completed or externally delivered.

## Verification

- `rtk bash scripts/go.sh test ./internal/kernel -run TestBuildHistoricalProductDeliveryManifestPreservesMissingEvidence -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestBackfillProductDeliveryManifestsRejectsMalformedManagementRequest -count=1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk git diff --check` — PASS.

No `POLIS_DSN`/`POLIS_BLOB_ROOT` management invocation, PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran. Backfill execution remains an explicit operator action.
