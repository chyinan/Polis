# Slice 315 — Registered search endpoint configuration

Date: 2026-10-07

## Scope

Schema126 extends Mission-scoped research source registration with an optional
same-origin HTTPS `search_endpoint` (no query, fragment or credentials).
Kernel registration/readback has pre-Schema126 compatibility; the concrete
HTTP JSON search adapter can construct its backend from the registered endpoint
only when an HTTP client is explicitly injected. No endpoint or client is
configured by default.

## Verification

```text
./scripts/go.sh test ./internal/control ./internal/kernel ./internal/research -run "(Research|Search|Adapter|Migration)" -count=1
ok   target packages
```

The full Go suite and `build ./cmd/...` were rerun after this slice and passed.
Source/core tests cover endpoint same-origin and query rejection. No migration
was applied and no network request ran.

## Boundary

Provider endpoint selection, authentication, endpoint qualification, adapter
injection and runtime Schema126 qualification remain open.

