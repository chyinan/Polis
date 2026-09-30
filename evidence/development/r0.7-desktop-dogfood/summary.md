# R0.7 desktop dogfood evidence summary

Date: 2026-09-22

- `windows_runtime_smoke = PASSED`: `r0.7-windows-runtime-smoke-fc4162adf96a472ea00342a7dccc2ad9/result.json`; private SCRAM PostgreSQL, migration, authenticated health, active-work read and shutdown passed; provider business egress and external notifications were zero.
- `packaged_process_smoke = PASSED_WITHOUT_GUI`: bundled PostgreSQL initialized from the release resources, the bundled Go sidecar stayed alive, `goose_db_version=11` was queried from the private database, and force-stopping the desktop left no `postgres.exe` or `polis.exe` children.
- `installer_build = PASSED`: `desktop/src-tauri/target/release/bundle/nsis/Polis_0.7.0_x64-setup.exe`, 38,902,429 bytes.
- `visual_desktop_e2e = NOT_VISUALLY_VERIFIED`: the Windows CUA helper returned `apps:[]` and `nodeRepl.fetch request failed` on two inventory attempts, so no guessed GUI clicks were performed.
