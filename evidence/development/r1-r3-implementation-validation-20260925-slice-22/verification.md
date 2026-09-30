# Slice 22 verification — R2 Linux/Node execution foundation

Date: 2026-09-25

## Changes covered

Schema 27 and the R2 Linux/Node profile, policy parser, bounded offline npm cache hashing/copying, `linux-node-toolchain@2` fingerprint, bubblewrap launch plans, opt-in Control preparation/JobRun executor, Linux process adapter, and fingerprint CLI. The frozen design snapshot is unchanged.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/environment ./internal/control -run TestLinuxNode -count=1` | PASS: Linux profile policy, bwrap plan, cache bounds/link denial/copy, fake preparation and batch JobRun argv, toolchain/cache drift rejection, process exit/cancellation, and workspace retention until an unconfirmed process exits. |
| `bash scripts/go.sh test -race ./internal/environment ./internal/control -run TestLinuxNode -count=1` | PASS with the final Linux process-cleanup test included. |
| `bash scripts/go.sh test ./... -count=1` | PASS after the final code changes. Database integration cases without a DSN were skipped in this full run. |
| `bash scripts/go.sh build ./cmd/...` | PASS after final code changes. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS after final code changes. |
| `git diff --check` | PASS. |
| `POLIS_TEST_DSN='host=/tmp/polis-r2-linux-node-20260925/socket port=56436 dbname=polis_r0_r2_linuxnode_20260925 user=polis_runtime' scripts/go.sh test ./internal/control -run TestLinuxProfileDoesNotReuseWindowsExecutorQualification -count=1` | PASS against the dedicated migrated PostgreSQL 18 / schema-27 test database. A Windows profile qualification did not authorize the Linux profile; the fake Windows executor was not called. |

The Linux executor unit test injects a fake launcher; it does not run bwrap, npm, or project code. No qualification event was created. Linux environment preparation remains blocked by the database qualification gate. `provider_egress=0`; no real model, QQ, MCP, GitHub account or external registry request was used.

## Limitations

No effective Linux namespace, Windows native runtime, clean-VM, service health-check, cross-backend handover, multi-day recovery, or R2/R3 domain qualification is claimed. The dedicated temporary PostgreSQL 18 cluster was stopped and its exact test directory removed after integration checks.
