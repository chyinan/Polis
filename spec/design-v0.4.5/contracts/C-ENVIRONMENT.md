### 45.3 C-ENVIRONMENT：正常依赖准备不是隐形插件安装 <a id="subsection-45-3"></a>

#### 45.3.1 控制软件的栈与目标项目的栈分离 <a id="subsection-45-3-1"></a>

控制面继续用 Go/PostgreSQL，工作台用 React。员工开发的项目可以采用不同语言；首版只认证一个明确的 Node/npm 项目 profile，其余语言在同一环境接口下逐项扩展，不因为仓库写了 Python/Java/Rust 就自动拥有相应运行能力。

`ProjectEnvironmentRevision` 保存：工具链/镜像指纹、代码输入 revision、lockfile 指纹、依赖来源、安装步骤/脚本政策、构建/测试/启动入口、必要服务与健康条件、工作根与 scratch、网络出口、资源上限、缓存边界与资格记录。项目内 README、package.json、devcontainer.json 可提出候选配置，但不自动成为可信部署脚本。

人可以一次批准项目环境政策；授权内重复安装、构建和重建无需逐次审批。`environment.ensure` 查询已有准备结果，必要时创建独立 EnvironmentPreparationRun；必须先准入再执行，多个员工同时请求同一准备键不会并行破坏同一依赖目录。共同可复用的是已固定的只读环境层，各员工的可写工作目录独立。

#### 45.3.2 明确允许的依赖操作 <a id="subsection-45-3-2"></a>

**首版禁止的是未授权的安装、宿主全局修改，以及发现/加载 Skill 或 MCP 时偷偷安装。不是禁止在隔离项目环境中，按批准政策自动准备依赖。** 第 44 章的自动安装范围以此限定。

Node 参考 profile 使用已确认 lockfile 的 clean install。`npm ci` 要求锁文件、会因清单不匹配而失败且不会替用户更新锁文件；它仍要单独处理安装脚本。`ignore-scripts` 不意味着之后显式 `npm run` 也不执行代码。[W02](../ARCHITECTURE.md#ref-w02)

参考安装按固定 npm 版本执行 `npm ci --ignore-scripts --no-audit --no-fund`，但这只是一个可审核步骤，不是万能沙箱。需要生命周期脚本的项目应通过明确的 install-script policy，限定解析到的包版本、网络和资源后执行；不能遇到失败就自动改成允许全部脚本。不同 npm 版本的脚本策略能力以资格测试为准，不凭某个新 flag 假定所有环境已支持。

lockfile 中的 tarball/git/file/registry 目标、项目 `.npmrc`、额外下载及工具遥测都是网络/文件输入，受批准来源和最小 env 限制。不要把广域 registry 凭据留给后续任意测试；支持凭据访问的准备阶段与无凭据执行阶段分开，或将必要短期凭据与该受信工具范围绑定。安装内容仍是不可信代码。

开发中确需新增依赖，可在预授权的来源、许可类别、脚本/体积/网络边界内提出并接受 DependencyChange，生成新 lockfile 和环境 revision；越过边界才找人。不得“每加一行 import 就审批”，也不得无声扩大宿主/平台权限。无锁文件或其他语言缺合格 profile 时，返回 `environment_plan_required` 并交固定规划/开发岗提出方案。

#### 45.3.3 附属服务和环境状态 <a id="subsection-45-3-3"></a>

测试数据库、缓存、模拟 API 使用独立 ToolService/临时卷和最小测试凭据；**禁止将组织控制数据库当作被测应用的业务库**。参考 Node demo 可以先不需要外部数据库；使用数据库项目的资格测试另包含初始化、迁移、清理和数据隔离。

环境状态为 `unprepared / preparing / ready / failed / dirty / revoked`，由准备回执和健康检查决定，不由模型自报 ready。准备失败保留原因与有界输出，不反复触发所有员工安装。源代码、lockfile、镜像、政策或工具链变化后，按受影响依赖失效；不把任意旧 node_modules 当验证基线。

生产/审查缓存按信任级别隔离，已变更可写缓存不作为独立验收唯一依据。环境资源计入公司/使命预算，长作业和数据库临时数据纳入回收；保留正在被任务、接班或交付引用的必要层。`EnvironmentReady` 是一个可核验依赖，可以唤醒等待任务；普通检查由程序完成，不烧模型轮询。

此文件为主文档对应段落的自动副本，请修改主文档后重新生成。
