import type {ReactElement} from 'react';
import {useMemoryCorrectionQueue} from '../data/workbench-query';
import type {WorkbenchApi} from '../data/workbench-api';
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

export function MemoryCorrectionQueuePanel({api, companyId}: Props): ReactElement | null {
  const query = useMemoryCorrectionQueue(api, companyId);
  if (api.mode !== 'real') return null;
  return <section className={styles.sectionBlock} data-testid="memory-correction-queue">
    <div className={styles.sectionBlockHeader}>
      <div><p className={styles.sectionKicker}>安全记忆</p><h2 className={styles.sectionTitle}>更正提案队列</h2></div>
      <StatusBadge label={query.data ? `${query.data.items.length}${query.data.truncated ? '+' : ''} 条` : '读取中'} tone={query.data?.items.some(item => item.state === 'proposed') ? 'warning' : 'info'} />
    </div>
    <article className={styles.sectionCard}>
      {query.isPending ? <p className={styles.formHint}>正在读取公司范围内的更正记录…</p> : null}
      {query.isError ? <p className={styles.errorText} role="alert">{query.error.message}</p> : null}
      {query.data?.items.length === 0 ? <p className={styles.formHint}>目前没有记忆更正提案。</p> : null}
      {query.data?.items.map(item => <div className={styles.boundaryItem} key={item.correctionId}>
        <div>
          <strong>{item.recordId} · {item.correctionId}</strong>
          <p>{labelDisplayValue(item.recordKind)} · {labelDisplayValue(item.recordScope)} · {labelDisplayValue(item.sensitivity)} · 基准版本 {item.baseRevision} · 当前版本 {item.currentRevision}</p>
          <p>提案人 {item.proposedBy} · {item.proposedAt}</p>
          <p>证据 {item.source.kind} / {item.source.id} / 版本 {item.source.revision} · SHA-256 <code>{item.source.sha256}</code></p>
          {item.reviewActor ? <p>裁定 {labelDisplayValue(item.reviewDecision)} · {item.reviewActor} · 序号 {item.reviewSequence}</p> : null}
          <p className={styles.formHint}>此队列只显示元数据，不返回记忆正文或自由文本理由。</p>
        </div>
        <StatusBadge label={labelDisplayValue(item.state)} tone={queueTone(item.state)} />
      </div>)}
      {query.data?.truncated ? <p className={styles.formHint}>当前显示最新 100 条；较早记录未包含在此快照中。</p> : null}
      <p className={styles.formHint}>此页只读。更正提案和裁定必须由固定员工的授权会话写入；Workbench 当前没有可验证该员工身份的会话入口。</p>
    </article>
  </section>;
}
