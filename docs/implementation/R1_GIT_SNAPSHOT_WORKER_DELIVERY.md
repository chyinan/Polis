# R1 Git snapshot Worker delivery

## Operator path

Use the local `polis` CLI to add a fixed commit to an existing Mission:

```text
polis import-git COMPANY_ID MISSION_ID REPOSITORY_PATH FULL_COMMIT_SHA REQUEST_ID [INPUT_ID]
```

`POLIS_DSN` and `POLIS_BLOB_ROOT` must point at the local Polis runtime. `REPOSITORY_PATH` must be the root of a non-bare working repository and `FULL_COMMIT_SHA` must be a complete 40- or 64-character object ID already present locally. If `INPUT_ID` is supplied, this adds a new immutable revision to that input; otherwise Polis creates a new input.

The import reads Git objects directly. It does not fetch, checkout, update refs, inspect worktree status, run hooks or filters, use credential helpers, or contact configured remotes. Working-tree files are never read or included; every Git snapshot is marked `partial` by policy, and its source note directs the operator to add a separate directory snapshot to include local changes.

## Snapshot contract

The importer stores a canonical `polis-git-snapshot@1` gzip/tar package in company CAS. Its manifest binds a repository identity digest, exact commit and tree IDs, selected ref, included file metadata, and per-path omissions. Absolute local paths and remote URLs are not stored. `.polis-git-source.json` is an included generated text file, so the Worker sees the commit provenance, the rule that worktree files were not included, and the omission summary alongside source files.

Regular files are read from the selected commit's blob objects. Symlinks are not followed; submodules and Git LFS pointer payloads are not fetched; unsupported or over-limit files are explicitly omitted. Their paths and reasons appear in the package manifest and bounded Worker source note. Any such omission produces a partial input. The source tree is limited to 250 entries and 7 MiB expanded; archive paths must be valid on Windows and Linux. Existing Worker limits still apply after parsing: at most eight text files, 16 KiB each, and 64 KiB total text, with four archives per Task context.

The Task manifest freezes the package digest and exact MissionInput revision. Control verifies the package from CAS, delivers the bounded text/provenance into the Worker turn, and writes the normal append-only per-file delivery receipt. Workbench classifies `git_snapshot` as an archive source and accepts partial Git candidates.

## Qualification status

The local importer and fake-Worker PostgreSQL path are verified offline. The dedicated test confirms the committed sentinel reaches the turn, uncommitted bytes do not, provenance is included, and the final receipt records both source and provenance files with `provider_egress=0`. This does not qualify live model ingestion, remote Git fetch, GitHub access, submodule retrieval, Git LFS, or project execution from the imported tree.
