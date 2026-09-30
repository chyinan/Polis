# Slice 12 — R2 PDF MissionInput and Worker delivery

Date: 2026-09-24

## Implemented

- Direct PDF upload now uses `PrepareMissionInput` and stores a canonical `pdf_snapshot` package containing the original PDF, a versioned extraction record, and extracted text when available. The outer CAS digest and inner per-file manifest bind the complete package; verification deterministically rebuilds it from the original PDF.
- Extraction pins `github.com/giraffesyo/pdf@v0.7.0`, accepts PDF sources up to 6 MiB, processes at most 40 pages with bounded parser budgets and an 8-second limit, and stores at most 16 KiB of cleaned UTF-8 text. The record binds parser version, source/text hashes, warning codes, page coverage, image pages, truncation and no-text outcomes. OCR is not included.
- Schema 26 admits `pdf_snapshot`. Kernel startup and Workbench read-store gates now require schema 26. The existing frozen Task manifest, archive extraction, per-file receipt, Workbench validation and file picker support the new source kind. The Worker context includes extracted text and metadata; it explicitly excludes the original PDF as `representation_not_supported`.
- PDFs nested in directory/ZIP packages remain opaque unsupported binary files and do not trigger recursive parsing.

## Verification

- `bash scripts/go.sh test ./internal/intake ./internal/control ./internal/kernel ./internal/workbench -count=1` — passed.
- `bash scripts/go.sh test -p 1 -count=1 ./...` — full Go suite passed serially.
- `bash scripts/go.sh build ./cmd/...` — all command binaries built.
- A new isolated temporary PostgreSQL 18 cluster applied migrations 00001 through 00026. With a non-superuser `polis_runtime` test role, these tests passed sequentially against the disposable schema-26 database:
  - `TestProductWorkerReceivesExtractedTextFromBoundPDFInput` — captured the extracted PDF sentinel and manifest digest in the fake Worker prompt, verified `local_context_loaded`, verified the original-PDF exclusion and `provider_egress=0`.
  - `TestWorkbenchMissionInputUploadStoresAndListsWithoutStartingMission` — verified HTTP PDF upload, CAS package storage/hash verification, and source readback.
  - `TestZIPInputIsDeliveredToWorkerAndReadBackPerFile` — verified PDF extracted-text inclusion and original-PDF exclusion in Workbench per-file receipt projection alongside ZIP inputs.
- `npm test -- --run` — 10 files, 57 tests passed. `npm run typecheck`, `npm run lint`, and `npm run build` — passed. Vite emitted its existing advisory for a 537.45 kB minified JS chunk.
- `bash scripts/go.sh mod tidy`, `git diff --check`, and `git diff --check` after documentation updates — passed.
- The temporary PostgreSQL cluster was stopped and its uniquely named `/tmp/polis-r2-pdf-test-20260924183740860` directory was removed.

## Limits

This is a bounded local support foundation, not R2 release qualification. No real model, OCR, external account, GitHub request, QQ send, MCP connection, or production operation was used. PDF rendering, encrypted PDFs, cross-backend use, multi-day recovery, and broader R2/R3 evidence remain open.
