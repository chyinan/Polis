# Slice 224 — REQ-23 closeout coverage reconciliation

Source review found the coverage row still described the formal closeout outcome, responsibility settlement and owner controls as missing even though later stages implemented them.

- Schema 101 adds durable `closing` and `ended_not_met` states and an immutable closeout intent/report.
- The Kernel requires exact ready independently passed Artifacts and settled Tasks/Obligations before admitting or finalizing `succeeded`; non-success outcomes explicitly settle unfinished Tasks, Obligations and Routine occurrences.
- Control records the intent before stopping project Jobs and Workers, checks for outstanding work, then finalizes. A stopped-process failure leaves the Mission in durable `closing`; the Workbench can resume using the recorded request ID and intent.
- The owner-facing Workbench offers `ended_not_met` and evidence-backed `succeeded` decisions, and projects the closeout report.

The live coverage row now records this implemented behavior accurately. It keeps REQ-23 partial for restart/recovery qualification and the mapped FT-57–60/72 frozen scenarios. Slice 224 evidence is linked from all seven mapped crosswalk rows; no disposition or execution state changed.

## Verification

- Crosswalk JSON parsed successfully; exactly seven scenarios map to REQ-23, all remain `partial` / `not_run`, and all 232 scenario execution states remain `not_run`.
- `git diff --check` passed.
- No tests were run. No migration or database operation, closeout command, Worker/provider action or frozen scenario ran.

This is a source/ledger reconciliation only. It does not qualify host restart recovery or the exact REQ-23 scenarios.
