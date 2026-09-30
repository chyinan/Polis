# Slice 90 verification — Workbench employee schedule projection

Date: 2026-09-30  
Schema: 62; no migration

## Changes

- Company overview reads `employee_schedules` using the same `company_id` and `employee_id` scope as the employee record.
- `EmployeeSummary.schedule` projects state, work generation, checked generation, next due time and pause reason. Generation values remain decimal strings so JavaScript does not round PostgreSQL `bigint` values.
- A missing schedule row is represented as JSON `null` and shown as unavailable. The display is separate from WorkerSession state and contains no admit/dispatch action.
- Frontend validation accepts the documented schedule states and rejects unknown states. Historical fixture data uses `null` because it has no schedule evidence.

## Verification

- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed; the new Workbench integration verifies a ready Task created by Mission start appears as `wake_pending` with generation `1/0`.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 scripts/go.sh build ./cmd/...'` — passed.
- `rtk npm run test` — 133 tests passed.
- `rtk npm run typecheck` — passed.
- `rtk npm run lint` — passed.
- `rtk npm run build` — passed with the existing bundle-size advisory.
- Independent read-only review found no Critical, Important or Minor findings.

No native Desktop visual smoke was run for this slice. No real Worker/model, QQ, MCP, GitHub account or production service was used.
