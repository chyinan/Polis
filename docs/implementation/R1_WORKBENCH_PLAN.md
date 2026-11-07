# Polis R1 Workbench Skeleton：最小前端目录与组件计划

## Chosen approach

采用 fixture-first adapter：`WorkbenchApi` 是页面唯一依赖，`FixtureWorkbenchApi` 提供可验证的 simulated snapshot，`RealWorkbenchApi` 预留同形 HTTP/SSE 读取边界。当前仓库没有前端、HTTP Workbench API 或 SSE 服务，因此不先生成一个与后端不存在的 schema 绑定的客户端，也不把 mock 写进 React 页面。

考虑过的方案：

- 直接把 fixture 常量写进页面：最快，但页面会和数据形状耦合，后续对 Go API 时需要重写。
- 先扩展 Go API 再做 UI：语义上完整，但超出本轮 read-only 产品范围，会干扰 Core Track。
- fixture-first adapter（采用）：页面先稳定 View DTO、scope cache key、ActivityEvent 表示；Real API 只在明确后端 read API 存在后接入。

## Directory

```text
frontend/
  package.json
  index.html
  tsconfig*.json
  vite.config.ts
  eslint.config.js
  src/
    app/
      App.tsx
      app-shell.module.css
    components/
      activity-timeline/
        ActivityTimeline.tsx
        ActivityTimeline.module.css
      workbench-shell/
        AppShell.tsx
        AppShell.module.css
        ThemeToggle.tsx
    data/
      workbench-api.ts
      fixture-workbench-api.ts
      real-workbench-api.ts
      workbench-query.ts
    domain/
      workbench.ts
      activity-presentation.ts
      workbench-validation.ts
    pages/
      CompanyOverviewPage.tsx
      CompanyOverviewPage.module.css
      ActivityPage.tsx
      ActivityPage.module.css
    styles/
      tokens.css
      globals.css
    test/
      workbench-validation.test.ts
      fixture-workbench-api.test.ts
```

## Component boundary

- `AppShell`: fixed 232px scoped navigation, scope header, simulated/real mode marker, theme toggle, keyboard skip link.
- `CompanyOverviewPage`: mission, recent effective progress, attention, resource quality, team operating summary, bounded recent timeline.
- `ActivityTimeline`: unified user-facing ActivityEvent; text/icon/state tone; expandable evidence/debug metadata; no raw protocol JSON.
- `FixtureWorkbenchApi`: static R0.3A-derived DTOs, explicit `simulated` mode, no writes/model calls.
- `RealWorkbenchApi`: `fetch` wrapper and one centralized `EventStreamClient` boundary; no fixture fallback on real errors.
- `workbench-validation`: pure runtime guard for cross-boundary View DTO invariants; tests cover null semantics, scope, lifecycle distinction and unknown statuses.

## Data flow

```text
WorkbenchApi → TanStack Query scoped key → View DTO → React components
      ↑                                  ↓
Fixture adapter / Real HTTP+SSE      ActivityEvent presentation
```

The first shell defaults to fixture mode. `VITE_WORKBENCH_MODE=real` requires an explicit `VITE_WORKBENCH_COMPANY_ID` and selects `RealWorkbenchApi`; if the real API fails, the UI shows an error/empty state and never silently falls back to fixture. Overview and activity share the overview snapshot watermark for the initial read and the centralized EventStreamClient; incoming activity invalidates both scoped queries.

## Visual baseline

Use the design pack tokens exactly: light/dark semantic colors with the Paperclip-inspired dark-first presentation, single accent `#264E73`, 232px sidebar, 28px desktop/16px mobile page padding, 4/8/12/16/24/32/48 spacing, compact 4–6px controls/panels, system font stack, 150ms motion with reduced-motion override. Text labels are Chinese by default; IDs and revision/cursor values use monospace. Paperclip is a layout/density reference only; Polis does not copy its brand assets or backend.

## Explicitly deferred

No QQ, MCP, skills library, GitHub feedback, office/3D, production publish, dynamic hiring/firing, dangerous commands, backend schema migration, qualification runner change, evidence rewrite, or Core Track experiment execution.
