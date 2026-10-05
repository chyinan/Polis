# Slice 228 — REQ-39 Planning impact-analysis gap audit

## Finding

The approved C-GUIDANCE contract says the program can calculate explicit dependencies but cannot claim full understanding of natural-language impacts; uncertain scope receives bounded analysis from the fixed Planning role, and high-risk scope is conservatively held. Current formal change handling pins deterministic Task/input/Artifact/writer snapshots, but no Planning analysis record or Worker tool exists.

Source evidence:

- `internal/kernel/mission_change_request_contract.go`: `MissionChangeImpact.NaturalLanguageImpactStatus` is validated only when equal to `not_assessed`.
- `frontend/src/pages/MissionChangeRequestPanel.tsx`: the request view states that natural-language dependencies have not been assessed.
- `spec/design-v0.4.5/contracts/C-GUIDANCE.md`: uncertain natural-language impact requires bounded Planning analysis and conservative handling of critical scope.
- `internal/kernel/mission_change_request.go`: `TXConsiderMissionChangeRequest` re-computes the structured impact and safe boundary, but records no Planning-authored analysis.

## Current local state

- Read-only PostgreSQL query against the Termux development database: `worker_sessions` active count `0`, total count `0`.
- `GET http://127.0.0.1:8080/healthz`: `{"service":"polis_backend","status":"ready","version":"r0.7"}`.
- No WorkerSession was created, started, or used. No analysis was submitted. No test or frozen scenario ran.

## Disposition

This audit does not close REQ-39. It identifies the next local implementation seam: an immutable bounded Planning analysis, tied to the exact change/request basis and accepted only from a database-bound active `emp-planning` WorkerSession, with operator review before apply. Keep any new surface isolated and fake-only until separately qualified; preserve real-provider @4 authorization. The current lack of an active WorkerSession blocks exercising that path, not source implementation.

The eight REQ-39 scenarios (`PP-09`, `UI-45`, `WF-21`–`WF-25`, `WF-36`) remain `partial` / `not_run`. All 232 frozen scenarios remain `not_run`.
