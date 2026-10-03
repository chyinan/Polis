# Slice 131 — Mission closing reserve

Date: 2026-10-03

## Scope

Schema 90 adds a separately revisioned Mission closing reserve within the explicit Mission tool-call cap. Only Kernel-created `review` and `peer_review` Tasks spend protected calls. Ordinary admission, accepted-call accounting, Handover and provider turn clamping preserve remaining reserve; closing calls consume reserve while still advancing the overall Mission cap. Pending Missions may configure reserve first, but first finite-cap configuration must cover current usage and unspent reserve. Mission budget rejection evidence binds the reserve amount, remaining amount and policy revision. The Settings budget panel exposes confirmed, reasoned reserve revisions.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis` | PASS |
| `cd frontend && npm run build` | PASS; Vite reports the existing large-chunk advisory (797.86 kB minified JS) |
| `cd db/migrations && sha256sum -c ../migration_hashes.sha256` | PASS; all 90 migrations |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution/live cost measurement | Not run |

## Files

- `db/migrations/00090_mission_tool_call_closing_reserves.sql`
- `db/migration_hashes.sha256`
- `internal/kernel/mission_tool_budget.go`
- `internal/kernel/worker.go`
- `internal/kernel/worker_state.go`
- `internal/control/problem_budget.go`
- `internal/workbench/http.go`
- `internal/desktop/auth.go` (existing Mission budget authorization rule covers the new route; no edit required)
- `frontend/src/domain/workbench.ts`
- `frontend/src/domain/problem-budget-validation.ts`
- `frontend/src/data/workbench-api.ts`
- `frontend/src/data/real-workbench-api.ts`
- `frontend/src/data/fixture-workbench-api.ts`
- `frontend/src/data/workbench-query.ts`
- `frontend/src/pages/ProblemToolBudgetPanel.tsx`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
- `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md`
- `docs/implementation/CLOUD_CODEX_HANDOFF.md`

This is build and source validation only. It does not establish PostgreSQL runtime behavior or frozen FT-42–45 qualification.
