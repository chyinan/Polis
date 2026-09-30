# Slice 27 verification — R2 Linux toolchain identity and cleanup retention

Date: 2026-09-25

## Changes covered

- `linux-node-toolchain@3` binds bwrap executable bytes, permission mode and UID/GID ownership, plus curated system-image entry path/type, permission mode, UID/GID, regular file bytes and symlink targets. After its first scan, it verifies the exact rootfs path set and revalidates entry identity/metadata while rehashing regular files. Fingerprinting fails closed if Linux UID/GID fields cannot be read or an entry changes during the scan. Tests verify bwrap and guest-executable mode drift changes the digest, owner-field changes alter the owner fingerprint, and in-flight directory/symlink mutations are rejected.
- Pending preparation cleanup retains the workspace ownership record, pending process handle, and preparation slot if removal fails. It retries removal ten times and deletes those records/releases the slot only after a confirmed cleanup. A fake process and injected remover exercise persistent failure without starting bwrap or project code.
- No schema migration was needed. The Linux profile remains opt-in and unqualified.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/environment ./internal/control -run TestLinuxNode -count=1` | PASS. Linux profile, mode/owner identity, workspace cleanup retention, process and cache cases passed. |
| `bash scripts/go.sh test -race ./internal/environment ./internal/control -run TestLinuxNode -count=1` | PASS. |
| `bash scripts/go.sh test ./... -count=1` | PASS. Database integrations without a dedicated DSN were skipped in this offline run. |
| `bash scripts/go.sh build ./cmd/...` | PASS for Linux. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| `git diff --check` | PASS after evidence and documentation updates. |

All verification was local/offline. No bubblewrap, npm, project script, external registry, real model, QQ, MCP endpoint, GitHub account, or business database was used. Hard per-job CPU/memory/PID/disk quotas, effective namespace/rootfs qualification, service probes, restart recovery, and clean-VM evidence remain open. No Linux executor qualification event was created.
