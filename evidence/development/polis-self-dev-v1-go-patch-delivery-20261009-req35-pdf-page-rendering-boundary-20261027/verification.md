# Slice 332 — REQ-35 explicit PDF page-rendering boundary

Date: 2026-10-27
Result: `FULL_PAGE_RENDERING_EXPLICITLY_UNSUPPORTED`; no renderer or network retrieval was added.

## Implemented

- The pinned `github.com/giraffesyo/pdf@v0.7.0` API in the current runtime exposes bounded page text and intrinsic embedded image objects, not a complete raster/compositor. No page-renderer dependency is present in `go.mod` or the cached pinned parser source.
- New direct-PDF snapshots use `polis-pdf-extraction@4`, with `fullPageRenderingStatus=unsupported`, reason `renderer_not_in_runtime_envelope`, and warning `full_page_rendering_unsupported`. This explicitly distinguishes page text/embedded objects from a complete page raster and prevents consumers from inferring page-image coverage.
- No synthetic “page image” is created by placing embedded objects on a blank canvas. Original PDF bytes, extracted text, existing embedded image input semantics and their bounds remain unchanged.
- Canonical verification remains byte-for-byte compatible with historical extraction `@1`, `@2`, and `@3`; `@3` intrinsic embedded images remain eligible for existing delivery. `@1`/`@2` active-delivery exclusions are unchanged.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Frontend typecheck, ESLint, Vite build, and Vitest: PASS — `frontend-typecheck.log`, `frontend-lint.log`, `frontend-build.log`, `frontend-tests.log` (19 files, 171 tests).
- Migration hash-manifest regression test: PASS — `migration-manifest-test.log`; no database schema migration was needed.
- Intake tests cover explicit unsupported status/warning on a new vector-only PDF, absence of page-raster files, byte-canonical verification of an extraction `@3` snapshot, and preservation of active embedded-image semantics for `@3`/`@4` while `@1`/`@2` remain excluded.
- No new dependency was installed, no external registry was contacted, and no Worker/provider operation ran.

## Remaining REQ-35 gate

Actual full-page rasterization remains open. It requires a bounded renderer dependency/license and runtime compatibility decision, pixel/output budgets, and validated fidelity/omission semantics, followed by separate image-profile and authorized Worker qualification. Until then, the structured unsupported status is the supported contract.
