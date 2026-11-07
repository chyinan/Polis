# R0.3A qualified checkpoint freshness

Status: PASSED. This phase consumed zero Medium, zero High, and zero provider
egress. No real model/provider run was authorized here.

The T21C failure is retained in its original evidence directory. Its unchanged
regression now denies artifact submission after effective-contract supersession.

Peer workspace checks persist the accepted contract ID, workspace revision and
`peer-semantic-checker@2`. Persisting check evidence revalidates workspace and
contract under the company row lock. Qualified checkpoint creation validates
the referenced checks against those current values; progress remains historical
handover information. Contract acceptance marks earlier qualified checkpoints
`superseded_for_finalization` without removing either checkpoint kind.

Session-backed peer submission now routes through `TXSubmitQualifiedPeer`.
Within one TXWrite/company-lock transaction it checks the current workspace,
accepted contract, qualified checkpoint, checker and policy revisions, and
persists staging, artifact, qualification binding and task transition together.
The durable workspace blob is reused. An invalid qualification creates neither
an artifact nor a staging record. Concurrent contract acceptance uses the same
lock: an artifact can commit before acceptance, or acceptance commits first and
old qualification is denied. Acceptance after a valid artifact commit does not
rewrite that artifact's historical provenance.

`artifact_qualifications` (migration 5) records checkpoint ID, workspace revision
and digest, contract ID, and semantic checker/checkpoint/artifact policy versions.
The existing trusted sessionless R0 fixture construction path remains separate;
business employee sessions cannot use it.

L2 manifests bind `checkpoint_policy_revision`,
`artifact_eligibility_policy_revision`, and `acceptance_checker_revision`.
Absent historical fields remain omitted when serializing historical manifests;
exact current comparison rejects their mismatch. No public input-schema change
was needed: the revised registry remains 11 tools, 2038 aggregate schema bytes,
manifest digest `8b26523174e9a198730c32c1a6ba547587bedfda7cb27bd0a5b1fa6dd7c1ff22`.

Regression coverage: valid submission, stale workspace, stale checker, stale
policy, stricter contract failure/recovery, retained progress checkpoint,
rejection of old check reuse, and eight simultaneous accept/submit schedules
with committed event-order assertions. R0.2 handover remains in the full suite.

One initial parallel full-suite invocation timed out in the existing 45-second
R0.2 compiler verifier. A fresh database and package-serial full-suite run
passed; the verifier deadline is unchanged. A second fresh database passed the
full race suite with package serialization.

Final status: `checkpoint_freshness_hardening=PASSED` and
`eligible_for_new_revised_L2_offline_qualification=YES`. The next L2
qualification remains a separate new run and is not started here.
