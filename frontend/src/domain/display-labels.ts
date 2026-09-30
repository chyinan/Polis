// pattern: Functional Core

import type {ProjectEnvironmentPolicyManifestView} from './workbench';

const DISPLAY_LABELS: Readonly<Record<string, string>> = {
  upload: '单文件输入',
  directory_snapshot: '目录快照',
  zip_snapshot: 'ZIP 项目包',
  pdf_snapshot: 'PDF 文本快照',
  git_snapshot: 'Git 快照',
  accepted: '已受理',
  blocked_policy: '等待策略批准',
  blocked_source_unverified: '等待固定来源核验',
  blocked_unqualified: '等待隔离执行资格',
  environment_executor_not_qualified: 'Windows 隔离执行器尚未取得资格',
  environment_policy_manifest_unverified: '环境策略清单未验证',
  environment_policy_not_approved: '环境策略尚未批准',
  environment_preparation: '环境准备',
  exited: '已退出',
  not_approved: '未批准',
  revocation_required: '需要撤销旧批准',
  starting: '启动中',
  running: '运行中',
  unqualified: '未取得执行器资格',
  active: '运行中',
  approved: '已批准',
  applied: '已应用',
  blocked: '已阻塞',
  bound: '已绑定',
  candidate: '候选',
  cancelled: '已取消',
  closed: '已关闭',
  complete: '覆盖完整',
  corrupt: '已损坏',
  declined: '已拒绝',
  denied: '权限被拒绝',
  deferred: '已延期',
  disabled: '已停用',
  draft: '草案',
  estimated: '估算',
  failed: '失败',
  fulfilled: '已履行',
  fresh: '最新',
  inactive: '未运行',
  inconclusive: '结论未定',
  invalid: '配置无效',
  invalidated: '资格已失效',
  limited: '受限',
  local: '本地',
  github: 'GitHub',
  metadata_verified: '本地核验通过',
  'read_only_skill@1': '只读 Skill 元数据档案',
  'stdio_mcp@1': 'stdio MCP 元数据档案',
  'streamable_http_mcp_2026_07_28@1': 'Streamable HTTP MCP 2026-07-28 档案',
  missing: '缺少配置',
  mixed: '混合状态',
  not_applicable: '不适用',
  not_required: '无需认证',
  needs_external_qualification: '需要运行时资格',
  never: '尚未采集',
  not_started: '未开始',
  not_read: '尚未读取评论',
  not_delivered: '尚未交付给 Worker',
  local_context_loaded: '本地 Worker 已读取',
  provider_delivered: '已包含在模型请求中',
  not_sent: '未发送给模型',
  observed: '已观察',
  operational: '运行正常',
  pending: '待处理',
  needs_clarification: '等待澄清',
  partial: '部分覆盖',
  passed: '已通过',
  paused: '已暂停',
  open: '待接管',
  acknowledged: '已确认接管',
  sending: '发送中',
  provider_accepted: 'QQ 平台已接收',
  retry_wait: '等待重试',
  outcome_unknown: '投递结果未知',
  exhausted: '重试已耗尽',
  proposed: '已提议',
  ready: '已就绪',
  reconnecting: '重新连接中',
  rejected: '已拒绝',
  reported: '已报告',
  restart_required: '需要重启',
  revoked: '已撤销',
  identity_stale: '主机或执行器指纹变化，需重新资格',
  stale: '已过期',
  source_unverified: '项目来源未验证',
  stopped: '已停止',
  succeeded: '已完成',
  superseded: '已被替代',
  unverified: '未验证',
  verified: '已验证',
  unavailable: '不可用',
  unknown: '未知',
  unsupported: '不支持',
  runtime_unqualified: '运行时资格未完成',
  runtime_qualified_dispatch_unavailable: 'MCP 运行资格已过；员工调用接线未开放',
  runtime_qualified_dispatch_disabled: '运行资格已过；Streamable HTTP 外部访问默认关闭',
  runtime_qualified_dispatch_available: '运行资格已过；已启用受控 Streamable HTTP 调用',
  observed_unqualified: '已记录本地观察，尚未资格化',
  qualified: '运行资格已审批',
  schema_drift: '工具 schema 已变化，运行资格失效',
  supported: '已支持',
  deterministic: '确定性',
  fake: '模拟',
  deterministic_fake: '确定性 / 模拟',
  working: '工作中',
};

const ACTIVITY_LABELS: Readonly<Record<string, string>> = {
  acceptance_passed: '验收通过',
  artifact_submitted: '产物已提交',
  company_created: '公司已创建',
  'company.created': '公司已创建',
  contract_revision_accepted: '合同修订已接受',
  contract_revision_proposed: '合同修订已提议',
  employee_started_task: '员工开始任务',
  employee_stopped: '员工已停止',
  mission_cancelled: '使命已取消',
  mission_created: '使命已创建',
  mission_started: '使命已启动',
  obligation_created: '责任已创建',
  obligation_observed: '责任已观察',
  old_writer_rejected: '旧写入者被拒绝',
  peer_message_sent: '同事消息已发送',
  provider_runtime_initialization_failed: '运行时初始化失败',
  provider_runtime_thread_start_failed: '运行线程启动失败',
  provider_turn_completed: '模型回合已完成',
  provider_turn_failed: '模型回合失败',
  'runtime.settings.update': '运行时配置已更新',
  successor_resumed: '接班流程已恢复',
  workspace_updated: '工作区已更新',
};

const TEXT_REPLACEMENTS: ReadonlyArray<Readonly<[string, string]>> = [
  ['controller.recovered', '控制器已恢复'],
  ['authoritative company_seq=', '公司事件序号='],
  ['; runtime event kind=', '；运行时事件类型='],
  ['Polis runtime', 'Polis 运行时'],
  ['workspace_updated', '工作区已更新'],
  ['company.created', '公司已创建'],
  ['provider_runtime_initialization_failed', '运行时初始化失败'],
  ['provider_turn_completed', '模型回合已完成'],
  ['provider_turn_failed', '模型回合失败'],
  ['runtime.settings.update', '运行时配置已更新'],
];

export function labelProjectEnvironmentPolicy(policy: ProjectEnvironmentPolicyManifestView): string {
  if (policy.profileId === 'linux-node-npm@1') return `Linux/Node · deny_all · registry metadata ${policy.registryHosts.join(', ')}`;
  return `Windows/Node · Registry ${policy.registryHosts.join(', ')}`;
}

export function labelDisplayValue(value: string | null | undefined): string {
  if (value === null || value === undefined || value === '') return '不可得';
  return DISPLAY_LABELS[value] ?? value;
}

export function labelDataMode(value: string): string {
  return ({real: '真实数据', simulated: '模拟数据', unavailable: '不可用'} as Readonly<Record<string, string>>)[value] ?? labelDisplayValue(value);
}

export function labelFreshness(value: string): string {
  return ({fresh: '最新', stale: '已过期', reconnecting: '重新连接中', unknown: '未知'} as Readonly<Record<string, string>>)[value] ?? labelDisplayValue(value);
}

export function labelRecoveryState(value: string): string {
  return ({operational: '运行正常', recovery_required: '需要恢复核对', unknown: '恢复状态未知'} as Readonly<Record<string, string>>)[value] ?? labelDisplayValue(value);
}

export function labelActivityKind(value: string): string {
  return ACTIVITY_LABELS[value] ?? labelDisplayValue(value);
}

export function labelActivityText(value: string): string {
  const exactLabel = ACTIVITY_LABELS[value];
  if (exactLabel !== undefined) return exactLabel;
  return TEXT_REPLACEMENTS.reduce((text, [source, replacement]) => text.replaceAll(source, replacement), value);
}

export function labelQuality(value: string): string {
  return ({reported: '已报告', estimated: '估算', unavailable: '不可得'} as Readonly<Record<string, string>>)[value] ?? labelDisplayValue(value);
}

export function labelResourceNote(value: string): string {
  return ({
    'money and token usage are not exposed by this read model': '当前只读数据视图未提供费用金额和 Token 用量。',
  } as Readonly<Record<string, string>>)[value] ?? value;
}

export function labelTransport(value: string): string {
  return ({stdio: '标准输入输出', streamable_http: '流式 HTTP'} as Readonly<Record<string, string>>)[value] ?? value;
}

export function labelNotificationAdapter(value: string): string {
  return ({local: '本地记录', webhook: 'Webhook', qq_official: 'QQ 官方 Bot'} as Readonly<Record<string, string>>)[value] ?? value;
}

export function labelRole(value: string): string {
  return ({planning: '规划', backend: '后端', frontend: '前端', review: '审查', reviewer: '审查'} as Readonly<Record<string, string>>)[value] ?? value;
}

export function labelErrorMessage(value: string): string {
  const replacements: ReadonlyArray<Readonly<[string, string]>> = [
    ['failed to load company directory', '读取公司目录失败'],
    ['failed to load companies', '读取公司目录失败'],
    ['company summary contains an unknown or malformed field', '公司摘要包含未知或格式不正确的字段'],
    ['database migration failed with exit status 1', '数据库迁移失败（退出状态 1）'],
    ['Polis backend readiness timeout on ', 'Polis 后端就绪检查超时：'],
    ['incompatible schema', '数据库版本不兼容'],
    ['unknown or malformed field', '未知或格式不正确的字段'],
    ['failed to ', '执行失败：'],
  ];
  return replacements.reduce((message, [source, replacement]) => message.replace(source, replacement), value);
}
