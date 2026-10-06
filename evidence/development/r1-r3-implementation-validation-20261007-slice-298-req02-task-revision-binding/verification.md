# Slice298 REQ-02 semantic TaskRevision binding

Date: 2026-10-07

## Scope

Added a pure Functional Core resolver for the owner-confirmed fixed-team
RoleRevision to produce a content-addressed semantic TaskRevision binding.
The resolver checks the semantic `task_type`, owner and persisted TaskKind;
returns distinct mismatch reason codes; and keeps every result
`qualification=unverified` with `requires_human=true`. `AdmissionFor` uses the
same binding gate and no Worker, Provider or external execution path changed.

This slice does not persist a new database revision, start a Worker, qualify a
Provider, or infer ambiguous semantic types from a TaskKind. It is the pure
contract needed before a later owner-approved durable lifecycle/qualification
slice.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./spec`
- `rtk git diff --check`

The tests cover deterministic content addressing, exact matching, owner
mismatch, TaskKind mismatch and uncovered semantic types. No database runtime,
Worker/provider, browser, external account or frozen scenario was used.

## Remaining boundary

The binding is now defined in source, but it remains unverified and does not
authorize execution. Durable Task/PlanRevision lifecycle, owner qualification,
provider qualification and runtime/scenario evidence remain open.
