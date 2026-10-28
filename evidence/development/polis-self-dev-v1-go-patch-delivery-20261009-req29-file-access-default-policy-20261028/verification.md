# Slice 333 — REQ-29 conservative file-access defaults

Date: 2026-10-28
Result: `PRIVATE_SNAPSHOT_ONLY_POLICY_SELECTED`; native host-root access remains unavailable.

## Selected product defaults

- `companies.workspace_root` is descriptive metadata, not filesystem authority.
- File visibility stays Company-private. There is no cross-Company or Group/public-library sharing by default.
- Work enters through explicit owner/user file-selection uploads and becomes immutable Company CAS snapshots. Workers use the existing logical CAS workspace; no Worker writes back to host roots.
- Workspace-snapshot revocation blocks later direct-ID reads but does not recall bytes already returned. Referenced snapshot CAS data is retained indefinitely by default; no automatic snapshot deletion or CAS reference release is introduced.
- A native root binding, if later implemented, must be read-only, Company-scoped, owner-selected through a supported OS mechanism, bounded, and host-qualified before any Worker can use it.

## Source audit

Confirmed existing behavior in `internal/kernel/workspace_tree.go`, `internal/intake/directory.go`, and the CAS snapshot/revocation paths: `workspace_root` has no runtime authority; logical Task files are Company CAS data; directory uploads produce bounded immutable snapshots; workspace-snapshot revocation denies subsequent direct-ID reads while preserving references; and the generic collector removes only unreferenced CAS objects.

## Verification and boundary

- Documentation/source audit only; no code, schema, or runtime state changed.
- No filesystem root was opened, no database/Worker/Provider operation ran, and no external request or package download occurred.
- This decision closes default sharing/write/retention policy choices. Actual native root access still requires OS-native owner selection, no-follow/special-file/TOCTOU protections, durable binding/revocation, and target-host qualification.
