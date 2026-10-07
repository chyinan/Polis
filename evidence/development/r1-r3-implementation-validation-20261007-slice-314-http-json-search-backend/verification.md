# Slice 314 — Explicit HTTP JSON search backend

Date: 2026-10-07

## Scope

Added an explicit `HTTPJSONSearchBackend` implementation for a future
provider/source endpoint. It binds the endpoint to the registered HTTPS
origin, sends only the bounded `q` parameter with JSON Accept, refuses
redirects and cross-origin responses, rejects unknown JSON fields and response
overflow, and delegates result validation to the Slice310 contract. It is an
injected dependency; no endpoint or network client is configured by default.

## Verification

```text
./scripts/go.sh test ./internal/research -count=1
ok   polis/internal/research  0.031s
```

Tests use an in-memory RoundTripper and cover query encoding, no credentials,
redirect rejection and unknown-field rejection. The full Go suite and command
build were rerun after this change and passed.

## Boundary

No provider endpoint, external network, Worker, Provider or account was used.
Provider authentication, endpoint qualification, ranking policy and runtime
adapter injection remain open.

