# REQ-02 TaskRevision owner-decision lifecycle

Slice300 adds the monotonic semantic TaskRevision decision lifecycle:
`proposed -> approved|rejected`, then `approved -> revoked`. It persists
append-only owner decision events through Schema123 and exposes a CSRF-protected
Workbench command. Decisions do not qualify or authorize execution.
