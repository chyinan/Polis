# R0.5B9 — Deterministic delivery transaction

## Result

`r0_5b9_deterministic_delivery_transaction = PASSED`.

The task remains offline-only: provider reservation `0`, provider egress `0`, Medium `0`, High `0`, live canary `NOT_STARTED`. LIVE_2 raw/adjudicated verdicts and evidence were not modified.

## Old and new flow

Old product delivery:

```text
workspace_replace
  → workspace_check PASS
  → Agent constructs work_checkpoint payload
  → qualified checkpoint
  → Agent calls artifact_submit
  → Artifact publication
  → Task candidate / fencing
```

New product delivery:

```text
workspace_replace
  → workspace_check PASS
  → Agent explicitly calls polis_task_submit({})
  → control plane derives current qualification
  → durable artifact_staging + CAS
  → one TXWrite publication transaction:
       qualified checkpoint
       Artifact + TaskValidationArtifactQualification
       Task candidate
  → post-publication workspace/check/checkpoint fencing
```

PASS never auto-publishes. The `task_submit` call is the required Agent decision.

## Responsibility boundary

Agent-owned:

- understand the Task;
- edit the authorized workspace with digest+revision CAS;
- read and react to validation feedback;
- explicitly choose delivery by calling `task_submit`.

Control-plane-owned:

- current Task/Employee/WorkerSession/epoch authorization;
- TaskValidationBinding and current PASS receipt validation;
- workspace content, digest and revision;
- deterministic qualified checkpoint fields and evidence reference;
- Artifact staging, CAS/blob verification, Artifact metadata and Task/Mission relation;
- qualification provenance, Task candidate transition and fencing.

The public input is `{}`. The Agent is not required to echo Task ID, session ID, epoch, workspace revision/digest, checkpoint kind, evidence arrays, Artifact relation or Mission relation.

## Output and errors

Success returns a candidate `Receipt` whose ID is the Artifact ID, plus `ProductDeliveryResult` containing Task, Mission, checkpoint, validation receipt, Artifact, workspace digest/revision, Task state and committed delivery state.

Structured public rejection reasons include:

`validation_required`, `validation_not_passed`, `validation_receipt_invalid`, `validation_receipt_stale`, `workspace_changed_since_validation`, `writer_fenced`, `task_state_disallows_delivery`, `delivery_already_committed`, `artifact_staging_failed`, and `artifact_publication_failed`.

Business preconditions and authorization/fencing remain policy failures; staging/publication failures identify their infrastructure phase and are retryable. No generic `delivery_failed` hides a known public reason.

## Transaction, recovery and duplicate semantics

This implementation does not claim distributed exactly-once semantics. The durable staging row and content-addressed blob are recoverable prerequisites. Qualified checkpoint, Artifact, qualification provenance and Task candidate are committed together in one database transaction under the existing company lifecycle lock.

If a failure occurs before final commit, no final checkpoint/Artifact/candidate state exists; replay reuses the durable staging/CAS boundary. If the response is uncertain after commit, the same receipt key or the unique Task qualification row returns the existing committed Artifact. Repeated delivery for the same Task and validated workspace creates no duplicate checkpoint or Artifact.

The disposable PostgreSQL/CAS harness injects and recovers at:

- after staging intent;
- after CAS publish;
- after checkpoint persistence;
- after Artifact qualification persistence;
- after Task transition;
- after commit/before response.

## Qualified checkpoint and Artifact semantics

`NewQualifiedProductDeliveryCheckpoint` creates a real `worker_checkpoints` record containing current PASS receipt, TaskValidationBinding digest, workspace digest/revision, session/epoch, runner revision, qualification state and policy revisions. The Artifact keeps the existing `task_validation_artifact_qualifications` provenance relation, digest/CAS policy, Task/Mission relation and candidate state semantics. Progress checkpoints remain independently available through `work_checkpoint`.

## Frozen LIVE_2 counterfactual

The frozen LIVE_2 workspace content/digest (`562b5c53a66e9e5c8e326d8d5d13a55cbf56504ea70ac1b085b5a7a4efd805f2`), revision `2`, and exact TaskValidationBinding digest (`2bbaa0948f92f21cb0b7ba7c167bacdce61c91e5ee42821dad2177ba1e91a721`) were reconstructed in a disposable database. One `task_submit` produced the qualified checkpoint, Artifact and candidate Task. This is an offline counterfactual only and does not change the frozen LIVE_2 verdict.

## Provider surface

The public product surface changed from historical/stale @3 to current @4 because the product choreography tool was replaced by `polis_task_submit`:

- surface: `polis-product-tool-surface@4`
- tools: 7
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- schema bytes: `2206`
- schema digest: `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- live qualification: stale/not run

No live canary, LIVE_3, High, real provider sample or multi-Agent E2E was started.

## Verification

- Go `test ./... -count=1`: PASS
- Go `test -race ./... -count=1`: PASS
- Go `vet ./...`: PASS
- Linux and Windows `build ./cmd/...`: PASS
- frontend test: 19 tests PASS
- frontend typecheck/lint/build: PASS
- disposable PostgreSQL/CAS delivery + recovery + control + Workbench integration: PASS
- offline browser smoke: PASS, no request failures or layout overflow
- Bash/Python syntax and `git diff --check`: PASS (line-ending warnings only)
