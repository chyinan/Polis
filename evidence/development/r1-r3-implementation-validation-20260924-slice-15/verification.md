# R1–R3 implementation validation — Slice 15

Date: 2026-09-24

## Scope

- Delete only the registry proxy permit filter from the WFP dynamic session while retaining the IPv4 and IPv6 AppContainer default-block filters.
- Retry permit deletion after a transient API failure. Prevent new AppContainer launches if permit revocation committed but loopback SID cleanup did not complete.
- Expose an Environment helper that revalidates the immutable Node/npm project plan before creating a registry-only snapshot, derives npm-ci argv from the frozen plan, checks toolchain path containment, and exposes registry-egress revocation before later deny-all JobRuns.
- No Control/JobRun wiring, trusted Node/npm bundle staging, actual npm execution, WFP installation, loopback configuration change, or external network request is included.

## Verification

- Red: Windows runner test cross-compilation failed at the new `RevokePermit` regression because the WFP API interface still only supported filter addition/session close.
- Green: `rtk bash scripts/go.sh test -race ./internal/runner -count=1` — PASS (`polis/internal/runner`, 1.923s).
- Green: `rtk bash scripts/go.sh test ./internal/environment -count=1` — PASS (`polis/internal/environment`, 0.934s).
- Green: Windows amd64 runner and environment test binaries cross-compiled successfully.
- Green: native Windows full runner test package — PASS; the injected WFP API tests verify exact permit deletion, retained block session, idempotency and retry after delete failure.
- Green: native Windows full environment test package — PASS; no registry-only constructor is invoked.
- No system WFP filter was installed, no loopback SID list was changed, no trusted toolchain was copied, and no npm process or external network request ran.

## Remaining boundary

`PrepareWindowsNodeNPMInstallSnapshot` and permit revocation are code foundations only. EnvironmentPreparation and JobRun do not invoke them. Toolchain discovery/staging and matching the configured toolchain digest to staged executable bytes remain open. Actual WFP rights, filter arbitration and clean-VM recovery are unqualified.
