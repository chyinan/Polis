// pattern: Imperative Shell

import {useRef, useState, type FormEvent} from 'react';
import {CircleAlert, Eye} from 'lucide-react';
import {CommandApiError, type WorkbenchApi} from '../data/workbench-api';
import {useMemoryTaskStatus, useRevalidateMemoryTask} from '../data/workbench-query';
import type {MemoryTaskImpactView, MemoryTaskRevalidationPreviewView} from '../domain/workbench';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {labelDisplayValue} from '../domain/display-labels';
import styles from '../styles/workbench.module.css';

type Props = Readonly<{api: WorkbenchApi; companyId: string; taskId: string}>;

function impactTone(state: string): 'warning' | 'danger' {
  return state === 'frozen' ? 'danger' : 'warning';
}

export function TaskMemoryImpactPanel({api, companyId, taskId}: Props) {
  const statusQuery = useMemoryTaskStatus(api, companyId, taskId);
  const revalidate = useRevalidateMemoryTask(api, companyId);
  const [selected, setSelected] = useState<Readonly<{dependencyId: string; correctionId: string}> | null>(null);
  const [preview, setPreview] = useState<MemoryTaskRevalidationPreviewView | null>(null);
  const [previewPending, setPreviewPending] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [reason, setReason] = useState('');
  const [reviewed, setReviewed] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [messageError, setMessageError] = useState(false);
  const pendingRequest = useRef<Readonly<{fingerprint: string; requestId: string}> | null>(null);

  if (api.mode !== 'real') return null;

  async function showPreview(impact: MemoryTaskImpactView) {
    setSelected({dependencyId: impact.dependencyId, correctionId: impact.correctionId});
    setPreview(null);
    setPreviewError(null);
    setPreviewPending(true);
    setReviewed(false);
    setReason('');
    setMessage(null);
    try {
      const next = await api.getMemoryTaskRevalidationPreview({companyId, taskId, dependencyId: impact.dependencyId, correctionId: impact.correctionId});
      setPreview(next);
    } catch (error) {
      setPreviewError(error instanceof Error ? error.message : '读取重验预览失败。');
    } finally {
      setPreviewPending(false);
    }
  }

  async function confirmRevalidation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (preview === null || selected === null || !reviewed) return;
    const trimmed = reason.trim();
    const reasonBytes = new TextEncoder().encode(trimmed).length;
    if (reasonBytes < 1 || reasonBytes > 2048) {
      setMessageError(true);
      setMessage('说明必须为 1–2048 字节。');
      return;
    }
    const fingerprint = JSON.stringify({taskId, dependencyId: selected.dependencyId, correctionId: selected.correctionId, contextSha256: preview.contextSha256, reason: trimmed});
    const pending = pendingRequest.current?.fingerprint === fingerprint
      ? pendingRequest.current
      : {fingerprint, requestId: crypto.randomUUID()};
    pendingRequest.current = pending;
    setMessage(null);
    setMessageError(false);
    try {
      const receipt = await revalidate.mutateAsync({taskId, dependencyId: selected.dependencyId, correctionId: selected.correctionId, contextSha256: preview.contextSha256, reason: trimmed, requestId: pending.requestId});
      pendingRequest.current = null;
      setSelected(null);
      setPreview(null);
      setReason('');
      setReviewed(false);
      setMessage(`已记录重验 ${receipt.id}。Task 已回到可执行状态；本操作不会启动 Worker。`);
    } catch (error) {
      setMessageError(true);
      setMessage(error instanceof CommandApiError ? error.message : error instanceof Error ? error.message : '记忆重验失败。');
    }
  }

  const status = statusQuery.data;
  const canConfirm = preview !== null && selected !== null && reviewed && !revalidate.isPending
    && (preview.taskState === 'ready' || preview.taskState === 'working');

  return <section className={styles.sectionBlock} data-testid="task-memory-impact">
    <div className={styles.sectionBlockHeader}><div><p className={styles.sectionKicker}>安全记忆</p><h2 className={styles.sectionTitle}>任务记忆影响</h2></div>
      {status ? <StatusBadge label={labelDisplayValue(status.state)} tone={status.state === 'clear' ? 'success' : impactTone(status.state)} /> : null}
    </div>
    <article className={styles.sectionCard}>
      {statusQuery.isPending ? <p className={styles.formHint}>正在读取任务记忆状态…</p> : null}
      {statusQuery.isError ? <p className={styles.errorText} role="alert">{statusQuery.error.message}</p> : null}
      {status?.state === 'clear' ? <p className={styles.formHint}>当前没有待处理的记忆影响。</p> : null}
      {status?.impacts.map(impact => <div className={styles.boundaryItem} key={`${impact.dependencyId}:${impact.correctionId}`}>
        <div><strong>{impact.recordId} · 版本 {impact.recordRevision} → {impact.replacementRevision}</strong>
          <p>依赖 {impact.dependencyId} · 更正 {impact.correctionId} · {labelDisplayValue(impact.riskLevel)} · {labelDisplayValue(impact.state)}</p>
          <p>{impact.reason}</p>
          <button className={styles.textButton} type="button" disabled={previewPending} onClick={() => void showPreview(impact)}><Eye aria-hidden="true" size={15} />查看完整重验预览</button>
        </div>
        <StatusBadge label={labelDisplayValue(impact.state)} tone={impactTone(impact.state)} />
      </div>)}
      {previewError !== null ? <p className={styles.errorText} role="alert"><CircleAlert aria-hidden="true" size={15} />{previewError}</p> : null}
      {previewPending ? <p className={styles.formHint} role="status">正在读取绑定的计划、工作区和更正内容…</p> : null}
      {preview !== null ? <form className={styles.formStack} onSubmit={event => void confirmRevalidation(event)}>
        <div className={styles.subsectionHeader}><span>精确上下文预览</span><code>{preview.taskId} · 第 {preview.taskGeneration} 代</code></div>
        <p className={styles.formHint}>Mission：{labelDisplayValue(preview.missionState)} · Task：{labelDisplayValue(preview.taskState)} · 系统已核实该 Task 下全部 WorkerSession 均已停止；原消费会话：{preview.stoppedSessionId}</p>
        <p className={styles.formHint}>任务计划 SHA-256：<code>{preview.taskPlanSha256}</code></p>
        <pre className={styles.codePreview} aria-label="已绑定的任务计划">{JSON.stringify(preview.taskPlan, null, 2)}</pre>
        <p className={styles.formHint}>工作区版本 {preview.workspaceRevision} · SHA-256：<code>{preview.workspaceDigest}</code></p>
        <pre className={styles.codePreview} aria-label="已绑定的任务工作区">{preview.workspaceContent}</pre>
        <div className={styles.boundaryItem}><div><strong>{preview.previousRecordId} · 版本 {preview.previousRevision} → {preview.replacementRevision}</strong>
          <p>风险：{labelDisplayValue(preview.riskLevel)} · 目标：{preview.targetKind} / {preview.targetId} / 版本 {preview.targetRevision}</p>
          <p>目标 SHA-256：<code>{preview.targetSha256}</code></p>
          <p>更正证据：{preview.correctionSource.kind} / {preview.correctionSource.id} / 版本 {preview.correctionSource.revision} · SHA-256 {preview.correctionSource.sha256}</p>
          <p>更正时间：{preview.correctionObservedAt}</p>
          <p>提交说明：{preview.correctionProposerReason}</p><p>独立审核说明：{preview.correctionReviewReason}</p>
          <p>更正后记忆内容 · SHA-256：<code>{preview.replacementContentSha256}</code></p>
          <pre className={styles.codePreview} aria-label="审核批准的更正内容">{preview.replacementContent}</pre>
        </div></div>
        {preview.otherImpacts.length > 0 ? <div className={styles.boundaryItem}><div><strong>该 Task 还有 {preview.otherImpacts.length} 项记忆影响</strong>{preview.otherImpacts.map(item => <p key={`${item.dependencyId}:${item.correctionId}`}>{item.recordId} · {item.state} · {item.correctionId}</p>)}</div></div> : null}
        <p className={styles.formHint}>确认只更新这条依赖并记录审计理由；工作区和计划都不会被修改。产品 Worker 的后继会话仍受现有准入策略限制，本操作不会自动启动 Worker。</p>
        <label className={styles.formLabel}>重验理由<textarea className={styles.formField} rows={3} maxLength={2048} value={reason} onChange={event => setReason(event.target.value)} required placeholder="说明你如何核对更正内容、任务计划与工作区" /></label>
        <label className={styles.reviewCheck}><input type="checkbox" checked={reviewed} onChange={event => setReviewed(event.target.checked)} /><span>我已核对以上更正内容、任务计划和完整工作区，并确认所有关联 WorkerSession 都已停止。</span></label>
        <button className={styles.commandButton} type="submit" disabled={!canConfirm}>{revalidate.isPending ? '正在记录…' : '确认记忆重验'}</button>
      </form> : null}
      {message !== null ? <p className={messageError ? styles.errorText : styles.formHint} role={messageError ? 'alert' : 'status'}>{message}</p> : null}
    </article>
  </section>;
}
