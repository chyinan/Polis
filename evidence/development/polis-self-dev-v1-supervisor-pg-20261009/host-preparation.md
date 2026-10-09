# 专用宿主准备：待授权，未执行

本轮只执行了构建、只读检查和有限可信 bwrap 探针。没有执行以下 root 操作，也没有运行候选 Go 项目。

实际宿主：WSL `Ubuntu-22.04`，UID/GID `1000`，内核 `6.6.87.2-microsoft-standard-WSL2`，systemd 为 PID 1。用户子树只有 memory/pids，CPU 未委派，`cgroup.kill` 不可写。专用 systemd unit 与 scratch 尚未安装。原始 inspect 输出保存在本目录的 `verification-runtime`。

## 待批准的精确操作

在该 WSL distribution 中，从 `/mnt/d/Programs/Polis-cloud-main` 运行。先审阅：

```bash
bash scripts/build-self-dev-go-supervisor.sh
bash scripts/self-dev-go-job-host.sh prepare
bash scripts/self-dev-go-job-host-root.sh plan --target-uid 1000
```

以下命令需要用户另行批准；这里仅提供操作说明：

```bash
# 本轮只读 stat 确认这两个目录不存在。若状态变化，先检查实际目录及权限。
sudo install -d -m 0755 /usr/local/libexec /etc/polkit-1/rules.d
sudo bash scripts/self-dev-go-job-host-root.sh apply --target-uid 1000 --confirm
```

精确新增路径：

- `/etc/systemd/system/polis-self-dev-go-job@.service`
- `/etc/systemd/system/polis-self-dev-go-probe@.service`
- `/etc/polkit-1/rules.d/49-polis-self-dev-go-job.rules`
- `/usr/local/libexec/polis-selfdev-go-job-supervisor`
- 同路径的 `.identity.json`
- `/etc/systemd/system/polis-self-dev-go-job-supervisor.sha256`
- `/run/polis-self-dev-go-job`，0700，属于 UID 1000。

只安装两个固定模板和限定实例 start/stop/kill 的 polkit 规则；不创建 sudoers 规则、不改全局 cgroup 权限、不改 WSL 配置、不自动启动 job。helper 拒绝覆盖已有文件、符号链接或身份不匹配的产物。

## 核验入口与限制

```bash
bash scripts/self-dev-go-job-host.sh verify
```

此入口只运行可信有限探针，验证 quota、禁网、只读路径和整组停止，不运行候选代码。CPU 200%、memory 2 GiB、swap 0、pids 256、总墙钟 120 秒、scratch 1 GiB/4096 inode、每路日志 512 KiB 均不放宽。4096 约束整个 tmpfs 的所有 inode，包括目录、文件和链接，不是“4096 个普通文件”。

scratch 保留 nodev/nosuid，但不能使用 noexec：Go 编译后的测试二进制需要在 scratch 执行。候选源码、工具链和依赖仍只读；候选挂载不含宿主 cgroupfs、handoff 目录、数据库或凭据。

`verify` 的探针结论不等于 supervisor admission receipt。supervisor 还要求由管理员审阅真实探针证据后封存的 root-owned `/etc/systemd/system/polis-self-dev-go-job-host-qualification.json`，绑定本次 boot、runner、完整 toolchain tree、bwrap、probe digest 和固定资源配置。本轮未生成或安装该 receipt，不得填入猜测 digest。当前产品控制路径只记录预检结果，即使探针通过也不会派发候选；正式 systemd dispatch/结果回收接线和成功交付仍待后续验证。

## 回滚

先停止实际存在的精确 job/probe unit 实例，不使用全局进程或 WSL cleanup。然后：

```bash
sudo bash scripts/self-dev-go-job-host-root.sh cleanup --target-uid 1000 --confirm
```

helper 先验证完整删除集合的 marker、二进制及 identity digest；只删除匹配路径。非空 runtime handoff 保留作证据。共享父目录保留；只有确认是本次新建且为空时，才可额外用 `rmdir` 删除两个父目录。未改变业务数据库、运行中的 Polis 或原始源码仓库。
