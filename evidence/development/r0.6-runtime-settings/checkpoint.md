# R0.6 B checkpoint — Provider / Model / Runtime settings

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added schema 9 desired runtime configuration fields: provider, model, effort and profile.
- Added safe runtime settings read projection with worker mode, provider/model/profile, auth readiness, runtime version/readiness, current product surface status, workspace root, PostgreSQL/CAS status and event-stream status.
- Added settings update command. It persists only non-secret desired configuration and returns `restart_required`; it does not restart workers or call a provider.
- Added structured Mission Start rejection state `provider_unavailable` when worker readiness fails.
- Added real Settings page controls and safe readiness rendering in the existing frontend.
- No provider credential path/content is returned by the API; auth is represented only as status.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-org-pg` — migration `00009_r06_runtime_settings.sql` applied; `TestRuntimeSettingsUpdatePersistsDesiredNonSecretConfiguration` passed; cluster stopped and removed.
- The disposable run caught and verified the fix for legacy `TXCreateCompany` inserting only `id` after the schema 8 non-empty company-name constraint.

## Known boundaries

- Runtime settings are desired configuration; applying a provider/model change requires the documented restart/readiness boundary and does not trigger a provider call.
- External credential provisioning remains outside this slice; no secret is accepted or stored by the settings command.
