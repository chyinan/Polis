# Slice 244 — REQ-13 company cursor wrap fairness

## Change

The product dispatcher now orders candidates by Company ID first and then by schedule age within each Company. This aligns the SQL order with the persisted Company-ID cursor: a cursor wrap cannot repeatedly select a globally oldest schedule from a later Company and starve earlier Companies. Quota-blocked and paused work remains ineligible, and no slot cap or quota-readiness policy was introduced.

## Verification

- The implementation commit's `go build ./...` passed in its isolated worktree.
- `git diff --check HEAD^ HEAD` passed after integration.
- Static inspection confirms the cursor predicate and `ORDER BY company_id, schedule.updated_at` use the same fairness key.
- Traceability remains 17 open requirements; all 232 scenarios remain `not_run`.
- No tests, DB operations, Worker/provider operations, or frozen scenarios ran.
