# Slice 310 — Research search result contract

Date: 2026-10-07

## Scope

Added a pure normalization contract for future registered-source search
adapters. Results are capped at 20, must remain on the exact registered HTTPS
origin, reject duplicates and empty/oversized fields, require an RFC3339 source
time, preserve deterministic rank order and receive a stable content digest.
No search provider or ranking backend is selected or called; the existing fake
@19 surface remains explicitly unavailable.

## Verification

```text
./scripts/go.sh test ./internal/research -count=1
ok   polis/internal/research  0.030s
```

The tests cover same-origin normalization, cross-origin/missing-time rejection,
duplicate rejection, field bounds and deterministic result digests.

## Boundary

This is the source/result contract only. Provider-specific search endpoints,
ranking semantics, external retrieval and evidence Artifact production remain
separate gates; no network request or Worker/Provider action ran.

