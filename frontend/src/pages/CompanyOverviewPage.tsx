// pattern: Imperative Shell

import {ArrowUpRight, BadgeCheck, CircleAlert, Clock3, Link2, LockKeyhole, MessageSquare, Play, Radio, Square, UsersRound} from 'lucide-react';
import {useRef, useState, type FormEvent} from 'react';
import {Link} from 'react-router-dom';
import type {AcceptanceContract, CompanyOverviewView, EmployeeScheduleState, EmployeeSummary, StatusTone} from '../domain/workbench';
import {formatEntityId, formatEventTime} from '../domain/activity-presentation';
import {labelDataMode, labelDisplayValue, labelRole} from '../domain/display-labels';
import {selectCheckpointForTask} from '../domain/checkpoint-projection';
import {useCancelMission, useCompanyOverview, useCreateMission, useStartMission} from '../data/workbench-query';
import {CommandApiError, type WorkbenchApi} from '../data/workbench-api';
import {isAcceptanceContract} from '../domain/workbench-validation';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {DailyRoutinePanel} from './DailyRoutinePanel';
import {MissionCloseoutControls} from './MissionCloseoutControls';
import styles from '../styles/workbench.module.css';

type CompanyOverviewPageProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
}>;

function statusLabel(status: EmployeeSummary['status']['primary']): string {
  const labels: Readonly<Record<EmployeeSummary['status']['primary'], string>> = {
    working: '工作中',
    sleeping: '休眠',
    wake_pending: '等待唤醒',
    waiting_tool: '等待工具',
    waiting_peer: '等待同事',
    waiting_external: '等待外部数据',
    waiting_quota: '等待额度',
    handover: '交接中',
    paused: '已暂停',
    lost_contact: '失去联系',
    stopped: '已停止',
  };
  return labels[status];
}

function statusIcon(status: EmployeeSummary['status']['primary']) {
  if (status === 'working') return Radio;
  if (status === 'sleeping') return Clock3;
  return LockKeyhole;
}

function statusTone(tone: StatusTone): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  return tone;
}

function employeeScheduleLabel(state: EmployeeScheduleState): string {
  const labels: Readonly<Record<EmployeeScheduleState, string>> = {
    quiescing: '准备休眠',
    sleeping: '休眠',
    wake_pending: '等待唤醒',
    admitted: '已准入',
    working: '工作中',
    paused: '已暂停',
    waiting_quota: '等待额度',
  };
  return labels[state];
}

function employeeScheduleTone(state: EmployeeScheduleState): StatusTone {
  if (state === 'wake_pending' || state === 'paused' || state === 'waiting_quota') return 'warning';
  if (state === 'working') return 'success';
  if (state === 'admitted' || state === 'quiescing') return 'info';
  return 'neutral';
}

function qualificationTone(status: EmployeeSummary['qualification']['status']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'supported') return 'success';
  if (status === 'limited') return 'warning';
  if (status === 'unsupported') return 'danger';
  return 'info';
}

function missionStateLabel(state: CompanyOverviewView['mission']['state']): string {
  const labels: Readonly<Record<CompanyOverviewView['mission']['state'], string>> = {draft: '草案', active: '进行中', paused: '已暂停', closing: '收尾中', succeeded: '已完成', ended_not_met: '未达成而结束', cancelled: '已取消'};
  return labels[state];
}

function missionStateTone(state: CompanyOverviewView['mission']['state']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (state === 'succeeded') return 'success';
  if (state === 'paused' || state === 'closing') return 'warning';
  if (state === 'ended_not_met') return 'danger';
  if (state === 'active') return 'info';
  return 'neutral';
}

function openObligationCount(overview: CompanyOverviewView): number {
  return overview.obligations.filter(item => item.state !== 'fulfilled' && item.state !== 'declined' && item.state !== 'superseded').length;
}

function pendingAcceptanceCount(overview: CompanyOverviewView): number {
  return overview.tasks.filter(task => task.acceptance !== 'passed').length;
}

function taskStateSummary(overview: CompanyOverviewView): string {
  const working = overview.tasks.filter(task => task.state === 'working').length;
  const candidate = overview.tasks.filter(task => task.state === 'candidate').length;
  return `${working} 工作中 · ${candidate} 个候选任务`;
}

function optionalTime(value: string | null): string {
  return value === null ? '不可得' : formatEventTime(value);
}

function ViewHeader() {
  return (
    <div className={styles.viewHeader}>
      <div>
        <p className={styles.eyebrow}>公司 / 工作台</p>
        <h1 className={styles.pageTitle}>公司总览</h1>
      </div>
    </div>
  );
}

function MetricCard({label, value, detail}: Readonly<{label: string; value: string; detail?: string}>) {
  return (
    <article className={styles.metricCard}>
      <div className={styles.metricHead}><span className={styles.metricLabel}>{label}</span></div>
      <strong className={styles.metricValue}>{value}</strong>
      {detail ? <span className={styles.metricDetail}>{detail}</span> : null}
    </article>
  );
}

function SnapshotStrip({overview, companyId}: Readonly<{overview: CompanyOverviewView; companyId: string}>) {
  return (
    <section className={styles.snapshotStrip}>
      <div className={styles.snapshotLeading}>
        <div className={styles.companyGlyph} aria-hidden="true">P</div>
        <div>
          <div className={styles.snapshotTitleRow}><h2>{overview.company.name}</h2><span className={styles.scopeTag}>作用域公司</span></div>
          <p className={styles.snapshotSubline}>{overview.company.companyId}</p>
        </div>
      </div>
      <div className={styles.snapshotMeta}>
        <span className={styles.snapshotState}>数据快照 · <strong>{labelDataMode(overview.meta.dataMode)}</strong></span>
        <Link className={styles.textButton} to={`/companies/${companyId}/activity`}>查看活动 <ArrowUpRight aria-hidden="true" size={14} /></Link>
      </div>
    </section>
  );
}

function MissionCard({overview, api, companyId}: Readonly<{overview: CompanyOverviewView; api: WorkbenchApi; companyId: string}>) {
  const {mission} = overview;
  const closeout = mission.closeout;
  const isMissionConfigured = mission.missionId !== 'unavailable';
  const revision = mission.currentContractRevision;
  return (
    <article className={`${styles.sectionCard} ${styles.missionCard}`} data-mission-state={mission.state}>
      <div className={styles.sectionHeader}>
        <div><span className={styles.cardEyebrow}>当前目标</span><h2 className={styles.sectionTitle}>{isMissionConfigured ? mission.title : '未配置'}</h2></div>
        <StatusBadge label={isMissionConfigured ? missionStateLabel(mission.state) : '未配置'} tone={isMissionConfigured ? missionStateTone(mission.state) : 'neutral'} />
      </div>
      <p className={styles.panelDescription}>{isMissionConfigured ? mission.goal : '当前公司尚未配置目标使命。'}</p>
      <div className={styles.missionFooter}>
        <div><span className={styles.sourceFieldLabel}>来源</span><code>{isMissionConfigured ? mission.contract : '未配置'}</code></div>
        <div><span className={styles.sourceFieldLabel}>合同版本</span><div className={styles.revisionValue}><strong>{revision ? `版本 ${revision.revision}` : isMissionConfigured ? '不可得' : '未配置'}</strong><span>{revision ? labelDisplayValue(revision.state) : isMissionConfigured ? '未知' : '未配置'}</span></div></div>
        <StatusBadge label={revision ? labelDisplayValue(revision.state) : isMissionConfigured ? '未知' : '未配置'} tone={revision?.state === 'accepted' ? 'success' : isMissionConfigured ? 'info' : 'neutral'} />
      </div>
      <section className={styles.acceptancePanel} data-acceptance-contract={mission.acceptanceContract ? 'configured' : 'not-configured'} aria-label="公开验收条件">
        <div className={styles.acceptanceHeader}><BadgeCheck aria-hidden="true" size={15} /><strong>公开验收条件</strong></div>
        {mission.acceptanceContract ? <ol>
          {mission.acceptanceContract.required_text.map((criterion, index) => <li key={`${index}-${criterion}`}>{criterion}</li>)}
        </ol> : <p>未配置。员工可以探索，但不能创建合格检查点或提交产物。</p>}
      </section>
      {closeout ? <section className={styles.acceptancePanel} data-mission-closeout={closeout.terminalOutcome ?? 'closing'} aria-label="使命收尾记录">
        <div className={styles.acceptanceHeader}><BadgeCheck aria-hidden="true" size={15} /><strong>收尾记录 · {closeout.terminalOutcome ?? '处理中'}</strong></div>
        <p>{closeout.rationale}</p>
        {closeout.acceptanceArtifactIds.length > 0 ? <p>验收证据：{closeout.acceptanceArtifactIds.join('、')}</p> : null}
        {closeout.report ? <p>收尾报告：取消任务 {String(closeout.report.cancelledTaskTotal ?? 0)} 项，撤销责任 {String(closeout.report.declinedObligationTotal ?? 0)} 项，移交责任 {String(closeout.report.supersededObligationTotal ?? 0)} 项，取消周期实例 {String(closeout.report.cancelledRoutineTotal ?? 0)} 项。</p> : null}
      </section> : null}
      <MissionControls api={api} companyId={companyId} mission={mission} />
      <MissionCloseoutControls api={api} companyId={companyId} missionId={mission.missionId} missionState={mission.state} closeout={mission.closeout} />
    </article>
  );
}

type CommandPhase = 'idle' | 'submitting' | 'accepted' | 'conflict' | 'error';

function MissionControls({api, companyId, mission}: Readonly<{api: WorkbenchApi; companyId: string; mission: CompanyOverviewView['mission']}>) {
  const createMutation = useCreateMission(api, companyId);
  const startMutation = useStartMission(api, companyId);
  const cancelMutation = useCancelMission(api, companyId);
  const [title, setTitle] = useState('');
  const [goal, setGoal] = useState('');
  const [protocolToolCallLimit, setProtocolToolCallLimit] = useState('');
  const [acceptanceCriteria, setAcceptanceCriteria] = useState('');
  const [phase, setPhase] = useState<CommandPhase>('idle');
  const [message, setMessage] = useState<string | null>(null);
  const createRequest = useRef<Readonly<{title: string; goal: string; criteria: string; toolCallLimit: number; requestId: string}> | null>(null);
  const startRequestID = useRef<string | null>(null);
  const cancelRequestID = useRef<string | null>(null);

  if (api.mode !== 'real') {
    return null;
  }

  const isSubmitting = createMutation.isPending || startMutation.isPending || cancelMutation.isPending || phase === 'submitting';
  const setFailure = (error: unknown) => {
    const conflict = error instanceof CommandApiError && error.status === 409;
    setPhase(conflict ? 'conflict' : 'error');
      setMessage(error instanceof Error ? error.message : '命令提交失败');
  };
  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedTitle = title.trim();
    const trimmedGoal = goal.trim();
    const toolCallLimit = Number(protocolToolCallLimit);
    const criteria = acceptanceCriteria.split(/\r?\n/).map(item => item.trim()).filter(Boolean);
    const acceptanceContract: AcceptanceContract | null = criteria.length === 0 ? null : {revision: 'text-acceptance@1', required_text: criteria};
    if (trimmedTitle === '' || trimmedGoal === '') {
      setPhase('error');
      setMessage('标题和目标正文不能为空');
      return;
    }
    if (!Number.isSafeInteger(toolCallLimit) || toolCallLimit < 1) {
      setPhase('error');
      setMessage('使命上限必须是大于 0 的安全整数');
      return;
    }
    if (acceptanceContract !== null && !isAcceptanceContract(acceptanceContract)) {
      setPhase('error');
      setMessage('验收条件需为 1–8 条非空文本，每条不超过 512 字节；仅支持 {{mission_id}} 和 {{task_id}} 占位符。');
      return;
    }
    const pending = createRequest.current?.title === trimmedTitle && createRequest.current.goal === trimmedGoal
      && createRequest.current.criteria === acceptanceCriteria && createRequest.current.toolCallLimit === toolCallLimit
      ? createRequest.current
      : {title: trimmedTitle, goal: trimmedGoal, criteria: acceptanceCriteria, toolCallLimit, requestId: crypto.randomUUID()};
    createRequest.current = pending;
    setPhase('submitting');
    setMessage(null);
    try {
      await createMutation.mutateAsync({title: pending.title, goal: pending.goal, acceptanceContract, protocolToolCallLimit: pending.toolCallLimit, requestId: pending.requestId});
      createRequest.current = null;
      setPhase('accepted');
      setMessage('使命目标已受理，正在重新读取权威快照。');
    } catch (error) {
      setFailure(error);
    }
  };
  const handleStart = async () => {
    if (mission.missionId === 'unavailable') return;
    const requestId = startRequestID.current ?? crypto.randomUUID();
    startRequestID.current = requestId;
    setPhase('submitting');
    setMessage(null);
    try {
      await startMutation.mutateAsync({missionId: mission.missionId, requestId});
      startRequestID.current = null;
      setPhase('accepted');
      setMessage('启动命令已受理，员工会话状态将由后端刷新。');
    } catch (error) {
      setFailure(error);
    }
  };
  const handleCancel = async () => {
    if (mission.missionId === 'unavailable') return;
    const requestId = cancelRequestID.current ?? crypto.randomUUID();
    cancelRequestID.current = requestId;
    setPhase('submitting');
    setMessage(null);
    try {
      await cancelMutation.mutateAsync({missionId: mission.missionId, requestId});
      cancelRequestID.current = null;
      setPhase('accepted');
      setMessage('停止命令已受理，正在确认停止证明和权威终态。');
    } catch (error) {
      setFailure(error);
    }
  };

  const canCreate = mission.missionId === 'unavailable';
  const canStart = mission.missionId !== 'unavailable' && mission.state === 'draft';
  const canCancel = mission.missionId !== 'unavailable' && (mission.state === 'active' || mission.state === 'paused');
  return <section className={styles.missionControls} aria-label="使命操作">
    <div className={styles.subsectionHeader}><span>人工操作</span><code>{missionStateLabel(mission.state)}</code></div>
    {canCreate ? <form className={styles.formStack} onSubmit={handleCreate}>
      <label className={styles.formLabel}>使命标题<input data-command-field="mission-title" className={styles.formField} value={title} onChange={event => setTitle(event.target.value)} placeholder="例如：完成本期目标" /></label>
      <label className={styles.formLabel}>目标正文<textarea data-command-field="mission-goal" className={styles.formField} rows={3} value={goal} onChange={event => setGoal(event.target.value)} placeholder="描述目标与可检查结果" /></label>
      <label className={styles.formLabel}>使命工具调用上限<input data-command-field="mission-tool-call-limit" className={styles.formField} inputMode="numeric" min="1" required step="1" type="number" value={protocolToolCallLimit} onChange={event => setProtocolToolCallLimit(event.target.value)} /></label>
      <p className={styles.formHint}>按 Kernel 接纳的协议工具调用计数，并与 WorkerSession、Task、ProblemKey 额度共同约束。它不代表 Token 或金额；CLI / 服务内部隐藏重试目前无法完整计数。</p>
      <label className={styles.formLabel}>公开验收条件（可选，每行一条）<textarea data-command-field="mission-acceptance" className={styles.formField} rows={4} value={acceptanceCriteria} onChange={event => setAcceptanceCriteria(event.target.value)} placeholder={'使命 ID：{{mission_id}}\n任务 ID：{{task_id}}\n确认说明：\n任务摘要：'} aria-describedby="mission-acceptance-help" /></label>
      <p className={styles.formHint} id="mission-acceptance-help">条件会随使命显示，并在启动时冻结到任务。无条件的任务只允许探索，不能通过正式验收提交产物。</p>
      <button className={styles.commandButton} data-command="mission-create" disabled={isSubmitting} type="submit"><Play aria-hidden="true" size={14} />{isSubmitting ? '提交中…' : '提交使命目标'}</button>
    </form> : <div className={styles.wizardActions}>
      {canStart ? <button className={styles.commandButton} data-command="mission-start" disabled={isSubmitting} onClick={() => void handleStart()} type="button"><Play aria-hidden="true" size={14} />{isSubmitting ? '启动中…' : '启动使命'}</button> : null}
      {canCancel ? <button className={styles.textButton} data-command="mission-cancel" disabled={isSubmitting} onClick={() => void handleCancel()} type="button"><Square aria-hidden="true" size={14} />{isSubmitting ? '停止中…' : '停止使命'}</button> : null}
      {!canStart && !canCancel && mission.state === 'closing' ? <StatusBadge label="收尾中：正在隔离旧工作并核对责任" tone="warning" /> : null}
      {!canStart && !canCancel && mission.state === 'cancelled' ? <StatusBadge label="已取消" tone="neutral" /> : null}
      {!canStart && !canCancel && mission.state === 'succeeded' ? <StatusBadge label="已完成" tone="success" /> : null}
      {!canStart && !canCancel && mission.state === 'ended_not_met' ? <StatusBadge label="未达成而结束" tone="danger" /> : null}
    </div>}
    {phase !== 'idle' ? <p className={phase === 'conflict' || phase === 'error' ? styles.errorText : styles.formHint} data-command-phase={phase} role={phase === 'conflict' || phase === 'error' ? 'alert' : 'status'}>{message}</p> : null}
  </section>;
}

function EpochCard({overview}: Readonly<{overview: CompanyOverviewView}>) {
  return (
    <article className={`${styles.sectionCard} ${styles.epochCard}`}>
      <div className={styles.sectionHeader}>
        <div><span className={styles.cardEyebrow}>当前运行周期</span><h2 className={styles.sectionTitle}>数据快照</h2></div>
        <span className={styles.epochState}>{labelDisplayValue(overview.meta.freshness)}</span>
      </div>
      <div className={styles.epochRows}>
        <div className={styles.infoRow}><span>快照游标</span><code title={overview.meta.snapshotCursor}>{formatEntityId(overview.meta.snapshotCursor)}</code></div>
        <div className={styles.infoRow}><span>实体版本</span><code>{overview.meta.entityRevision}</code></div>
        <div className={styles.infoRow}><span>观测时间</span><code>{optionalTime(overview.meta.observedAt)}</code></div>
        <div className={styles.infoRow}><span>数据来源</span><span>{overview.meta.sourceLabel}</span></div>
      </div>
    </article>
  );
}

function EmployeeCard({employee}: Readonly<{employee: EmployeeSummary}>) {
  const Icon = statusIcon(employee.status.primary);
  return (
    <article className={styles.employeeCard} data-employee-id={employee.employeeId} data-employee-state={employee.status.primary}>
      <div className={styles.employeeHeader}>
        <div><p className={styles.cardEyebrow}>{labelRole(employee.role)}</p><h3 className={styles.employeeName}>{employee.displayName}</h3></div>
        <StatusBadge label={statusLabel(employee.status.primary)} tone={statusTone(employee.status.tone)} />
      </div>
      <div className={styles.identityRow}>
        <span className={styles.identityIcon}><UsersRound aria-hidden="true" size={14} /></span>
        <span><span className={styles.identityLabel}>员工逻辑身份</span><code className={styles.identityId}>{employee.employeeId}</code></span>
      </div>
      <div className={styles.sessionBlock}>
        <div className={styles.subsectionHeader}><span>WorkerSession</span><code>{employee.sessionId ? formatEntityId(employee.sessionId) : '未启动'}</code></div>
        <div className={styles.sessionGrid}>
          <div><span className={styles.fieldLabel}>模型配置</span><span className={styles.fieldValue}>{employee.profile ?? '不可得'}</span></div>
          <div><span className={styles.fieldLabel}>会话周期</span><span className={styles.fieldValue}>{employee.epoch}</span></div>
          <div><span className={styles.fieldLabel}>进行中的请求</span><span className={styles.fieldValue}>{employee.status.activeModelRequests}</span></div>
          <div><span className={styles.fieldLabel}>执行中的工具</span><span className={styles.fieldValue}>{employee.status.inFlightTools}</span></div>
        </div>
      </div>
      <div className={styles.taskRow}>
        <span className={styles.taskGlyph}><Icon aria-hidden="true" size={14} /></span>
        <span className={styles.taskContent}>
          <span className={styles.fieldLabel}>当前任务 · {employee.currentTask?.id ?? '无'}</span>
          <span className={styles.taskTitle}>{employee.currentTask?.label ?? '当前无任务占用'}</span>
          <span className={styles.taskState}>{employee.currentTask ? '已绑定当前作用域' : '当前没有进行中的任务'}</span>
        </span>
      </div>
      <div className={styles.scheduleProjection} data-testid="employee-schedule-projection">
        <div><span className={styles.fieldLabel}>持久调度</span>{employee.schedule === null ? <span className={styles.fieldValue}>不可得</span> : <StatusBadge label={employeeScheduleLabel(employee.schedule.state)} tone={employeeScheduleTone(employee.schedule.state)} />}</div>
        <div><span className={styles.fieldLabel}>工作代次</span><code>{employee.schedule === null ? '不可得' : `${employee.schedule.workGeneration} / ${employee.schedule.checkedGeneration}`}</code></div>
        <div><span className={styles.fieldLabel}>下次到期</span><span className={styles.fieldValue}>{employee.schedule === null ? '不可得' : employee.schedule.nextDueAt === null ? '未安排' : optionalTime(employee.schedule.nextDueAt)}</span></div>
        {employee.schedule?.pauseReason ? <div><span className={styles.fieldLabel}>暂停原因</span><span className={styles.fieldValue}>{labelDisplayValue(employee.schedule.pauseReason)}</span></div> : null}
      </div>
      <div className={styles.employeeFooter}>
        <div><span className={styles.fieldLabel}>资格状态</span><StatusBadge label={labelDisplayValue(employee.qualification.status)} tone={qualificationTone(employee.qualification.status)} /><code className={styles.handoverDetail}>{employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '暂无证据'}</code></div>
        <div className={styles.handoverInfo}><span className={styles.fieldLabel}>交接 / 接班人</span><span className={styles.handoverValue}>{employee.sessionState === 'stopped' ? '可继续查看' : '暂无接班人'}</span></div>
      </div>
    </article>
  );
}

function EvidenceBoundary({overview, companyId}: Readonly<{overview: CompanyOverviewView; companyId: string}>) {
  const obligation = overview.obligations[0] ?? null;
  const artifact = overview.artifacts[0] ?? null;
  const task = overview.tasks.find(item => item.taskId === artifact?.taskId) ?? overview.tasks[0] ?? null;
  const checkpoint = task === null ? null : selectCheckpointForTask(overview.checkpoints, task.taskId, artifact);
  const checkpointId = checkpoint?.checkpointId ?? artifact?.checkpointId ?? null;
  return (
    <article className={`${styles.sectionCard} ${styles.evidenceCard}`}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>证据边界</span><h2 className={styles.sectionTitle}>检查点、产物与验收</h2></div><BadgeCheck aria-hidden="true" size={18} /></div>
      <div className={styles.boundaryStack}>
        <div className={styles.boundaryItem} data-artifact-id={artifact?.artifactId ?? undefined} data-checkpoint-id={checkpointId ?? undefined} data-checkpoint-state={checkpoint?.qualificationState ?? 'unavailable'}><span className={`${styles.boundaryIndex} ${styles.toneBlue}`}>01</span><div><strong>工作区检查点</strong><p>{checkpointId ? `${formatEntityId(checkpointId)} · ${checkpoint?.kind ?? 'qualified'} / ${checkpoint?.qualificationState ? labelDisplayValue(checkpoint.qualificationState) : '未验证'}` : '暂无检查点'} · 当前员工状态记录</p></div><StatusBadge label={checkpoint ? '已观察' : '未知'} tone="info" /></div>
        <div className={styles.boundaryItem}><span className={`${styles.boundaryIndex} ${styles.toneBlue}`}>02</span><div><strong>产物 / 证据</strong><p>{artifact ? formatEntityId(artifact.artifactId) : '暂无产物'} · 结论 {artifact?.verdict ? labelDisplayValue(artifact.verdict) : '不可得'}</p></div><StatusBadge label={artifact?.verdict ? labelDisplayValue(artifact.verdict) : '未知'} tone={artifact?.verdict === 'passed' ? 'success' : 'info'} /></div>
        <div className={styles.boundaryItem}><span className={`${styles.boundaryIndex} ${styles.toneGreen}`}>03</span><div><strong>最终验收</strong><p>{task?.acceptance === 'passed' ? '独立验收已通过' : '仍需验收'}</p></div><StatusBadge label={task?.acceptance === 'passed' ? '已通过' : '待处理'} tone={task?.acceptance === 'passed' ? 'success' : 'warning'} /></div>
      </div>
      <div className={styles.boundaryFoot}><MessageSquare aria-hidden="true" size={14} /><span>{obligation ? `责任 ${formatEntityId(obligation.obligationId)} · ${labelDisplayValue(obligation.state)}` : '当前没有责任记录'}</span><Link aria-label="打开活动时间线" to={`/companies/${companyId}/activity`}><Link2 aria-hidden="true" size={14} /></Link></div>
    </article>
  );
}

function AttentionCard({overview}: Readonly<{overview: CompanyOverviewView}>) {
  return (
    <article className={`${styles.sectionCard} ${styles.decisionCard}`}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>需要人决定</span><h2 className={styles.sectionTitle}>待处理事项</h2></div><StatusBadge label={`${overview.attention.length} 项`} tone={overview.attention.length > 0 ? 'warning' : 'success'} /></div>
      {overview.attention.length > 0 ? <div className={styles.decisionList}>{overview.attention.map(item => <div className={styles.decisionItem} key={item.id}><div><strong>{item.title}</strong><p>{item.description}</p></div><StatusBadge label={labelDisplayValue(item.tone)} tone={item.tone === 'danger' ? 'danger' : item.tone} /></div>)}</div> : <div className={styles.emptyState}><span>当前没有待处理事项。</span></div>}
    </article>
  );
}

function LoadingState() {
  return <div className={styles.emptyState} role="status"><Radio aria-hidden="true" className={styles.loadingIcon} size={18} /><span>正在读取公司快照</span></div>;
}

function ErrorState({message}: Readonly<{message: string}>) {
  return <div className={styles.errorState} role="alert"><CircleAlert aria-hidden="true" size={18} /><div><strong>公司快照读取失败</strong><p>{message}</p></div></div>;
}

export function CompanyOverviewPage({api, companyId}: CompanyOverviewPageProps) {
  const query = useCompanyOverview(api, companyId);
  if (query.isPending) return <div className={styles.viewStack}><ViewHeader /><LoadingState /></div>;
  if (query.isError) return <div className={styles.viewStack}><ViewHeader /><ErrorState message={query.error.message} /></div>;
  const overview = query.data;
  return (
    <div className={styles.viewStack} data-od-id="company-overview-view">
      <ViewHeader />
      <SnapshotStrip companyId={companyId} overview={overview} />
      <section className={styles.metricGrid}>
        <MetricCard label="员工" value={overview.team.total} detail={`${overview.team.working} 个工作中 · 逻辑身份`} />
        <MetricCard label="当前任务" value={String(overview.tasks.length)} detail={taskStateSummary(overview)} />
        <MetricCard label="开放义务" value={String(openObligationCount(overview))} />
        <MetricCard label="待验收" value={String(pendingAcceptanceCount(overview))} />
      </section>
      <section className={styles.panelGrid}>
        <MissionCard api={api} companyId={companyId} overview={overview} />
        <EpochCard overview={overview} />
      </section>
      {overview.mission.missionId !== 'unavailable' ? <DailyRoutinePanel api={api} companyId={companyId} mission={overview.mission} employees={overview.employees} /> : null}
      <section className={styles.sectionBlock}>
        <div className={styles.sectionBlockHeader}><div><p className={styles.sectionKicker}>人员 / 执行</p><h2 className={styles.sectionTitle}>员工与 WorkerSession</h2></div></div>
        <div className={styles.employeeGrid}>{overview.employees.map(employee => <EmployeeCard employee={employee} key={employee.employeeId} />)}</div>
      </section>
      <section className={styles.bottomGrid}>
        <EvidenceBoundary companyId={companyId} overview={overview} />
        <AttentionCard overview={overview} />
      </section>
    </div>
  );
}
