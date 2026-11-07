# R0.6 A checkpoint — Company / Organization management

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added schema 8 organization fields for company name, workspace root, archive state, and fixed employee presentation/model profile.
- Preserved the four runtime-owned logical Employee IDs (`emp-planning`, `emp-backend`, `emp-frontend`, `emp-review`) while allowing human configuration of display name, role label, and model profile.
- Added authoritative Company create/list/read/update/archive operations behind the existing Go control plane and scoped HTTP Workbench API.
- Added existing-frontend group company directory, real create wizard, company switch links, and company settings edit/archive controls.
- Kept create separate from Mission start; creation does not start a Mission, worker, provider, probe, or external send.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/organization ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` from `frontend/` — 10 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-org-pg` — migrations `00001` through `00008` applied; `TestCompanyOrganizationRoundTrip` passed using `POLIS_TEST_DSN` as `polis_runtime`; cluster stopped and removed.

## Known boundaries

- The roster remains a fixed runtime-compatible four-member template; dynamic hiring/firing is outside R0.6.
- The archive operation preserves history and refuses active/paused Mission companies.
- Provider credentials are not part of the organization surface; provider readiness belongs to B.
