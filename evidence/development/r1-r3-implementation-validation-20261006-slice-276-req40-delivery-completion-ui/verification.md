# Slice 276 — REQ-40 delivery completion UI

Date: 2026-10-06

## Scope

Connected the existing installation-owner/CSRF delivery completion endpoint to the real Workbench API and assembling-delivery form. The form submits five bounded evidence sections, validates the ready/system completion receipt, invalidates the delivery query after success, and never treats completion as user acceptance or Worker execution.

## Verification

- `frontend/node_modules/.bin/vitest.cmd run src/domain/workbench-validation.test.ts -t "durable delivery completion receipt"` — PASS.
- `frontend/node_modules/.bin/vitest.cmd run src/data/real-workbench-api.test.ts -t "legacy durable delivery response"` — PASS.
- `frontend/node_modules/.bin/vitest.cmd run src/data/real-workbench-api.test.ts -t "assembling delivery evidence"` — PASS; owner completion POST path and body are covered.
- The completion receipt test also covers the returned revision fence and timestamp validation.
- direct `tsc -b` — PASS.
- direct `vite build` — PASS; existing large-chunk advisory only.
- `rtk git diff --check` — PASS.

The full frontend test file still contains unrelated pre-existing Mission-change/tool-limit fixture failures. No PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran.
