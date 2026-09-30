# Slice 31 verification — bounded Linux/Node workspace filesystem

Date: 2026-09-25

## Changes covered

- `linux-node-toolchain@5` binds `POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES` (default 16 GiB); invalid explicit values are rejected by `polis` config parsing.
- The production Linux executor requires the workspace root to be on a separate ext-family or XFS filesystem. `statfs` total filesystem capacity must be positive and no larger than the configured bound; the workspace device must differ from the runtime, system image, npm cache and cgroup devices.
- The disk-bound verifier runs before preparation snapshot loading/materialization, before every cgroup process launch, before workspace deletion, at executor startup and from the toolchain fingerprint CLI. A normal directory on the root filesystem fails closed.
- This uses a pre-sized dedicated filesystem for an aggregate hard cap. Polis does not mount, resize or change host filesystem/quota settings.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/environment ./internal/control ./cmd/polis -run LinuxNode -count=1` | PASS. Capacity/type/device validation, fingerprint binding, environment parsing, same-filesystem refusal and preparation denial before snapshot loading passed. |
| `bash scripts/go.sh test -race ./internal/environment ./internal/control ./cmd/polis -run LinuxNode -count=1` | PASS. |
| `bash scripts/go.sh test ./... -count=1` | PASS. Database tests without a configured DSN were skipped. |
| `bash scripts/go.sh build ./cmd/...` | PASS for Linux. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| `npm --prefix frontend run build` | PASS. Vite reported a 558.07 kB JavaScript chunk-size advisory. |
| `git diff --check` | PASS. |

No filesystem mount was created or modified. No cgroup, bubblewrap process, npm command, project script, model, QQ, MCP or GitHub account was used. The Linux profile is still unqualified because no dedicated ext-family/XFS host filesystem or clean VM was provisioned for qualification.
