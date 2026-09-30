# R1 read-only Skill source import

> Updated: 2026-09-25, Slice 34. Import and local source verification are implemented; Worker loading and execution remain unavailable.

## Supported package profile

The token-gated Workbench accepts one local ZIP at `/companies/{companyId}/capabilities/skills`. Tokenless desktop mode rejects this route even for an empty Origin. The upload includes a revision and request ID; the server derives the package name, file manifest and content digest from the ZIP. It never accepts a caller-supplied digest as proof of file contents.

`polis-read-only-skill@1` requires `SKILL.md` at the ZIP root or in one top-level directory. Its YAML frontmatter requires a lowercase kebab-case `name` and a non-empty `description`. The importer accepts optional string `license` and `compatibility`, a string or string-list `allowed-tools`, and string-only `metadata`. `allowed-tools` is retained only as source text; it does not grant permissions.

The fixed file allowlist is `SKILL.md`, `README.md`, `LICENSE`, `LICENSE.md`, `LICENSE.txt`, Markdown/TXT files under `references/`, and Markdown/TXT/PNG/JPEG files under `assets/`. The existing ZIP parser enforces path traversal, link/special-entry, duplicate and nested-archive rejection. The Skill profile additionally rejects hidden path segments and every unsupported extension, including executable scripts and SVG. The compressed archive is capped at 8 MiB; expanded content is capped at 7 MiB, each text document at 64 KiB for `SKILL.md` / 1 MiB for attachments, and raster assets are validated by the existing intake decoder.

The YAML parser uses the repository's pinned `go.yaml.in/yaml/v3` package with known-field checking. A bounded node walk rejects duplicate keys, aliases, custom tags, merge keys, nested structures outside the supported frontmatter shape, and multiple documents. The package API exposes `Decoder.KnownFields` and `yaml.Node`; the v3 API is documented as stable by the [official Go YAML package documentation](https://pkg.go.dev/go.yaml.in/yaml/v3).

## Storage and governance

Each accepted file's exact bytes are stored in the company's content-addressed store. The immutable `skill_revisions.manifest` records sorted relative paths, media types, byte lengths, per-file SHA-256 values, the package name/description and the fixed profile version. The revision digest is SHA-256 over the canonical JSON manifest, so ZIP entry order and compression do not change the identity. The ZIP envelope itself is not stored; its verified file contents and manifest are.

The existing metadata-only candidate API remains for compatibility, but the Kernel replaces its submitted manifest with a `metadata-only` marker. A client cannot qualify or approve such a candidate by setting `readOnly: true`, inventing a source reference, or supplying a digest. Before writing CAS blobs, the Kernel takes a non-blocking PostgreSQL advisory lock keyed by company, publisher scope, package ID and revision. Both metadata and source import paths use that lock, so competing revision writes return a conflict before writing content. Lock sessions come from a separate small connection pool; import queries and transactions continue to use the main pool. If acquisition or release returns an ambiguous error, the Kernel hijacks and closes that session so a possibly held session lock cannot leak back into the pool. The importer also checks the request fingerprint and existing package/revision key before CAS writes. A temporary PostgreSQL race test verifies one same-revision contender wins without leaving the loser's unique CAS blob, and twelve distinct package imports complete concurrently without exhausting the main pool.

Local Skill qualification reads every manifest file back from company CAS, verifies file length and SHA-256, validates the canonical manifest digest and re-parses `SKILL.md`. Missing, changed or cross-company bytes fail closed. After that check, an administrator still records a separate approval decision and binds the exact Skill revision to an employee. The binding remains `runtime_unqualified`.

This slice does not implement `skills.load`, Worker-context delivery, script capability, dependency installation or Skill execution. It does not contact a registry, MCP endpoint, QQ or another external service. A local package is only a candidate until the existing human qualification/approval/binding steps are completed. A storage or database failure after successful CAS writes but before the receipt commits can still leave unreferenced content-addressed blobs. The importer does not delete them because a blob may be shared by another company record; reference-aware CAS reconciliation remains a storage-maintenance gap.

## Verification

Full serial Go tests, tokenless desktop auth tests, a fresh Schema 31 PostgreSQL race test for import/qualification/approval/binding/revocation, duplicate-import preflight and missing-CAS rejection, 80 frontend tests, typecheck, lint, production frontend build and Linux/Windows amd64 command builds pass. The frontend build reports the existing Vite chunk-size advisory (>500 kB). Detailed command results: `evidence/development/r1-r3-implementation-validation-20260925-slice-34/verification.md`.
