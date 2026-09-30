# Slice 26 verification — browser artifact delivery and manifest auth

Date: 2026-09-25

## Changes covered

- The Task page's artifact action was exercised in Chromium through `RealWorkbenchApi` and the actual `desktop.Middleware`. A manifest request without the test session token received HTTP 401; the browser context carrying the token loaded the full Manifest and downloaded the ZIP from the UI button.
- The downloaded package was checked for the exact `artifact.bin`, `manifest.json`, and `SHA256SUMS` entry set. The browser verified the response package digest, embedded Manifest digest, Artifact size/digest and checksum file. Evidence JSON and a screenshot are saved alongside this report.
- `buildArtifactDeliveryPackage` now uses `zip.Writer.CreateRaw` with the known stored CRC/size and clears the data-descriptor flag. This matches `RealWorkbenchApi`'s bounded ZIP parser. A contract test fails if a package entry sets that flag.
- The actual Workbench HTTP handler test verifies unauthenticated manifest/download rejection and the authenticated response headers/body. A dedicated PostgreSQL/CAS test verifies the package built from the actual qualified Artifact read model.
- `RealWorkbenchApi` tests confirm the Desktop session token header is added to both manifest and package fetches. The smoke runner uses the local service lifecycle helper and stops its Vite/Go fixture processes after the run. The Python runner rejects frontend/API URL or token overrides, the Go fixture rejects non-pinned bind addresses, and the PowerShell wrapper sends a fixture-authenticated shutdown request from `finally` even if Vite startup fails before Python begins. It snapshots and restores all process environment values, with smoke values applied only after port preflight.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/workbench ./scripts -count=1` | PASS, including the ZIP flag regression test and Workbench artifact route auth test. |
| `POLIS_TEST_DSN='<dedicated schema-28 runtime DSN>' bash scripts/go.sh test ./internal/workbench -run '^TestQualifiedArtifactDeliveryManifestAndPackageReadFromPostgresAndCAS$' -count=1` | PASS on dedicated PostgreSQL 18.6 / schema 28. Actual qualified Artifact bytes, manifest digest, ZIP entries and checksum file matched the PostgreSQL/CAS reader result. |
| Playwright `scripts/r1-artifact-delivery-browser-smoke.py` via `with_server.py` | PASS. Unauthenticated Manifest status=401; authenticated browser click downloaded `polis-delivery.zip`. Package SHA-256 `f35b9f1c…3465cb`, Manifest SHA-256 `9372537a…ceb1db`, Artifact SHA-256 `89d2ebe1…087783`; browser errors=0. |
| Harness confinement checks | PASS. Direct Python runs rejected an external frontend URL and non-fixture token; Go fixture rejected `0.0.0.0:18084` without opening a listener. |
| Final cleanup and `git diff --check` | PASS. Ports 4173 and 18084 were released after the browser smoke; wrapper `finally` sends authenticated fixture shutdown on post-preflight exits. |
| PowerShell environment restoration | PASS. Full browser smoke restored all eight caller environment values; an occupied-port preflight failed before changing them. |
| Frontend `npm test`, `npm run typecheck`, `npm run lint`, `npm run build` | PASS; 67 tests. Existing >500 kB chunk advisory remains. |
| `bash scripts/go.sh test ./... -count=1` | PASS. |
| Linux `polis`/`polisd`/fixture builds and Windows amd64 `polis`/`polisd`/fixture cross-build | PASS. |
| `git diff --check` | PASS. |

The browser API fixture uses static local Artifact data behind the actual `desktop.Middleware`; the separate PostgreSQL test verifies the actual read-store package. The browser sent a fixture desktop token through its request headers. This does not qualify the packaged Tauri runtime, a clean VM, production data, real model execution, or external providers.

The server helper initially hit an occupied stale fixture port; the smoke was rerun on isolated port 18084 and passed. The stale fixture was stopped. The final smoke runner stops both fixture and Vite processes and verifies their ports are released. The dedicated PostgreSQL cluster was stopped and its exact `/tmp/polis-r1-s26.*` root removed.
