# R0.5B3 Product Employee Surface Audit

The B2 provider-visible manifest was `730b9b2aba9de7ca015e800b1a597de37a22bd0c299dc7ad10e022c8399388d9` (7 tools; schema 1318 bytes; schema digest `12008e917b4b5a71dc5fa70e8c2388450205bc08fce1bddc84bae3742e155717`). Its evidence remains frozen. Product runtime now registers `codex.ProductEmployeeTools`; legacy `codex.Tools()` and peer registries remain isolated for historical consumers.

## Previous seven tools

| Tool | Previous public contract | Dispatcher / authorization / backing operation | Classification |
| --- | --- | --- | --- |
| `work_current` | Read authoritative task, responsibility and neutral handover context | `EmployeeTools.call` → `Kernel.Handover`; trusted adapter supplies the session `Binding`, and guarded read checks employee/session/task | `GENERIC_OK` |
| `context_read` | Read approved context, facts and decisions | Same Handover projection; no caller-selected task or employee | `GENERIC_BUT_DESCRIPTION_STALE` — “approved” was not a precise description of the full current-task/handover projection |
| `workspace_read` | Read the sole allowed `formatter.go` and digest | Handover reads the task's one content blob plus digest/revision, after session authorization | `TASK_SPECIFIC` — named a fictitious product file and did not describe the actual single-blob workspace |
| `workspace_replace` | Replace `formatter.go`; formatting-only/import/source restrictions; expected digest | `TXReplace` wrote the authorized session's blob with digest-only CAS | `SEMANTICALLY_WEAK` — task-specific restrictions were absent from the dispatcher, and revision/ABA conflict was not expressed |
| `workspace_check` | Run frozen task checks | Adapter-injected `CheckRunner`; product registration supplied `OfflineProductCheckRunner`, which passed any nonempty `single`-phase content | `SEMANTICALLY_WEAK` — checker selection was hidden and acceptance was not meaningful |
| `work_checkpoint` | Persist progress/qualified checkpoint; passed check receipt qualifies | `TXCheckpoint` persisted session/workspace evidence and consulted peer-contract rules, but had no typed product Task validation binding | `GENERIC_BUT_DESCRIPTION_STALE` — product acceptance was not bound to a public Task contract |
| `artifact_submit` | Submit current fixed file; independent controller acceptance | `TXSubmit` staged/published the current Task blob as candidate after a separate Handover checkpoint check | `TASK_SPECIFIC` — fixed-file and independent-verifier language did not describe the current product submission/finalization path |

## Current product contract

| Tool | Current public meaning | Typed input / backing operation |
| --- | --- | --- |
| `work_current` | Read current authorized Task, public validation binding, handover and budget | No arguments; session-bound Handover |
| `context_read` | Read approved context and persisted facts/decisions for this session | No arguments; session-bound Handover |
| `workspace_read` | Read current Task workspace content, digest and revision | No arguments; one-blob workspace, no invented filesystem path |
| `workspace_replace` | Replace full current Task workspace; current worker is sole writer; stale digest or revision conflicts | Requires `expected_digest`, `expected_revision`, `content`; revision-aware CAS and persisted receipt; Task must remain `working` in an active Mission |
| `workspace_check` | Validate current workspace against the Task's bound public acceptance contract | No arguments; returns `PASS`, `FAIL`, `VALIDATION_NOT_CONFIGURED`, `VALIDATOR_UNAVAILABLE`, or `INFRASTRUCTURE_ERROR`; only while the Task is `working` |
| `work_checkpoint` | Persist progress or qualification evidence; qualified requires a current passing receipt for the exact Task binding/session/workspace revision | Existing typed checkpoint input; transactional receipt validation and `working`-Task fence |
| `artifact_submit` | Submit current workspace as a candidate only with a matching current qualified checkpoint/binding | No arguments; product finalizer revalidates checkpoint and check receipts in the artifact publication transaction; candidate transition fences later writes/checks/checkpoints even while the provider session is active |

Product runtime authorization remains in the trusted adapter and kernel guard: company, employee, Task, WorkerSession, epoch/incarnation, mission state and tool-call budget are not model-selected. The B3 surface qualification identity is `polis-product-tool-surface@2`.

Current manifest:

- tool count: 7
- manifest digest: `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`
- aggregate schema bytes: 1470
- aggregate schema digest: `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`
- old B2 manifest: `730b9b2aba9de7ca015e800b1a597de37a22bd0c299dc7ad10e022c8399388d9`
- `provider_surface_changed = true`; the old qualification is stale.

The exact provider-visible registrations are emitted by `TestR05B3ProductSurfaceIsVersionedAndGeneric` and captured in `evidence/development/r0.5b3-product-employee-surface-semantic-remediation/product-surface.txt`. No live provider or business database is part of this qualification.
