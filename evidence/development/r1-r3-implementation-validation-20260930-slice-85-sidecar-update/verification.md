# Slice 85: local sidecar update verification

Date: 2026-09-30

Scope: local Tauri/Go sidecar stage, same-migration-manifest activation, one retained binary, explicit rollback, startup recovery, and stale managed-file cleanup. The active PostgreSQL/data generation pointer remains independent from the sidecar pointer.

## Verification results

- `rtk cargo test --manifest-path desktop/src-tauri/Cargo.toml`: exit 0; 45 tests passed on the Windows host. This exercises Windows write-through file publication and replacement.
- `rtk cargo clippy --manifest-path desktop/src-tauri/Cargo.toml --all-targets -- -D warnings`: exit 0; no findings.
- `rtk cargo check --manifest-path desktop/src-tauri/Cargo.toml --target x86_64-pc-windows-msvc`: exit 0.
- `rtk run bash scripts/go.sh test ./...`: exit 0; all packages passed or reported no test files.
- `rtk run bash -lc 'GOOS=windows GOARCH=amd64 scripts/go.sh build ./cmd/polis ./cmd/polisd'`: exit 0.
- `rtk npm --prefix frontend run typecheck`: exit 0.
- `rtk npm --prefix frontend run lint`: exit 0.
- `rtk npm --prefix frontend test -- --run`: exit 0; 132 tests passed.
- `rtk npm --prefix frontend run build`: exit 0. Vite emitted the existing advisory for a minified chunk over 500 kB.
- `rtk git diff --check`: exit 0.

Rust tests cover exact migration-manifest matching, pinned identity mismatch, candidate hash tampering, flat content-addressed paths, durable file replacement, retention, state persistence, pruning of unreferenced candidates, preservation of active/previous/staged files, and refusal to prune symlinks or non-regular matching entries. They also cover removal of only exact version-4 UUID temp filenames after valid state load/persist, preservation of unrelated config files, pending-operation role validation, and a successful rollback followed by a second rollback to the candidate.

## Limits

The WSL environment has no Cargo binary, so the POSIX rename plus parent-directory sync path and Unix symlink tests were not run. The Windows host tests exercised `MoveFileExW` write-through publication, and the Windows MSVC target check passed. No native Desktop/PostgreSQL runtime or real sidecar update was started; the packaged Windows PostgreSQL runtime is absent from this checkout. The updater rejects a different migration-manifest digest before maintenance quiescence and does not claim or perform database rollback. Publisher signature verification remains unavailable without a valid signing certificate.
