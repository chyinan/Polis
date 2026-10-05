# Mission input contracts

Last verified: 2026-09-24

## Purpose

Keep uploaded originals, derived file trees, and text actually supplied to a Worker distinct and hash-bound.

## Contracts

- `PrepareMissionInput` returns the exact bytes to persist. Text, images, and ZIP keep their submitted bytes; a direct PDF becomes a canonical `pdf_snapshot` package that preserves the original PDF, bounded extracted text, and extraction metadata. Always persist the returned bytes, never the raw PDF after preparing it.
- Nested PDFs inside directory and ZIP snapshots stay as preserved unsupported binary files; only a direct `.pdf` upload invokes extraction.
- PDF extraction pins `github.com/giraffesyo/pdf@v0.7.0`, caps the source at 6 MiB, parses at most 40 pages with per-stream/operator/glyph/image limits, and caps extracted text at 16 KiB. It may include at most eight supported embedded image objects as PNG, each at most 1,000,000 pixels and 256 KiB, with a 960 KiB total. The immutable PDF extraction record binds each object to its page, dimensions, digest, page geometry and placement; Worker receipts repeat page number, dimensions, path and digest. This is not a complete page raster: vector drawing, page composition and unsupported images remain unavailable, and no OCR is performed. The original PDF stays excluded from Worker input.
- Preserve canonical verification for historical `polis-pdf-extraction@1` packages while writing the embedded-image `@2` representation for newly prepared PDFs.
- ZIP input is stored as `zip_snapshot` and is never extracted to a host path.
- Git input is a canonical `git_snapshot` made from a complete locally present commit OID. Import reads blob objects without fetch/checkout and does not use hooks, filters, credential helpers, or remotes. Dirty worktree bytes remain excluded and are called out in the generated source note; submodules, Git LFS payloads, links, and bounded unsupported entries are listed as omissions.
- `ExtractVerifiedZIPFiles` reads at most 512 entries, 250 files, 7 MiB expanded content, and 8 MiB compressed input. It rejects unsafe/colliding paths, special entries, nested archives, empty files, and invalid size/CRC data.
- `ImportGitCommit` accepts a local repository root and a full commit OID only. It reads no more than 250 tree entries, caps the included source at 7 MiB, and stores no absolute source path or remote URL. Git snapshot files are validated from the canonical package before Worker context generation.
- Only validated UTF-8 text representations enter the bounded Worker context. Unsupported siblings receive explicit per-file exclusions.
- `ModelInputManifest` pins an immutable MissionInput revision. The control boundary regenerates the context from company CAS before recording delivery receipts.

## Dependencies and invariants

- Uses Go standard-library archive readers and `intake` validation; CAS writes remain in the Kernel shell.
- Archive paths are data labels only. No archive entry may choose a host destination, command, network target, or credential.
- R2 CSV and ZIP paths are local foundations, not release-qualified support profiles.
