# Slice 319 — Research source read projection

Date: 2026-10-07

## Scope

Added a Company/Mission-scoped `GET /domain-workflows/research-sources?missionId=...`
projection. Kernel returns latest immutable source states with origin, endpoint
and non-secret search policy metadata; pre-registration schemas return an empty
list. The Workbench panel now reads and displays the current source states after
registration/revocation.

## Verification

```text
./scripts/go.sh test ./internal/workbench -run ResearchSource -count=1
ok   polis/internal/workbench  0.061s

npm test -- --run
147 tests passed

npm run typecheck && npm run build
passed
```

No Worker, Provider, browser, credential, migration runtime or network action
ran.

