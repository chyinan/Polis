# Runtime settings and Codex UI

## Relevant flow

- `frontend/src/pages/WorkbenchPages.tsx`: `SettingsPage` reads `RuntimeSettingsView`; `RuntimeSettingsPanel` owns the editable provider, model, effort, and profile state and submits it through `useUpdateRuntimeSettings`.
- `frontend/src/data/workbench-query.ts`: the update mutation invalidates the company-scoped runtime settings query after success.
- `frontend/src/domain/workbench.ts`: the runtime settings view carries provider, model, effort, profile, authentication readiness, runtime readiness, and dependency statuses.
- `internal/control/service.go`: provider values are restricted to `deterministic`, `fake`, or `codex`; model, effort, and profile are stored as required strings. Updating desired settings does not restart the active WorkerAdapter.
- `frontend/src/styles/workbench.module.css`: shared design tokens and responsive breakpoints style the settings panel; the detail grid collapses below 860px and form controls use a shared field style.

## Codex catalog boundary

- `provider.CodexModelCatalogReader` starts a short-lived Codex app-server process, initializes it, reads paginated `model/list` responses, then stops it and removes its dedicated temporary directory after stop is confirmed. It does not start a thread or turn. When provider environment variables are absent, it can locate the installed OpenAI Codex CLI/helper under `%LOCALAPPDATA%\OpenAI\Codex\bin`, verify its version, stage a temporary hash-verified runtime manifest, and use the current user's `CODEX_HOME` or default `.codex/auth.json` path. The auth file is not packaged.
- The backend exposes this through the company-scoped `/settings/models` read route. The frontend only requests it when Codex is selected and uses the returned model-specific `supportedReasoningEfforts`; model IDs and effort values are not maintained in frontend code.
- If the runtime/catalog is unavailable, the UI reports the backend reason and offers retry or manual IDs. Runtime setting updates still persist desired strings and do not restart workers or start a model turn.
