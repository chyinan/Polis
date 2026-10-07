# Slice 316 — Search auth/ranking policy metadata

Date: 2026-10-07

## Scope

Schema127 adds opaque `search_credential_ref` and fixed
`research-ranking@1` metadata to sources that have a search endpoint. The
credential ref stores no secret. The HTTP JSON backend accepts an explicitly
injected credential provider, applies its headers only after rejecting Cookie,
Set-Cookie, proxy-auth, Host, connection and content-length headers, and keeps
the ranking contract deterministic. Pre-Schema127 reads remain compatible.

## Verification

```text
./scripts/go.sh test ./internal/research ./internal/control ./internal/kernel -run "(Search|Research|Adapter|Migration)" -count=1
ok   target packages
```

The full Go suite and `build ./cmd/...` were rerun after this slice and passed.
Tests use an in-memory client/provider; no credential, endpoint or network was
used.

## Boundary

Credential storage/resolution, provider endpoint authorization, ranking
qualification and runtime Schema127 qualification remain open. No secret is
persisted by this feature.

