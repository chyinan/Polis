# Slice 306 — Linux runner verifier qualification

Date: 2026-10-07

## Scope

This slice closes the previously skipped Linux `POLIS_GO_ROOT` verifier check
using only the disposable workspace and the existing WSL2 toolchain. It does
not change production source behavior and does not authorize a Worker,
Provider, account, endpoint, or business database.

## Environment

- Host: Windows with WSL2 `Ubuntu-22.04`
- Kernel: `6.6.87.2-microsoft-standard-WSL2`
- Containment helper: existing `/usr/bin/bwrap`
- Go root: `/mnt/d/Programs/Polis-cloud-main/.tools/go`
- Scratch: Go test temporary directories under the test process
- Persistent host policy changes: none

## Commands and results

Targeted verifier test, which previously skipped when `POLIS_GO_ROOT` was
unset:

```text
rtk wsl -d Ubuntu-22.04 -- bash -lc 'cd /mnt/d/Programs/Polis-cloud-main && POLIS_GO_ROOT=/mnt/d/Programs/Polis-cloud-main/.tools/go ./scripts/go.sh test ./internal/runner -run TestIsolatedVerifierRejectsBaselineAndAcceptsValidNeighbor -count=1'
```

Result:

```text
ok  polis/internal/runner  19.938s
```

Full package regression:

```text
rtk wsl -d Ubuntu-22.04 -- bash -lc 'cd /mnt/d/Programs/Polis-cloud-main && POLIS_GO_ROOT=/mnt/d/Programs/Polis-cloud-main/.tools/go ./scripts/go.sh test ./internal/runner -count=1'
```

Result:

```text
ok  polis/internal/runner  18.655s
```

The verifier rejected the constant-zero candidate and the `os.Exit` lifecycle
bypass, accepted the valid formatter, and confirmed that the candidate could
not create the outside `escaped` path. The full runner package passed with the
environment set.

The full repository Go package suite also passed with the same
`POLIS_GO_ROOT` environment, including `internal/control`, `environment`,
`kernel`, `probe`, `provider`, `runner` and `workbench`. The command build
`./scripts/go.sh build ./cmd/...` passed as well.

## Boundary

This is Linux `bwrap` verifier evidence only. It does not qualify Windows
AppContainer/Job Object effectiveness, Windows WFP policy arbitration,
packaged Node/npm materialization, clean-VM recovery, browser profile/network
isolation, or any production Provider path. Frozen scenarios remain
`not_run`.
