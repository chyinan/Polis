# Slice 33 — R3 verified evidence preview

Date: 2026-09-25

## Implemented

- Added bounded verified file manifests and selected-file previews for domain evidence linked to company-scoped MissionInputs. Kernel reads the immutable input package/CAS and validates the company, record, area, revision, source digest, selected safe relative path, byte limit and content digest.
- Added session-token-protected Workbench routes. Responses are `no-store`, `nosniff`, length-bounded and include source/input binding headers required for frontend verification. Desktop CORS exposes only the headers needed by this flow.
- Added Workbench review UI for manifest selection and preview. Supported media are plain text, Markdown, CSV, JSON, PNG and JPEG. Text is rendered as text; images are rendered as images. PDF original bytes and source/parser metadata cannot be previewed; extracted PDF text is static and bounded. No PDF iframe or active document rendering is used.
- Added Schema 31 `previewed_evidence` and `review_contract_revision`. New review commands re-read the exact previewed paths from CAS, compare source/content digests and media type, then persist one safe, unique attestation for every submitted evidence area. Kernel rules and the PostgreSQL trigger independently validate the binding. Schema 30 review rows remain readable as contract revision 0.
- Reference review outcomes remain reference-only. Both R3 profiles remain `not_run` with execution disabled. No real domain evidence or account was used.

## Verification

| Command | Result |
|---|---|
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite |
| `bash scripts/r3-domain-evidence-postgres-test.sh --race` | PASS; fresh temporary PostgreSQL 18 cluster migrated through Schema 31; Kernel and Workbench domain-evidence integration passed under race detection; script removed only its temporary cluster/database |
| `npm run test` | PASS; 78 tests in 12 files |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS with zero warnings |
| `npm run build` | PASS; Vite built production assets; advisory notes the generated JS chunk is over 500 kB |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux command packages |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 cross-build |
| `.tools/go/bin/gofmt -d` on changed Go files | PASS; no formatting diff |
| `git diff --check` | PASS after the code, docs and evidence updates |

The frontend API and validation tests cover manifest path/source binding, response-content digest verification, unsafe media/path rejection and fixture-mode denial. Intake tests cover direct text, extracted PDF text, PDF original restrictions, verified directory/ZIP members, and unsafe paths/media. Kernel/Workbench coverage checks CAS hashes, company/record binding, selected preview responses and persisted/replayed review attestations.

## Review follow-up

The read-only review found that reserved metadata names were hidden conditionally in intake while workflow/database rules rejected them without the same source-kind condition. Regression tests first reproduced the mismatch: ZIP/directory manifests listed `pdf/extraction.json` as previewable, and workflow validation accepted a nested `.polis-git-source.json` path. Intake now exposes one pure reserved-path rule used for listing, preview and domain workflow validation; Schema 31's trigger applies the same exact PDF path and Git metadata basename checks. Added tests cover both archive kinds, Kernel selected-file denial, direct workflow validation and direct database inserts. The focused re-review approved the fix and found no new issues. It confirmed the dependency from domainworkflow to intake is one-way and non-cyclic.

Integrity boundary: Kernel verifies actual CAS bytes before recording an attestation; the SQL trigger enforces the structure and path policy but cannot read the external CAS store. Review writes go through the trusted Kernel API. A direct SQL writer with insert privileges is outside the CAS byte-verification guarantee.

## Not qualified by this slice

- No substantive content-operations or research-simulation qualification; no real quality, intervention, recovery, cost or organization-benefit evidence was assessed.
- No model/provider turn, QQ send, external MCP, GitHub account, registry request, WFP/loopback mutation or production action.
- Schema migration/integration and cross-compilation do not establish packaged Windows GUI, installer, native isolation, service readiness, or Linux host qualification.
