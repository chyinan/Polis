# R1 artifact delivery browser path

## Browser path and authorization

The Task page's “下载含完整 Manifest 的 ZIP” action fetches the qualified delivery manifest and package through `RealWorkbenchApi`. `DesktopRuntimeGate` passes the ephemeral desktop session token into the API client; manifest and package paths are token-gated by `desktop.Middleware`. The browser smoke uses a local token-protected fixture, verifies a tokenless manifest request returns HTTP 401, then clicks the real Workbench button with a session header and observes the ZIP download event.

The browser verifies the embedded manifest digest against `X-Polis-Manifest-SHA256`, the package digest against `X-Content-SHA256`, the artifact bytes against the manifest SHA-256/size, the checksum file, and the exact ZIP entry set. The fixture runs the production `desktop.Middleware`; a separate Workbench handler test verifies the production route names and response headers. A dedicated PostgreSQL/CAS integration test verifies that the read store builds the package from the qualified Artifact and manifest.

## ZIP format compatibility

The browser ZIP reader intentionally accepts stored entries only and rejects data-descriptor ZIP entries. Go's `archive/zip.Writer.CreateHeader` emits data descriptors for streamed files, even when a CRC and sizes are known. `buildArtifactDeliveryPackage` now uses `CreateRaw` with the already computed CRC and sizes and clears the descriptor flag. The package contract test rejects a data-descriptor flag, and the browser smoke confirms the generated format parses and downloads successfully.

## Smoke harness boundary

The Python browser runner pins the frontend URL, API URL, and desktop session token to the local fixture constants and rejects environment overrides before registering its shutdown hook. The Go fixture accepts only `127.0.0.1:18084` and the fixture-only token. The PowerShell wrapper sends an authenticated shutdown request in `finally`, including when Vite fails before the browser runner starts; the Python runner also requests shutdown on its own exit. PowerShell saves its prior process environment values, applies smoke-only values after port preflight, and restores all of them in `finally`.

## Verification scope

The browser fixture uses deterministic local artifact bytes and the actual desktop token middleware. It does not use a packaged Tauri runtime, production database, real model, or external provider. The companion PostgreSQL test uses the fake Worker to create a qualified local Artifact and verifies package contents from CAS. Evidence is in `evidence/development/r1-r3-implementation-validation-20260925-slice-26/`.
