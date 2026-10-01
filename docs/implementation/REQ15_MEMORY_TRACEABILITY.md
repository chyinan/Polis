# REQ-15 memory traceability disposition

Updated: 2026-10-01, Slice 107. This audits only the FT/PP IDs mapped to REQ-15 in the frozen `spec/design-v0.4.5/tests/traceability.json`. It does not change that design package or its test catalog. `execution_status: not_run` remains separate from the implementation disposition below.

| Trace ID | Current disposition | Code and evidence mapping |
|---|---|---|
| FT-27 | Implemented | Task Handover reads durable Task obligations directly from `obligations`; an omitted summary cannot erase this source. `internal/kernel/worker_state.go` (`Kernel.Handover`). Scenario execution remains `not_run`. |
| FT-37 | Unimplemented | No versioned memory record, correction decision, explicit `MemoryDependency`, dependency invalidation, or high-risk dirty-task gate is present. The design contract is `spec/design-v0.4.5/contracts/C-MEMORY.md`; see the finite-work entry in `R1_R3_IMPLEMENTATION_COVERAGE.md`. |
| FT-40 | Unimplemented | Blob/CAS and Artifact foundations exist, but no memory-specific reference/pin lifecycle or concurrent memory-GC protection is wired. No C-MEMORY implementation evidence currently closes this scenario. |
| FT-41 | Unimplemented | There is no durable memory deletion tombstone/revocation overlay applied when importing an older backup. Historical content-removal behavior remains unverified. |
| FT-74 | Partial | Mission change proposals and versioned contract workflows provide adjacent proposal infrastructure (`internal/kernel/mission_change_request.go`), but there is no memory-correction proposal or review flow. Scenario execution remains `not_run`. |
| PP-07 | Partial | Cross-backend Handover and its recovery anchors exist (`internal/kernel/cross_backend_handover.go`); the required behavioral experiment across models, including recovery cost and failure accounting, has not run. The product catalog remains `not_run`. |

This is a traceability audit, not a feature-completion claim. The open REQ-15 implementation work remains explicit correction records, revision history, dependency invalidation, active-context revalidation/freeze, and deletion propagation across restored backups. No tests, model turns, migrations, or external actions were run for this audit.
