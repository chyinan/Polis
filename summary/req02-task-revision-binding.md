# REQ-02 semantic TaskRevision binding

Slice298 adds a pure, content-addressed binding from the fixed-team
RoleRevision to a concrete semantic `task_type`/owner/TaskKind tuple. Exact
matches, owner mismatches, kind mismatches and uncovered types have explicit
reason codes; all results remain `unverified` and `requires_human=true`. This
does not persist a new TaskRevision or enable Worker/provider execution.
