// pattern: Imperative Shell

import {BadgeCheck, Check, FileCheck2, FolderOpen, Handshake, History, LockKeyhole, MessageSquare, Settings2, ShieldAlert, Wrench} from 'lucide-react';
import type {ReactNode} from 'react';
import type {ActivityEvent, CompanyOverviewView, EmployeeSummary, TaskSummary} from '../domain/workbench';
import type {WorkbenchApi} from '../data/workbench-api';
import {useArtifactDetail, useTaskWorkspace} from '../data/workbench-query';
import {formatEntityId} from '../domain/activity-presentation';
import {labelDisplayValue, labelRole} from '../domain/display-labels';
import {selectCheckpointForTask} from '../domain/checkpoint-projection';
import {ActivityTimeline} from '../components/activity-timeline/ActivityTimeline';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

function DataRow({label, value, mono = false}: Readonly<{label: string; value: ReactNode; mono?: boolean}>) {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={mono ? styles.detailValueMono : styles.detailValue}>{value}</span></div>;
}

function EmptyPanel({title, detail}: Readonly<{title: string; detail: string}>) {
  return <div className={styles.emptyState}><span className={styles.emptyStateIcon}><LockKeyhole aria-hidden="true" size={18} /></span><strong>{title}</strong><span>{detail}</span></div>;
}

function qualificationTone(status: EmployeeSummary['qualification']['status']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'supported') return 'success';
  if (status === 'limited') return 'warning';
  if (status === 'unsupported') return 'danger';
  return 'info';
}

function taskTone(task: TaskSummary): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (task.state === 'working') return 'info';
  if (task.state === 'candidate') return 'warning';
  if (task.state === 'completed') return 'success';
  if (task.state === 'blocked') return 'danger';
  return 'neutral';
}

function employeeEvents(overview: CompanyOverviewView, employeeId: string): ReadonlyArray<ActivityEvent> {
  return overview.recentActivity.filter(event => event.actor.id === employeeId || event.subject.id === employeeId);
}

export type MissionSubpageTab = 'inputs' | 'guidance' | 'deliveries';

export function MissionSubpage({overview, tab}: Readonly<{overview: CompanyOverviewView; tab: MissionSubpageTab}>) {
  const mission = overview.mission;
  const revision = mission.currentContractRevision;
  if (tab === 'inputs') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>使命输入</span><h2 className={styles.sectionTitle}>资料边界</h2></div><FolderOpen aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="已登记资料" value="当前只读快照未提供" /><DataRow label="绑定版本" value={revision ? '合同版本 ' + revision.revision : '不可得'} mono /><DataRow label="读取事实" value="登记不等于模型已读取" /><DataRow label="当前来源" value={mission.contract} mono /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>读取边界</span><h2 className={styles.sectionTitle}>输入状态</h2></div><StatusBadge label="待接入" tone="neutral" /></div><div className={styles.checkList}><div><Check aria-hidden="true" size={15} /><span>资料不会被自动外发</span></div><div><Check aria-hidden="true" size={15} /><span>资料版本不会覆盖合同版本</span></div><div><ShieldAlert aria-hidden="true" size={15} /><span>当前页面不执行解析或绑定</span></div></div></article>
    </section>;
  }
  if (tab === 'guidance') {
    const obligations = overview.obligations;
    return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>指导 / 责任</span><h2 className={styles.sectionTitle}>指导与责任变化</h2></div><MessageSquare aria-hidden="true" className={styles.icon} size={18} /></div>{obligations.length > 0 ? <div className={styles.recordList}>{obligations.map(item => <div className={styles.recordRow} key={item.obligationId}><div className={styles.recordLead}><MessageSquare aria-hidden="true" size={16} /><div><strong>{item.note || '责任 ' + formatEntityId(item.obligationId)}</strong><span>{formatEntityId(item.messageId)} · 负责人 {item.ownerEmployeeId}</span></div></div><div className={styles.recordMeta}><code>{item.evidenceRef ? formatEntityId(item.evidenceRef) : '暂无证据'}</code><StatusBadge label={labelDisplayValue(item.state)} tone={item.state === 'fulfilled' ? 'success' : item.state === 'declined' ? 'danger' : 'warning'} /></div></div>)}</div> : <EmptyPanel detail="当前使命快照没有责任记录。" title="暂无指导责任" />}</section>;
  }
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>交付边界</span><h2 className={styles.sectionTitle}>使命交付</h2></div><BadgeCheck aria-hidden="true" className={styles.icon} size={18} /></div>{overview.artifacts.length > 0 ? <div className={styles.recordList}>{overview.artifacts.map(artifact => <div className={styles.recordRow} key={artifact.artifactId}><div className={styles.recordLead}><FileCheck2 aria-hidden="true" size={16} /><div><strong>{formatEntityId(artifact.artifactId)}</strong><span>{artifact.authorEmployeeId} · {artifact.bytes} · {labelDisplayValue(artifact.state)}</span></div></div><div className={styles.recordMeta}><code>{artifact.contractRevisionId ? formatEntityId(artifact.contractRevisionId) : '暂无合同版本'}</code><StatusBadge label={labelDisplayValue(artifact.verdict)} tone={artifact.verdict === 'passed' ? 'success' : artifact.verdict === 'failed' ? 'danger' : 'warning'} /></div></div>)}</div> : <EmptyPanel detail="当前使命快照没有产物；不把任务完成状态推断为交付。" title="暂无交付产物" />}</section>;
}

export type TaskSubpageTab = 'jobs' | 'browser';

export function TaskSubpage({api, companyId, overview, task, tab}: Readonly<{api: WorkbenchApi; companyId: string; overview: CompanyOverviewView; task: TaskSummary | null; tab: TaskSubpageTab}>) {
  const workspaceQuery = useTaskWorkspace(api, companyId, task?.taskId ?? null);
  const artifactForTask = task === null ? null : overview.artifacts.find(item => item.taskId === task.taskId) ?? null;
  const artifactQuery = useArtifactDetail(api, companyId, artifactForTask?.artifactId ?? null);
  if (task === null) return <section className={styles.sectionCard}><EmptyPanel detail="当前快照没有可选任务对象。" title="暂无任务详情" /></section>;
  const taskEvents = overview.recentActivity.filter(event => event.subject.id === task.taskId);
  const artifact = artifactForTask;
  const checkpoint = selectCheckpointForTask(overview.checkpoints, task.taskId, artifact);
  if (tab === 'jobs') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>作业 / 执行</span><h2 className={styles.sectionTitle}>运行作业</h2></div><Wrench aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="执行状态" value={labelDisplayValue(task.state)} /><DataRow label="生成代数" value={task.generation} mono /><DataRow label="工作区版本" value={task.workspaceRevision ?? '不可得'} mono /><DataRow label="检查点" value={checkpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="检查点状态" value={checkpoint ? `${checkpoint.kind} / ${labelDisplayValue(checkpoint.qualificationState)}` : '暂无检查点'} /><DataRow label="运行作业记录" value="当前只读接口未提供" /><DataRow label="写入动作" value="关闭" /></div>{workspaceQuery.isPending ? <div className={styles.emptyState}>正在读取授权工作区</div> : workspaceQuery.isError ? <div className={styles.errorState} role="alert">工作区读取失败：{workspaceQuery.error.message}</div> : workspaceQuery.data ? <div className={styles.detailRows}><DataRow label="CAS 指纹" value={workspaceQuery.data.digest} mono /><DataRow label="变更文件" value={workspaceQuery.data.changedFiles.join(', ')} /><DataRow label="预览" value={workspaceQuery.data.files[0]?.content || '内容不可得'} /></div> : null}</article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>任务活动</span><h2 className={styles.sectionTitle}>任务相关事件</h2></div><History aria-hidden="true" className={styles.icon} size={18} /></div>{taskEvents.length > 0 ? <ActivityTimeline compact events={taskEvents} /> : <EmptyPanel detail="当前活动快照没有直接绑定到该任务的事件。" title="暂无任务事件" />}</article>
    </section>;
  }
  const artifacts = overview.artifacts.filter(artifact => artifact.taskId === task.taskId);
  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>浏览器 / 产物</span><h2 className={styles.sectionTitle}>测试与预览边界</h2></div><FolderOpen aria-hidden="true" className={styles.icon} size={18} /></div>{artifacts.length > 0 ? <div className={styles.recordList}>{artifacts.map(artifact => <div className={styles.recordRow} key={artifact.artifactId}><div className={styles.recordLead}><BadgeCheck aria-hidden="true" size={16} /><div><strong>{formatEntityId(artifact.artifactId)}</strong><span>{artifact.digest} · {artifact.bytes}</span></div></div><StatusBadge label={labelDisplayValue(artifact.verdict)} tone={artifact.verdict === 'passed' ? 'success' : artifact.verdict === 'failed' ? 'danger' : 'warning'} /></div>)}</div> : <EmptyPanel detail="当前只读数据没有该任务的预览地址或产物。" title="暂无可预览产物" />}{artifactQuery.isPending ? <div className={styles.emptyState}>正在读取产物内容</div> : artifactQuery.isError ? <div className={styles.errorState} role="alert">产物读取失败：{artifactQuery.error.message}</div> : artifactQuery.data ? <div className={styles.detailRows}><DataRow label="内容是否可用" value={artifactQuery.data.contentAvailable ? '是' : '否'} /><DataRow label="内容预览" value={artifactQuery.data.content || '内容不可得'} /></div> : null}</article>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>验收边界</span><h2 className={styles.sectionTitle}>验收不是预览</h2></div><ShieldAlert aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="验收状态" value={labelDisplayValue(task.acceptance)} /><DataRow label="合同版本" value={task.contractRevisionId ?? '不可得'} mono /><DataRow label="检查点" value={checkpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="预览状态" value="未提供" /><DataRow label="最终验收" value={task.acceptance === 'passed' ? '已通过' : '未通过'}/></div></article>
  </section>;
}

export type EmployeeSubpageTab = 'responsibilities' | 'collaboration' | 'memory' | 'handover' | 'tools' | 'workspace';

export function EmployeeSubpage({overview, employee, tab}: Readonly<{overview: CompanyOverviewView; employee: EmployeeSummary; tab: EmployeeSubpageTab}>) {
  const currentTask = employee.currentTask === null ? null : overview.tasks.find(task => task.taskId === employee.currentTask?.id) ?? null;
  const currentArtifact = currentTask === null ? null : overview.artifacts.find(artifact => artifact.taskId === currentTask.taskId) ?? null;
  const currentCheckpoint = currentTask === null ? null : selectCheckpointForTask(overview.checkpoints, currentTask.taskId, currentArtifact);
  if (tab === 'responsibilities') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>职责</span><h2 className={styles.sectionTitle}>责任边界</h2></div><Handshake aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="员工身份" value={employee.employeeId} mono /><DataRow label="岗位" value={labelRole(employee.role)} /><DataRow label="岗位版本" value={employee.roleRevision ?? '不可得'} mono /><DataRow label="当前任务" value={employee.currentTask?.label ?? '当前无任务'} /><DataRow label="待处理责任" value={employee.openObligationCount} /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>资格</span><h2 className={styles.sectionTitle}>资格与证据</h2></div><BadgeCheck aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="状态" value={<StatusBadge label={labelDisplayValue(employee.qualification.status)} tone={qualificationTone(employee.qualification.status)} />} /><DataRow label="证据" value={employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '不可得'} mono /><DataRow label="政策版本" value={employee.qualification.policyRevision ?? '不可得'} mono /><DataRow label="状态原因" value={employee.status.reason} /></div></article>
    </section>;
  }
  if (tab === 'collaboration') {
    const events = employeeEvents(overview, employee.employeeId);
    return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>协作</span><h2 className={styles.sectionTitle}>消息与协作事件</h2></div><MessageSquare aria-hidden="true" className={styles.icon} size={18} /></div>{events.length > 0 ? <ActivityTimeline events={events} /> : <EmptyPanel detail="当前快照没有与该员工绑定的消息或活动事件。" title="暂无协作事件" />}</section>;
  }
  if (tab === 'memory') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>记忆 / 连续性</span><h2 className={styles.sectionTitle}>连续性边界</h2></div><History aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="员工会话" value={employee.sessionId ? formatEntityId(employee.sessionId) : '未启动'} mono /><DataRow label="会话周期" value={employee.epoch} mono /><DataRow label="当前任务" value={employee.currentTask?.label ?? '无'} /><DataRow label="记忆读取接口" value="未提供" /><DataRow label="读取行为" value="不从界面推断记忆内容" /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>检查点</span><h2 className={styles.sectionTitle}>可恢复记录</h2></div><FileCheck2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="快照游标" value={formatEntityId(overview.meta.snapshotCursor)} mono /><DataRow label="实体版本" value={overview.meta.entityRevision} mono /><DataRow label="检查点" value={currentCheckpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="检查点状态" value={currentCheckpoint ? `${currentCheckpoint.kind} / ${labelDisplayValue(currentCheckpoint.qualificationState)}` : '暂无检查点'} /><DataRow label="最终验收" value="独立字段，不由检查点替代" /></div></article>
    </section>;
  }
  if (tab === 'handover') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>交接</span><h2 className={styles.sectionTitle}>交接连续性</h2></div><Handshake aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="逻辑身份" value={employee.employeeId} mono /><DataRow label="会话状态" value={labelDisplayValue(employee.sessionState)} /><DataRow label="会话周期" value={employee.epoch} mono /><DataRow label="交接就绪" value={employee.sessionState === 'stopped' ? '可继续查看' : '未声明'} /><DataRow label="接班人" value="当前只读数据未提供" /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>写入隔离</span><h2 className={styles.sectionTitle}>旧写入者边界</h2></div><ShieldAlert aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.checkList}><div><Check aria-hidden="true" size={15} /><span>会话周期代表一次会话实例，不是新员工</span></div><div><Check aria-hidden="true" size={15} /><span>停止与接班恢复分开观察</span></div><div><ShieldAlert aria-hidden="true" size={15} /><span>旧写入者拒绝不简化为网络失败</span></div></div></article>
    </section>;
  }
  if (tab === 'tools') {
    return <section className={styles.resourceGrid}>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>模型配置</span><strong>{employee.profile ?? '不可得'}</strong><p>员工会话配置</p><StatusBadge label={employee.status.activeModelRequests + ' 个进行中'} tone={employee.status.activeModelRequests === '0' ? 'neutral' : 'warning'} /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具调用</span><strong>{employee.status.inFlightTools}</strong><p>执行中的工具</p><StatusBadge label={labelDisplayValue(employee.toolBudget.quality)} tone={employee.toolBudget.quality === 'reported' ? 'success' : 'warning'} /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具预算</span><strong>{employee.toolBudget.used === null ? '不可得' : employee.toolBudget.used + ' / ' + employee.toolBudget.limit}</strong><p>剩余 {employee.toolBudget.remaining ?? '不可得'}</p><StatusBadge label="只读" tone="info" /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>资格</span><strong>{labelDisplayValue(employee.qualification.status)}</strong><p>{employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '暂无证据引用'}</p><StatusBadge label="证据边界" tone={qualificationTone(employee.qualification.status)} /></article>
    </section>;
  }
  const ownedTasks = overview.tasks.filter(task => task.ownerEmployeeId === employee.employeeId);
  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>工作区</span><h2 className={styles.sectionTitle}>责任工作区</h2></div><FolderOpen aria-hidden="true" className={styles.icon} size={18} /></div>{ownedTasks.length > 0 ? <div className={styles.recordList}>{ownedTasks.map(task => <div className={styles.recordRow} key={task.taskId}><div className={styles.recordLead}><FolderOpen aria-hidden="true" size={16} /><div><strong>{task.title}</strong><span>{task.taskId} · 工作区版本 {task.workspaceRevision ?? '不可得'}</span></div></div><StatusBadge label={labelDisplayValue(task.acceptance)} tone={taskTone(task)} /></div>)}</div> : <EmptyPanel detail="当前员工没有负责人绑定的任务。" title="暂无责任工作区" />}</article>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>工作区边界</span><h2 className={styles.sectionTitle}>运行目录</h2></div><LockKeyhole aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="工作区读取接口" value="当前未提供" /><DataRow label="工作区修改" value="关闭" /><DataRow label="任务检出" value="由后端状态决定" /><DataRow label="产物发布" value="独立于工作区更新" /></div></article>
  </section>;
}

export function CapabilitiesSubpage({overview}: Readonly<{overview: CompanyOverviewView}>) {
  const qualified = overview.employees.filter(employee => employee.qualification.status === 'supported').length;
  return <div className={styles.viewStack}><section className={styles.resourceGrid}><article className={styles.resourceCard}><span className={styles.cardEyebrow}>员工配置</span><strong>{overview.employees.length}</strong><p>当前快照中的员工会话配置</p><StatusBadge label="当前快照" tone="info" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>资格</span><strong>{qualified + ' / ' + overview.employees.length}</strong><p>已支持 / 全部员工</p><StatusBadge label={qualified === overview.employees.length ? '全部支持' : '混合状态'} tone={qualified === overview.employees.length ? 'success' : 'warning'} /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具预算</span><strong>{overview.resources.toolCallsUsed + ' / ' + overview.resources.toolCallsLimit}</strong><p>公司资源摘要</p><StatusBadge label={labelDisplayValue(overview.resources.toolBudgetQuality)} tone={overview.resources.toolBudgetQuality === 'reported' ? 'success' : 'warning'} /></article></section><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>能力边界</span><h2 className={styles.sectionTitle}>能力目录</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="模型 / 运行时目录" value="当前只读接口未提供" /><DataRow label="工具目录" value="当前只读接口未提供" /><DataRow label="资格证据" value="按员工资格独立显示" /><DataRow label="外部动作" value="关闭" /></div></section></div>;
}
