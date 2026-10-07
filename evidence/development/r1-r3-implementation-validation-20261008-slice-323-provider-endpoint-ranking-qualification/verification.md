# Slice 323 — REQ-38 provider endpoint and ranking qualification

Date: 2026-10-08

## Result

```ini
req38_provider_configuration_qualification = BLOCKED
REQ-38 = NOT_COMPLETE
provider_traffic = 0
real_worker = NOT_STARTED
external_retrieval = NOT_RUN
```

This is an independent qualification slice. It preserves the Windows profile,
loopback and source implementation evidence from earlier slices.

## Environment and configuration observed

No Windows `polis`/`polisd` process or WSL `polis`/`polisd` process was
running during this qualification. The caller environment had no values set
for `POLIS_WORKER_MODE`, `POLIS_PROVIDER_TRANSPORT`,
`POLIS_PROVIDER_BINARY`, `POLIS_PROVIDER_AUTH_FILE`, `POLIS_PROVIDER_MODEL`,
`POLIS_PROVIDER_EFFORT`, `POLIS_PROVIDER_RUNTIME_MANIFEST`,
`POLIS_OPERATION_ADAPTERS_ENABLED`, `POLIS_DSN`, `POLIS_BLOB_ROOT`,
`CODEX_HOME`, `OPENAI_API_KEY` or `OPENAI_BASE_URL`. Only set/unset states were
queried; no secret values were printed.

With `POLIS_WORKER_MODE` unset, `cmd/polis` selects its documented
`deterministic` default. There is no active product provider configuration to
qualify. `POLIS_DSN` is unset, so this run did not read a business database or
research-source registration.

## Phase A — credential boundary

The Codex Worker configuration expects an explicit
`POLIS_PROVIDER_AUTH_FILE` and identifies it as a mounted Codex auth file. That
path is absent here. The Worker path does not fall back to the ambient Codex
home. The settings-only model catalog has a separate default path to
`CODEX_HOME/auth.json` or the user's `.codex/auth.json`; it was not invoked.

The research search adapter uses an opaque `searchCredentialRef` and an
injected `CredentialProvider`. The production wiring constructs the HTTP
backend without a credential provider. A registered credential reference
therefore fails with `search credential provider is unavailable` before HTTP
dispatch. The opaque reference may appear in the Workbench source projection;
raw credential material is not part of the projection.

The qualification exposed a concrete false-readiness defect: the Workbench
reported `ready` when the auth file merely existed, while the runtime also
accepted unreadable or malformed auth content as an unavailable identity
snapshot. Commit `e76fc44` changes readiness to inspect only a supported local
credential shape and return safe states (`missing`, `invalid`, `configured`).
The provider runtime rejects missing, unreadable, non-file, malformed, empty
or unsupported credential shapes. `configured` means locally readable and
structurally supported; it does not mean the remote provider authenticated the
credential.

Raw credential material is not returned by the readiness API or written by
this check to logs/evidence. Existing Windows session launch behavior creates
a temporary auth snapshot in the runtime home and removes it when the session
stops; this qualification started no session and created no such snapshot.

## Phase B — endpoint identity

There is no active endpoint in the current environment. The Codex launch
profile selects `model_provider = "polis-openai"`, requires OpenAI auth and
uses the native app-server over local stdio. It does not configure `base_url`;
the upstream endpoint host is delegated to the installed Codex runtime and is
not pinned in the Polis configuration. No `model/list` request or endpoint
probe ran.

Research search has a different, optional path: a Mission-scoped source may
store one HTTPS `search_endpoint` on the same origin as its registered source.
The endpoint is selected by the explicit `sourceId`; no alternate endpoint is
configured. No active database/source record was available to resolve one.

```ini
provider_endpoint = UNRESOLVED
endpoint_configuration = BLOCKED
```

## Phase C — selection and ranking

The Codex Worker accepts one configured `POLIS_PROVIDER_MODEL` and effort.
Thread start sets `allowProviderModelFallback=false`. There is no provider,
model or profile ranking among alternatives in the product path. The model
catalog is discovery data, not an automatic selector. Since neither a model
nor endpoint is configured, model/profile availability remains unverified.

Research search requires one explicit source ID and uses that source's
registered endpoint. There is no endpoint fallback. `research-ranking@1`
assigns result ranks in the order returned by the selected backend after
normalization; it does not score candidates or rank providers/endpoints.

```ini
provider_candidate_ranking = NOT_APPLICABLE
research_result_ordering = research-ranking@1, preserves backend order
fallback_candidates = NONE
```

## Phase D — negative configuration and fallback

Local tests cover missing auth path, directory instead of a file, malformed
JSON, empty auth object, unsupported auth mode, supported ChatGPT token shape
and API-key shape. The runtime readiness regression test confirmed malformed
auth is rejected. The Workbench projection tests confirmed missing/invalid
states and that only the `fake` transport reports `not_required`.

Source inspection and existing offline tests establish these fail-closed paths:

| Case | Behavior |
| --- | --- |
| Missing Codex auth path | `missing`; runtime readiness rejects it |
| Invalid/unreadable auth file | `invalid`; runtime readiness rejects it |
| Missing search credential provider for a registered reference | returns `search credential provider is unavailable` before HTTP dispatch |
| Malformed or cross-origin search endpoint | rejected by source/backend validation |
| Unavailable configured search endpoint | HTTP client error propagates; no alternate endpoint is selected |
| Unsupported provider transport in real mode | adapter construction errors; no fake-provider substitution |
| Unsupported/unavailable model | not qualified; determining remote availability requires model discovery or model start |
| Explicit Codex model unavailable | thread start disallows provider model fallback |

No product configuration was switched to a fake or alternate provider to
continue this qualification. Synthetic credential fixtures remained inside
the offline tests.

## Phase E — readiness smoke

The local auth-file check requires no provider call and passed offline tests.
It only establishes local structure/readability. Verifying remote
authentication, endpoint reachability, protocol compatibility or model
availability would require Codex runtime discovery or a provider request. The
endpoint is not pinned and the active config is absent, so no such request was
made.

```ini
credential_source = mounted_codex_auth_file (not configured)
credential_readiness = MISSING
remote_authentication = NOT_CHECKED
endpoint_reachability = NOT_CHECKED
protocol_compatibility = NOT_CHECKED
model_profile_availability = NOT_CHECKED
provider_traffic = 0
```

## Source change and verification

The base implementation identity was `229289e`; the prior qualification docs
were at `5782bdb2`. Source commit `e76fc44` fixes local credential readiness
validation. It does not configure a provider endpoint, add a credential
resolver, select a model, start a Worker or enable retrieval.

Verification on 2026-10-08:

```text
bash scripts/go.sh test ./...       PASS
bash scripts/go.sh build ./cmd/...  PASS
git diff --cached --check           PASS before source commit
```

The full Go suite includes the provider, control, kernel, research and
Workbench packages. Provider-related tests used local fixtures; no provider
traffic was generated.

## Remaining REQ-38 gates

```ini
provider_endpoint_and_model_qualification = BLOCKED
real_worker_qualification = OUTSTANDING
external_retrieval_qualification = OUTSTANDING
clean_vm_qualification = PARTIAL_WITH_EXPLICIT_LIMITATION
wfp_profile_qualification = PARTIAL_WITH_EXPLICIT_LIMITATION
REQ-38 = NOT_COMPLETE
```
