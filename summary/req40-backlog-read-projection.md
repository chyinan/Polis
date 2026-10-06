# REQ-40 Company backlog read projection

- Slice 273 persisted terminal `delivery_feedback_backlog_events` but exposed only the write path.
- Slice 274 adds a read-only, company/delivery-scoped projection capped at 32 open events in `DurableDeliveryLifecycleResponse`.
- Go Workbench readback validates event IDs, positive canonical revisions, Artifact/Mission/Task scope, `open` status, system actor, non-empty reason and request ID before returning data.
- Frontend DTO validation requires the scoped `feedbackBacklog` array; the Task delivery panel renders the backlog without creating Tasks, waking Missions or changing terminal state.
- No database runtime, Worker, provider, browser or external account action is needed for this slice; Schema 114 is the source boundary and recorded runtime remains Schema 108.
