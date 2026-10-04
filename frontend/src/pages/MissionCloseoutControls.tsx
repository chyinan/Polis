// pattern: Imperative Shell

import {useRef, useState, type FormEvent} from 'react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCancelMission, useCloseMission} from '../data/workbench-query';
import type {MissionCloseoutSummary} from '../domain/workbench';
import styles from '../styles/workbench.module.css';

type Props = Readonly<{api: WorkbenchApi; companyId: string; missionId: string; missionState: string; closeout?: MissionCloseoutSummary | null}>;
type PendingIntent = Readonly<{outcome: 'succeeded' | 'ended_not_met'; rationale: string; acceptanceArtifactIds: ReadonlyArray<string>; requestId: string}>;

export function MissionCloseoutControls({api, companyId, missionId, missionState, closeout: closeoutSummary}: Props) {
  const close = useCloseMission(api, companyId);
  const cancel = useCancelMission(api, companyId);
  const [outcome, setOutcome] = useState<'succeeded' | 'ended_not_met'>('ended_not_met');
  const [rationale, setRationale] = useState('');
  const [artifactText, setArtifactText] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const pendingIntent = useRef<PendingIntent | null>(null);
  const closeout = closeoutSummary ?? null;
  const busy = close.isPending || cancel.isPending;

  if (api.mode !== 'real' || missionId === 'unavailable') return null;

  async function sendIntent(intent: PendingIntent): Promise<void> {
    setMessage(null);
    try {
      await close.mutateAsync({missionId, ...intent});
      pendingIntent.current = null;
      setMessage('收尾决定已受理，正在刷新权威状态。');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '使命收尾失败；可用相同 requestId 重试。');
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const normalizedRationale = rationale.trim();
    const acceptanceArtifactIds = artifactText.split(/[\r\n,，]+/).map(item => item.trim()).filter(Boolean);
    if (normalizedRationale === '') {
      setMessage('请填写本次结束决定的理由。');
      return;
    }
    if (outcome === 'succeeded' && acceptanceArtifactIds.length === 0) {
      setMessage('成功结束必须引用至少一个独立通过验收的 Artifact ID。');
      return;
    }
    const previous = pendingIntent.current;
    const sameIntent = previous !== null && previous.outcome === outcome && previous.rationale === normalizedRationale
      && previous.acceptanceArtifactIds.length === acceptanceArtifactIds.length
      && previous.acceptanceArtifactIds.every((id, index) => id === acceptanceArtifactIds[index]);
    const intent: PendingIntent = sameIntent && previous !== null
      ? previous
      : {outcome, rationale: normalizedRationale, acceptanceArtifactIds, requestId: crypto.randomUUID()};
    pendingIntent.current = intent;
    await sendIntent(intent);
  }

  async function resumeCloseout(): Promise<void> {
    if (!closeout || closeout.terminalOutcome !== null) return;
    setMessage(null);
    try {
      if (closeout.requestedOutcome === 'cancelled') {
        await cancel.mutateAsync({missionId, requestId: closeout.requestId});
        setMessage('收尾重试已受理，正在刷新权威状态。');
      } else {
        await sendIntent({
          outcome: closeout.requestedOutcome,
          rationale: closeout.rationale,
          acceptanceArtifactIds: closeout.acceptanceArtifactIds,
          requestId: closeout.requestId,
        });
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '收尾仍未完成；可以用原决定继续重试。');
    }
  }

  const canRequestCloseout = missionState === 'draft' || missionState === 'active' || missionState === 'paused';
  const isInterrupted = missionState === 'closing' && closeout != null && closeout.terminalOutcome === null;
  if (!canRequestCloseout && !isInterrupted) return null;

  return <section className={styles.sectionCard} data-testid="mission-closeout-controls" aria-label="使命收尾决定">
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>所有者决定</span><h2 className={styles.sectionTitle}>结束使命</h2></div></div>
    {isInterrupted ? <div className={styles.formStack}>
      <p className={styles.formHint}>使命正在收尾。继续时会按已记录的决定、证据和 requestId 重试，不会改写原意图。</p>
      <button className={styles.commandButton} data-testid="mission-closeout-resume" disabled={busy} onClick={() => void resumeCloseout()} type="button">{busy ? '处理中…' : '继续完成收尾'}</button>
    </div> : <form className={styles.formStack} onSubmit={event => void submit(event)}>
      <label className={styles.formLabel}>结束结果
        <select className={styles.formField} value={outcome} onChange={event => setOutcome(event.target.value as 'succeeded' | 'ended_not_met')}>
          <option value="ended_not_met">未达成而结束</option>
          <option value="succeeded">按验收证据成功</option>
        </select>
      </label>
      <label className={styles.formLabel}>所有者理由
        <textarea className={styles.formField} rows={3} maxLength={4096} required value={rationale} onChange={event => setRationale(event.target.value)} />
      </label>
      {outcome === 'succeeded' ? <label className={styles.formLabel}>验收 Artifact ID（每行或逗号分隔）
        <textarea className={styles.formField} rows={2} required value={artifactText} onChange={event => setArtifactText(event.target.value)} />
        <span className={styles.formHint}>Kernel 会核实它们属于本使命且状态为 ready、独立 verdict 为 passed，并再次检查所有任务和责任均已结清。</span>
      </label> : null}
      <button className={styles.commandButton} data-testid="mission-closeout-submit" disabled={busy} type="submit">{busy ? '处理中…' : '提交结束决定'}</button>
    </form>}
    {message ? <p className={styles.formHint} role="status">{message}</p> : null}
  </section>;
}
