# Slice 311 — Executor product wiring seam

Date: 2026-10-07

## Scope

The product EmployeeTools path now accepts optional BrowserRun and
ResearchOperation executors. If an executor is injected, successful results
must pass through the Kernel BrowserRun/ResearchOperation completion fences;
executor failures return the already-persisted control record without bypassing
the ledger. `RealProviderWorkerAdapter` exposes setters and passes the injected
adapters into each EmployeeTools instance. The default values are nil, so the
existing BrowserRun `blocked` and research `unavailable` behavior is unchanged.

## Verification

```text
./scripts/go.sh test ./internal/kernel ./internal/control -run "(ProductJob|Research|Browser|RealProvider)" -count=1
ok   polis/internal/kernel   0.069s
ok   polis/internal/control  0.096s
```

The complete Go suite and `build ./cmd/...` were rerun after the wiring change
and passed. No executor was configured, no browser or HTTP request was started,
and no Worker/Provider/account/scenario action ran.

## Boundary

This closes the source wiring seam only. Host/profile/WFP qualification,
provider-specific adapter construction, Artifact production and external
retrieval remain separately gated.

