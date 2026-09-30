# R0.5B7 — Checkpoint schema and error-feedback hardening

## Result

`r0_5b7_checkpoint_schema_error_feedback_hardening = PASSED`.

No provider reservation, provider egress, Medium turn, High turn, live canary, LIVE_2 retry, or business sample occurred. LIVE_2 raw and forensic evidence remain untouched.

| Boundary | Result |
|---|---|
| checkpoint schema semantics | **PASSED** |
| receipt-reference contract | **PASSED** |
| explicit no-rejection representation | **PASSED** (`rejected: []`) |
| structured checkpoint feedback | **PASSED** |
| checkpoint policy relaxed | **NO** |
| Artifact policy relaxed | **NO** |
| frozen LIVE_2 malformed payloads | **FAIL_AS_EXPECTED** |
| corrected call 7 equivalent | **PASS** |
| offline check → checkpoint → Artifact lifecycle | **PASS** |
| post-publication fencing | **PASS** |

## Phase A — Frozen contract and LIVE_2 replay

Before hardening, the product provider surface was `polis-product-tool-surface@2`: seven tools, manifest `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`, schema bytes 1470, schema digest `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`.

The old product checkpoint schema required `kind`, `summary`, `facts`, `decisions`, `rejected`, `evidence_refs`, `next_action`, and `failed_checks`, because the generic schema builder marked every property required. `rejected` and `evidence_refs` were both non-empty string arrays. The dispatcher then repeated the non-empty checks and returned only `MALFORMED_INPUT`. Runtime validation additionally required each evidence reference to be an ID and, for qualified product checkpoints, a current passed `workspace_check` receipt for the same TaskValidationBinding, session/epoch, workspace digest/revision, contract and checker revision.

The six frozen LIVE_2 checkpoint attempts are preserved as deterministic replay cases:

| LIVE_2 call | Defect | Old rejecting layer | Old visible response |
|---:|---|---|---|
| 6 | `rejected=[]`; evidence references were prose | Provider schema first rejects `rejected` `minItems`; runtime receipt validation was not reached | `MALFORMED_INPUT` |
| 7 | Correct PASS check receipt was supplied, but `rejected=[]` | Provider schema/dispatcher `minItems:rejected` | `MALFORMED_INPUT` |
| 9 | Progress payload had empty `decisions`, `rejected`, and `evidence_refs` | Provider schema/dispatcher non-empty array checks | `MALFORMED_INPUT` |
| 10 | Arrays met the old JSON schema, but evidence references were prose rather than receipt IDs | Runtime `core.ValidID`/receipt-reference validation | `MALFORMED_INPUT` |
| 11 | `facts`, `decisions`, `rejected`, and `evidence_refs` were empty | Provider schema/dispatcher non-empty array checks | `MALFORMED_INPUT` |
| 13 | `rejected=[]`; one ref was the PASS receipt and another was a workspace-replace receipt | Provider schema/dispatcher `minItems:rejected`; the second ref would also fail qualified receipt policy if reached | `MALFORMED_INPUT` |

The new replay tests keep all six cases failing, but identify the precise field and defect category. The minimally corrected call 7 equivalent uses the same current PASS receipt, `rejected: []`, and typed evidence reference `{receipt_id, proves: "workspace_check_pass"}`; the pure contract parser accepts it, and the disposable PostgreSQL lifecycle test persists the qualified checkpoint and Artifact.

## Phase B — Semantic field classification

| Field | Classification | Current contract |
|---|---|---|
| `kind` | **SEMANTICALLY_REQUIRED** | `progress` or `qualified`; qualified is the publication gate. |
| `summary` | **SEMANTICALLY_REQUIRED** | Non-empty string, max 512 characters. |
| `facts` | **SEMANTICALLY_REQUIRED** | One to eight non-empty strings. |
| `decisions` | **SEMANTICALLY_REQUIRED** | One to eight non-empty strings. |
| `rejected` | **SEMANTICALLY_REQUIRED** | Required array of reasons; **empty array is the sole no-rejection representation**. |
| `evidence_refs` | **CONDITIONALLY_REQUIRED** | Required for the current checkpoint policy; one to eight typed receipt references. Qualified refs must be current passing `workspace_check` receipts. |
| `next_action` | **CONDITIONALLY_REQUIRED** | Required and non-empty for `progress`; optional for `qualified`. |
| `failed_checks` | **OPTIONAL** | Zero to eight strings describing failed checks. |
| `workspace_revision` / `workspace_digest` | **DERIVABLE** | Bound from the authorized current workspace; not caller-supplied. |
| Task/session/epoch/TaskValidationBinding | **DERIVABLE** | Bound by the trusted worker session and database policy; not caller-supplied. |
| rejected evidence references | **LEGACY_ENCODING** | `rejected` contains human-readable reasons, not receipt IDs. Receipt evidence belongs in typed `evidence_refs`. |

No field is accepted merely because an old payload happened to contain it. The product schema makes `failed_checks` and `next_action` optional at the JSON boundary while retaining the runtime rule that progress needs `next_action`.

## Phase C — New product checkpoint contract

The product registry now has a product-specific schema builder. Historical peer registries retain their prior schema and semantics.

`rejected` is a required array with no `minItems`; the provider-visible description says: “use `rejected: []` when no evidence was rejected.” Empty values are not silently coerced.

`evidence_refs` is an array of one to eight objects:

```json
{
  "receipt_id": "<receipt ID returned by workspace_check>",
  "proves": "workspace_check_pass"
}
```

The schema encodes `additionalProperties: false`, required `receipt_id` and `proves`, the receipt-ID character constraint, the single allowed proof purpose, and a description that the receipt must belong to the current Task/session/workspace. The runtime still performs the authoritative Task/session/epoch/binding/digest/revision/checker validation; the public shape does not replace authorization fencing.

The descriptions now state the lifecycle with one responsibility per tool:

- `workspace_check` returns a validation receipt and explicitly says PASS validates workspace content only; use that receipt in a qualified checkpoint.
- `work_checkpoint` explains `rejected: []`, typed receipt references, progress `next_action`, and that a successful qualified checkpoint enables `artifact_submit`.
- `artifact_submit` states its qualified-checkpoint and binding prerequisites, and that success creates a candidate Artifact and moves the Task to `candidate`; it is not independent final acceptance.

## Phase D/E — Structured feedback

`PeerToolRejection` now exposes safe public fields:

- `reason_code`
- `actionable_summary`
- `failing_field`
- `expected_public_shape`
- `actual_category`

The product dispatcher returns this structured object in `ToolResult.data` for parser and policy failures instead of dropping it when the wrapped error also matches a generic `core.Code`. The parser reports categories such as `missing_required_field`, `invalid_field_type`, `invalid_field_value`, and `invalid_receipt_ref`. Runtime mappings cover `writer_fenced`, `stale_workspace_revision`, `validation_not_configured`, `receipt_not_accepted`, and `checkpoint_not_allowed_in_task_state` where the trusted layer can classify them without exposing SQL or checker internals.

The successful path remains strict: a checkpoint cannot be created without the existing validation receipt, stale or foreign receipts remain denied, and the current session/workspace binding is still checked inside the transaction.

## Phase F/G — Publication and policy fences

The lifecycle remains:

```text
workspace mutation → workspace_check PASS → qualified checkpoint → Artifact submission → Task candidate
```

No policy gate was removed. Qualified checkpoint creation still requires a passing current product check and immutable TaskValidationBinding. Artifact submission still requires a current qualified checkpoint and matching binding. The offline PostgreSQL regression confirms that after Artifact publication, workspace replacement, workspace check, and checkpoint calls remain denied and the Artifact digest remains equal to the validated workspace digest.

## Phase H/I — Offline qualification

The disposable PostgreSQL harness [r0-5b7-offline-tests.sh](../../../scripts/r0-5b7-offline-tests.sh) ran the focused product lifecycle against a fresh schema-7 database. It passed:

- product workspace CAS replacement and product `workspace_check` PASS;
- typed qualified checkpoint with `rejected: []` and the current check receipt;
- Artifact submission and candidate Task transition;
- post-publication workspace/checkpoint fencing;
- Workbench Task/Artifact projection;
- offline FakeRuntime product bridge with zero provider egress.

The full no-DSN Go suite and race suite passed separately, so historical opt-in integration fixtures were not reinterpreted as part of this product qualification.

Token/context overhead was not changed or reclassified in this task. No provider traffic occurred.

## Phase J — Provider-visible surface impact

The schema and descriptions changed, so the @2 qualification is stale. The exact current surface is:

| Field | Current value |
|---|---|
| surface ID | `polis-product-tool-surface@3` |
| tool count | 7 |
| tool names | `polis_work_current`, `polis_context_read`, `polis_workspace_read`, `polis_workspace_replace`, `polis_workspace_check`, `polis_work_checkpoint`, `polis_artifact_submit` |
| manifest | `95d4f2e2b1551096f05c7683786ec09a2d75189f8411fc66ffc759514de32257` |
| aggregate schema bytes | 2206 |
| aggregate schema digest | `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc` |
| @2 status | **STALE** |
| @3 live L2 qualification | **NOT RUN / STALE** |
| future product sample eligibility | **NO** until separately authorized @3 live qualification |

Exact provider-visible materialization is recorded in [provider-surface-verification.txt](provider-surface-verification.txt).

## Phase K — Verification boundary

Recorded verification results are in [qualification.json](qualification.json):

- `go test ./... -count=1`: PASS;
- `go test -race ./... -count=1`: PASS;
- `go vet ./...`: PASS;
- Linux and Windows cross-builds: PASS;
- frontend test (19 tests), typecheck, lint and production build: PASS;
- focused disposable PostgreSQL product lifecycle/read-model/control tests: PASS;
- Bash syntax and `git diff --check`: PASS.

No code or tool change was made to frozen LIVE_2 evidence. No live canary, Medium, High, provider reservation, or provider egress was run. The next permissible step is a separately authorized @3 qualification canary; this task does not authorize it.
