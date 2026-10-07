# Slice 312 — Concrete operation adapters

Date: 2026-10-07

## Scope

Added concrete optional adapters behind the Slice311 executor seam:

- `PlaywrightOperationAdapter` runs the bounded Playwright protocol, stores its
  page snapshot as an `operation_evidence` Artifact and returns the BrowserRun
  completion input.
- `ResearchFetchOperationAdapter` resolves the authorized Mission source,
  runs the bounded fetcher, stores the response body as a page-snapshot
  Artifact and returns the ResearchOperation evidence envelope.
- Schema125 admits immutable `operation_evidence` Artifacts up to 8 MiB while
  preserving deliverable/workspace constraints.

Adapters are explicit dependencies; no constructor or runtime enables them by
default.

## Verification

```text
./scripts/go.sh test ./internal/control ./internal/kernel -run "(Research|Browser|Adapter|OperationEvidence)" -count=1
ok   polis/internal/control  0.097s
ok   polis/internal/kernel   0.070s
```

The full Go suite and `build ./cmd/...` were rerun after this slice and passed.
No adapter was injected, no browser or HTTP request ran, and no database
migration was applied.

## Boundary

Search remains without a provider-specific backend. Browser/profile/WFP host
qualification, external source qualification, active Worker/provider use and
runtime Schema125 qualification remain open.

