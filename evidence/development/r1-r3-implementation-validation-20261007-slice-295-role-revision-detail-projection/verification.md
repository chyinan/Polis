# Slice295 verification: RoleRevision detail projection

Date: 2026-10-07

## Scope

- Extended optional Schema121 EmployeeSummary RoleRevision readback with role name, semantic task types/kinds, owner decision and qualification metadata.
- The read store selects the latest Company/Employee revision and rejects malformed JSON or metadata; old schemas still return null.
- Frontend validation binds detail digest and Employee identity to the summary while keeping qualification unverified.
- No database runtime, owner action, Worker/provider, browser, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/workbench -run TestCanonicalDurableDeliveryManifestAllowsIncompleteAssemblingEvidence -count=1` — passed.
- `frontend/node_modules/.bin/vitest.cmd run src/domain/workbench-validation.test.ts` — 50/50 passed.
- `frontend/node_modules/.bin/tsc.cmd -b --pretty false` — passed.
- `frontend/node_modules/.bin/vite.cmd build` — passed.
- `git diff --check` — passed.
