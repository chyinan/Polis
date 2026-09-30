# Slice 84: Desktop generation cutover verification

Date: 2026-09-30  
Schema: 60 (no migration)

## Implemented

The Tauri Supervisor can stage a verified recovery package into a separate PostgreSQL/CAS generation, keep the currently active generation intact, and atomically select the candidate during a maintenance window. The Go backend closes admission to new Workbench requests, drains admitted requests, and checks active work before accepting shutdown. An explicit force path can quiesce with active work only when requested; generation cutover always uses the safe path. The candidate is committed as active only after backend startup and health checks succeed. A failed startup restores the previous generation pointer and attempts to restart the previous services. Spawned PostgreSQL/backend child handles remain tracked until exit is confirmed; uncertain stop blocks pointer rollback. Exactly one previous generation is retained for an explicit operator-confirmed rollback. The Group Settings recovery panel uses the typed desktop bridge for staging, activation, and rollback.

The command and UI flow do not send QQ notices, invoke external providers, connect to MCP or GitHub, or remove retained generation data.

## Verification

- rtk cargo fmt --manifest-path desktop/src-tauri/Cargo.toml --check — passed.
- rtk cargo test --manifest-path desktop/src-tauri/Cargo.toml — 22 passed.
- rtk cargo clippy --manifest-path desktop/src-tauri/Cargo.toml --all-targets -- -D warnings — passed.
- rtk cargo check --manifest-path desktop/src-tauri/Cargo.toml --target x86_64-pc-windows-msvc — passed (254 crates compiled).
- rtk bash scripts/go.sh test ./... -count=1 — passed.
- rtk bash scripts/go.sh test -race ./cmd/polis -run TestDesktopMaintenance -count=1 — passed.
- rtk bash scripts/go.sh test ./internal/runner -run TestReconcileLinuxWorkerProcessGroupWaitsForDescendants -count=10 — passed. The existing process-group test now polls briefly after SIGKILL for OS process-table reaping instead of assuming it is immediate.
- rtk npm --prefix frontend run typecheck — passed.
- rtk npm --prefix frontend run lint — passed.
- rtk npm --prefix frontend run test -- --run — 132 tests passed.
- rtk npm --prefix frontend run build — passed; Vite reported the existing large-chunk advisory.
- rtk git diff --check — passed.
- Final read-only review confirmed the process cleanup and maintenance admission barriers; no remaining Critical, Important or Minor findings.

## Not exercised

The configured bundled Windows PostgreSQL runtime directory is absent from this checkout. Therefore the packaged Tauri application did not run a real backup restore, database-generation switch, rollback, clean-VM install, or tray/browser flow. The local Rust tests and Windows target check are software evidence only. No provider, QQ, MCP, GitHub, or business account was contacted.
