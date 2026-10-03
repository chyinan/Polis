import {useState} from 'react';
import type {ReactElement} from 'react';
import {useAllocateProblemToolCalls, useAllocateTaskToolCalls, useChangeMissionToolCallBudget, useCloseTaskToolBudgetIncomplete, useMissionToolCallBudgets, useProblemToolCallBudgets, useSetMissionToolCallClosingReserve, useSetProblemToolCallClosingReserve} from '../data/workbench-query';
import type {WorkbenchApi} from '../data/workbench-api';
import type {MissionToolCallBudgetView, ProblemToolCallBudgetView, TaskToolCallBudgetView} from '../domain/workbench';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type Props = Readonly<{api: WorkbenchApi; companyId: string}>;

function stateLabel(state: ProblemToolCallBudgetView['state']): string {
  switch (state) {
    case 'pending': return '等待首次分配';
    case 'unbounded': return '无限额';
    case 'available': return '可用';
    case 'closing_reserved': return '仅关闭类可用';
    case 'exhausted': return '已耗尽';
  }
}

function stateTone(state: ProblemToolCallBudgetView['state']): 'success' | 'warning' | 'danger' | 'neutral' {
  if (state === 'available') return 'success';
  if (state === 'closing_reserved') return 'warning';
  if (state === 'exhausted') return 'danger';
  if (state === 'pending') return 'warning';
  return 'neutral';
}

function rejectionRouteLabel(route: NonNullable<ProblemToolCallBudgetView['lastRejectionRoute']>): string {
  return route === 'worker_admission' ? 'Worker admission' : '工具调用';
}

function rejectionReasonLabel(reason: NonNullable<ProblemToolCallBudgetView['lastRejectionReason']>): string {
  switch (reason) {
    case 'session_limit': return 'WorkerSession 额度已耗尽';
    case 'task_limit': return 'Task 额度已耗尽';
    case 'problem_limit': return 'ProblemKey 额度已耗尽';
    case 'closing_reserve': return '剩余额度受关闭预留保护';
    case 'initial_closing_reserve': return '首个额度不能覆盖关闭预留';
  }
}

function taskRejectionReasonLabel(reason: NonNullable<TaskToolCallBudgetView['lastRejectionReason']>): string {
  switch (reason) {
    case 'session_limit': return 'WorkerSession 额度已耗尽';
    case 'task_limit': return 'Task 额度已耗尽';
    case 'problem_limit': return 'ProblemKey 额度已耗尽';
    case 'closing_reserve': return '剩余额度受关闭预留保护';
    case 'initial_closing_reserve': return '首个额度不能覆盖关闭预留';
  }
}

export function ProblemToolBudgetPanel({api, companyId}: Props): ReactElement | null {
  const query = useProblemToolCallBudgets(api, companyId);
  const missions = useMissionToolCallBudgets(api, companyId);
  if (api.mode !== 'real') return null;
  return <div className={styles.viewStack}>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div><span className={styles.cardEyebrow}>预算审计</span><h2 className={styles.sectionTitle}>使命工具调用额度</h2></div>
        <StatusBadge label={missions.data ? `${missions.data.items.length}${missions.data.truncated ? '+' : ''} 个使命` : '读取中'} tone="info" />
      </div>
      <p className={styles.formHint}>每次协议工具调用会在同一事务中计入 Mission、WorkerSession、Task 与 ProblemKey。旧使命会从持久 Task 计数回填已用量，但不会从下级默认额度推算 Mission 上限；本地操作者需为待配置使命明确设置有限上限。隐藏重试、Token 和金额不在此计数范围内。</p>
      {missions.isPending ? <p className={styles.formHint}>正在读取 Mission 额度…</p> : null}
      {missions.isError ? <p className={styles.errorText} role="alert">读取 Mission 额度失败：{missions.error.message}</p> : null}
      {missions.data?.items.length === 0 ? <p className={styles.formHint}>当前没有可显示的 Mission 预算。</p> : null}
      {missions.data?.items.map(item => <MissionBudgetRow api={api} budget={item} companyId={companyId} key={item.missionId} />)}
      {missions.data?.truncated ? <p className={styles.formHint}>当前最多显示 100 个 Mission。</p> : null}
    </section>
    <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>预算审计</span><h2 className={styles.sectionTitle}>问题级工具调用额度</h2></div>
      <StatusBadge label={query.data ? `${query.data.items.length}${query.data.truncated ? '+' : ''} 个问题` : '读取中'} tone="info" />
    </div>
    <p className={styles.formHint}>额度按 ProblemKey 在多个任务间共享。ProblemKey 与单个 Task 的追加额度都需要本地操作者确认并保留不可撤回的版本记录。提高 Task 上限仍受共享 ProblemKey 额度约束；普通 Task 还会保留关闭类预留。关闭预留只保护 Kernel 创建的 review 与 peer_review Task，可在首个 Worker 前预先配置。此处统计工具调用，不代表 Token、金额或隐藏重试成本。</p>
    {query.isPending ? <p className={styles.formHint}>正在读取 ProblemKey 额度…</p> : null}
    {query.isError ? <p className={styles.errorText} role="alert">读取额度失败：{query.error.message}</p> : null}
    {query.data?.items.length === 0 ? <p className={styles.formHint}>当前没有可显示的 ProblemKey 预算。</p> : null}
    {query.data?.items.map(item => <ProblemBudgetRow api={api} budget={item} companyId={companyId} key={item.problemKey} />)}
    {query.data?.truncated ? <p className={styles.formHint}>当前最多显示 100 个 ProblemKey。</p> : null}
    </section>
  </div>;
}

function missionBudgetStateLabel(state: MissionToolCallBudgetView['state']): string {
  if (state === 'pending') return '待配置上限';
  if (state === 'exhausted') return '已耗尽';
  if (state === 'closing_reserved') return '额度受关闭预留保护';
  return '可用';
}

function missionRejectionLabel(reason: NonNullable<MissionToolCallBudgetView['lastRejectionReason']>): string {
  if (reason === 'mission_budget_pending') return '使命尚未配置上限';
  if (reason === 'mission_closing_reserve') return '剩余额度受关闭预留保护';
  return '使命额度已耗尽';
}

function MissionBudgetRow({api, budget, companyId}: Readonly<{api: WorkbenchApi; budget: MissionToolCallBudgetView; companyId: string}>): ReactElement {
  const change = useChangeMissionToolCallBudget(api, companyId);
  const reserveMutation = useSetMissionToolCallClosingReserve(api, companyId);
  const [resultingLimit, setResultingLimit] = useState('');
  const [reason, setReason] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [reserveValue, setReserveValue] = useState('');
  const [reserveReason, setReserveReason] = useState('');
  const [reserveConfirmed, setReserveConfirmed] = useState(false);
  const proposed = Number(resultingLimit);
  const canSubmit = confirmed && Number.isSafeInteger(proposed) && proposed > 0
    && (budget.toolCallLimit === null || proposed > budget.toolCallLimit)
    && reason.trim().length > 0 && Array.from(reason.trim()).length <= 500 && !change.isPending;
  const reservedCalls = Number(reserveValue);
  const canSetReserve = reserveValue.trim() !== '' && Number.isSafeInteger(reservedCalls) && reservedCalls >= 0
    && (budget.toolCallLimit === null || reservedCalls <= budget.toolCallsRemaining)
    && reserveConfirmed && reserveReason.trim().length > 0 && Array.from(reserveReason.trim()).length <= 500
    && !reserveMutation.isPending;

  async function submit(): Promise<void> {
    if (!canSubmit) return;
    await change.mutateAsync({
      missionId: budget.missionId,
      expectedToolCallLimit: budget.toolCallLimit,
      expectedRevision: budget.revision,
      resultingToolCallLimit: proposed,
      reason,
      requestId: `mission-budget-${crypto.randomUUID()}`,
    });
    setResultingLimit('');
    setReason('');
    setConfirmed(false);
  }

  async function submitReserve(): Promise<void> {
    if (!canSetReserve) return;
    await reserveMutation.mutateAsync({
      missionId: budget.missionId,
      reservedToolCalls: reservedCalls,
      expectedMissionToolCallLimit: budget.toolCallLimit,
      expectedMissionBudgetRevision: budget.revision,
      expectedReserveRevision: budget.closingReserveRevision,
      reason: reserveReason,
      requestId: `mission-reserve-${crypto.randomUUID()}`,
    });
    setReserveValue('');
    setReserveReason('');
    setReserveConfirmed(false);
  }

  const tone = budget.state === 'available' ? 'success' : budget.state === 'exhausted' ? 'danger' : 'warning';
  return <article className={styles.boundaryItem}>
    <div className={styles.viewStack}>
      <div><strong>{budget.title}</strong><p>Mission <code>{budget.missionId}</code> · 状态 {budget.missionState}</p></div>
      <p>已用 {budget.toolCallsUsed.toLocaleString('zh-CN')} / 上限 {budget.toolCallLimit === null ? '待配置' : budget.toolCallLimit.toLocaleString('zh-CN')} · 剩余 {budget.toolCallsRemaining.toLocaleString('zh-CN')} · 额度版本 {budget.revision}</p>
      {budget.lastAllocatedAt ? <p className={styles.formHint}>最近配置或追加：{budget.lastAllocatedAt} · {budget.lastReason}</p> : null}
      {budget.closingReserveRevision > 0
        ? <p className={styles.formHint}>关闭类预留：保护 {budget.closingReserveRemaining.toLocaleString('zh-CN')} 次（本策略设为 {budget.closingReserveToolCalls.toLocaleString('zh-CN')}）· 版本 {budget.closingReserveRevision} · {budget.lastClosingReserveAt} · {budget.lastClosingReserveReason}</p>
        : <p className={styles.formHint}>Mission 尚未设置关闭类预留；当前按 0 次保护。</p>}
      {budget.rejectionCount > 0 && budget.lastRejectionAt && budget.lastRejectionRoute && budget.lastRejectionReason && budget.lastRejectionTaskId
        ? <p className={styles.formHint}>拒绝 {budget.rejectionCount.toLocaleString('zh-CN')} 次 · 最近：{budget.lastRejectionAt} · {budget.lastRejectionRoute === 'worker_admission' ? 'Worker 准入' : '工具调用'} · {missionRejectionLabel(budget.lastRejectionReason)} · Task <code>{budget.lastRejectionTaskId}</code></p>
        : <p className={styles.formHint}>尚无 Mission 额度拒绝记录。</p>}
      <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submit().catch(() => undefined); }}>
        <label className={styles.formLabel}>{budget.toolCallLimit === null ? '设置使命工具调用总上限' : '提高使命工具调用总上限'}<input className={styles.formField} inputMode="numeric" min={budget.toolCallLimit === null ? Math.max(1, budget.toolCallsUsed) : budget.toolCallLimit + 1} step="1" type="number" value={resultingLimit} onChange={event => setResultingLimit(event.target.value)} /></label>
        <label className={styles.formLabel}>授权理由<textarea className={styles.formField} maxLength={500} rows={2} value={reason} onChange={event => setReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={confirmed} onChange={event => setConfirmed(event.target.checked)} type="checkbox" /> 我确认设置或追加此使命的总工具调用额度；授权记录不可撤回</label>
        <button className={styles.commandButton} disabled={!canSubmit} type="submit">{change.isPending ? '正在记录…' : budget.toolCallLimit === null ? '配置 Mission 上限' : '追加 Mission 额度'}</button>
        {change.isError ? <p className={styles.errorText} role="alert">Mission 额度变更失败：{change.error.message}；如版本已变化，请刷新后重新确认。</p> : null}
      </form>
      <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submitReserve().catch(() => undefined); }}>
        <label className={styles.formLabel}>保护关闭类任务的剩余 Mission 调用数<input className={styles.formField} inputMode="numeric" min="0" max={budget.toolCallLimit === null ? undefined : budget.toolCallsRemaining} step="1" type="number" value={reserveValue} onChange={event => setReserveValue(event.target.value)} /></label>
        <label className={styles.formLabel}>关闭预留策略理由<textarea className={styles.formField} maxLength={500} rows={2} value={reserveReason} onChange={event => setReserveReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={reserveConfirmed} onChange={event => setReserveConfirmed(event.target.checked)} type="checkbox" /> 我确认此预留只供 Kernel 创建的 review 与 peer_review Mission 任务使用</label>
        <button className={styles.commandButton} disabled={!canSetReserve} type="submit">{reserveMutation.isPending ? '正在记录…' : '更新 Mission 关闭预留'}</button>
        {reserveMutation.isError ? <p className={styles.errorText} role="alert">Mission 关闭预留更新失败：{reserveMutation.error.message}；如额度或版本已变化，请刷新后重新确认。</p> : null}
      </form>
    </div>
    <StatusBadge label={missionBudgetStateLabel(budget.state)} tone={tone} />
  </article>;
}

function ProblemBudgetRow({api, budget, companyId}: Readonly<{api: WorkbenchApi; budget: ProblemToolCallBudgetView; companyId: string}>): ReactElement {
  const allocation = useAllocateProblemToolCalls(api, companyId);
  const reserveMutation = useSetProblemToolCallClosingReserve(api, companyId);
  const [additional, setAdditional] = useState('');
  const [reason, setReason] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [reserveValue, setReserveValue] = useState('');
  const [reserveReason, setReserveReason] = useState('');
  const [reserveConfirmed, setReserveConfirmed] = useState(false);
  const finite = budget.toolCallLimit !== null && budget.toolCallLimit > 0;
  const reserveConfigurable = budget.state === 'pending' || finite;
  const reserveMaximum = budget.state === 'pending' ? Number.MAX_SAFE_INTEGER : budget.toolCallsRemaining;
  const additionalCalls = Number(additional);
  const canSubmit = finite && confirmed && Number.isSafeInteger(additionalCalls) && additionalCalls > 0
    && reason.trim().length > 0 && !allocation.isPending;
  const reservedCalls = Number(reserveValue);
  const canSubmitReserve = reserveConfigurable && reserveConfirmed && reserveValue.trim() !== ''
    && Number.isSafeInteger(reservedCalls) && reservedCalls >= 0 && reservedCalls <= reserveMaximum
    && reserveReason.trim().length > 0 && !reserveMutation.isPending;

  async function submit(): Promise<void> {
    if (!canSubmit || budget.toolCallLimit === null) return;
    await allocation.mutateAsync({
      problemKey: budget.problemKey,
      additionalToolCalls: additionalCalls,
      expectedToolCallLimit: budget.toolCallLimit,
      expectedRevision: budget.allocationRevision,
      reason,
      requestId: `budget-${crypto.randomUUID()}`,
    });
    setAdditional('');
    setReason('');
    setConfirmed(false);
  }

  async function submitReserve(): Promise<void> {
    if (!canSubmitReserve) return;
    await reserveMutation.mutateAsync({
      problemKey: budget.problemKey,
      reservedToolCalls: reservedCalls,
      expectedToolCallLimit: budget.toolCallLimit ?? 0,
      expectedBudgetRevision: budget.allocationRevision,
      expectedReserveRevision: budget.closingReserveRevision,
      reason: reserveReason,
      requestId: `reserve-${crypto.randomUUID()}`,
    });
    setReserveValue('');
    setReserveReason('');
    setReserveConfirmed(false);
  }

  const limitLabel = budget.toolCallLimit === null ? '尚未定额' : budget.toolCallLimit === 0 ? '无上限' : budget.toolCallLimit.toLocaleString('zh-CN');
  const remainingLabel = budget.toolCallsRemaining < 0 ? '无上限' : budget.toolCallsRemaining.toLocaleString('zh-CN');
  return <article className={styles.boundaryItem}>
    <div className={styles.viewStack}>
      <div><strong><code>{budget.problemKey}</code></strong><p>Mission {budget.missionId} · {budget.taskCount} 个任务 · {budget.workerSessionAttempts} 次会话</p></div>
      <p>已用 {budget.toolCallsUsed.toLocaleString('zh-CN')} / 上限 {limitLabel} · 剩余 {remainingLabel} · 额度版本 {budget.allocationRevision}</p>
      {budget.lastAllocatedAt ? <p className={styles.formHint}>最近追加：{budget.lastAllocatedAt} · {budget.lastAllocationReason}</p> : null}
      {budget.closingReserveRevision > 0 ? <p className={styles.formHint}>关闭类预留：当前保护 {budget.closingReserveRemaining.toLocaleString('zh-CN')} 次（本次策略设为 {budget.closingReserveToolCalls.toLocaleString('zh-CN')}）· 版本 {budget.closingReserveRevision} · {budget.lastClosingReserveAt} · {budget.lastClosingReserveReason}</p> : <p className={styles.formHint}>尚未设置关闭类预留；当前按 0 次保护。</p>}
      {budget.budgetRejectionCount > 0 && budget.lastRejectionRoute && budget.lastRejectionReason && budget.lastRejectionAt && budget.lastRejectionTaskId ? <p className={styles.formHint}>预算拒绝 {budget.budgetRejectionCount.toLocaleString('zh-CN')} 次 · 最近：{budget.lastRejectionAt} · {rejectionRouteLabel(budget.lastRejectionRoute)} · {rejectionReasonLabel(budget.lastRejectionReason)} · Task <code>{budget.lastRejectionTaskId}</code></p> : <p className={styles.formHint}>尚无预算拒绝记录。</p>}
      {finite ? <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submit().catch(() => undefined); }}>
        <label className={styles.formLabel}>追加调用次数<input className={styles.formField} inputMode="numeric" min="1" step="1" type="number" value={additional} onChange={event => setAdditional(event.target.value)} /></label>
        <label className={styles.formLabel}>授权理由<textarea className={styles.formField} maxLength={500} rows={2} value={reason} onChange={event => setReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={confirmed} onChange={event => setConfirmed(event.target.checked)} type="checkbox" /> 我确认追加的工具调用额度并理解此记录不可撤回</label>
        <button className={styles.commandButton} disabled={!canSubmit} type="submit">{allocation.isPending ? '正在记录…' : '追加 ProblemKey 额度'}</button>
        {allocation.isError ? <p className={styles.errorText} role="alert">追加失败：{allocation.error.message}；如额度版本已变化，请刷新后重新确认。</p> : null}
      </form> : null}
      {reserveConfigurable ? <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submitReserve().catch(() => undefined); }}>
        <label className={styles.formLabel}>保护关闭类任务的剩余调用数<input className={styles.formField} inputMode="numeric" min="0" max={budget.state === 'pending' ? undefined : reserveMaximum} step="1" type="number" value={reserveValue} onChange={event => setReserveValue(event.target.value)} /></label>
        <label className={styles.formLabel}>预留策略理由<textarea className={styles.formField} maxLength={500} rows={2} value={reserveReason} onChange={event => setReserveReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={reserveConfirmed} onChange={event => setReserveConfirmed(event.target.checked)} type="checkbox" /> 我确认此预留仅供 review 与 peer_review Task 使用</label>
        <button className={styles.commandButton} disabled={!canSubmitReserve} type="submit">{reserveMutation.isPending ? '正在记录…' : '更新关闭类预留'}</button>
        {reserveMutation.isError ? <p className={styles.errorText} role="alert">更新预留失败：{reserveMutation.error.message}；如预算版本已变化，请刷新后重新确认。</p> : null}
      </form> : <p className={styles.formHint}>无限额 ProblemKey 不接受关闭类预留。</p>}
      {budget.state === 'pending' && budget.closingReserveRevision > 0 ? <p className={styles.formHint}>当前预留在首个 Worker admission 时生效；其初始 ProblemKey 上限必须是有限额度且不小于预留数。</p> : null}
      {budget.tasks.length > 0 ? <div className={styles.viewStack}>
        <strong>Task 固定上限</strong>
        {budget.tasks.map(task => <TaskBudgetRow api={api} budget={budget} companyId={companyId} key={task.taskId} task={task} />)}
        {budget.tasksTruncated ? <p className={styles.formHint}>当前最多显示此 ProblemKey 下的 20 个 Task。</p> : null}
      </div> : null}
    </div>
    <StatusBadge label={stateLabel(budget.state)} tone={stateTone(budget.state)} />
  </article>;
}

function TaskBudgetRow({api, budget, companyId, task}: Readonly<{api: WorkbenchApi; budget: ProblemToolCallBudgetView; companyId: string; task: TaskToolCallBudgetView}>): ReactElement {
  const allocation = useAllocateTaskToolCalls(api, companyId);
  const closeout = useCloseTaskToolBudgetIncomplete(api, companyId);
  const [additional, setAdditional] = useState('');
  const [reason, setReason] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [closeoutReason, setCloseoutReason] = useState('');
  const [closeoutConfirmed, setCloseoutConfirmed] = useState(false);
  const additionalCalls = Number(additional);
  const closingTask = task.kind === 'review' || task.kind === 'peer_review';
  const problemAvailable = budget.state === 'unbounded' ? Number.MAX_SAFE_INTEGER
    : Math.max(budget.toolCallsRemaining - (closingTask ? 0 : budget.closingReserveRemaining), 0);
  const finiteTask = task.toolCallLimit !== null && task.toolCallLimit > 0;
  const canSubmit = finiteTask && task.allocationEligible && confirmed
    && Number.isSafeInteger(additionalCalls) && additionalCalls > 0 && additionalCalls <= problemAvailable
    && reason.trim().length > 0 && !allocation.isPending;

  async function submit(): Promise<void> {
    if (!canSubmit || task.toolCallLimit === null) return;
    await allocation.mutateAsync({
      problemKey: budget.problemKey,
      taskId: task.taskId,
      additionalToolCalls: additionalCalls,
      expectedToolCallLimit: task.toolCallLimit,
      expectedTaskRevision: task.allocationRevision,
      expectedProblemRevision: budget.allocationRevision,
      expectedReserveRevision: budget.closingReserveRevision,
      reason,
      requestId: `task-budget-${crypto.randomUUID()}`,
    });
    setAdditional('');
    setReason('');
    setConfirmed(false);
  }

  const canCloseIncomplete = task.closureEligible && closeoutConfirmed && closeoutReason.trim().length > 0
    && Array.from(closeoutReason.trim()).length <= 500 && !closeout.isPending;

  async function submitCloseIncomplete(): Promise<void> {
    if (!canCloseIncomplete) return;
    await closeout.mutateAsync({
      problemKey: budget.problemKey,
      taskId: task.taskId,
      expectedTaskToolCallLimit: task.toolCallLimit,
      expectedTaskToolCallsUsed: task.toolCallsUsed,
      expectedTaskRevision: task.allocationRevision,
      expectedProblemToolCallLimit: budget.toolCallLimit,
      expectedProblemToolCallsUsed: budget.toolCallsUsed,
      expectedProblemRevision: budget.allocationRevision,
      expectedClosingReserveToolCalls: budget.closingReserveToolCalls,
      expectedClosingReserveRemaining: budget.closingReserveRemaining,
      expectedReserveRevision: budget.closingReserveRevision,
      reason: closeoutReason,
      requestId: `task-close-${crypto.randomUUID()}`,
    });
    setCloseoutReason('');
    setCloseoutConfirmed(false);
  }

  const limitLabel = task.toolCallLimit === null ? '尚未初始化' : task.toolCallLimit === 0 ? '无上限' : task.toolCallLimit.toLocaleString('zh-CN');
  const remainingLabel = task.toolCallsRemaining < 0 ? '无上限' : task.toolCallsRemaining.toLocaleString('zh-CN');
  return <div className={styles.boundaryItem}>
    <div className={styles.viewStack}>
      <p><strong><code>{task.taskId}</code></strong> · {task.kind} · 已用 {task.toolCallsUsed.toLocaleString('zh-CN')} / 上限 {limitLabel} · 剩余 {remainingLabel} · 额度版本 {task.allocationRevision}</p>
      {task.lastAllocatedAt ? <p className={styles.formHint}>最近追加：{task.lastAllocatedAt} · {task.lastAllocationReason}</p> : null}
      {task.budgetRejectionCount > 0 && task.lastRejectionReason && task.lastRejectionAt ? <p className={styles.formHint}>预算拒绝 {task.budgetRejectionCount.toLocaleString('zh-CN')} 次 · 最近：{task.lastRejectionAt} · {taskRejectionReasonLabel(task.lastRejectionReason)}</p> : null}
      {task.closedIncomplete ? <p className={styles.formHint}>已关闭为未完成 · {task.closedAt} · 理由：{task.closureReason}</p> : null}
      {finiteTask && task.allocationEligible ? <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submit().catch(() => undefined); }}>
        <label className={styles.formLabel}>追加此 Task 的调用次数<input className={styles.formField} inputMode="numeric" min="1" max={problemAvailable} step="1" type="number" value={additional} onChange={event => setAdditional(event.target.value)} /></label>
        <label className={styles.formLabel}>授权理由<textarea className={styles.formField} maxLength={500} rows={2} value={reason} onChange={event => setReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={confirmed} onChange={event => setConfirmed(event.target.checked)} type="checkbox" /> 我确认追加此 Task 的工具调用额度并理解此记录不可撤回</label>
        <button className={styles.commandButton} disabled={!canSubmit} type="submit">{allocation.isPending ? '正在记录…' : '追加 Task 额度'}</button>
        {allocation.isError ? <p className={styles.errorText} role="alert">Task 额度追加失败：{allocation.error.message}；如预算版本已变化，请刷新后重新确认。</p> : null}
        {problemAvailable === 0 ? <p className={styles.formHint}>当前 ProblemKey 没有可供此 Task 使用的额度；请先调整 ProblemKey 额度或关闭预留。</p> : null}
      </form> : <p className={styles.formHint}>{finiteTask && task.toolCallsRemaining > 0 ? '此 Task 的固定额度尚未耗尽；额度耗尽且所有 WorkerSession 停止后，可申请追加。' : finiteTask ? '此 Task 已运行、已关闭或仍有活动 WorkerSession，当前不能追加额度。' : '只有已初始化的有限 Task 上限可以追加。'}</p>}
      {task.closureEligible ? <form className={styles.formStack} onSubmit={event => { event.preventDefault(); void submitCloseIncomplete().catch(() => undefined); }}>
        <label className={styles.formLabel}>关闭为未完成的理由<textarea className={styles.formField} maxLength={500} rows={2} value={closeoutReason} onChange={event => setCloseoutReason(event.target.value)} /></label>
        <label className={styles.formLabel}><input checked={closeoutConfirmed} onChange={event => setCloseoutConfirmed(event.target.checked)} type="checkbox" /> 我确认此 Task 以未完成状态不可撤回地关闭</label>
        <button className={styles.commandButton} disabled={!canCloseIncomplete} type="submit">{closeout.isPending ? '正在记录…' : '关闭为未完成'}</button>
        {closeout.isError ? <p className={styles.errorText} role="alert">未完成关闭失败：{closeout.error.message}；如快照已变化，请刷新后重新确认。</p> : null}
      </form> : null}
    </div>
  </div>;
}
