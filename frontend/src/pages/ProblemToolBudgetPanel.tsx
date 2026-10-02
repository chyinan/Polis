import {useState} from 'react';
import type {ReactElement} from 'react';
import {useAllocateProblemToolCalls, useProblemToolCallBudgets, useSetProblemToolCallClosingReserve} from '../data/workbench-query';
import type {WorkbenchApi} from '../data/workbench-api';
import type {ProblemToolCallBudgetView} from '../domain/workbench';
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

export function ProblemToolBudgetPanel({api, companyId}: Props): ReactElement | null {
  const query = useProblemToolCallBudgets(api, companyId);
  if (api.mode !== 'real') return null;
  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>预算审计</span><h2 className={styles.sectionTitle}>问题级工具调用额度</h2></div>
      <StatusBadge label={query.data ? `${query.data.items.length}${query.data.truncated ? '+' : ''} 个问题` : '读取中'} tone="info" />
    </div>
    <p className={styles.formHint}>额度按 ProblemKey 在多个任务间共享。追加额度与关闭预留都需要本地操作者确认并保留不可撤回的版本记录；已存在 Task 的固定本地上限不会被追加命令重置。关闭预留只保护 Kernel 创建的 review 与 peer_review Task，可在首个 Worker 前预先配置。此处统计工具调用，不代表 Token、金额或隐藏重试成本。</p>
    {query.isPending ? <p className={styles.formHint}>正在读取 ProblemKey 额度…</p> : null}
    {query.isError ? <p className={styles.errorText} role="alert">读取额度失败：{query.error.message}</p> : null}
    {query.data?.items.length === 0 ? <p className={styles.formHint}>当前没有可显示的 ProblemKey 预算。</p> : null}
    {query.data?.items.map(item => <ProblemBudgetRow api={api} budget={item} companyId={companyId} key={item.problemKey} />)}
    {query.data?.truncated ? <p className={styles.formHint}>当前最多显示 100 个 ProblemKey。</p> : null}
  </section>;
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
    </div>
    <StatusBadge label={stateLabel(budget.state)} tone={stateTone(budget.state)} />
  </article>;
}
