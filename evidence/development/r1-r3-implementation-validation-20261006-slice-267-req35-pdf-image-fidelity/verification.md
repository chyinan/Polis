# Slice 267 — REQ-35 PDF embedded-image fidelity limits

Date: 2026-10-06

## Change

New direct PDF snapshots use extraction schema `polis-pdf-extraction@3`. The strict visual path rejects stencil images and filtered codecs other than DCT, and checks unpacked sample length against width, component count, bit depth, and byte padding at each row before decoding. When the parser reports more page images than it returns, extraction records an omission warning and marks the image representation truncated. Image numbers are explicitly described as one-based ordinals in the parser's page image array, not PDF object IDs.

The extraction record and Worker prompt state that the bytes represent only an intrinsic embedded-image object. The pinned parser does not expose or apply soft masks/transparency, color-key masks, or rendering intent, so the visible PDF appearance may differ. Original PDF and text preservation remain intact; no full-page rendering or OCR was added. Canonical verification retains the `@1` and `@2` formats. The active Task/Worker path excludes images from those historical formats, while legacy delivery receipt reconstruction remains supported.

## Verification

- `go build ./...` — passed.
- `git diff --check` — passed.
- Integrated `go build ./...` recheck on main after Slice266 — passed.
- Independent read-only review of integrated commit `28f267f` found no compatibility or canonical snapshot/receipt regression.
- No tests were run or added. No database, WorkerSession, provider, scenario, migration, or external service was used.

The implementation was cherry-picked onto main after Slice266 as commit `28f267f`. The source and handoff commits are published together for this stage.
