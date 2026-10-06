# REQ-40 historical delivery backfill

- Slice 275 adds `Kernel.TXBackfillProductDeliveryManifests` and the explicit `polis delivery-backfill COMPANY REQUEST_ID` management command.
- The command is Company-scoped and idempotent through `TXWrite`; it selects only deliverable/ready Artifacts without a durable delivery revision.
- Each backfilled revision is `assembling` with file inventory available, all unavailable evidence sections explicit, and initial UserDisposition `not_requested`.
- It never infers qualification, user acceptance, Mission completion, Worker execution or external delivery.
- Runtime execution requires an explicit `POLIS_DSN`/`POLIS_BLOB_ROOT` management invocation and was not run in this offline pass.
