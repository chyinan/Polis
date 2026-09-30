# Slice 97 — Opt-in Windows login startup

Date: 2026-09-30

## Change

Group Settings exposes login-startup status and an opt-in toggle through typed Tauri commands. On Windows, the command stores the quoted current executable path in the current user's `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` value named `Polis`; turning the setting off removes only that value. It is off by default. Other platforms report the feature unsupported. The setting starts the desktop only and has no Mission, Worker, or model-start behavior. The value name matches Tauri `productName`; the pinned Tauri 2.9.4 NSIS template removes that value on normal uninstall and preserves it on upgrade. A packaged uninstall was not run.

The registry helper tests use a unique direct child key under `HKCU\Software`, verify write/read/delete, and clean only that key. They do not modify the real Run key.

## Verification

- `rtk powershell.exe -NoProfile -Command 'cargo test --target x86_64-pc-windows-msvc --lib'` — passed, 48 tests including the isolated registry test.
- `rtk powershell.exe -NoProfile -Command 'cargo build --target x86_64-pc-windows-msvc --release'` — passed without compiler warnings.
- `rtk powershell.exe -NoProfile -Command 'cargo fmt --check'` — passed.
- `rtk powershell.exe -NoProfile -Command 'npm --prefix frontend run test'` — passed, 135 tests.
- `rtk powershell.exe -NoProfile -Command 'npm --prefix frontend run lint'` — passed.
- `rtk powershell.exe -NoProfile -Command 'npm --prefix frontend run build'` — passed; Vite emitted the existing large-chunk advisory. The build's TypeScript check passed.
- `rtk git diff --check` — passed.

Clean-VM startup, install/uninstall, stale Run-entry cleanup, and signed-package qualification remain unverified. No actual user Run key was changed, no provider/Worker was started, and no external account or production resource was used.
