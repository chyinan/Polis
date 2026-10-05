# Slice 265 — REQ-35 bounded PDF embedded-image representation

Date: 2026-10-06

## Change

New PDF uploads use the already pinned, pure-Go `github.com/giraffesyo/pdf@v0.7.0` parser to preserve a bounded subset of embedded page image objects as PNG files inside the canonical `pdf_snapshot` package. The extraction record binds each image to its page and image number, dimensions, content digest, page boxes, rotation, and four page-space placement points. PDF Task context can attach these PNGs as typed image inputs; delivery references retain page number, image number, dimensions, path, and digest and are checked against the immutable package metadata.

Bounds are 40 parsed pages, at most eight image decode attempts per PDF, 1,000,000 pixels per image, 256 KiB per encoded PNG, and 960 KiB total encoded PNG data. Existing parser stream/operator/glyph bounds and the 16 KiB extracted-text cap remain in force. Unsupported and over-budget images are omitted and marked in extraction metadata. The original PDF remains excluded from Worker context. New snapshots use `polis-pdf-extraction@2`; canonical verification retains the legacy `@1` path for previously stored snapshots.

This slice extracts independent PDF image objects. It does not render/composite full pages, vector text, or page backgrounds, and it does not add OCR. PDFs with text converted to vector outlines, or without supported embedded images, still lack visual page input. Real-provider image support and PDF format-matrix qualification remain open.

## Verification

- `go build ./...` — passed.
- `git diff --check` — passed.
- No tests were run or added. No database, migration, WorkerSession, provider, external service, or frozen scenario was used.

The code was developed on `agent/pdf-local-slice`, based on `83592318bc49b6f717d68054da7b213a1fbc63af`. The local runtime/database was not touched.
