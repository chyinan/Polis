# Slice 320 — Explicit operation-adapter configuration

Date: 2026-10-07

## Scope

`cmd/polis serve` now has an explicit, default-off
`POLIS_OPERATION_ADAPTERS_ENABLED=1` configuration gate. When enabled it
requires the Playwright Python/script/profile paths, injects the concrete
BrowserRun adapter and a ResearchOperation fetch/search mux into the real
provider worker adapter. With the flag absent, no adapters are constructed and
the existing blocked/unavailable behavior remains unchanged.

## Verification

```text
./scripts/go.sh test ./cmd/polis ./internal/control -run "(Adapter|RealProvider|Research)" -count=1
ok   target packages

./scripts/go.sh build ./cmd/...
exit 0
```

The full Go suite was rerun after this wiring change and passed. The adapter
flag was not enabled; no browser, HTTP, Worker, Provider, account or network
action ran.

## Boundary

This adds configuration/wiring only. Real host/profile/WFP qualification,
credential resolution, provider endpoint/ranking authorization and actual
adapter execution remain separately gated.

