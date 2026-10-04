# Slice 204 — Windows code-signing integration

## Scope

Added a separate Tauri signing configuration and explicit signed-build wrapper. The signer requires `POLIS_WINDOWS_SIGNING_CERT_THUMBPRINT` and an operator-selected `POLIS_WINDOWS_TIMESTAMP_URL`, checks the current-user publisher certificate's private-key availability, dates and Code Signing EKU, uses Windows SDK SignTool for SHA-256 Authenticode plus an RFC 3161 timestamp, then verifies with the Authenticode policy. Missing certificate, timestamp endpoint, signing tools, or packaged PostgreSQL runtime fail the signed-build path before an installer is produced. Ordinary development configuration is unchanged.

## Verification

- Parsed the signing and base Tauri configuration JSON — passed.
- `cd frontend && npm run build` — passed (`tsc -b` and Vite production build).
- `git diff --check` — passed.
- Vite reported its existing large-chunk warning (856.14 kB minified JavaScript); build succeeded.
- No automated tests were run.
- No Tauri bundle, PowerShell signer, Authenticode signature, timestamp request, Windows installer, or host operation was run.

## Limits

The current environment has no publisher certificate, Windows SDK host, or packaged Windows PostgreSQL runtime. The PowerShell/Tauri packaging path has not been exercised on Windows. Signature-chain/timestamp verification and clean-VM install, update, rollback, and GUI qualification remain open.
