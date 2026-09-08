# 技术基线与本轮增补 · Draft 0.4.3

主规范：[第 18 章](../ARCHITECTURE.md#section-18)，能力规范：[第 44 章](../ARCHITECTURE.md#section-44)。

Go + PostgreSQL 18 + pgx/sqlc/goose + React/TypeScript/Vite/TanStack Query + HTTP/SSE 保留。前端继续 React Router/Radix/Lucide/语义 CSS Modules，办公室延期。

能力模块在现有 Go 控制面内实现；MCP 协议首选官方 Go SDK，精确版本和 profile 能力在 G0 认证。服务自己所需 Python/Node 是独立工具环境依赖，不是第二控制面技术栈。

只部署一套权威任务/费用/事件存储；Skill 固定文件产物，数据库保存索引、绑定与调用，MCP 不另建业务账本。公共目录是导入来源，不是任意执行路径。

本轮 ADR-049～056 接受为实现方向，不是性能或安全已经通过的声明。


## 0.4.4 工具环境补充
控制面Go/PG/React基线不变。目标项目可用隔离Node/npm，浏览器参考Playwright/Chromium，不引入第二控制后端。允许批准政策内的项目依赖准备，不允许加载技能/服务时隐形安装。首个外部反馈选择GitHub Issues只读轮询；所有具体版本与账号资格待测。
