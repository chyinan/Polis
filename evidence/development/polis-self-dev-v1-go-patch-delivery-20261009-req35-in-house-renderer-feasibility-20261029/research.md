# Slice 338 — REQ-35 in-house renderer feasibility

Date: 2026-10-29
Result: `NO_COMPLETE_RENDERER_FROM_CURRENT_PUBLIC_PARSER_API`.

## Codebase observations

- Polis pins `github.com/giraffesyo/pdf@v0.7.0` and calls `pdf.ExtractPages` from `internal/intake/pdf_snapshot.go`.
- The dependency's public `pdf.Page` contains extracted `Glyphs`, decoded `Images`, `ImageCount`, page boxes, rotation, annotations and warnings. It exposes no page paint-operator stream, resolved graphics-state sequence, clipping region or render display list.
- Public `pdf.Glyph` reports decoded text and geometric bounds, not font glyph outlines or the complete text-paint state required to reproduce the page.
- Public `pdf.Image` exposes encoded/sample data and `Quad`, but documents that soft masks, color-key masking and rendering intent are not reported. Image placements alone do not describe the entire page.
- The dependency's content interpreter uses unexported `opKind`, `operand`, `contentLexer` and `interpretContentPooled`; its object parser is under Go's `internal/object` package. Polis cannot directly call those internals without maintaining a fork or adding a public upstream parser API.

## Decision

A raster assembled from public Glyph/Image outputs would omit vectors, complete text rendering, clipping and compositing. It would repeat the unsupported blank-canvas approximation, so it cannot be labeled a complete page. No subset renderer or parser fork was created. New PDF extraction @4 remains unchanged and explicitly reports full-page rendering as unsupported.

A truthful self-built route would first need a bounded typed drawing IR from a maintained parser extension/fork, then a renderer that supports every operation on a page or rejects/marks that page incomplete. That is a substantial security-sensitive project, not a small adapter.

## Sources and verification

- [Pinned parser v0.7.0](https://github.com/giraffesyo/pdf/tree/v0.7.0)
- [Public Page/Glyph API](https://github.com/giraffesyo/pdf/blob/v0.7.0/extract.go)
- [Public image API and appearance caveats](https://github.com/giraffesyo/pdf/blob/v0.7.0/image.go)
- [Polis integration](../../../internal/intake/pdf_snapshot.go)

Read-only source inspection only. No code, dependency, migration, database, Worker, Provider or external runtime was changed or run.
