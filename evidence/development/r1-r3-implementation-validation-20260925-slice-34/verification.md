# Slice 34 — controlled read-only Skill source import

Date: 2026-09-25

## Implemented

- Added a fixed `polis-read-only-skill@1` ZIP importer. It requires a bounded `SKILL.md` YAML frontmatter block and accepts only the declared read-only document/reference/raster asset allowlist. Script files, nested archives, links/special entries, hidden paths, unsupported files, active SVG, malformed/duplicate/aliased/custom-tag YAML and unsafe paths are rejected.
- The server parses a multipart ZIP upload; the client does not submit a digest, publisher scope, package name or read-only trust bit. The package name and description come from the parsed frontmatter. The server computes per-file SHA-256 values and a stable sorted canonical manifest digest.
- Accepted file bytes are written to company-scoped CAS. A new immutable SkillRevision stores the fixed profile, per-file manifest, content digest, local-upload source reference and candidate status. ZIP order and compression do not affect the manifest digest.
- The older metadata-only Skill registration remains candidate-only. The Kernel stores a metadata-only marker and will not qualify it based on caller-provided `readOnly`, digest or source reference.
- Local Skill qualification re-reads all source files from CAS and checks content lengths, hashes, manifest identity and frontmatter before recording `metadata_verified`. Administrator approval and Employee/version binding remain separate operations. Bindings still report `runtime_unqualified`.
- The desktop token predicate now gates both Skill import and MCP registration in tokenless mode; the desktop middleware regression test covers a tokenless POST with an empty Origin.
- A non-blocking PostgreSQL advisory lock keyed by company/publisher/package/revision serializes source and metadata-only imports. Lock sessions use a separate two-connection pool so the transaction pool remains available; uncertain acquisition or unlock outcomes hijack and close the session rather than returning it to the pool. The Kernel also checks an existing RequestID fingerprint and package/revision key before CAS writes. Concurrent-import integration verifies a same-revision loser writes no unique blob and 12 distinct package imports all complete without exhausting the main pool. Tokenless desktop mode gates Skill and MCP registration writes even when Origin is empty.
- No Skill body is loaded into a Worker. No script or dependency runs, and no MCP process, external endpoint, QQ, registry, model or business account was used.

## Verification

| Command | Result |
|---|---|
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite after lock-pool isolation and ambiguous-session disposal changes |
| `bash scripts/r1-capability-source-postgres-test.sh` | PASS after lock-pool and ambiguous-session disposal changes; fresh temporary PostgreSQL 18 cluster migrated through Schema 31; Kernel source import, CAS verification, manual approval/revocation, Employee binding, forged-metadata denial, missing-CAS failure, same-revision CAS conflict and 12-way distinct-revision concurrency passed under race detection; script removed only its uniquely named temporary cluster/database |
| `go test ./internal/desktop -run TestTokenlessMiddlewareRejectsSkillImportWithoutSessionToken -count=1` | PASS; first reproduced the missing token barrier (202 instead of 401), then passed after gating Skill/MCP registration |
| `npm run test` | PASS; 80 tests in 12 files |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS with zero warnings |
| `npm run build` | PASS; production frontend assets built; Vite reports the existing >500 kB chunk-size advisory |
| `bash scripts/go.sh build ./cmd/...` | PASS after final changes; Linux command packages |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS after final changes; Windows amd64 cross-build |
| `.tools/go/bin/gofmt -d` on changed Go files | PASS; no formatting diff |
| `git diff --check` | PASS after code, docs and evidence updates |

Pure parser tests cover deterministic digest stability across ZIP order, the frontmatter profile, script/SVG rejection, unsafe and multiple-root packages, duplicate YAML fields, aliases, custom tags, missing Skill entrypoint, changed CAS bytes and missing manifest files. Workbench HTTP tests cover company scoping, strict multipart field names and upload bounds. Frontend tests verify the same-origin session-token request and multipart fields; the digest is computed on the server.

CAS is company-shared and content-addressed. Same-revision metadata/source writes serialize before CAS access, and known idempotency/revision conflicts are rejected first. Unexpected storage or database commit failures after file writes can still leave unreachable CAS content. This slice does not delete blobs because another record may reference the same digest; a reference-aware CAS reconciliation job remains a maintenance gap.

## Not qualified by this slice

- `skills.load` and Worker-context delivery are not implemented. A bound Skill cannot execute or run scripts.
- No public/group Skill publisher or arbitrary package marketplace is added. The Workbench imports a local company ZIP only.
- No real-world Skill package or third-party source was imported; the PostgreSQL test uses a local disposable ZIP fixture.
