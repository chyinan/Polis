import {useRef, useState, type ReactElement} from 'react';
import {useCompanyOverview, useMemoryCorrectionQueue, useProposeMemoryCorrection, useReviewMemoryCorrection} from '../data/workbench-query';
import type {WorkbenchApi} from '../data/workbench-api';
import type {EmployeeSummary, MemoryCorrectionQueueItemView} from '../domain/workbench';
import {labelDisplayValue} from '../domain/display-labels';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type Props = Readonly<{api: WorkbenchApi; companyId: string}>;

function queueTone(state: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (state === 'approved') return 'success';
  if (state === 'proposed') return 'warning';
  if (state === 'revoked' || state === 'stale') return 'danger';
  return 'neutral';
}

function activeSessions(employees: ReadonlyArray<EmployeeSummary> | undefined): ReadonlyArray<EmployeeSummary> {
  return (employees ?? []).filter(employee => employee.sessionId !== null && employee.sessionState === 'active');
}

function sessionLabel(employee: EmployeeSummary): string {
  return `${employee.displayName} · ${employee.role} · ${employee.currentTask?.id ?? 'Task 未显示'} · ${employee.sessionId}`;
}

function newCommandID(prefix: string): string {
  return `${prefix}-${crypto.randomUUID()}`;
}

function ProposeCorrectionForm({api, companyId, sessions}: Readonly<{
  api: WorkbenchApi;
  companyId: string;
  sessions: ReadonlyArray<EmployeeSummary>;
}>): ReactElement {
  const mutation = useProposeMemoryCorrection(api, companyId);
  const pendingCommand = useRef<Readonly<{fingerprint: string; correctionId: string; requestId: string}> | null>(null);
  const [workerSessionId, setWorkerSessionId] = useState('');
  const [recordId, setRecordId] = useState('');
  const [baseRevision, setBaseRevision] = useState('');
  const [content, setContent] = useState('');
  const [sourceKind, setSourceKind] = useState<'mission_input' | 'artifact'>('mission_input');
  const [sourceId, setSourceId] = useState('');
  const [sourceRevision, setSourceRevision] = useState('1');
  const [sourceSHA256, setSourceSHA256] = useState('');
  const [reason, setReason] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const [isError, setIsError] = useState(false);

  async function submit(): Promise<void> {
    const trimmedContent = content.trim();
    const trimmedReason = reason.trim();
    const revision = Number(baseRevision);
    const evidenceRevision = Number(sourceRevision);
    if (workerSessionId === '' || !/^[a-zA-Z0-9_-]{1,80}$/.test(recordId) || !Number.isSafeInteger(revision) || revision < 1
      || trimmedContent === '' || new TextEncoder().encode(trimmedContent).length > 32 * 1024
      || !/^[a-zA-Z0-9_-]{1,80}$/.test(sourceId) || !Number.isSafeInteger(evidenceRevision) || evidenceRevision < 1
      || (sourceKind === 'artifact' && evidenceRevision !== 1) || !/^[0-9a-f]{64}$/.test(sourceSHA256)
      || trimmedReason === '' || new TextEncoder().encode(trimmedReason).length > 2048) {
      setIsError(true);
      setMessage('请检查记录 ID、版本号、内容、来源证据和理由；内容最多 32 KiB，理由最多 2048 字节。');
      return;
    }
    setIsError(false);
    setMessage(null);
    const fingerprint = JSON.stringify({workerSessionId, recordId, revision, content: trimmedContent, sourceKind, sourceId, evidenceRevision, sourceSHA256, reason: trimmedReason});
    const pending = pendingCommand.current?.fingerprint === fingerprint
      ? pendingCommand.current
      : {fingerprint, correctionId: newCommandID('memory-correction'), requestId: newCommandID('memory-propose')};
    pendingCommand.current = pending;
    try {
      const receipt = await mutation.mutateAsync({
        workerSessionId, correctionId: pending.correctionId, recordId, baseRevision: revision,
        content: trimmedContent, source: {kind: sourceKind, id: sourceId, revision: evidenceRevision, sha256: sourceSHA256},
        reason: trimmedReason, requestId: pending.requestId,
      });
      pendingCommand.current = null;
      setMessage(`提案 ${receipt.id} 已记录，等待独立 Planning / Review 员工裁定。`);
      setContent('');
      setReason('');
    } catch (error) {
      setIsError(true);
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setMessage(`更正提案结果尚未确认：${detail} 请先核对已刷新的更正队列，再继续。`);
    }
  }

  return <div className={styles.formStack}>
    <p className={styles.formHint}>提案仅接受数据库确认的活动 WorkerSession。服务端从 session 解析员工与 Task，并在写入时复核；这里不会提交 Employee ID。</p>
    {sessions.length === 0 ? <p className={styles.formHint}>公司当前没有活动 session。需要先启动一个活动的 provider WorkerSession，才能提交提案。</p> : <>
      <label className={styles.formLabel}>提案 WorkerSession<select className={styles.formField} value={workerSessionId} onChange={event => setWorkerSessionId(event.target.value)}>
        <option value="">选择活动 session</option>{sessions.map(employee => <option key={employee.sessionId} value={employee.sessionId ?? ''}>{sessionLabel(employee)}</option>)}
      </select></label>
      <label className={styles.formLabel}>记忆记录 ID<input className={styles.formField} value={recordId} onChange={event => setRecordId(event.target.value)} /></label>
      <label className={styles.formLabel}>基准版本<input className={styles.formField} inputMode="numeric" type="number" min="1" step="1" value={baseRevision} onChange={event => setBaseRevision(event.target.value)} /></label>
      <label className={styles.formLabel}>更正内容<textarea className={styles.formField} rows={4} maxLength={32 * 1024} value={content} onChange={event => setContent(event.target.value)} /></label>
      <label className={styles.formLabel}>证据类型<select className={styles.formField} value={sourceKind} onChange={event => setSourceKind(event.target.value as 'mission_input' | 'artifact')}>
        <option value="mission_input">MissionInput</option><option value="artifact">Artifact</option>
      </select></label>
      <label className={styles.formLabel}>证据 ID<input className={styles.formField} value={sourceId} onChange={event => setSourceId(event.target.value)} /></label>
      <label className={styles.formLabel}>证据版本<input className={styles.formField} inputMode="numeric" type="number" min="1" step="1" value={sourceRevision} onChange={event => setSourceRevision(event.target.value)} /></label>
      <label className={styles.formLabel}>证据 SHA-256<input className={styles.formField} value={sourceSHA256} onChange={event => setSourceSHA256(event.target.value)} /></label>
      <label className={styles.formLabel}>提案理由<textarea className={styles.formField} rows={2} maxLength={2048} value={reason} onChange={event => setReason(event.target.value)} /></label>
      <button className={styles.textButton} type="button" disabled={mutation.isPending || workerSessionId === ''} onClick={() => void submit()}>{mutation.isPending ? '提交中…' : '提交更正提案'}</button>
    </>}
    {message !== null ? <p className={isError ? styles.errorText : styles.formHint} role={isError ? 'alert' : 'status'}>{message}</p> : null}
  </div>;
}

function ReviewCorrectionForm({api, companyId, item, reviewers}: Readonly<{
  api: WorkbenchApi;
  companyId: string;
  item: MemoryCorrectionQueueItemView;
  reviewers: ReadonlyArray<EmployeeSummary>;
}>): ReactElement {
  const mutation = useReviewMemoryCorrection(api, companyId);
  const eligibleReviewers = reviewers.filter(employee => employee.employeeId !== item.proposedBy);
  const pendingCommand = useRef<Readonly<{fingerprint: string; requestId: string}> | null>(null);
  const [workerSessionId, setWorkerSessionId] = useState('');
  const [decision, setDecision] = useState<'approved' | 'rejected'>('approved');
  const [reason, setReason] = useState('');
  const [confirmApproval, setConfirmApproval] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [isError, setIsError] = useState(false);

  async function submit(): Promise<void> {
    const trimmedReason = reason.trim();
    if (workerSessionId === '' || trimmedReason === '' || new TextEncoder().encode(trimmedReason).length > 2048 || (decision === 'approved' && !confirmApproval)) {
      setIsError(true);
      setMessage(decision === 'approved' && !confirmApproval ? '请确认批准会更新记忆修订并使依赖该记忆的工作失效。' : '请选择活动审阅 session 并填写不超过 2048 字节的理由。');
      return;
    }
    setIsError(false);
    setMessage(null);
    const fingerprint = JSON.stringify({workerSessionId, correctionId: item.correctionId, decision, reason: trimmedReason});
    const pending = pendingCommand.current?.fingerprint === fingerprint
      ? pendingCommand.current
      : {fingerprint, requestId: newCommandID('memory-review')};
    pendingCommand.current = pending;
    try {
      const receipt = await mutation.mutateAsync({
        workerSessionId, correctionId: item.correctionId, decision, reason: trimmedReason,
        requestId: pending.requestId,
      });
      pendingCommand.current = null;
      setMessage(`提案 ${receipt.id} 已${decision === 'approved' ? '批准' : '拒绝'}。`);
      setReason('');
      setConfirmApproval(false);
    } catch (error) {
      setIsError(true);
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setMessage(`更正裁定结果尚未确认：${detail} 请先核对已刷新的队列与记忆状态，再继续。`);
    }
  }

  return <div className={styles.formStack}>
    <p className={styles.formHint}>裁定只能由固定的 Planning 或 Review 员工通过自己的活动 WorkerSession 作出，并且不得审阅本人提案。服务端会再次验证角色、session 和记忆访问范围。</p>
    {eligibleReviewers.length === 0 ? <p className={styles.formHint}>当前没有可独立审阅此提案的活动 Planning / Review WorkerSession。</p> : <>
      <label className={styles.formLabel}>审阅 WorkerSession<select className={styles.formField} value={workerSessionId} onChange={event => setWorkerSessionId(event.target.value)}>
        <option value="">选择活动审阅 session</option>{eligibleReviewers.map(employee => <option key={employee.sessionId} value={employee.sessionId ?? ''}>{sessionLabel(employee)}</option>)}
      </select></label>
      <label className={styles.formLabel}>裁定<select className={styles.formField} value={decision} onChange={event => { setDecision(event.target.value as 'approved' | 'rejected'); setConfirmApproval(false); }}>
        <option value="approved">批准</option><option value="rejected">拒绝</option>
      </select></label>
      <label className={styles.formLabel}>裁定理由<textarea className={styles.formField} rows={2} maxLength={2048} value={reason} onChange={event => setReason(event.target.value)} /></label>
      {decision === 'approved' ? <label className={styles.reviewCheck}><input type="checkbox" checked={confirmApproval} onChange={event => setConfirmApproval(event.target.checked)} />批准会创建新记忆版本，并使依赖旧版本的任务需要重新验证。</label> : null}
      <button className={styles.textButton} type="button" disabled={mutation.isPending || workerSessionId === ''} onClick={() => void submit()}>{mutation.isPending ? '提交中…' : decision === 'approved' ? '确认批准' : '提交拒绝'}</button>
    </>}
    {message !== null ? <p className={isError ? styles.errorText : styles.formHint} role={isError ? 'alert' : 'status'}>{message}</p> : null}
  </div>;
}

export function MemoryCorrectionQueuePanel({api, companyId}: Props): ReactElement | null {
  const query = useMemoryCorrectionQueue(api, companyId);
  const overview = useCompanyOverview(api, companyId);
  if (api.mode !== 'real') return null;
  const sessions = activeSessions(overview.data?.employees);
  const reviewers = sessions.filter(employee => employee.employeeId === 'emp-planning' || employee.employeeId === 'emp-review');
  return <section className={styles.sectionBlock} data-testid="memory-correction-queue">
    <div className={styles.sectionBlockHeader}>
      <div><p className={styles.sectionKicker}>安全记忆</p><h2 className={styles.sectionTitle}>更正提案队列</h2></div>
      <StatusBadge label={query.data ? `${query.data.items.length}${query.data.truncated ? '+' : ''} 条` : '读取中'} tone={query.data?.items.some(item => item.state === 'proposed') ? 'warning' : 'info'} />
    </div>
    <article className={styles.sectionCard}>
      {query.isPending ? <p className={styles.formHint}>正在读取公司范围内的更正记录…</p> : null}
      {query.isError ? <p className={styles.errorText} role="alert">{query.error.message}</p> : null}
      {overview.isError ? <p className={styles.errorText} role="alert">无法读取活动 WorkerSession：{overview.error.message}</p> : null}
      <h3>提交更正提案</h3>
      <ProposeCorrectionForm api={api} companyId={companyId} sessions={sessions} />
      <h3>待裁定提案</h3>
      {query.data?.items.length === 0 ? <p className={styles.formHint}>目前没有记忆更正提案。</p> : null}
      {query.data?.items.map(item => <div className={styles.boundaryItem} key={item.correctionId}>
        <div>
          <strong>{item.recordId} · {item.correctionId}</strong>
          <p>{labelDisplayValue(item.recordKind)} · {labelDisplayValue(item.recordScope)} · {labelDisplayValue(item.sensitivity)} · 基准版本 {item.baseRevision} · 当前版本 {item.currentRevision}</p>
          <p>提案人 {item.proposedBy} · {item.proposedAt} · WorkerSession {item.proposedBySessionId ?? '旧记录未绑定'} · Task {item.proposedByTaskId ?? '—'}</p>
          <p>证据 {item.source.kind} / {item.source.id} / 版本 {item.source.revision} · SHA-256 <code>{item.source.sha256}</code></p>
          {item.reviewActor ? <p>裁定 {labelDisplayValue(item.reviewDecision)} · {item.reviewActor} · WorkerSession {item.reviewSessionId ?? '旧记录未绑定'} · Task {item.reviewTaskId ?? '—'} · 序号 {item.reviewSequence}</p> : null}
          <p className={styles.formHint}>队列仅显示元数据，不返回记忆正文或自由文本理由。</p>
          {item.state === 'proposed' ? <ReviewCorrectionForm api={api} companyId={companyId} item={item} reviewers={reviewers} /> : null}
        </div>
        <StatusBadge label={labelDisplayValue(item.state)} tone={queueTone(item.state)} />
      </div>)}
      {query.data?.truncated ? <p className={styles.formHint}>当前显示最新 100 条；较早记录未包含在此快照中。</p> : null}
    </article>
  </section>;
}
