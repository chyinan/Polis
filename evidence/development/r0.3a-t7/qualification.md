# R0.3A-T7 Native Execution Qualification Gate

Status: `PASSED` for the local runtime gate implementation and deterministic qualification.

No Medium, High, reset, or provider call was used.

## Layered model

- L0: native process qualification;
- L1: base provider transport qualification;
- L2: dynamic tool-surface qualification;
- L3: employee/business capability qualification.

Business execution requires an exact current L1 qualified record. L2 tool-surface qualification is separate and does not promote or repair L1.

## Execution key

The stable fingerprint covers:

- Codex/app-server version;
- binary SHA256;
- model and effort;
- runtime profile;
- sandbox class;
- proxy configuration digest;
- auth source classification without secrets;
- code-mode-host SHA256;
- capability digest;
- native protocol digest.

Business tool-surface digest is intentionally excluded from the L1 key and is handled by L2.

The current T6 combination is recorded in `qualification-state.json` as:

```ini
qualification = unqualified_for_business_execution
evidence_result = inconclusive
reason = no_first_valid_output_after_structured_reconnects
```

This preserves the epistemic result `INCONCLUSIVE` while making the scheduling decision explicit: business execution is blocked.

## Gate behavior

The gate denies `unverified`, `stale`, `inconclusive`, and `unqualified` records before a business allowance is created or a business worker starts. It allows only an exact current execution fingerprint with `status=qualified` and `evidence_result=passed`.

Qualification becomes stale when the fingerprint changes, when the explicit freshness deadline expires, or when the requested L2 tool-surface digest does not match. Older qualified evidence cannot revive a newer stale record. The gate is pure/read-only at decision time, so concurrent readers cannot bypass it and it creates no Message, Obligation, artifact, or allowance.

## Verification

Deterministic tests cover exact allow, T6 inconclusive block, binary/proxy/model/effort/runtime/code-host/capability invalidation, fixture independence, L1/L2 separation, stale-record non-revival, concurrent readers, and no side effects. The business driver entry points now require a qualification record and current execution manifest before `NewBudget` or business setup.

Commands used:

```text
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test ./internal/codex ./internal/probe ./internal/kernel -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh test -race ./internal/codex ./internal/probe ./internal/kernel -count=1
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh vet ./internal/codex ./internal/probe ./internal/kernel
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/go.sh build ./cmd/...
rtk git diff --check
```

T6 evidence, T2/T3 allowances, all R0.1/R0.2/H2/H3 evidence and historical hashes were not modified. T7 does not authorize another canary; any future transport run requires a new explicit allowance and a fresh execution combination record.
