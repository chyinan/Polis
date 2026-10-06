# REQ-37 durable BorrowerLease lifecycle

Slice283 adds Schema116 append-only consumer-bound service BorrowerLeases.

Acquisition requires a current service generation in `ready` state, same Company/Mission, a distinct borrower Task, exact active borrower WorkerSession and owner Session identity. TTL is capped at 15 minutes and idle grace at 2 minutes, both bounded by the endpoint lease. Release, touch and owner/generation revoke paths append immutable events; Artifact, process and external service contents are never modified.

The implementation is control-plane lifecycle only. It does not start/stop services, expose arbitrary commands or qualify a native executor.
