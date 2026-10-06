# REQ-36 DependencyChange proposal/approval layer

Slice282 adds the offline durable intent layer for dependency changes.

Proposals are canonical, bounded JSON with exact semver dependency versions and registry hosts limited to the base environment policy. They are append-only, Company/Mission/revision scoped, and owner decisions append immutable `approved`/`rejected` events. Approval does not run npm, alter package files, create a lockfile, or generate a new environment revision; a qualified executor must later materialize and re-register a verified revision.
