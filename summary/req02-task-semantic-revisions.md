# REQ-02 durable semantic TaskRevision context

Slice299 adds Schema122 `task_semantic_revisions` and writes the exact
`compat`/Backend semantic binding at product Worker admission when the schema
is available. Rows are append-only, FK-bound to the Task and RoleRevision,
idempotent and read-back verified. They remain unverified/human-gated and do
not authorize execution; absence of Schema122 skips only this optional write,
while the existing Schema121 admission gate remains required.
