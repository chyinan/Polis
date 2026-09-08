# Polis

Canonical project and product name: **Polis**.

Positioning: “Polis — A runtime for persistent autonomous AI organizations.”

Short product idea: “Build AI organizations that outlive their models.”

Chinese positioning: “Polis｜长期自治 AI 组织运行时”

## Implementation naming

- CLI binary: `polis`.
- Daemon/service binary: `polisd`.
- Repository/project display name, documentation and UI branding: `Polis`.
- Use neutral internal Go package names; avoid unnecessary legacy `agent-org` branding in module/package names. The canonical module import path is not yet specified.
- Use Polis naming for systemd, service and configuration artifacts when implemented.
- Draft 0.4.5 names such as `orgctl` and `orgd` are provisional conceptual names. Do not mass-rename historical design artifacts or quoted examples.
- New concrete artifacts must use Polis naming directly. If a provisional name enters a real public API, filesystem contract, migration, service name or package, stop and normalize it before continuing.

This naming update does not change the architecture, scope, permissions, testing gates or implementation slice defined by Draft 0.4.5 and the kickoff instructions.

## Baseline availability

On 2026-09-08, the designated workspace was empty and was not a Git repository. The supplied kickoff instructions are preserved in `CODEX_START_v0.4.5_2026-09-08.md`.

The design pack was subsequently supplied and verified. `spec/design-v0.4.5/` directly contains `00_START_HERE.md` and `ARCHITECTURE.md`; `spec/archives/` preserves the source ZIP. Current implementation and verification status is recorded in `docs/implementation/PROGRESS.md`.
