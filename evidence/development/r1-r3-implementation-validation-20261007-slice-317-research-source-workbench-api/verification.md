# Slice 317 — Research source Workbench API contract

Date: 2026-10-07

## Scope

Completed the frontend source-registration contract for the existing owner
route: `ResearchSourceView`, authorization options, runtime validation,
`RealWorkbenchApi.setResearchSourceAuthorization`, fixture-mode denial and the
React Query mutation hook. Authorized requests validate HTTPS origin, same-
origin search endpoint, fixed profile/ranking revisions and SHA-256 digests;
the response is revalidated against Company/Mission/source/request scope.

## Verification

```text
npm test -- --run
Test Files 16 passed
Tests 147 passed

npm run typecheck
tsc -b passed
```

No browser, backend, Worker, Provider, account or network action ran.

## Boundary

This adds Workbench data/API wiring only; no UI form was enabled and the
fixture mode remains explicitly unavailable. Runtime endpoint/auth/Provider
qualification remains open.

