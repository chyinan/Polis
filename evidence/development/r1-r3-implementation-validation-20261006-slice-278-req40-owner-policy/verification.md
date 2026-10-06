# Slice278 verification: REQ-40 owner policy decisions

Date: 2026-10-06

## Scope

- Ready delivery completion opens a seven-day `awaiting_feedback` window.
- Feedback expiry is derived from the immutable deadline and blocks later disposition writes.
- Mission success closeout requires explicit accepted delivery dispositions for all supplied acceptance Artifacts.
- Failed/cancelled closeout remains available without acceptance.
- Historical backfill remains `not_requested` and does not invent user intent.

## Verification

- `bash scripts/go.sh fmt ./internal/kernel` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestProductDeliveryFeedbackExpiryIsDerivedWithoutImplicitAcceptance` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestValidateMissionSuccessDeliveryAcceptanceRequiresExplicitOwnerAcceptance` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestCompleteProductDeliveryManifestRejectsMalformedCommandBeforeDatabaseAccess` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed (`BUILD_OK`).
- `git diff --check` — passed.

Expiry writes use PostgreSQL `clock_timestamp()` inside the transaction; the pure policy helper is separately tested. No PostgreSQL migration or runtime operation ran. No Worker, native process, provider, browser, external account or frozen scenario ran. The source remains ahead of the recorded runtime schema until the migration gate is explicitly handled.
