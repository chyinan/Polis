# REQ-35 in-house PDF renderer plan

Status: active; Slice339 completed the parser fork and bounded operator-name audit. This plan does not claim full-page rendering is implemented or qualified.

## Goal

Add complete-page raster input without falsely presenting extracted text or intrinsic embedded images as a page. A page may be marked `rendered` only when every painting operation affecting its visible output was parsed and rendered under fixed resource limits. Any unsupported, malformed, truncated, or over-budget operation keeps that page explicitly incomplete/unsupported.

## Constraints

- Preserve canonical verification and delivery of historical PDF extraction `@1`–`@4` snapshots.
- Keep the original PDF bytes and bind every derived raster to those bytes, parser revision, page identity, dimensions, color model, and raster digest.
- Keep parsing sequential and bounded by source size, page count, per-page operator/stream count, time, memory, pixels, encoded output, and total package size.
- Do not execute PDF JavaScript, access host files/network, use ambient fonts, or claim OCR.
- Never compose only embedded images on a blank canvas and label the result a page.
- No Worker/Provider change until independent tests prove the image input representation and delivery bounds.

## Increment sequence

### Slice339 — pinned parser fork and operator-coverage audit (complete)

- Copy the exact `github.com/giraffesyo/pdf@v0.7.0` module, including its license and tests, under `third_party/giraffesyo-pdf`; preserve the original module path with a local `replace`.
- Add an opt-in bounded page operator-name audit to the parser. It reports a sorted unique inventory and explicit `complete=false` when parsing, stream, name-count, or name-length bounds prevent a complete inventory.
- Keep the audit disabled by default so existing extraction behavior and canonical @1–@4 outputs remain unchanged.
- Verify the fork's own tests and Polis's full Go/race/vet plus Linux/Windows command builds.

Delivered API: `Options.AuditContentOperators`; `Page.ContentOperatorNames`; and `Page.ContentOperatorNamesComplete`. The audit inventories unique names only and never implies renderer coverage.

### Slice340 — typed bounded display list

- Add a per-page ordered IR for graphics-state changes, path construction/painting, text runs, images, and form boundaries. Values must be resolved/bound to the exact page resources and remain within explicit operand/byte/event budgets.
- Preserve unknown operation names and mark affected pages incomplete. Do not infer support from a parser not returning an error.
- Test nested forms, multiple content streams, malformed operands, repeated resources, and all budget boundaries with generated PDFs.

### Slice341+ — renderer coverage in small operator families

- Implement deterministic Go rasterization in independently testable families (page transforms/background; paths/strokes/fills/clips; text/glyph outlines; images/masks/color; forms/annotations/blend state).
- Each family adds golden tests and generated malformed cases. The page completeness decision is the conjunction of coverage for every observed operation and its effective state.
- Keep a page unsupported until the renderer covers all content that can affect its visible appearance. Omitted optional content must be explicit in metadata and must not yield `rendered`.

### Integration gate

- Only after the renderer has a stable bounded contract, add a new PDF snapshot schema (never rewrite @1–@4), canonical revalidation, Worker page-image delivery, and an owner-visible representation/omission summary.
- Verify renderer corpus equality against independent reference output for the supported profile, then run fuzz/race/Windows+Linux build checks and a disposable database flow. Provider and host qualification remain separate gates.

## Current next increment

Slice340 will add a typed, ordered, resource-bounded display list for a deliberately limited operation family. It must retain unknown names and report incomplete coverage; it does not rasterize pages or change the current `unsupported` product status.
