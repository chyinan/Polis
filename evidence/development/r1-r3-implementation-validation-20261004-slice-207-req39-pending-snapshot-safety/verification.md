# Slice 207 — REQ-39 pending snapshot safety

## Source review

The Workbench stores an unresolved snapshot return as a pending immutable payload containing the lease ID, generated request ID, frozen workspace digest/revision, candidate text and effort value. A retry reuses that exact object. Before this slice, the same panel also allowed release of the granted lease while that return remained unresolved; a successful or ambiguous release could strand the pending return against a lease that was no longer granted.

`MissionTakeoverPanel.tsx` now blocks release from both the handler and disabled-button state while a pending return exists. It also locks the effort field with the candidate editor so the visible form cannot suggest that a changed value would alter the already-frozen retry payload. This preserves the exact retry path until its receipt settles. The patch importer itself remains bounded to a strict UTF-8 single-file diff for frozen `workspace.txt`; the existing return path still binds the snapshot to the lease digest/revision and records it as a MissionInput. No broader multi-file or automatic-apply workflow was added.

## Verification

- `cd frontend && npm run build` — passed (`tsc -b` and Vite production build).
- Vite reported the existing advisory that the minified main chunk is 856.16 kB; build succeeded.
- `git diff --check` — passed.
- No tests were run.
- No lease, snapshot, Worker/provider action, host operation, or frozen scenario was run.

## Limits

REQ-39 remains partial: multi-file patches, broader semantic impact analysis, and frozen takeover/return qualification remain open.
