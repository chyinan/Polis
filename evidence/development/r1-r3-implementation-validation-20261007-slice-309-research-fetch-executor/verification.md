# Slice 309 — Controlled research fetch executor

Date: 2026-10-07

## Scope

Added a source-level HTTPS fetch executor for an already registered source.
The pure core canonicalizes the exact HTTPS origin/target, bounds timeout and
response bytes, accepts only text/JSON/XML media, and binds the body digest.
The shell uses a caller-supplied HTTP client, disables proxy use in its default
client, sends no Cookie or Authorization header, and refuses redirects rather
than following them. ResearchOperation now has a database-bound success fence
that accepts only a same-Task/Session evidence envelope whose Artifact rows are
real ready candidate/passed records.

## Verification

```text
./scripts/go.sh test ./internal/research -count=1
ok   polis/internal/research  0.028s

./scripts/go.sh test ./internal/kernel -run "(ResearchOperation|OperationEvidence)" -count=1
ok   polis/internal/kernel  0.056s
```

The fetch tests use an in-memory RoundTripper and verify no Cookie/Auth header,
body digest, redirect rejection and bounds. No external HTTP request was made.

## Boundary

This is an executor and persistence boundary, not live retrieval qualification.
The fake @19 product surface still records unavailable operations unless a
future qualified control path explicitly calls this executor. Search ranking,
source-specific search endpoints, Playwright/browser wiring, WFP/profile
qualification, Provider execution and external sources remain open.

