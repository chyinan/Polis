# Implementation Status & Next Steps

This document is a lightweight, current view of where the project is. Detailed historical reports and verification artifacts remain under `docs/implementation/` and `evidence/` for reference.

## Current state (high level)

- **Workbench & product flows:** company/organization management, mission/task flows, workspaces/checkpoints/artifacts, collaboration and activity views, operator instructions, operations/health views, and local deterministic notification records/routes are implemented in the current backend + frontend (`RealWorkbenchApi` is the default frontend path).
- **Runtime configuration:** provider/model/profile settings (including local deterministic/fake modes and configured HTTP-compatible providers) are supported with secret-safe readiness handling. Credentials are stored/handled by the backend and are not exposed to the UI.
- **Capabilities:** skill/MCP catalog import/registration, bindings, probing, descriptor/summary reads, and MCP tool-call execution with request validation and audit receipts are implemented. Runtime policy favors usability where prerequisites are satisfied while preserving kernel safety invariants and receipts.
- **Desktop:** a Tauri-based desktop shell exists (see `R0.7_DESKTOP_REPORT.md`).

## Known gaps / areas to improve

- **Real external integrations:** external notification delivery to third-party platforms, remote MCP usage in all environments, and real provider-backed autonomous execution depend on environment, credentials, adapters, and further hardening.
- **Operations & metrics:** some usage metrics (tokens, cost, elapsed runtime) may be unavailable until authoritative runtime sources are fully wired.
- **UX depth:** richer workspace/artifact browsing, more advanced collaboration workflows, and additional operator controls can be expanded incrementally.
- **Verification:** continue running full Go tests, frontend checks, and DB-backed integration tests in your environment; some tests require disposable PostgreSQL instances/configuration.

## Suggested next steps

1. **Validate end-to-end locally:** start PostgreSQL, run migrations, start the backend/workbench API, and run the frontend to exercise the main local flow.
2. **Configure runtime & capabilities:** set up a provider/profile and register/bind the skills/MCP servers you need; verify probing and tool calls against your targets.
3. **Harden integrations:** add/adjust adapters, auth, and deployment configuration for external services you rely on (notifications, remote MCP, hosted providers).
4. **Iterate on gaps:** pick a concrete workflow gap (metrics, workspace UX, notification adapter, etc.) and implement it with tests.

## References

- `README.md` — project overview and setup
- `AGENTS.md` — contributor/agent guide
- `R0.6_DOGFOOD_MVP_REPORT.md` / `R0.6_DOGFOOD_GAP_MATRIX.md` — baseline capability/gap snapshot
- `docs/implementation/` and `evidence/` — historical reports and verification artifacts (archival)
