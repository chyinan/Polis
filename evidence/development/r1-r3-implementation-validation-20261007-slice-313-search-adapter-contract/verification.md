# Slice 313 — Search backend adapter contract

Date: 2026-10-07

## Scope

Added an injectable `SearchBackend` contract and a concrete
`ResearchSearchOperationAdapter`. It resolves the authorized Mission source,
normalizes same-origin/timestamped results, stores them as `search_result`
operation-evidence Artifacts and builds the ResearchOperation completion
envelope. No provider-specific backend is supplied by default; absent backend
remains an explicit failure and the fake @19 surface remains unavailable.

## Verification

```text
./scripts/go.sh test ./internal/control ./internal/research -run "(Research|Search|Adapter)" -count=1
ok   polis/internal/control  0.072s
ok   polis/internal/research 0.031s
```

The full Go suite and command build were rerun after this change and passed. No
search backend, network, Worker or Provider action ran.

## Boundary

Provider-specific search transport, ranking semantics, host qualification and
adapter injection remain open. The contract cannot claim search availability
until a backend is explicitly configured and qualified.

