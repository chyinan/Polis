# REQ-15 memory traceability disposition

Updated: 2026-10-01, Slice 108. This reconciles only the FT/PP IDs mapped to REQ-15 in the frozen `spec/design-v0.4.5/tests/traceability.json`. It does not change that design package or its test catalog. `execution_status: not_run` remains separate from the implementation disposition below.

| Trace ID | Current disposition | Code and evidence mapping |
|---|---|---|
| FT-27 | Implemented | Task Handover reads durable Task obligations directly from `obligations`; an omitted summary cannot erase this source. `internal/kernel/worker_state.go` (`Kernel.Handover`). Scenario execution remains `not_run`. |
| FT-37 | Partial | Schema 74 and `internal/kernel/memory.go` add immutable source-pinned revisions, separate review state, exact dependency pins, and invalidation-event storage. Correction requests/replacement revisions, invalidation emission, dirty/high-risk freeze gates, and clean-context revalidation remain unimplemented. |
| FT-40 | Partial | Schema 74 dependency rows pin a specific memory revision to exact targets. Memory content/CAS retention references, concurrent reference-aware GC protection, and unique recovery-evidence lifecycle remain unimplemented. |
| FT-41 | Unimplemented | There is no durable memory deletion tombstone/revocation overlay applied when importing an older backup. Historical content-removal behavior remains unverified. |
| FT-74 | Partial | Mission change proposals and versioned contract workflows provide adjacent proposal infrastructure (`internal/kernel/mission_change_request.go`). Schema 74 adds a separate first-revision memory proposal/review flow, but correction proposals and revision supersession are not implemented. Scenario execution remains `not_run`. |
| PP-07 | Partial | Cross-backend Handover and its recovery anchors exist (`internal/kernel/cross_backend_handover.go`); the required behavioral experiment across models, including recovery cost and failure accounting, has not run. The product catalog remains `not_run`. |

This is a traceability update, not a feature-completion claim. The open REQ-15 implementation work remains correction requests and replacement revisions, dependency invalidation, active-context revalidation/freeze, retention pins and deletion propagation across restored backups. Slice 108 command build and diff checks passed; tests and PostgreSQL migration execution were not run. No model turns or external actions occurred.
