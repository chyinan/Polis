# Slice 307 — Research source binding foundation

Date: 2026-10-07

## Scope

This slice implements the owner-controlled, Mission-scoped source binding
boundary for REQ-38 research operations. A source registration canonicalizes a
HTTPS origin, binds dedicated identity/data digests and a fixed profile
revision, and records append-only authorization/revocation events. A research
operation may optionally carry a source ID; when Schema124 is available, the
Kernel requires the current authorized source, the same Mission and an exact
origin match for fetch requests. The actual search/fetch result remains
`unavailable/research_backend_unavailable` with `provider_egress=0`.

## Implemented files and contract

- `internal/kernel/research_source_core.go`: pure registration normalization,
  canonical origin, registration digest and operation binding validation.
- `internal/kernel/research_source.go`: transactional registration/revocation
  and readback by request ID.
- `db/migrations/00124_research_source_bindings.sql`: immutable binding/event
  tables plus nullable `research_operations.source_id`, with hash manifest entry
  `04e62a57661025f80c95ac24ae41da3ea19af058bd45e84606b48be1e2306227`.
- `internal/control` and `internal/workbench`: owner command route
  `POST /api/workbench/companies/{companyId}/domain-workflows/research-sources`.
- Fake-only research surface @19 now accepts optional `source_id`; its exact
  surface is `manifest=6149e0fd55fcdfe19575347c7b141536190e48d8cb4713b84ac1399f2e2c9554`,
  `schema_bytes=4501`, `schema=5a853c10ef4f97d9c39304acef65cd42dae2aaabab0d12dc1d3ee0fd42f89a0c`.

## Verification

```text
POLIS_GO_ROOT=/mnt/d/Programs/Polis-cloud-main/.tools/go ./scripts/go.sh test ./internal/kernel -run ResearchSource -count=1
ok   polis/internal/kernel  0.059s

./scripts/go.sh test ./db ./internal/kernel -run "(Migration|Research)" -count=1
ok   polis/db              0.051s
ok   polis/internal/kernel 0.063s

POLIS_GO_ROOT=/mnt/d/Programs/Polis-cloud-main/.tools/go ./scripts/go.sh test ./...
ok   all Go packages, including control, kernel, provider, runner and workbench

./scripts/go.sh build ./cmd/...
exit 0
```

The Workbench company-scoped route test and the complete Go suite passed. No
PostgreSQL migration was applied, no HTTP request was sent to a registered
source, and no Worker, Provider, BrowserRun, external account or frozen
scenario ran.

## Boundary

This closes the source-registration and request-binding gap only. It does not
implement search ranking, HTTP retrieval, redirect/subresource/download/
WebSocket controls, Playwright/Chromium execution, browser profile isolation,
or external source qualification. Until those gates are separately qualified,
research operations remain unavailable and BrowserRun remains blocked.

