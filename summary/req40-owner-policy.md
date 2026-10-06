# REQ-40 owner policy decisions

Slice278 records and enforces the selected operational policy:

- A completed `ready` DeliveryManifest starts a seven-day `awaiting_feedback` window.
- Acceptance is explicit; preview, download, internal validation and Artifact publication never imply acceptance.
- A feedback deadline is derived, and after expiry new owner disposition writes are rejected with `feedback_expired`; no implicit acceptance or automatic revision Task is created.
- Mission `succeeded` closeout requires every recorded acceptance Artifact to have a current ready Manifest and an explicit latest `accepted` disposition. `ended_not_met` and `cancelled` do not require user acceptance.
- Historical backfill remains `not_requested` because it cannot invent a user feedback request or deadline.

The policy is source-level and fail-closed on pre-Schema111 runtimes: if durable delivery tables are unavailable, successful Mission closeout cannot be falsely certified. No migration, Worker, provider, browser, external account or frozen scenario was run.
