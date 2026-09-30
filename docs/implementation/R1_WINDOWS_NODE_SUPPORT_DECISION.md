# R1 Windows/Node support decision

> Accepted: 2026-09-23. This records the user's platform choice for the R1–R3 implementation.

## Decision

R1 project execution uses a native Windows/Node environment. R2 adds the Linux/Node reference environment from design v0.4.5. Keep the original design package and archive unchanged; this record supersedes only the implementation schedule and supported-environment matrix.

## Reason

The current product has a Windows Tauri host. The user selected native Windows/Node as the first project execution profile. Running R1 through WSL would not satisfy that choice.

## Implementation constraints

- Keep operating-system process, path and filesystem behavior behind Windows-specific adapters. Keep policy and domain rules platform-independent.
- Pin the exact Windows and Node/npm versions during offline qualification against a disposable project and its lockfile.
- Use Windows process-tree ownership, NTFS path/reparse-point checks, and isolated browser profiles. Do not assume POSIX path or symlink semantics.
- Qualify Linux/Node separately in R2. Do not infer its support from Windows test results.
- Do not change the semantic contents, checksums or historical evidence of `spec/design-v0.4.5/`.

## Acceptance

The R1 support record identifies exact Windows, Node/npm, browser and isolation versions; local fixtures pass path-boundary, install-policy, process-stop, browser-isolation and recovery checks. External accounts and live provider calls remain separately authorized.
