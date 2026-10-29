# Slice 337 — REQ-35 PDF renderer candidate review

Date: 2026-10-29
Result: `NO_RENDERER_SELECTED`; retain `fullPageRenderingStatus=unsupported`.

## Question and decision

Can a current renderer be added to Polis's Go runtime with a verified license/notice bundle and a bounded, host-independent memory/filesystem envelope?

No candidate met that bar from the reviewed evidence. The current PDF @4 unsupported contract remains safer than adding a binary whose exact third-party notices and effective runtime limits have not been verified. No dependency or binary was installed or downloaded, and no PDF was rendered.

## Candidates

### `github.com/klippa-app/go-pdfium@v1.21.4`

- The tag is published 2026-10-21. Its `go.mod` declares Go 1.26.0 and toolchain 1.27.1; Polis currently declares Go 1.26.0.
- The upstream Go package is MIT-licensed and offers a CGO-free WebAssembly backend using Wazero. Its embedded `internal/pdfium_wasm/pdfium.wasm` is 5,828,543 bytes and is built from the separate `jerbob92/pdfium-binaries` `wasm-standalone` branch.
- The go-pdfium README describes PDFium as Apache-2.0. The exact wasm fork has no GitHub SPDX license metadata; its `LICENSE` is the binary-fork project's MIT notice, and the go-pdfium tag tree contains no separate PDFium/third-party notice file. The canonical PDFium source `LICENSE` contains BSD and Apache terms; upstream binary-maintainer discussion documents that linked third-party components can require additional notices. The exact license/notice set for this embedded wasm artifact therefore remains unverified. This records an evidence gap, not a legal conclusion.
- The versioned Wazero adapter mounts the full root filesystem by default (`/` on Linux, the current volume on Windows) unless an explicit `FSConfig` is supplied. The README documents a 32-bit wasm ceiling: a single bitmap allocation below 2 GiB and a module ceiling up to 4 GiB. Although the API accepts a custom Wazero runtime configuration, Polis has not validated a lower memory limit, context interruption, an empty filesystem configuration, process-wide concurrency, or aggregate rendered-output caps.
- The native backend is not a substitute here: it requires CGO and a platform PDFium library installation.

### `github.com/gen2brain/go-fitz`

- Upstream repository metadata identifies the project as AGPL-3.0 and its binding uses MuPDF.
- No commercial license or product approval was supplied. It was not selected.

## Sources

Primary repository/API sources:

- [go-pdfium v1.21.4](https://github.com/klippa-app/go-pdfium/tree/v1.21.4) — README, module metadata and embedded wasm tree.
- [go-pdfium v1.21.4 Wazero adapter](https://github.com/klippa-app/go-pdfium/blob/v1.21.4/webassembly/webassembly.go) — runtime configuration and default filesystem mounts.
- [go-pdfium native vs. WebAssembly notes](https://github.com/klippa-app/go-pdfium/blob/v1.21.4/NATIVE_VS_WEBASSEMBLY.md) — provenance and backend behavior.
- [PDFium source license](https://pdfium.googlesource.com/pdfium/+/refs/heads/main/LICENSE).
- [PDFium wasm binary fork](https://github.com/jerbob92/pdfium-binaries/tree/wasm-standalone) — build patches and fork metadata.
- [PDFium binary third-party license discussion](https://github.com/bblanchon/pdfium-binaries/issues/41) — maintainer and contributor discussion of linked components/notices; supporting evidence, not a substitute for an artifact-specific inventory.
- [go-fitz](https://github.com/gen2brain/go-fitz) — upstream AGPL-3.0 project.

The agent-reach doctor reported Exa unavailable and GitHub CLI active. Read-only GitHub repository/API queries and web search were used. No project source, lockfile, dependency graph, runtime qualification, or execution policy changed in this review.
