# Slice 223 — REQ-39 returned directory snapshot successor delivery

Source review traced the directory handback through the existing MissionInput and successor workflow:

1. Directory handback inserts a ready `directory_snapshot` MissionInput and an immutable takeover return receipt in one transaction.
2. `missionInputClonesTx` selects the latest ready MissionInput revision. Its lease-event join identifies returned takeover inputs and labels their mapping origin `human_takeover`.
3. Applying the reviewed formal change request clones the source kind, archive metadata and content digest into the successor Mission. The bytes remain in the same Company-scoped CAS.
4. Starting a successor Task binds its MissionInputs in the immutable Task input manifest. `directory_snapshot` delivery reopens and verifies the archive, then applies the established path, per-file and context limits when preparing Worker input.

The Workbench already listed the human-returned input among the formal change impact inputs, but its apply summary counted only `mission_input` and `task_workspace` mapping origins. The summary now separately counts returned human takeover snapshots so the operator can see them included in the successor.

The live traceability crosswalk now cites the Slice 222 handback and this successor-delivery evidence on the eight REQ-39 frozen scenarios: `PP-09`, `UI-45`, `WF-21`–`WF-25`, and `WF-36`. Their dispositions stay `partial`, their execution status stays `not_run`, and all 232 scenario records remain unchanged otherwise.

## Verification

- `npm run build` passed (`tsc -b` and Vite production build).
- Crosswalk JSON parsed successfully; its total stayed at 232, every execution status stayed `not_run`, and all eight REQ-39 scenario rows retained `partial` while gaining both evidence references.
- `git diff --check` passed.
- No tests were run. No migration, database operation, formal change request, Mission, Task, WorkerSession/provider activity or frozen scenario ran.

The source path for successor delivery exists, but this audit does not qualify the live handback-to-successor workflow or the frozen scenarios. REQ-39 remains partial pending natural-language impact assessment and scenario qualification.
