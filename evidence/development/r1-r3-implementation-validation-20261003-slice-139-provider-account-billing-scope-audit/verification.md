# Slice 139 — ProviderAccount billing-scope and owner-boundary audit

Date: 2026-10-03

## Findings

- The Slice 138 Codex account locator identifies a selected Codex account for request routing. It does not by itself identify a universal billable unit for every supported authentication and plan mode.
- Current official guidance distinguishes ChatGPT shared allowance/credit plans, Enterprise token-based USD billing, and API Platform organization/project usage. Credit estimates are not invoices, and some chat-level reporting may omit billable activity or lag.
- Codex Enterprise Analytics is available only to eligible Enterprise workspaces and requires a workspace-scoped Admin key with `codex.enterprise.analytics.read`. API Platform organization keys do not authorize ChatGPT workspace analytics. The source documents a reporting API; they do not establish local per-attempt settlement or real-time hard-cap guarantees.
- `internal/kernel/kernel.go` defines `Scope` using a Company ID only. There is no installation-level owner scope in the current kernel API. A Company owner route cannot safely configure a budget shared by the same provider account across Companies.
- The repository contains no `OPENAI_ADMIN_KEY` or `CODEX_ENTERPRISE_ANALYTICS_KEY` integration, and those environment variables are absent in this environment. No external account or workspace was queried.

## Required follow-up

1. Add an installation-owner authorization boundary and a global registry for observed provider-account locators before exposing shared cross-Company budgets.
2. Have a workspace/account owner identify the applicable billing mode and scope. For eligible Enterprise analytics, an authorized workspace admin must provision the documented scoped Admin key before live reconciliation can be exercised.
3. Keep financial reserve/settlement gated until the selected data source, time/account scope, data delay, and unknown-liability treatment are implemented and qualified. Do not convert protocol calls or estimated dollars into invoice facts.

## Sources inspected

- [Using Codex with your ChatGPT plan](https://help.openai.com/en/articles/11369540-using-codex-with-your-chatgpt-plan)
- [Reviewing Work and Codex usage and using Personal Analytics](https://help.openai.com/en/articles/20001478-reviewing-work-and-codex-usage-and-using-personal-analytics-in-chatgpt-desktop)
- [OpenAI API Platform organization usage and cost API](https://developers.openai.com/api/reference/python/resources/admin/subresources/organization/subresources/usage)
- Local code: `internal/kernel/kernel.go` (`Scope`); budget HTTP and command APIs remain company-scoped.

## Verification

| Check | Result |
| --- | --- |
| Official source review | PASS; current pages inspected 2026-10-03 |
| Repository scope/credential integration search | PASS; no installation scope or Admin analytics client found |
| Environment presence check | PASS; `OPENAI_ADMIN_KEY` and `CODEX_ENTERPRISE_ANALYTICS_KEY` absent (values were not read) |
| PostgreSQL/provider usage query | Not run; no authorized workspace key is configured |
| Tests/build | Not run; no code or schema changed |
| `git diff --check` | PASS |
