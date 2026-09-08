### 42.3 C-EMPLOYEE-OPS：员工如何使用组织，而非只让组织管理会话 <a id="subsection-42-3"></a>

#### 42.3.1 固定员工有独立生命周期 <a id="subsection-42-3-1"></a>

每个固定 Employee 由运行时管理独立 Session/Attempt。规划员不是所有员工必需依附的父会话；规划员掉线不删除其他成员。底层 harness 原生 subagent 不是新 Employee，默认禁用未受管理的递归 delegation；确需启用时按第 14、20、29 章单独认证并计费。

定义小型、版本化 `employee-ops@1` 工具合同。可由适配器映射到经过资格测试的动态工具/MCP 等入口，内部语义保持一致；不为接一个后端再建立第二份任务真相。未经验证的工具接入标为 unverified，不用 TUI 敲键或自然语言猜测冒充稳定协议。

#### 42.3.2 首版最小操作集 <a id="subsection-42-3-2"></a>

| 操作 | 允许输入 | 确定性结果与权限限制 |
|---|---|---|
| `work.current` | 任务选择/游标，默认当前任务 | 返回授权输入、相关责任、版本和剩余额度口径；不调用模型 |
| `context.read` | 明确资源引用、范围或有限检索请求 | 先鉴权再读取；大小有界、带来源/版本；不自动全仓探索 |
| `workspace.list/search/read/snapshot` | 授权根、文件/范围/版本，快照限当前拥有者 | 明确文件发现与快照；细则见 44.3/44.7；读取不等于写或自动共享 |
| `skills.list/load`、`scripts.run` | 固定技能版本或已批准脚本入口 | 分层加载；不因读到说明就执行或提权 |
| `tools.list/describe/call`、`resources.list/read` | 当前绑定下的工具/资源句柄 | MCP 中介、逐工具授权、外部效果/结果核对沿既有规则 |
| `collab.read` | 当前作用域、服务端游标、有界页大小 | 读取可见消息，返回安全水位；阅读不自动关闭责任 |
| `collab.ack` | 消息 ID、阶段、相应版本 | 记录被纳入当前上下文/已观察的声明，去重；不等于 applied |
| `collab.send` | 接收 Employee、任务、消息类型、期望动作 | 持久消息 ID；需要行动时经准入建 Obligation/WorkSignal；不是强制抢占 |
| `obligation.resolve` | 责任 ID、期望版本、处理方式和证据引用 | 提议 fulfilled/declined/superseded；内核验证责任人、规则和证据后闭合；不能自填成功 |
| `contract.propose` | 当前合同 revision、变更产物、影响范围 | 形成 ContractProposal；批准由已有权责规则执行，不直接改当前合同 |
| `memory.propose` | 类型、来源、适用范围、引用；更正时指向旧 revision | 保存事实/决定/假设或更正提议；verified/生效更正必须经过现有确认规则 |
| `work.checkpoint` | 简短进展、已完成证据、未决事项、下步建议 | 保存已接受的结构化记录；自由文本不改验收/预算/权限 |
| `artifact.submit` | 产物引用、基线/输入版本、验证说明 | 校验已固定产物，进入 candidate，绝不自动 accepted |
| `work.block` | 原因类别、关联依赖/ProblemKey、恢复条件 | 明确阻塞，可交还推理槽；不无限发同一问题 |
| `work.yield` | 等待原因、检查点及工具句柄 | 程序核对在途动作与所有权；释放推理不等于释放写租约 |
| `plan.propose` | 有限 PlanProposal 与 expected_revision | 仅规划职责可请求；受使命、资源、依赖和岗位检查 |
| `review.submit` | finding/判断、准确产物版本及证据 | 仅审查职责、检查作者来源分离；模型自述仍标明来源 |

进入接班核对的会话另有 `handover.confirm`，只用于报告版本和未决事项；它不提供任务写权限，不等同 activation。`employee-ops@1` 尚未发布，因此本轮直接补全 v1 草案；首个真实 release 后新增能力必须协商/版本化，不能静默改变已部署 v1 含义。

#### 42.3.3 身份、幂等、错误与大小 <a id="subsection-42-3-3"></a>

实际 actor、company、employee、session、attempt、epoch、授权能力来自运行时签发的会话绑定。worker 提交的上下文 ID 只作为交叉核对和定位，不能用于自授身份。restoring 与 active 使用不同 grant 类别。

写操作带 request_id、idempotency_key、expected_revision、协议版本；相同键且不同正文返回冲突。工具结果至少区分 persisted、operation_pending、rejected、outcome_unknown，并提供 receipt/operation ID。服务端返回 `STALE_EPOCH`、`REVISION_CONFLICT`、`OUT_OF_SCOPE`、`INPUT_TOO_LARGE`、`POLICY_DENIED`、`MALFORMED_INPUT` 等稳定错误码，而不是让下一层解析中文错误句子。

事件正文、单次检索、日志片段和重试均有可配置上限。格式错误允许受预算约束的修正；不能把提交失败解释为完成。任务/工具/进程真实观察由程序增量落盘；模型不写日报也不能使已知回执丢失。服务端游标只确认扫描进度，不冒充员工已读；work.checkpoint 不能删掉未闭合 Obligation。恢复只读会话仅可使用授权读取和 handover.confirm，以上新的写操作均不可用。

#### 42.3.4 接入边界 <a id="subsection-42-3-4"></a>

Codex app-server 继续是第一后端；员工协议绑定须在固定版本上验证注册、调用、事件、重复请求与恢复。第二条真实路径必须报告是“同 harness 换模型”还是“跨 harness/供应商接班”。只完成前者不得声称后者通过。

普通 QQ 通知不向任何员工公开 `notify.send` 或 QQ 凭据。员工只能提出有证据的阻塞请求，由运行时判断是否转化为需要人处理的通知。

此文件为主文档对应段落的自动副本，请修改主文档后重新生成。
