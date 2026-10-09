# Go 派发、Patch 交付和人工审核源码接线

整体状态：`PARTIAL`。源码已接入现有流程，并通过离线验证。宿主未安装、未取得资格；真实受限候选 Go 成功执行和完整 Patch 审核 E2E 均为 `NOT_RUN`。

## 开发身份

唯一工作目录 `D:/Programs/Polis-cloud-main`。开始 HEAD `de9089bce39039538ab0e5acad7ea7215f1d13c4`，tracked worktree 干净。保留 cdfcd169 及其后历史，未读取、修改或暂存原 reconciliation 未跟踪目录。未访问旧 checkout 的源码/数据，未 reset、clean、stash、切换 checkout 或 push。

源码 checkpoint：`f824e06a1ccd2abcd06a39d80f9daf18c1b95da6`。之后只补充初始化诊断测试的退出同步及本轮证据。生产 Go source identity 保持 `247346a11a1ed5bb935f24f0a155e0f93459a112adc43db27468c326de844b13`。

## A：待批准的现有宿主方案

完整实物展开见 `host-installation-review.md`，包括现有 installer 的两份 systemd unit、polkit rule、安装/卸载命令、文件拥有者及权限、修改者、允许调用者动作和删除集合。这是现有脚本的审阅副本，没有新增平行方案。

当前待审批 supervisor：`D:/Programs/Polis-cloud-main/bin/polis-selfdev-go-job-supervisor`，Linux amd64，协议 `polis-go-job-supervisor@1`，profile `polis-go-test-fixture@1`。

完整 SHA256：`b07af7b7aeca924721d6fba4ac49ba01f0d9e7bee95d3f3c8f642c14dd66e377`。基线 `76fe…` 仅作历史记录，不能授权替换当前产物。没有向 `/usr/local/libexec` 安装任何版本。

管理员修改 root-owned unit/rule/installed binary/identity/sha256；chyinan 仅获限定实例 start/stop/kill 的拟议授权。workspace 中的脚本、探针和 toolchain 仍可由开发用户修改；签发 root-owned qualification receipt 前必须审阅实际源码、摘要及真实探针证据。候选不见 cgroupfs、runtime handoff、数据库、凭据或原始仓库。

仅这一 WSL distribution 中的专用 systemd 实例和私有 tmpfs：CPU 200%、memory 2 GiB、swap 0、pids 256、墙钟 120 秒、scratch 1 GiB/4096 全部 inode、每路日志 512 KiB。限制未放宽。没有安装、提权、mount、全局安全策略、WSL 配置、宽泛 sudoers 或 cgroup 权限修改。

```ini
supervisor_binary_built = YES
setup_plan_valid = YES
privileged_apply_authorized = NO
host_execution_profile_qualified = NO
```

## B：已实现的源码路径

1. 旧“仅预检”入口保持默认关闭。真正执行需要操作者显式 `executeGoTest=true`，以及批准的固定 Go Task、精确冻结快照和合格宿主。安装或构建成功不触发执行。
2. 使用现有 JobRun/project-job lifecycle 的 context、active map、状态机和取消边界。数据库串行化同一 Task 的未完成 Go 作业，原 RequestID 不重复执行，未知结果不自动重跑。
3. systemd adapter 只调用固定 start/show/stop 和固定模板实例。执行程序须为可信 root-owned 安装产物，字节摘要匹配 root-owned seal。作业内 supervisor 在任何候选代码前再次核验实际 cgroup/scratch 条件。
4. 观察 invocation 后才认为 unit 已完成。回收接口只读取已有请求/结果和 unit 状态，不调用 start。丢失结果、未观察退出或清理未确认保持 `outcome_unknown`，不生成 exit code 0 或测试 PASS。
5. 注入 executor 一律强制标为 simulation；伪造 real/PASS 输出被拒绝。通过结果必须从有界 Go JSON event stream 重算完整固定测试集及 result digest，不能信任 PASS 标签。
6. Go 验收绑定使用明确 fixed Go runner。文本条件不能替代执行，既有冻结绑定没有重写。Patch producer 从真实 JobRun/CAS 证据和两个冻结快照重建输入，复核候选 revision/digest、当前 writer、memory clean 和正式变更边界。
7. 可信控制面构建 Patch 并重放到独立逻辑 base 副本，验证文件内容/元数据与候选一致。复用 staged CAS、worker check、qualified checkpoint、Artifact、TaskValidationBinding qualification、DeliveryManifest 和 UserDisposition；没有第二套调度或审核状态。
8. `content_format=go_patch` 明确区分 Patch 与旧 opaque Artifact。schema 133 保存 Patch/JobRun/candidate/result 的不可变关联和数据库约束，原 opaque 限额和既有 qualification policy 保留。不会将 workspace_snapshot 改名标为 ready。
9. 正式交付包从已验证 CAS 读取完整 Patch 字节；Workbench 单独显示 base/candidate/patch/test 身份、文件摘要、64 KiB 有界 diff 和受保护日志。Ready manifest 仍需现有完成策略及限制/来源声明；人工接受/请求修改沿用原表单，只记录决定，不自动 apply、merge 或 deploy。

修复了接线测试揭示的具体缺陷：原 Patch inventory digest 与真实 Go JobBinding 算法不兼容。Patch 现在复用原 Go inventory digest，未改变已冻结 JobBinding 的语义。

## 最终验证

最终命令入口：`POLIS_SELF_DEV_EVIDENCE_SLICE=polis-self-dev-v1-go-patch-delivery-20261009 bash scripts/self-dev-go-job-verify.sh`。复用现有 PG18 harness，所有记录位于本轮新 slice，没有覆盖历史 qualification evidence。

完整最终运行：`verification-runtime/20261009T023901Z-303/status.txt`，exit 0。

| 检查 | 结果 |
|---|---|
| Go tests / race / vet | 全部 exit 0。Go/race 各 1630 pass、0 fail、239 skip；无 PG DSN 模式的 skip 不计为 PG PASS。 |
| Linux / Windows command build | 均 exit 0；Windows build 不代表执行资格。 |
| supervisor build、help/input/identity、prepare、脚本语法 | PASS，均为非候选检查。 |
| 完整 PG18/CAS suite | 1884 pass、0 fail、7 skip（含子用例）；exit 0。 |
| migrations / grants | PostgreSQL 18.6，schema identity `18:133`；历史 1–132 未改；S/I/U 与 sequence usage/select 保持，DELETE 拒绝。 |
| frontend | 19 files / 163 tests，typecheck、lint、build 均 exit 0，未关闭规则或扩大 ignore。 |
| diff-check | 完整 staged diff 检查 exit 0；原 reconciliation 未暂存。 |
| property test | Go inventory ordering fuzz，5 秒预算，9307 次执行，exit 0。只运行可信纯函数测试。 |

最终专用 database `polis_r0_self_dev_go_1791513994_8698`、runtime role `polis_self_dev_go_runtime_1791513994_8698`，私有 Unix socket、TCP 关闭，独立 case databases 与 CAS。cluster 正常停止并清理，未用业务 DB 或历史 specimen。

7 个 skip 为 Windows Chrome host、三个缺少固定 native binary 的 Codex 检查、两个需历史 R0.2 evidence 的检查和 controlled-provider runtime manifest 检查。未计入 PASS，没有数据库用例因缺 DSN 跳过。

真实 Go API + real PostgreSQL/CAS/read-store 定向测试覆盖显式模拟初始化失败、伪造 PASS、取消、默认关闭、重复请求与并发拒绝；每个作业只有一份终态结果，qualification 和 Patch rows 均为 0。过期候选、错误 receipt、摘要篡改及重放不一致由定向事务/纯契约测试覆盖。纯合成数据和 SSR 展示测试不写入真实 qualification，也不代表真实 Go 运行。

首次完整 race 曾在未修改的初始化诊断测试失败：EOF 抢先于 runner 记录进程退出。补齐“已观察退出”的有界测试前提，保留全部断言和生产诊断；单用例连续 30 次 race 及最终完整 suite 通过。第一次失败记录仍保留。没有增加真实 provider 请求或修改 @4 工具注册表。

## 分开报告

```ini
source_wiring = IMPLEMENTED_WITH_OFFLINE_VERIFICATION
host_execution_profile_qualification = BLOCKED
real_restricted_go_job_success = NOT_RUN
real_patch_delivery_browser_review_e2e = NOT_RUN
self_dev_go_job_patch_offline_slice = PARTIAL

provider_egress = 0
real_model_worker_started = 0
candidate_project_execution = NOT_RUN
live_canary = NOT_RUN
automatic_merge = NO
automatic_deploy = NO
```

源码接通不代表真实成功路径已取得资格。下一步等待对 `host-installation-review.md` 中当前摘要和精确配置的明确批准；安装后还须真实探针、root-owned seal、真实受限 Go 成功/失败/取消清理和正式 Patch/浏览器审核 E2E。缺失的真实环境证据不以测试 double 填补。

本地复核仅覆盖本轮 diff。仓库不允许在没有经确认 active WorkerSession 的情况下委派，本轮未为审查创建 Worker，未声称完成外派独立审查。真实开发 Worker surface/admission 和 provider live qualification 仍须另外授权；本轮不开始下一阶段。
