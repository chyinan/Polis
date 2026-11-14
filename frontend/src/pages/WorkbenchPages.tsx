// pattern: Imperative Shell

import {useEffect, useState, type ReactElement, type ReactNode} from 'react';
import {BadgeCheck, Check, CircleAlert, FileCheck2, Filter, Layers3, LockKeyhole, MessageSquare, Radio, Settings2, ShieldAlert, Target, UsersRound} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCapabilityCatalog, useCodexModelCatalog, useCollaboration, useCompanyList, useCompanyOverview, useArchiveCompany, useConfigureNotificationRoute, useNotifications, useOperations, useOperatorInstructions, useRuntimeSettings, useSendOperatorInstruction, useTestNotification, useUpdateCompany, useUpdateRuntimeSettings} from '../data/workbench-query';
import type {CompanyOverviewView, CompanySummaryView, EmployeeSummary, RuntimeSettingsView, StatusTone, TaskSummary} from '../domain/workbench';
import {useNavigate} from 'react-router-dom';
import {formatEntityId, formatEventTime} from '../domain/activity-presentation';
import {labelDataMode, labelDisplayValue, labelErrorMessage, labelFreshness, labelNotificationAdapter, labelQuality, labelResourceNote, labelRole} from '../domain/display-labels';
import {selectCheckpointForTask} from '../domain/checkpoint-projection';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {EmployeeSubpage, MissionSubpage, TaskSubpage} from './SubpagePanels';
import {CapabilityCatalogPanel} from './CapabilityCatalogPanel';
import styles from '../styles/workbench.module.css';

type PageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

const CUSTOM_CODEX_MODEL = '__custom_codex_model__';
const CUSTOM_DEEPSEEK_MODEL = '__custom_deepseek_model__';
const DEEPSEEK_MODELS = ['deepseek-flash', 'deepseek-v4-pro'] as const;
const DEEPSEEK_EFFORTS = [
  {id: 'none', label: '关闭思考'},
  {id: 'low', label: '低'},
  {id: 'high', label: '高 · 官方默认'},
  {id: 'max', label: '最大'},
] as const;

function ViewHeader({eyebrow, title, action}: Readonly<{eyebrow: string; title: string; action?: ReactNode}>) {
  return <div className={styles.viewHeader}><div><p className={styles.eyebrow}>{eyebrow}</p><h1 className={styles.pageTitle}>{title}</h1></div>{action ? <div className={styles.viewHeaderAction}>{action}</div> : null}</div>;
}

function TabBar<T extends string>({tabs, active, onChange}: Readonly<{tabs: ReadonlyArray<Readonly<{id: T; label: string}>>; active: T; onChange: (tab: T) => void}>) {
  return <div className={styles.tabBar} role="tablist">{tabs.map(tab => <button aria-selected={active === tab.id} className={styles.tabButton + ' ' + (active === tab.id ? styles.tabButtonActive : '')} key={tab.id} onClick={() => onChange(tab.id)} role="tab" type="button">{tab.label}</button>)}</div>;
}

function DataRow({label, value, mono = false}: Readonly<{label: string; value: ReactNode; mono?: boolean}>) {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={mono ? styles.detailValueMono : styles.detailValue}>{value}</span></div>;
}

function LoadingPanel({label}: Readonly<{label: string}>) {
  return <div className={styles.emptyState} role="status"><Radio aria-hidden="true" size={18} /><span>{label}</span></div>;
}

function ErrorPanel({message}: Readonly<{message: string}>) {
  return <div className={styles.errorState} role="alert"><CircleAlert aria-hidden="true" size={18} /><div><strong>只读数据读取失败</strong><p>{labelErrorMessage(message)}</p></div></div>;
}

function UnavailablePanel({title, detail}: Readonly<{title: string; detail: string}>) {
  return <section className={styles.sectionCard}><div className={styles.emptyState}><span className={styles.emptyStateIcon}><LockKeyhole aria-hidden="true" size={18} /></span><strong>{title}</strong><span>{detail}</span></div></section>;
}

function readStatusTone(tone: StatusTone): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  return tone;
}

function employeeStateTone(status: EmployeeSummary['status']['tone']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  return status;
}

function renderOverviewState(query: ReturnType<typeof useCompanyOverview>, label: string): ReactElement | null {
  if (query.isPending) return <LoadingPanel label={label} />;
  if (query.isError) return <ErrorPanel message={query.error.message} />;
  return null;
}

function getOverview(query: ReturnType<typeof useCompanyOverview>): CompanyOverviewView {
  if (query.data === undefined) throw new Error('overview data unavailable after query completed');
  return query.data;
}

function MissionOverview({overview}: Readonly<{overview: CompanyOverviewView}>) {
  const mission = overview.mission;
  const revision = mission.currentContractRevision;
  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>目标合同</span><h2 className={styles.sectionTitle}>{mission.missionId}</h2></div><StatusBadge label={revision ? labelDisplayValue(revision.state) : '不可得'} tone={revision?.state === 'accepted' ? 'success' : 'info'} /></div>
      <p className={styles.panelDescription}>{mission.goal}</p>
      <div className={styles.detailRows}><DataRow label="合同版本" value={revision ? '版本 ' + revision.revision : '不可得'} mono /><DataRow label="来源" value={mission.contract} mono /><DataRow label="下一里程碑" value={mission.nextMilestone} /><DataRow label="里程碑" value={mission.verifiedMilestones + ' / ' + mission.milestoneTotal + ' 已验证'} /></div>
    </article>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>责任边界</span><h2 className={styles.sectionTitle}>当前阶段</h2></div><Target aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.checkList}><div><Check aria-hidden="true" size={15} /><span>员工逻辑身份与员工会话分开</span></div><div><Check aria-hidden="true" size={15} /><span>消息与责任分开</span></div><div><Check aria-hidden="true" size={15} /><span>检查点、产物与验收分开</span></div><div><ShieldAlert aria-hidden="true" size={15} /><span>当前页面只读，不改变合同或验收</span></div></div>
    </article>
  </section>;
}

export function MissionPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const instructionsQuery = useOperatorInstructions(api, companyId);
  const sendInstruction = useSendOperatorInstruction(api, companyId);
  const [tab, setTab] = useState<'overview' | 'inputs' | 'guidance' | 'deliveries'>('overview');
  const tabs = [{id: 'overview' as const, label: '概览'}, {id: 'inputs' as const, label: '资料'}, {id: 'guidance' as const, label: '指导'}, {id: 'deliveries' as const, label: '交付'}];
  const state = renderOverviewState(query, '正在读取使命快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 使命" title="使命" />{state}</div>;
  const overview = getOverview(query);
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={labelDisplayValue(overview.mission.state)} tone={overview.mission.state === 'active' ? 'success' : 'info'} />} eyebrow="公司 / 使命" title="使命" /><TabBar active={tab} onChange={setTab} tabs={tabs} />{tab === 'overview' ? <MissionOverview overview={overview} /> : <MissionSubpage overview={overview} tab={tab === 'inputs' ? 'inputs' : tab === 'guidance' ? 'guidance' : 'deliveries'} />}<OperatorInstructionPanel companyId={companyId} instructionsQuery={instructionsQuery} missionId={overview.mission.missionId} overview={overview} sendInstruction={sendInstruction} /></div>;
}

function OperatorInstructionPanel({companyId, instructionsQuery, missionId, overview, sendInstruction}: Readonly<{companyId: string; instructionsQuery: ReturnType<typeof useOperatorInstructions>; missionId: string; overview: CompanyOverviewView; sendInstruction: ReturnType<typeof useSendOperatorInstruction>}>): ReactElement {
  const [content, setContent] = useState('');
  const [employeeId, setEmployeeId] = useState('');
  const [taskId, setTaskId] = useState('');
  async function submit(): Promise<void> {
    const trimmed = content.trim();
    if (trimmed === '') return;
    try {
      await sendInstruction.mutateAsync({missionId, employeeId: employeeId || null, taskId: taskId || null, content: trimmed, requestId: `operator-instruction-${companyId}-${Date.now()}`});
      setContent('');
    } catch {
      // mutation state surfaces the error
    }
  }
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>人工介入</span><h2 className={styles.sectionTitle}>给运行中的员工补充信息</h2></div><StatusBadge label="权威来源" tone="info" /></div><div className={styles.formStack}><label className={styles.formLabel}>指令内容<textarea className={styles.formField} rows={3} value={content} onChange={event => setContent(event.target.value)} placeholder="例如：继续当前方向，并优先处理最新验证反馈。" /></label><div className={styles.wizardGrid}><label className={styles.formLabel}>目标员工<select className={styles.formField} value={employeeId} onChange={event => setEmployeeId(event.target.value)}><option value="">公司级指令</option>{overview.employees.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}</select></label><label className={styles.formLabel}>目标任务<select className={styles.formField} value={taskId} onChange={event => setTaskId(event.target.value)}><option value="">不绑定具体任务</option>{overview.tasks.map(task => <option key={task.taskId} value={task.taskId}>{task.title} · {task.taskId}</option>)}</select></label></div>{sendInstruction.isError ? <p className={styles.formError} role="alert">指令未受理：{sendInstruction.error.message}</p> : null}<button className={styles.commandButton} disabled={sendInstruction.isPending || overview.mission.state !== 'active' || content.trim() === ''} onClick={() => { void submit(); }} type="button">{sendInstruction.isPending ? '正在受理…' : '受理指令'}</button><p className={styles.formHint}>受理会写入控制面并进入活动时间线；待处理不等于员工已经应用。</p></div><div className={styles.recordList}>{instructionsQuery.isPending ? <div className={styles.emptyState} role="status">正在读取历史指令</div> : instructionsQuery.isError ? <div className={styles.errorState} role="alert">指令历史读取失败：{instructionsQuery.error.message}</div> : instructionsQuery.data.length === 0 ? <div className={styles.emptyState}>当前没有人工指令。</div> : instructionsQuery.data.map(instruction => <div className={styles.recordRow} key={instruction.instructionId}><div className={styles.recordLead}><MessageSquare aria-hidden="true" size={16} /><div><strong>{instruction.content}</strong><span>{instruction.instructionId} · {instruction.employeeId ?? '公司级'} · {instruction.taskId ?? '使命作用域'}</span></div></div><StatusBadge label={labelDisplayValue(instruction.state)} tone={instruction.state === 'applied' ? 'success' : instruction.state === 'rejected' ? 'danger' : 'warning'} /></div>)}</div></section>;
}

function taskTone(task: TaskSummary): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (task.state === 'working') return 'info';
  if (task.state === 'candidate') return 'warning';
  if (task.state === 'completed') return 'success';
  if (task.state === 'blocked') return 'danger';
  return 'neutral';
}

function TaskDetail({overview, task}: Readonly<{overview: CompanyOverviewView; task: TaskSummary}>) {
  const artifact = overview.artifacts.find(item => item.taskId === task.taskId) ?? null;
  const checkpoint = selectCheckpointForTask(overview.checkpoints, task.taskId, artifact);
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>任务 / 工作</span><h2 className={styles.sectionTitle}>{task.title}</h2></div><StatusBadge label={labelDisplayValue(task.state)} tone={taskTone(task)} /></div><div className={styles.detailRows}><DataRow label="任务 ID" value={task.taskId} mono /><DataRow label="负责人" value={task.ownerEmployeeId} mono /><DataRow label="生成代数" value={task.generation} mono /><DataRow label="合同版本" value={task.contractRevisionId ?? '不可得'} mono /><DataRow label="工作区版本" value={task.workspaceRevision ?? '不可得'} mono /><DataRow label="检查点" value={checkpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="检查点状态" value={checkpoint ? `${checkpoint.kind} / ${labelDisplayValue(checkpoint.qualificationState)}` : '暂无检查点'} /><DataRow label="验收状态" value={labelDisplayValue(task.acceptance)} /><DataRow label="依赖" value={task.dependencyLabel ?? '无已声明依赖'} /></div></section>;
}

export function TaskPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const [tab, setTab] = useState<'overview' | 'jobs' | 'browser'>('overview');
  const [filter, setFilter] = useState<'all' | 'active' | 'waiting' | 'stopped'>('all');
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const tabs = [{id: 'overview' as const, label: '任务概览'}, {id: 'jobs' as const, label: '作业'}, {id: 'browser' as const, label: '测试与预览'}];
  const state = renderOverviewState(query, '正在读取任务快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 任务" title="任务" />{state}</div>;
  const overview = getOverview(query);
  const tasks = overview.tasks;
  const visibleTasks = tasks.filter(task => filter === 'all' || (filter === 'active' && task.state === 'working') || (filter === 'waiting' && task.state === 'candidate') || (filter === 'stopped' && (task.state === 'completed' || task.state === 'blocked')));
  const selectedTask = tasks.find(task => task.taskId === selectedTaskId) ?? visibleTasks[0] ?? tasks[0] ?? null;
  return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 任务" title="任务" /><TabBar active={tab} onChange={setTab} tabs={tabs} />{tab !== 'overview' ? <TaskSubpage api={api} companyId={companyId} overview={overview} task={selectedTask} tab={tab === 'jobs' ? 'jobs' : 'browser'} /> : <div className={styles.taskWorkspace}><section className={styles.sectionCard}><div className={styles.toolbarLabel}><Filter aria-hidden="true" className={styles.icon} size={15} />任务筛选</div><div className={styles.filterGroup}>{(['all', 'active', 'waiting', 'stopped'] as const).map(option => <button aria-pressed={filter === option} className={styles.filterButton + ' ' + (filter === option ? styles.filterButtonActive : '')} key={option} onClick={() => setFilter(option)} type="button">{{all: '全部', active: '进行中', waiting: '等待', stopped: '已停止'}[option]}</button>)}</div><div className={styles.recordList}>{visibleTasks.map(task => <button className={styles.taskListRow + ' ' + (selectedTask?.taskId === task.taskId ? styles.taskListRowActive : '')} data-task-state={task.state} key={task.taskId} onClick={() => setSelectedTaskId(task.taskId)} type="button"><span className={styles.taskListIndex}><FileCheck2 aria-hidden="true" size={15} /></span><span><strong>{task.title}</strong><small>{task.taskId} · {task.ownerEmployeeId}</small></span><StatusBadge label={labelDisplayValue(task.state)} tone={taskTone(task)} /></button>)}</div></section>{selectedTask ? <TaskDetail overview={overview} task={selectedTask} /> : <UnavailablePanel detail="当前快照没有任务对象。" title="没有可展示的任务" />}</div>}</div>;
}

function EmployeeDetail({employee}: Readonly<{employee: EmployeeSummary}>) {
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>{labelRole(employee.role)}</span><h2 className={styles.sectionTitle}>{employee.displayName}</h2></div><StatusBadge label={labelDisplayValue(employee.status.primary)} tone={employeeStateTone(employee.status.tone)} /></div><div className={styles.detailGrid}><div className={styles.detailRows}><DataRow label="员工逻辑身份" value={employee.employeeId} mono /><DataRow label="岗位版本" value={employee.roleRevision ?? '不可得'} mono /><DataRow label="员工会话" value={employee.sessionId ? formatEntityId(employee.sessionId) : '未启动'} mono /><DataRow label="会话状态" value={labelDisplayValue(employee.sessionState)} /><DataRow label="会话周期" value={employee.epoch} mono /></div><div className={styles.detailRows}><DataRow label="模型配置" value={employee.profile ?? '不可得'} /><DataRow label="进行中的请求" value={employee.status.activeModelRequests} /><DataRow label="执行中的工具" value={employee.status.inFlightTools} /><DataRow label="工具预算" value={employee.toolBudget.used === null ? '不可得' : employee.toolBudget.used + ' / ' + employee.toolBudget.limit} /><DataRow label="资格状态" value={labelDisplayValue(employee.qualification.status)} /><DataRow label="证据" value={employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '不可得'} mono /></div></div><div className={styles.detailRows}><DataRow label="当前任务" value={employee.currentTask ? employee.currentTask.label : '当前无任务'} /><DataRow label="交接" value={employee.sessionState === 'stopped' ? '可继续查看' : '暂无接班人'} /></div></section>;
}

export function EmployeesPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const [tab, setTab] = useState<'overview' | 'responsibilities' | 'collaboration' | 'memory' | 'handover' | 'tools' | 'workspace'>('overview');
  const [selectedEmployeeId, setSelectedEmployeeId] = useState<string | null>(null);
  const tabs = [{id: 'overview' as const, label: '概览'}, {id: 'responsibilities' as const, label: '职责'}, {id: 'collaboration' as const, label: '协作'}, {id: 'memory' as const, label: '记忆'}, {id: 'handover' as const, label: '交接'}, {id: 'tools' as const, label: '工具'}, {id: 'workspace' as const, label: '工作区'}];
  const state = renderOverviewState(query, '正在读取员工快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 员工" title="员工" />{state}</div>;
  const overview = getOverview(query);
  const employees = overview.employees;
  const selected = employees.find(employee => employee.employeeId === selectedEmployeeId) ?? employees[0] ?? null;
  return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 员工" title="员工" /><TabBar active={tab} onChange={setTab} tabs={tabs} />{tab !== 'overview' ? selected ? <EmployeeSubpage overview={overview} employee={selected} tab={tab === 'responsibilities' ? 'responsibilities' : tab === 'collaboration' ? 'collaboration' : tab === 'memory' ? 'memory' : tab === 'handover' ? 'handover' : tab === 'tools' ? 'tools' : 'workspace'} /> : <UnavailablePanel detail="当前快照没有员工对象。" title="没有可展示的员工" /> : <div className={styles.employeeWorkspace}><section className={styles.sectionCard}><div className={styles.toolbarLabel}><UsersRound aria-hidden="true" className={styles.icon} size={15} />员工列表</div><div className={styles.employeeList}>{employees.map(employee => <button className={styles.employeeListRow + ' ' + (selected?.employeeId === employee.employeeId ? styles.employeeListRowActive : '')} key={employee.employeeId} onClick={() => setSelectedEmployeeId(employee.employeeId)} type="button"><span className={styles.employeeAvatar}><UsersRound aria-hidden="true" size={15} /></span><span><strong>{employee.displayName}</strong><small>{labelRole(employee.role)} · {employee.employeeId}</small></span></button>)}</div></section>{selected ? <EmployeeDetail employee={selected} /> : <UnavailablePanel detail="当前快照没有员工对象。" title="没有可展示的员工" />}</div>}</div>;
}

export function OrgChartPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const state = renderOverviewState(query, '正在读取组织关系快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 组织关系" title="组织关系" />{state}</div>;
  const overview = getOverview(query);
  return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 组织关系" title="组织关系" /><section className={styles.orgChartCard}><div className={styles.orgChartToolbar}><div><h2 className={styles.sectionTitle}>组织关系</h2><span className={styles.orgChartMeta}>{overview.employees.length} 个员工对象已读入；节点按作用域展示，不推断汇报关系。</span></div><div className={styles.orgControls}><button aria-label="缩小" className={styles.orgZoomButton} disabled type="button">−</button><span className={styles.orgZoomValue}>100%</span><button aria-label="放大" className={styles.orgZoomButton} disabled type="button">+</button></div></div><div className={styles.orgCanvas}><div className={styles.orgTree}><div className={styles.orgRootWrap}><div className={styles.orgRoot}><span className={styles.orgRootMark}><Layers3 aria-hidden="true" size={18} /></span><div><strong>{overview.company.name}</strong><span>作用域根节点</span><small>{overview.company.companyId}</small></div></div></div><div className={styles.orgChildren}>{overview.employees.map(employee => <article className={styles.orgNode} key={employee.employeeId}><div className={styles.orgNodeHeader}><span className={styles.orgAvatar}><UsersRound aria-hidden="true" size={16} /></span></div><div className={styles.orgNodeBody}><strong>{employee.displayName}</strong><span>{labelRole(employee.role)}</span><small>{employee.employeeId}</small></div><div className={styles.orgNodeFooter}><StatusBadge label={labelDisplayValue(employee.status.primary)} tone={employeeStateTone(employee.status.tone)} /><code>关系未提供</code></div></article>)}</div></div></div></section></div>;
}

function resourceQualityLabel(quality: CompanyOverviewView['resources']['moneyQuality']): string {
  return labelQuality(quality);
}

function resourceQualityTone(quality: CompanyOverviewView['resources']['moneyQuality']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  return quality === 'reported' ? 'success' : quality === 'estimated' ? 'info' : 'warning';
}

export function ResourcesPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const state = renderOverviewState(query, '正在读取资源快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 资源" title="资源" />{state}</div>;
  const overview = getOverview(query);
  const resource = overview.resources;
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={resourceQualityLabel(resource.moneyQuality)} tone={resourceQualityTone(resource.moneyQuality)} />} eyebrow="公司 / 资源" title="资源" /><section className={styles.resourceGrid}><article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具调用</span><strong>{resource.toolCallsUsed + ' / ' + resource.toolCallsLimit}</strong><p>工具预算 · {labelQuality(resource.toolBudgetQuality)}</p><StatusBadge label="当前快照" tone="info" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>费用</span><strong>{resource.moneyAmount === null ? '不可得' : resource.moneyAmount + ' ' + (resource.currency ?? '')}</strong><p>费用口径 · {labelQuality(resource.moneyQuality)}</p><StatusBadge label={resourceQualityLabel(resource.moneyQuality)} tone={resourceQualityTone(resource.moneyQuality)} /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>截止时间</span><strong className={styles.resourceDate}>{formatEventTime(resource.asOf)}</strong><p>{labelResourceNote(resource.note)}</p><StatusBadge label="作用域隔离" tone="neutral" /></article></section><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>资源边界</span><h2 className={styles.sectionTitle}>资源口径</h2></div><Layers3 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="当前公司" value={companyId} mono /><DataRow label="工具预算" value="来自员工会话" /><DataRow label="费用" value={resource.moneyAmount === null ? '当前不可得' : '已报告'} /><DataRow label="合并规则" value="不跨作用域重复相加" /></div></section></div>;
}

export function EvidencePage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const state = renderOverviewState(query, '正在读取证据快照');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 证据" title="证据" />{state}</div>;
  const overview = getOverview(query);
  const artifacts = overview.artifacts;
  return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 证据" title="证据" /><section className={styles.sectionCard}><div className={styles.recordList}>{artifacts.length > 0 ? artifacts.map(artifact => <div className={styles.recordRow} key={artifact.artifactId}><div className={styles.recordLead}><BadgeCheck aria-hidden="true" size={16} /><div><strong>产物 / 证据</strong><span>{artifact.artifactId} · {artifact.authorEmployeeId} · {artifact.bytes}</span></div></div><div className={styles.recordMeta}><code>{artifact.contractRevisionId ?? '暂无合同版本'}</code><StatusBadge label={labelDisplayValue(artifact.verdict)} tone={artifact.verdict === 'passed' ? 'success' : artifact.verdict === 'failed' ? 'danger' : 'warning'} /></div></div>) : <div className={styles.emptyState}><span>当前快照没有产物。</span></div>}</div></section><section className={styles.detailGrid}><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>验收边界</span><h2 className={styles.sectionTitle}>验收状态</h2></div><Check aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="任务数" value={overview.tasks.length} /><DataRow label="待验收" value={overview.tasks.filter(task => task.acceptance !== 'passed').length} /><DataRow label="已通过" value={overview.tasks.filter(task => task.acceptance === 'passed').length} /></div></article><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>检查点</span><h2 className={styles.sectionTitle}>进度证据</h2></div><FileCheck2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="员工会话" value={overview.employees.filter(employee => employee.sessionId !== null).length + ' 个已启动'} /><DataRow label="当前版本" value={overview.meta.entityRevision} mono /><DataRow label="说明" value="检查点不等于最终验收" /></div></article></section></div>;
}

export function DecisionsPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const state = renderOverviewState(query, '正在读取待决事项');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 待决事项" title="待决事项" />{state}</div>;
  const overview = getOverview(query);
  const items = overview.attention;
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={items.length + ' 项待处理'} tone={items.length > 0 ? 'warning' : 'success'} />} eyebrow="公司 / 待决事项" title="待决事项" /><section className={styles.sectionCard}><div className={styles.recordList}>{items.length > 0 ? items.map(item => <div className={styles.decisionRow} key={item.id}><div className={styles.decisionRowLead}><span className={styles.decisionMarker}><CircleAlert aria-hidden="true" size={15} /></span><div><strong>{item.title}</strong><span>{item.id} · {item.subject.label}</span><p>{item.description}</p></div></div><div className={styles.recordMeta}><code>{item.evidenceRefs.length} 条证据引用</code><StatusBadge label={labelDisplayValue(item.tone)} tone={readStatusTone(item.tone)} /></div></div>) : <div className={styles.emptyState}><span>当前没有待处理事项。</span></div>}</div></section></div>;
}

export function CollaborationPage({api, companyId}: PageProps) {
  const query = useCollaboration(api, companyId);
  if (query.isPending) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 协作" title="协作收件箱" /><LoadingPanel label="正在读取消息与责任" /></div>;
  if (query.isError) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 协作" title="协作收件箱" /><ErrorPanel message={query.error.message} /></div>;
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={`${query.data.length} 条`} tone="info" />} eyebrow="公司 / 协作" title="协作收件箱" /><section className={styles.sectionCard}><div className={styles.recordList}>{query.data.length === 0 ? <div className={styles.emptyState}><MessageSquare aria-hidden="true" size={18} /><span>当前没有可见的消息或责任。</span></div> : query.data.map(item => <div className={styles.recordRow} key={item.messageId}><div className={styles.recordLead}><MessageSquare aria-hidden="true" size={16} /><div><strong>{item.content}</strong><span>{item.senderEmployeeId} → {item.recipientEmployeeId} · 任务 {item.taskId} · {item.kind}</span><span>合同版本 {item.contractRevisionId ?? '不可得'} · 版本 {item.taskRevision}</span></div></div><div className={styles.recordMeta}><StatusBadge label={labelDisplayValue(item.deliveryState)} tone={item.deliveryState === 'resolved' || item.deliveryState === 'applied' ? 'success' : 'info'} />{item.obligationState ? <StatusBadge label={`责任：${labelDisplayValue(item.obligationState)}`} tone={item.obligationState === 'fulfilled' ? 'success' : 'warning'} /> : null}<code>{item.evidenceRef ?? '暂无证据'}</code></div></div>)}</div></section></div>;
}

export function OperationsPage({api, companyId}: PageProps) {
  const query = useOperations(api, companyId);
  if (query.isPending) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 运行" title="运行与资源" /><LoadingPanel label="正在读取用量与健康状态" /></div>;
  if (query.isError) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 运行" title="运行与资源" /><ErrorPanel message={query.error.message} /></div>;
  const operations = query.data;
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={labelDisplayValue(operations.postgresqlStatus)} tone={operations.postgresqlStatus === 'ready' ? 'success' : 'warning'} />} eyebrow="公司 / 运行" title="运行与资源" /><section className={styles.resourceGrid}><article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具调用</span><strong>{operations.toolCallsUsed} / {operations.toolCallsLimit}</strong><p>当前已报告口径</p><StatusBadge label={labelQuality(operations.toolBudgetQuality)} tone={operations.toolBudgetQuality === 'reported' ? 'success' : 'warning'} /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>员工会话</span><strong>{operations.workerCount}</strong><p>未停止的员工会话</p><StatusBadge label="权威来源" tone="info" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>令牌</span><strong>{operations.inputTokens === null ? '不可得' : operations.inputTokens}</strong><p>输入 / 输出不伪造</p><StatusBadge label={operations.outputTokens === null ? '不可得' : '已报告'} tone={operations.outputTokens === null ? 'warning' : 'success'} /></article></section><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>运行时健康</span><h2 className={styles.sectionTitle}>运行时健康</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="PostgreSQL" value={labelDisplayValue(operations.postgresqlStatus)} /><DataRow label="CAS 存储" value={labelDisplayValue(operations.casStatus)} /><DataRow label="事件流" value={labelDisplayValue(operations.eventStreamStatus)} /><DataRow label="运行时长" value={operations.elapsedRuntime ?? '不可得'} /><DataRow label="最近运行时错误" value={operations.lastRuntimeError ?? '暂无已报告错误'} /></div></section></div>;
}

export function FeedbackPage() {
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label="来源未连接" tone="neutral" />} eyebrow="公司 / 反馈" title="反馈待办" /><UnavailablePanel detail="当前项目没有真实反馈采集或读取接口；不伪装已连接外部来源，也不创建模拟反馈记录。" title="当前没有真实反馈源" /></div>;
}

export function NotificationsPage({api, companyId}: PageProps) {
  const query = useNotifications(api, companyId);
  const configure = useConfigureNotificationRoute(api, companyId);
  const test = useTestNotification(api, companyId);
  const [destination, setDestination] = useState('local://workbench');
  const [enabled, setEnabled] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  useEffect(() => {
    const route = query.data?.routes[0];
    if (!route) return;
    setDestination(route.destination || 'local://workbench');
    setEnabled(route.enabled);
  }, [query.data]);
  if (query.isPending) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 通知" title="对外通知" /><LoadingPanel label="正在读取通知路由" /></div>;
  if (query.isError) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 通知" title="对外通知" /><ErrorPanel message={query.error.message} /></div>;
  const route = query.data.routes[0] ?? null;
  async function saveRoute(): Promise<void> {
    const trimmedDestination = destination.trim();
    if (trimmedDestination === '') {
      setFormError('请填写目标地址');
      return;
    }
    setFormError(null);
    try {
      await configure.mutateAsync({adapter: 'local', destination: trimmedDestination, enabled, requestId: `notification-route-${companyId}-${Date.now()}`});
    } catch {
      // mutation state surfaces the error
    }
  }
  async function sendTest(): Promise<void> {
    setFormError(null);
    try {
      await test.mutateAsync({requestId: `notification-test-${companyId}-${Date.now()}`});
    } catch {
      // mutation state surfaces the error
    }
  }
  const actionError = formError ?? (configure.isError ? `通知操作失败：${configure.error.message}` : test.isError ? `通知操作失败：${test.error.message}` : null);
  return <div className={styles.viewStack}><ViewHeader action={<StatusBadge label={route?.enabled ? '已启用' : '已停用'} tone={route?.enabled ? 'success' : 'neutral'} />} eyebrow="公司 / 通知" title="对外通知" /><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>通知通道</span><h2 className={styles.sectionTitle}>通道配置</h2></div><Radio aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.formStack}><label className={styles.formLabel}>适配器<input className={styles.formField} value="本地记录" readOnly /></label><label className={styles.formLabel}>目标地址<input className={styles.formField} value={destination} onChange={event => setDestination(event.target.value)} placeholder="例如 local://workbench" /></label><label className={styles.formLabel}><input checked={enabled} onChange={event => setEnabled(event.target.checked)} type="checkbox" /> 启用本地通知记录</label><div className={styles.wizardActions}><button className={styles.commandButton} disabled={configure.isPending || test.isPending} onClick={() => { void saveRoute(); }} type="button">{configure.isPending ? '正在保存…' : '保存路由'}</button><button className={styles.textButton} disabled={test.isPending || configure.isPending || !(route?.enabled ?? enabled)} onClick={() => { void sendTest(); }} type="button">{test.isPending ? '正在测试…' : '发送通知'}</button></div>{actionError ? <p className={styles.formError} role="alert">{actionError}</p> : null}{configure.isSuccess && !configure.isPending ? <p className={styles.formHint} role="status">路由配置已保存。</p> : null}<p className={styles.formHint}>当前实现仅保存本地记录；未实现 QQ 或其他外部账号绑定，保存不等于外部发送。发送通知会按当前已保存且启用的路由创建本地投递记录。</p></div></section><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>投递结果</span><h2 className={styles.sectionTitle}>投递结果</h2></div><StatusBadge label={`${query.data.deliveries.length} 条`} tone="info" /></div><div className={styles.recordList}>{query.data.deliveries.length === 0 ? <div className={styles.emptyState}>暂无通知投递。</div> : query.data.deliveries.map(delivery => <div className={styles.recordRow} key={delivery.deliveryId}><div className={styles.recordLead}><Radio aria-hidden="true" size={16} /><div><strong>{labelNotificationAdapter(delivery.adapter)} · {labelDisplayValue(delivery.state)}</strong><span>{delivery.deliveryId} · 意图 {delivery.intentId}</span></div></div><div className={styles.recordMeta}><StatusBadge label={delivery.errorCode ?? '无错误'} tone={delivery.errorCode ? 'danger' : 'success'} /></div></div>)}</div></section></div>;
}


export function SettingsPage({api, companyId}: PageProps) {
  const query = useCompanyOverview(api, companyId);
  const companiesQuery = useCompanyList(api);
  const runtimeQuery = useRuntimeSettings(api, companyId);
  const catalogQuery = useCapabilityCatalog(api, companyId);
  const [tab, setTab] = useState<'workspace' | 'capabilities'>('workspace');
  const state = renderOverviewState(query, '正在读取工作台设置');
  if (state) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 设置" title="设置" />{state}</div>;
  const overview = getOverview(query);
  const company = companiesQuery.data?.find(item => item.id === companyId) ?? null;
  if (tab === 'capabilities') return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 设置" title="设置" /><TabBar active={tab} onChange={setTab} tabs={[{id: 'workspace' as const, label: '工作区'}, {id: 'capabilities' as const, label: '能力'}]} /><CapabilityCatalogPanel api={api} companyId={companyId} query={catalogQuery} /></div>;
  if (runtimeQuery.isPending) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 设置" title="设置" /><div className={styles.emptyState} role="status"><Radio aria-hidden="true" size={18} /><span>正在读取运行时就绪状态</span></div></div>;
  if (runtimeQuery.isError) return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 设置" title="设置" /><ErrorPanel message={runtimeQuery.error.message} /></div>;
  const runtime = runtimeQuery.data;
  return <div className={styles.viewStack}><ViewHeader eyebrow="公司 / 设置" title="设置" /><TabBar active={tab} onChange={setTab} tabs={[{id: 'workspace' as const, label: '工作区'}, {id: 'capabilities' as const, label: '能力'}]} /><section className={styles.detailGrid}><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>工作区</span><h2 className={styles.sectionTitle}>组织配置</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="公司作用域" value={companyId} mono /><DataRow label="数据模式" value={labelDataMode(overview.meta.dataMode)} /><DataRow label="数据来源" value={overview.meta.sourceLabel} /><DataRow label="数据新鲜度" value={labelFreshness(overview.meta.freshness)} /></div></article><RuntimeSettingsPanel api={api} runtime={runtime} /><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>安全边界</span><h2 className={styles.sectionTitle}>当前限制</h2></div><ShieldAlert aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.checkList}><div><Check aria-hidden="true" size={15} /><span>浏览不会唤醒员工</span></div><div><Check aria-hidden="true" size={15} /><span>浏览不会发送命令</span></div><div><ShieldAlert aria-hidden="true" size={15} /><span>凭据只显示就绪状态，不返回内容</span></div></div></article></section>{company === null ? <UnavailablePanel detail="公司目录尚未返回当前作用域；不会用总览名称伪造可编辑配置。" title="组织配置不可用" /> : <OrganizationSettings api={api} company={company} />}</div>;
}

function RuntimeSettingsPanel({api, runtime}: Readonly<{api: WorkbenchApi; runtime: RuntimeSettingsView}>): ReactElement {
  const update = useUpdateRuntimeSettings(api);
  const [provider, setProvider] = useState(runtime.provider);
  const [model, setModel] = useState(runtime.model);
  const [effort, setEffort] = useState(runtime.effort);
  const [profile, setProfile] = useState(runtime.profile);
  const [apiKey, setApiKey] = useState('');
  const [clearApiKey, setClearApiKey] = useState(false);
  useEffect(() => {
    setProvider(runtime.provider);
    setModel(runtime.model);
    setEffort(runtime.effort);
    setProfile(runtime.profile);
    setApiKey('');
    setClearApiKey(false);
  }, [runtime.effort, runtime.model, runtime.profile, runtime.provider, runtime.authReadiness]);
  const isCodexProvider = provider === 'codex';
  const isOpenAIProvider = provider === 'openai';
  const isDeepSeekProvider = provider === 'deepseek';
  const isHTTPProvider = isOpenAIProvider || isDeepSeekProvider;
  const catalogQuery = useCodexModelCatalog(api, runtime.companyId, isCodexProvider);
  const codexModels = catalogQuery.data?.models ?? [];
  const selectedModel = codexModels.find(option => option.model === model);
  const selectedEffort = selectedModel?.supportedReasoningEfforts.find(option => option.reasoningEffort === effort);
  const runtimeStatusLabel = runtime.runtimeReadiness === 'restart_required'
    ? labelDisplayValue(runtime.runtimeReadiness)
    : isCodexProvider && catalogQuery.isError
      ? 'Codex 目录不可用'
      : isCodexProvider && catalogQuery.isPending
        ? '正在读取模型目录'
        : isCodexProvider && catalogQuery.isSuccess && codexModels.length === 0
          ? '没有可用模型'
          : isCodexProvider && catalogQuery.isSuccess
            ? '模型目录已加载'
            : labelDisplayValue(runtime.runtimeReadiness);
  const runtimeStatusTone: StatusTone = runtime.runtimeReadiness === 'restart_required' || (isCodexProvider && (catalogQuery.isError || (catalogQuery.isSuccess && codexModels.length === 0)))
    ? 'warning'
    : isCodexProvider && catalogQuery.isPending
      ? 'info'
      : runtime.runtimeReadiness === 'ready'
        ? 'success'
        : 'warning';
  const modelSelection = selectedModel === undefined ? CUSTOM_CODEX_MODEL : model;
  const deepSeekModelSelection = DEEPSEEK_MODELS.includes(model as typeof DEEPSEEK_MODELS[number]) ? model : CUSTOM_DEEPSEEK_MODEL;
  const profileToSave = isCodexProvider ? `${model}/${effort}` : profile;
  const keyReady = !isHTTPProvider || apiKey.trim() !== '' || (runtime.provider === provider && runtime.authReadiness === 'configured' && !clearApiKey);
  const hasUnsavedChanges = provider !== runtime.provider || model !== runtime.model || apiKey.trim() !== '' || clearApiKey || (isOpenAIProvider
    ? profile !== runtime.profile || runtime.effort !== 'chat'
    : isDeepSeekProvider
      ? profile !== runtime.profile || effort !== runtime.effort
      : effort !== runtime.effort || (!isCodexProvider && profile !== runtime.profile));

  function changeProvider(nextProvider: string): void {
    setProvider(nextProvider);
    if (nextProvider === 'codex') {
      if (provider !== 'codex') {
        setModel('');
        setEffort('');
      }
      setApiKey('');
      setClearApiKey(false);
      return;
    }
    if (nextProvider === 'openai') {
      if (provider !== 'openai') {
        setModel('');
        setProfile('https://api.openai.com/v1');
      }
      setEffort('chat');
      setApiKey('');
      setClearApiKey(false);
      return;
    }
    if (nextProvider === 'deepseek') {
      if (provider !== 'deepseek') {
        setModel('deepseek-flash');
        setProfile('https://api.deepseek.com');
        setEffort('high');
      }
      setApiKey('');
      setClearApiKey(false);
      return;
    }
    setModel('deterministic/fake');
    setEffort('bounded');
    setProfile('deterministic/fake');
    setApiKey('');
    setClearApiKey(false);
  }

  function changeCodexModel(nextModel: string): void {
    if (nextModel === CUSTOM_CODEX_MODEL) {
      setModel('');
      setEffort('');
      return;
    }
    setModel(nextModel);
    const nextOption = codexModels.find(option => option.model === nextModel);
    if (nextOption !== undefined) {
      const currentEffort = nextOption.supportedReasoningEfforts.find(option => option.reasoningEffort === effort);
      const defaultEffort = nextOption.supportedReasoningEfforts.find(option => option.reasoningEffort === nextOption.defaultReasoningEffort);
      setEffort(currentEffort?.reasoningEffort ?? defaultEffort?.reasoningEffort ?? nextOption.supportedReasoningEfforts[0]?.reasoningEffort ?? '');
    }
  }

  async function saveRuntimeSettings(): Promise<void> {
    await update.mutateAsync({companyId: runtime.companyId, provider, model, effort: isOpenAIProvider ? 'chat' : effort, profile: profileToSave, apiKey: isHTTPProvider ? apiKey.trim() : undefined, clearApiKey: isHTTPProvider && clearApiKey, requestId: `runtime-settings-${runtime.companyId}-${Date.now()}`});
    setApiKey('');
    setClearApiKey(false);
  }
  return <article className={`${styles.sectionCard} ${styles.runtimeSettingsCard}`}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>运行时配置</span><h2 className={styles.sectionTitle}>运行时就绪</h2></div>
      <StatusBadge label={runtimeStatusLabel} tone={runtimeStatusTone} />
    </div>
    <div className={styles.runtimeFieldsGrid}>
      <label className={styles.formLabel}>模型提供方
        <select className={`${styles.formField} ${styles.runtimeSelect}`} value={provider} onChange={event => changeProvider(event.target.value)}>
          <option value="deterministic">本地确定性</option>
          <option value="fake">离线 Fake</option>
          <option value="codex">Codex</option>
          <option value="openai">OpenAI 协议</option>
          <option value="deepseek">DeepSeek</option>
          {!['deterministic', 'fake', 'codex', 'openai', 'deepseek'].includes(provider) ? <option value={provider}>当前：{provider}</option> : null}
        </select>
      </label>
      {isHTTPProvider ? <label className={styles.formLabel}>接口地址
        <input className={styles.formField} value={profile} onChange={event => setProfile(event.target.value)} placeholder={isDeepSeekProvider ? 'https://api.deepseek.com' : 'https://api.openai.com/v1'} spellCheck={false} />
        <span className={styles.formHint}>{isDeepSeekProvider ? 'DeepSeek 官方 OpenAI 兼容地址是 https://api.deepseek.com。' : '兼容 OpenAI Chat Completions 的基址。公网地址必须是 https；本机回环地址可以用 http。'}</span>
      </label> : null}
      {isCodexProvider ? <div className={styles.runtimeField}>
        <label className={styles.formLabel}>模型
          <select className={`${styles.formField} ${styles.runtimeSelect}`} disabled={catalogQuery.isFetching && !catalogQuery.isSuccess} value={modelSelection} onChange={event => changeCodexModel(event.target.value)}>
            {codexModels.map(option => <option key={option.model} value={option.model}>{option.displayName}{option.isDefault ? ' · 默认' : ''}</option>)}
            <option value={CUSTOM_CODEX_MODEL}>自定义模型 ID…</option>
          </select>
        </label>
        {selectedModel === undefined ? <input aria-label="自定义 Codex 模型 ID" className={styles.formField} onChange={event => setModel(event.target.value)} placeholder="输入 Codex 模型 ID" value={model} /> : <p className={styles.formHint}>{selectedModel.description}</p>}
        {catalogQuery.isPending ? <p className={styles.formHint} role="status">正在从 Codex 读取模型目录…</p> : null}
        {catalogQuery.isError ? <div className={styles.catalogError} role="alert"><span>Codex 模型目录读取失败：{catalogQuery.error.message}</span><button className={styles.textButton} onClick={() => { void catalogQuery.refetch(); }} type="button">重试</button></div> : null}
        {catalogQuery.isSuccess && codexModels.length === 0 ? <p className={styles.formHint}>Codex 当前没有返回可选模型，可输入模型 ID。</p> : null}
        <p className={styles.formHint}>列表来自本机 Codex runtime；仅读取模型元数据，不会创建对话或发送模型内容。</p>
      </div> : isDeepSeekProvider ? <label className={styles.formLabel}>模型
        <select className={`${styles.formField} ${styles.runtimeSelect}`} value={deepSeekModelSelection} onChange={event => setModel(event.target.value === CUSTOM_DEEPSEEK_MODEL ? '' : event.target.value)}>
          {DEEPSEEK_MODELS.map(item => <option key={item} value={item}>{item}</option>)}
          <option value={CUSTOM_DEEPSEEK_MODEL}>自定义模型 ID…</option>
        </select>
        {deepSeekModelSelection === CUSTOM_DEEPSEEK_MODEL ? <input aria-label="自定义 DeepSeek 模型 ID" className={styles.formField} onChange={event => setModel(event.target.value)} placeholder="输入 DeepSeek 模型 ID" spellCheck={false} value={model} /> : null}
      </label> : <label className={styles.formLabel}>模型
        <input className={styles.formField} value={model} onChange={event => setModel(event.target.value)} placeholder={isOpenAIProvider ? '例如 gpt-4o-mini' : undefined} spellCheck={false} />
      </label>}
      {isHTTPProvider ? <label className={styles.formLabel}>API Key
        <input className={styles.formField} autoComplete="off" onChange={event => { setApiKey(event.target.value); setClearApiKey(false); }} placeholder={runtime.provider === provider && runtime.authReadiness === 'configured' && !clearApiKey ? '已保存，留空则不修改' : '只写入本机，不会回显'} type="password" value={apiKey} />
        <span className={styles.formHint}>{clearApiKey ? '保存后会删除已保存的密钥。' : runtime.provider === provider && runtime.authReadiness === 'configured' ? '本机已有密钥。页面不会读回密钥内容。' : '保存后只在本机凭据文件中保留密钥。'}</span>
        {runtime.provider === provider && runtime.authReadiness === 'configured' ? <button className={styles.textButton} onClick={() => { setClearApiKey(true); setApiKey(''); }} type="button">清除已保存的密钥</button> : null}
      </label> : null}
      {isCodexProvider ? <div className={styles.runtimeField}>
        {selectedModel !== undefined && selectedModel.supportedReasoningEfforts.length > 0 ? <label className={styles.formLabel}>推理强度
          <select className={`${styles.formField} ${styles.runtimeSelect}`} value={selectedEffort?.reasoningEffort ?? ''} onChange={event => setEffort(event.target.value)}>
            <option disabled value="">选择此模型支持的推理强度</option>
            {selectedModel.supportedReasoningEfforts.map(option => <option key={option.reasoningEffort} value={option.reasoningEffort}>{option.description} · {option.reasoningEffort}</option>)}
          </select>
        </label> : <label className={styles.formLabel}>推理强度
          <input className={styles.formField} value={effort} onChange={event => setEffort(event.target.value)} placeholder="先从 Codex 目录选择模型" />
        </label>}
        <p className={styles.formHint}>{selectedModel === undefined ? '自定义模型需要手动填写 Codex 接受的推理强度 ID。' : `此列表由 Codex 为 ${selectedModel.displayName} 返回。`}</p>
      </div> : isDeepSeekProvider ? <label className={styles.formLabel}>思考强度
        <select className={`${styles.formField} ${styles.runtimeSelect}`} value={DEEPSEEK_EFFORTS.some(item => item.id === effort) ? effort : 'high'} onChange={event => setEffort(event.target.value)}>
          {DEEPSEEK_EFFORTS.map(item => <option key={item.id} value={item.id}>{item.label}</option>)}
        </select>
        <span className={styles.formHint}>关闭思考对应 reasoning_effort=none。低、高、最大对应 low、high、max，并开启思考。思考模式下 temperature 不生效。</span>
      </label> : isOpenAIProvider ? null : <label className={styles.formLabel}>推理强度
        <input className={styles.formField} value={effort} onChange={event => setEffort(event.target.value)} />
      </label>}
      {isCodexProvider ? <label className={styles.formLabel}>配置档案
        <input className={`${styles.formField} ${styles.runtimeReadOnlyField}`} readOnly value={profileToSave} />
        <span className={styles.formHint}>由所选模型和推理强度生成。</span>
      </label> : isHTTPProvider ? null : <label className={styles.formLabel}>配置档案
        <input className={styles.formField} value={profile} onChange={event => setProfile(event.target.value)} />
      </label>}
    </div>
    <div className={styles.runtimeStatusGrid}>
      <DataRow label="认证就绪状态" value={<StatusBadge label={labelDisplayValue(runtime.authReadiness)} tone={runtime.authReadiness === 'ready' || runtime.authReadiness === 'configured' || runtime.authReadiness === 'not_required' ? 'success' : 'warning'} />} />
      <DataRow label="运行时版本" value={labelDisplayValue(runtime.runtimeVersion)} mono />
      <DataRow label="产品工具面资格" value={labelDisplayValue(runtime.productSurfaceQualification)} mono />
      <DataRow label="PostgreSQL" value={labelDisplayValue(runtime.postgresqlStatus)} />
      <DataRow label="CAS" value={labelDisplayValue(runtime.casStatus)} />
      <DataRow label="事件流" value={labelDisplayValue(runtime.eventStreamStatus)} />
    </div>
    {update.isError ? <p className={styles.formError} role="alert">保存失败：{update.error.message}</p> : null}
    {update.isSuccess && !hasUnsavedChanges ? <p className={styles.runtimeSaved} role="status">配置已保存，将在运行时重启后生效。</p> : null}
    <div className={styles.runtimeActions}>
      <button className={styles.commandButton} disabled={update.isPending || (isCodexProvider && catalogQuery.isFetching) || (isCodexProvider && selectedModel !== undefined && selectedModel.supportedReasoningEfforts.length > 0 && selectedEffort === undefined) || (isHTTPProvider && !keyReady) || !hasUnsavedChanges || model.trim() === '' || (!isOpenAIProvider && effort.trim() === '') || profileToSave.trim() === ''} onClick={() => { void saveRuntimeSettings().catch(() => undefined); }} type="button">{update.isPending ? '正在保存…' : '保存运行时配置'}</button>
      <p className={styles.formHint}>{isHTTPProvider ? '密钥只写入本机凭据文件，接口不返回密钥，保存也不会向该地址发送请求。员工回合仍使用当前已启动的运行时，变更在重启后才会被下一次执行读取。' : '凭据只显示就绪状态；保存不会读取、返回或记录秘密，也不会发起模型调用。变更在运行时重启后生效。'}</p>
    </div>
  </article>;
}

function OrganizationSettings({api, company}: Readonly<{api: WorkbenchApi; company: CompanySummaryView}>): ReactElement {
  const navigate = useNavigate();
  const update = useUpdateCompany(api);
  const archive = useArchiveCompany(api);
  const [name, setName] = useState(company.name);
  const [workspaceRoot, setWorkspaceRoot] = useState(company.workspaceRoot);
  const [validationError, setValidationError] = useState<string | null>(null);
  useEffect(() => {
    setName(company.name);
    setWorkspaceRoot(company.workspaceRoot);
    setValidationError(null);
  }, [company.name, company.workspaceRoot, company.id]);
  const trimmedName = name.trim();
  const trimmedWorkspaceRoot = workspaceRoot.trim();
  const nameError = trimmedName === '' ? '请填写公司名称' : trimmedName.length > 200 ? '公司名称不能超过 200 个字符' : null;
  const workspaceError = trimmedWorkspaceRoot === '' ? '请填写工作区 / 项目目录' : trimmedWorkspaceRoot.length > 1024 ? '工作区目录不能超过 1024 个字符' : null;
  const hasChanges = trimmedName !== company.name || trimmedWorkspaceRoot !== company.workspaceRoot;
  async function save(): Promise<void> {
    setValidationError(null);
    if (nameError || workspaceError) {
      setValidationError(nameError ?? workspaceError);
      return;
    }
    if (!hasChanges) return;
    try {
      await update.mutateAsync({companyId: company.id, name: trimmedName, workspaceRoot: trimmedWorkspaceRoot, roster: company.roster, requestId: `company-update-${company.id}-${Date.now()}`});
    } catch {
      // mutation state surfaces the error
    }
  }
  async function archiveCompany(): Promise<void> {
    if (!window.confirm('确认归档此公司？历史记录会保留，但进行中的使命将被停止。')) return;
    setValidationError(null);
    try {
      await archive.mutateAsync({companyId: company.id, requestId: `company-archive-${company.id}-${Date.now()}`});
      navigate('/group/overview');
    } catch {
      // mutation state surfaces the error
    }
  }
  const actionError = validationError ?? (update.isError ? `操作失败：${update.error.message}` : archive.isError ? `操作失败：${archive.error.message}` : null);
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>公司 / 组织</span><h2 className={styles.sectionTitle}>公司基本信息</h2></div><StatusBadge label={company.state === 'active' ? '活跃' : '已归档'} tone={company.state === 'active' ? 'success' : 'neutral'} /></div><div className={styles.formStack}><label className={styles.formLabel}>公司名称<input className={styles.formField} value={name} onChange={event => setName(event.target.value)} /></label><label className={styles.formLabel}>工作区 / 项目目录<input className={styles.formField} value={workspaceRoot} onChange={event => setWorkspaceRoot(event.target.value)} /></label><div className={styles.detailRows}><DataRow label="固定员工" value={`${company.roster.length} 个逻辑员工`} /><DataRow label="员工 ID" value={company.roster.map(employee => employee.id).join(' · ')} mono /></div><p className={styles.formHint}>固定员工的显示名、岗位与模型配置由创建时的组织草案确定；此处可更新公司名称与工作区目录。</p>{actionError ? <p className={styles.formError} role="alert">{actionError}</p> : null}{update.isSuccess && !update.isPending && !hasChanges ? <p className={styles.formHint} role="status">基本信息已保存。</p> : null}<div className={styles.wizardActions}><button className={styles.commandButton} disabled={update.isPending || archive.isPending || company.state === 'archived' || !hasChanges} onClick={() => { void save(); }} type="button">{update.isPending ? '正在保存…' : '保存基本信息'}</button>{company.state === 'active' ? <button className={styles.textButton} disabled={archive.isPending || update.isPending} onClick={() => { void archiveCompany(); }} type="button">{archive.isPending ? '正在归档…' : '归档公司'}</button> : null}</div></div></section>;
}

