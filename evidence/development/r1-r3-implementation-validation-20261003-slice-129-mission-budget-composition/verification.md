# Slice 129 — Mission budget composition audit

Date: 2026-10-03

## Result

Reviewed the frozen design sources for budget composition and inspected the current Worker admission, Worker tool-call charge, and Mission cancellation transactions. The source-backed next unit is an explicitly configured Mission cap for admitted protocol tool calls. One call must be projected to applicable caps once, and the Mission closing reserve must remain inside the Mission total. Hidden provider CLI retries, token cost, and USD cost remain unobserved/unaccounted here.

The source review also identified an inverse-lock risk if a Mission row lock is simply appended after the current Task lock: cancellation updates Mission before Tasks, while Worker charging locks session/Task before ProblemKey. The next enforcement step must audit and unify those transaction orders before introducing Mission-level serialization.

Decision and bounded sequence: `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md`.

## Sources reviewed

- `spec/design-v0.4.5/ARCHITECTURE.md` §14.1, §14.2, §14.5
- `spec/design-v0.4.5/contracts/C-RETRY.md` §34.1–34.3
- `spec/design-v0.4.5/tests/scenario_catalog.json` FT-42–45
- `internal/kernel/worker_state.go`: Worker admission and `Kernel.TXConsumeToolCall`
- `internal/kernel/mission.go`: `Kernel.TXCancelMission`
- `internal/kernel/problem_budget.go`: ProblemKey allocation and Task closeout transactions

## Validation boundary

Documentation-only stage; no code or migration changed. No tests, builds, database migration/runtime, provider execution, or live cost measurement were run. Frozen FT-42–45 execution statuses remain `not_run`.
