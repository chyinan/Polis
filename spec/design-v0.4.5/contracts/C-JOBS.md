### 45.4 C-JOBS：命令、长作业与开发服务 <a id="subsection-45-4"></a>

#### 45.4.1 一份执行身份，不造第二个 Shell 运行时 <a id="subsection-45-4-1"></a>

普通本地开发允许在 Task/Attempt 的受限环境内运行合格命令。`JobRun` 统一包装已认证 harness 原生命令或 supervisor 执行；同一次真实执行只有一个 JobRun，不将原生工具事件与外围脚本记成两次独立执行。`scripts.run` 是固定 Skill 入口的专用路径，不等于员工只能运行提前写好的技能脚本。

命令记录 cwd/workspace、argv 或显式 shell 脚本文本的 hash、工具环境 revision、env 白名单、输入策略、所用授权、超时/输出/资源限制，以及 company/employee/task/attempt/incarnation/epoch。执行自由度由实际沙箱约束；不能只靠字符串禁词宣称 Shell 安全。共享分支更新、真实发布和外部账号写入仍必须走 Action，不能藏在普通命令后面。

| Job 种类 | 生命周期目标 | 完成/异常判定 |
|---|---|---|
| batch | 构建、测试、有限数据处理 | 退出码/结果文件与证据齐备；0 只表示进程正常结束，不等于任务验收 |
| service | 前端 dev server、后端 API、测试 DB | 独立 startup timeout、就绪探测、有限驻留寿命；持续运行不是命令超时 |
| controlled_input | 明确获准的有限交互程序 | 有 typed prompt 与输入渠道；默认 stdin 关闭，不无期限等人 |

首版不提供通用 Web Terminal。遇到未支持的登录/密码/确认交互时报告 `input_required`，只通过受信身份流程处理，不将密钥写进普通聊天。测试脚本需要 TTY 且未获资格时明确 unsupported，而不是让 Agent 反复空等。

#### 45.4.2 状态、日志与暂停 <a id="subsection-45-4-2"></a>

`accepted → starting → running`，service 另记录 `readiness=not_ready / ready / unhealthy`；结束为 exited/failed/cancelled，无法核对保留 outcome_unknown。取消 ACK 不等于进程树和远端工作已经停止。日志有 offset、truncated/gap、大小限额和来源；长输出落有界文件引用，不把所有 stdout 注入模型或控制 DB。

员工可用 `jobs.start/status/logs/stop`；状态和日志查询不额外调用模型。`jobs.start` 带幂等键，回执丢失先核对现有 JobRun。普通工作等到服务/测试事件时休眠；JobRun 的运行时长、工具费用与资源不因模型休眠而清零。全局急停路径不被日志洪峰或满员作业阻塞。

#### 45.4.3 服务地址、借用与真实就绪 <a id="subsection-45-4-3"></a>

服务只能绑定其隔离网络内的允许地址/端口。记录 `ServiceEndpoint` 与服务实例 generation，不能按“3000 端口有人回应”就认定是当前项目；ready 探测必须同时验证所属进程/容器、预期代码 revision 和配置的健康响应。禁止自动 reuse 不明来源服务。Playwright 已提供命令、ready URL、多服务及关闭配置；本项目需要在其外保证资源身份和版本正确。[W03](../ARCHITECTURE.md#ref-w03)

前端/测试员工经已批准的 peer access 借用服务，得到对特定服务端口的访问授权，不获得所有者文件写权、宿主网络或 DB 权限。记录消费者和 TTL；有获授权消费者时不会仅因原员工睡眠立即关闭，无消费者且到 idle deadline 后由程序回收。明确关闭/撤权优先于普通借用；调用者得到清晰失效状态，不转发到刚被复用的新端口。

开发模式可以带热更新，但记录 mutable=true，不能用它的截图为不可变 candidate 最终验收。验收服务从冻结产物/干净快照启动，截图和结果绑定该 service generation。

#### 45.4.4 换班、重启与回收 <a id="subsection-45-4-4"></a>

长期服务由 supervisor 管理，而非依赖模型连接维持。换脑时核对旧作业：写当前工作区的构建/测试必须排空或隔离，不能旧写者仍在、新员工就接着改；冻结快照上的只读服务可在明确重新授权后保留，但旧会话不能再控制它。转交控制 handle 使用新 epoch，历史 producer attempt 不改写。

宿主重启后不凭复用 PID 或 open port 认领进程，需 supervisor 的 job/container identity 与 generation；无法确认则停止/隔离重建或保持 blocked，不猜。收尾撤销预览/借用与测试账号，清理只针对所属服务、缓存和卷，不杀其他公司进程。`JobFinished`、`ServiceReady` 与 `ArtifactAccepted` 是不同事件。

此文件为主文档对应段落的自动副本，请修改主文档后重新生成。
