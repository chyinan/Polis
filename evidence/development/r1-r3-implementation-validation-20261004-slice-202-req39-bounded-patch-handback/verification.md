# Slice 202 — bounded REQ-39 patch handback

## Scope

The human takeover editor can apply a strict UTF-8 unified diff to the existing frozen `workspace.txt` only. File paths and every hunk are checked before the candidate changes; stale context, multi-file/other-path input, invalid UTF-8, no-op patches, and results over 4 KiB are rejected. The frozen baseline and resulting candidate remain visible side by side. The existing version-bound takeover snapshot is still the only submit path, so patch content is stored as a provenance-bearing MissionInput and never writes the old Task workspace or executes scripts.

## Verification

- `cd frontend && npm run build` — passed (`tsc -b` and Vite production build).
- `git diff --check` — passed.
- Vite reported the existing large-chunk warning (856.14 kB minified JavaScript); build succeeded.
- No automated tests were run.
- No lease, snapshot, WorkerSession, Worker/provider, host, or frozen scenario command was run.
- No schema or frozen design catalog was changed.

## Limits

This is only a single-text-file importer. It does not import multi-file patches, operate on general filesystem trees, establish semantic impact, qualify a live Worker, or complete the end-to-end takeover scenarios. The Kernel's frozen digest/revision and stopped-writer checks remain authoritative at handback.
